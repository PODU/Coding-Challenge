package main

import (
	"encoding/hex"
	"hash"
	"io"
	"os"
	"runtime"
	"sort"
	"sync"
)

const defaultPartialSize = 4096

// FindDuplicates scans roots for duplicate files. Errors reading individual
// files go into Report.Errors; the returned error is only for bad options.
func FindDuplicates(roots []string, opts Options) (*Report, *Stats, error) {
	if opts.PartialSize == 0 {
		opts.PartialSize = defaultPartialSize
	}
	if opts.Workers <= 0 {
		opts.Workers = runtime.GOMAXPROCS(0)
	}
	newHash, err := newHasher(opts.Algo)
	if err != nil {
		return nil, nil, err
	}

	files, scanErrs := walkRoots(roots, opts)
	stats := &Stats{Walked: len(files)}

	sizeGroups := bucketBySize(files, opts.MinSize, stats)
	partialGroups := bucketByPartialHash(sizeGroups, newHash, opts.PartialSize, &scanErrs, stats)
	fullGroups := bucketByFullHash(partialGroups, newHash, opts.Workers, &scanErrs, stats)

	var groups []DuplicateGroup
	for _, fg := range fullGroups {
		for _, confirmed := range verifyGroup(fg.files, &scanErrs) {
			if len(confirmed) < 2 {
				continue
			}
			stats.Confirmed += len(confirmed)
			groups = append(groups, makeGroup(confirmed, fg.hash))
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Size != groups[j].Size {
			return groups[i].Size > groups[j].Size // largest first
		}
		return groups[i].Paths[0] < groups[j].Paths[0]
	})

	rep := &Report{Groups: groups}
	for _, se := range scanErrs {
		rep.Errors = append(rep.Errors, se.Error())
	}
	sort.Strings(rep.Errors)
	rep.Summary.Groups = len(groups)
	for _, g := range groups {
		rep.Summary.Files += len(g.Paths)
		rep.Summary.WastedBytes += g.Wasted
	}
	return rep, stats, nil
}

// bucketBySize groups files by size, dropping files below minSize and sizes
// that only one file has.
func bucketBySize(files []FileInfo, minSize int64, stats *Stats) [][]FileInfo {
	bySize := make(map[int64][]FileInfo)
	for _, f := range files {
		if f.Size < minSize {
			continue
		}
		bySize[f.Size] = append(bySize[f.Size], f)
	}
	var out [][]FileInfo
	for _, g := range bySize {
		if len(g) > 1 {
			stats.SizeCandidates += len(g)
			out = append(out, g)
		}
	}
	return out
}

// bucketByPartialHash splits each group by a hash of the first partialSize
// bytes, dropping files left on their own.
func bucketByPartialHash(groups [][]FileInfo, newHash func() hash.Hash, partialSize int64, errs *[]ScanError, stats *Stats) [][]FileInfo {
	var out [][]FileInfo
	for _, g := range groups {
		sub := make(map[string][]FileInfo)
		for _, f := range g {
			h, err := hashFile(f.Path, newHash(), partialSize)
			if err != nil {
				*errs = append(*errs, ScanError{f.Path, err})
				continue
			}
			sub[h] = append(sub[h], f)
		}
		for _, s := range sub {
			if len(s) > 1 {
				stats.PartialCandidates += len(s)
				out = append(out, s)
			}
		}
	}
	return out
}

type fullGroup struct {
	hash  string
	files []FileInfo
}

// bucketByFullHash splits each group by a hash of the whole file, using a pool of
// workers, and drops files left on their own.
func bucketByFullHash(groups [][]FileInfo, newHash func() hash.Hash, workers int, errs *[]ScanError, stats *Stats) []fullGroup {
	type task struct {
		gi int
		f  FileInfo
	}
	type result struct {
		gi   int
		f    FileInfo
		hash string
		err  error
	}

	tasks := make(chan task)
	results := make(chan result)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				h, err := hashFile(t.f.Path, newHash(), 0)
				results <- result{t.gi, t.f, h, err}
			}
		}()
	}
	go func() {
		for gi, g := range groups {
			for _, f := range g {
				tasks <- task{gi, f}
			}
		}
		close(tasks)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	subs := make([]map[string][]FileInfo, len(groups))
	for i := range subs {
		subs[i] = make(map[string][]FileInfo)
	}
	for r := range results {
		if r.err != nil {
			*errs = append(*errs, ScanError{r.f.Path, r.err})
			continue
		}
		subs[r.gi][r.hash] = append(subs[r.gi][r.hash], r.f)
	}

	var out []fullGroup
	for _, sub := range subs {
		for h, g := range sub {
			if len(g) > 1 {
				stats.FullCandidates += len(g)
				out = append(out, fullGroup{hash: h, files: g})
			}
		}
	}
	return out
}

// hashFile hashes the first limit bytes of a file, or all of it if limit is 0.
func hashFile(path string, h hash.Hash, limit int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var r io.Reader = f
	if limit > 0 {
		r = io.LimitReader(f, limit)
	}
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func makeGroup(members []FileInfo, hash string) DuplicateGroup {
	sort.Slice(members, func(i, j int) bool { return members[i].Path < members[j].Path })
	paths := make([]string, len(members))
	for i, m := range members {
		paths[i] = m.Path
	}
	size := members[0].Size
	return DuplicateGroup{
		Size:    size,
		Hash:    hash,
		Paths:   paths,
		Wasted:  size * int64(len(members)-1),
		members: members,
	}
}
