// Package engine runs a worker pool that hashes candidates and checks them
// against the target digests, stopping once every target is matched.
package engine

import (
	"context"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"passcrack/hashreg"
	"passcrack/saltutil"
)

type Result struct {
	Hash      string // lowercase hex digest that was cracked
	Plaintext string // recovered password
}

type Config struct {
	Algo    hashreg.Algo
	Salter  saltutil.Salter
	Targets []string // hex digests; case and whitespace are normalized
	Workers int      // <= 0 means runtime.NumCPU()
}

// Cracker runs candidate passwords against a fixed target set.
type Cracker struct {
	algo      hashreg.Algo
	salter    saltutil.Salter
	workers   int
	targets   map[string]string // hex digest -> "" (read-only during cracking)
	totalReq  int               // distinct targets requested
	remaining int64
}

// New builds a Cracker, normalizing each target to trimmed lowercase hex and
// checking its length against the algorithm.
func New(cfg Config) (*Cracker, error) {
	if cfg.Algo == nil {
		return nil, fmt.Errorf("engine: algorithm is required")
	}
	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("engine: at least one target hash is required")
	}
	workers := cfg.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	targets := make(map[string]string, len(cfg.Targets))
	for _, t := range cfg.Targets {
		norm := strings.ToLower(strings.TrimSpace(t))
		if norm == "" {
			continue
		}
		if len(norm) != cfg.Algo.HexLen() {
			return nil, fmt.Errorf("engine: target %q has length %d, expected %d for %s",
				t, len(norm), cfg.Algo.HexLen(), cfg.Algo.Name())
		}
		if _, err := hex.DecodeString(norm); err != nil {
			return nil, fmt.Errorf("engine: target %q is not valid hex", t)
		}
		targets[norm] = ""
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("engine: no valid target hashes after normalization")
	}
	return &Cracker{
		algo:      cfg.Algo,
		salter:    cfg.Salter,
		workers:   workers,
		targets:   targets,
		totalReq:  len(targets),
		remaining: int64(len(targets)),
	}, nil
}

// Workers reports the configured worker count.
func (c *Cracker) Workers() int { return c.workers }

// TargetCount reports the number of distinct targets.
func (c *Cracker) TargetCount() int { return c.totalReq }

// Run consumes candidates and returns the map of cracked digests to plaintexts
// plus the sorted list of digests left uncracked. It cancels ctx-derived work
// (and thus the producer feeding candidates) as soon as every target is found.
// The onMatch callback, if non-nil, is invoked for each match as it happens
// (useful for streaming output); it must be safe for concurrent use.
func (c *Cracker) Run(ctx context.Context, candidates <-chan string, onMatch func(Result)) (map[string]string, []string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan Result, c.workers)
	var wg sync.WaitGroup
	wg.Add(c.workers)
	for i := 0; i < c.workers; i++ {
		go func() {
			defer wg.Done()
			c.worker(ctx, candidates, results)
		}()
	}
	// Close results once every worker has finished.
	go func() {
		wg.Wait()
		close(results)
	}()

	cracked := make(map[string]string)
	for r := range results {
		if _, ok := c.targets[r.Hash]; !ok {
			continue
		}
		if _, dup := cracked[r.Hash]; dup {
			continue
		}
		cracked[r.Hash] = r.Plaintext
		atomic.AddInt64(&c.remaining, -1)
		if onMatch != nil {
			onMatch(r)
		}
		if len(cracked) == c.totalReq {
			cancel() // all cracked; stop the producer and workers
		}
	}

	uncracked := make([]string, 0, c.totalReq-len(cracked))
	for h := range c.targets {
		if _, ok := cracked[h]; !ok {
			uncracked = append(uncracked, h)
		}
	}
	sortStrings(uncracked)
	return cracked, uncracked
}

func (c *Cracker) worker(ctx context.Context, candidates <-chan string, results chan<- Result) {
	for {
		select {
		case <-ctx.Done():
			return
		case cand, ok := <-candidates:
			if !ok {
				return
			}
			digest := hex.EncodeToString(c.algo.Sum(c.salter.Apply(cand)))
			if _, hit := c.targets[digest]; hit {
				select {
				case results <- Result{Hash: digest, Plaintext: cand}:
				case <-ctx.Done():
					return
				}
			}
			// Best-effort fast exit even without a match on this worker.
			if atomic.LoadInt64(&c.remaining) <= 0 {
				return
			}
		}
	}
}

// Insertion sort; the slice is usually empty or tiny.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
