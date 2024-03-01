package main

import "strings"

// Job is one command invocation.
type Job struct {
	Argv []string
}

// BuildConfig controls how items are grouped into jobs.
type BuildConfig struct {
	// Prefix is the command and its initial arguments.
	Prefix []string

	MaxArgs  int // -n: max items per invocation; 0 = unlimited
	MaxChars int // -s: max bytes per command line; 0 = unlimited

	Replace    bool   // -I
	ReplaceStr string // e.g. "{}"

	// RunIfEmpty runs the command once with no items when the input is empty.
	// -r turns it off.
	RunIfEmpty bool
}

// Build groups items into command invocations.
func Build(cfg BuildConfig, items []string) []Job {
	if cfg.Replace {
		return buildReplace(cfg, items)
	}
	return buildAppend(cfg, items)
}

// argvLen approximates a command line's size: each argument plus a separator
// (or terminating NUL).
func argvLen(argv []string) int {
	n := 0
	for _, a := range argv {
		n += len(a) + 1
	}
	return n
}

func buildAppend(cfg BuildConfig, items []string) []Job {
	if len(items) == 0 {
		if cfg.RunIfEmpty {
			return []Job{{Argv: append([]string{}, cfg.Prefix...)}}
		}
		return nil
	}

	base := argvLen(cfg.Prefix)
	var jobs []Job
	var batch []string
	curLen := base

	flush := func() {
		if len(batch) == 0 {
			return
		}
		argv := make([]string, 0, len(cfg.Prefix)+len(batch))
		argv = append(argv, cfg.Prefix...)
		argv = append(argv, batch...)
		jobs = append(jobs, Job{Argv: argv})
		batch = nil
		curLen = base
	}

	for _, it := range items {
		add := len(it) + 1
		if cfg.MaxChars > 0 && len(batch) > 0 && curLen+add > cfg.MaxChars {
			flush()
		}
		batch = append(batch, it)
		curLen += add

		if cfg.MaxArgs > 0 && len(batch) >= cfg.MaxArgs {
			flush()
			continue
		}
		// An item too big for the limit on its own can't be split, so run it by
		// itself.
		if cfg.MaxChars > 0 && len(batch) == 1 && curLen > cfg.MaxChars {
			flush()
		}
	}
	flush()
	return jobs
}

func buildReplace(cfg BuildConfig, items []string) []Job {
	// One job per item, so no items means no jobs, regardless of -r.
	if len(items) == 0 || len(cfg.Prefix) == 0 {
		return nil
	}
	jobs := make([]Job, 0, len(items))
	for _, it := range items {
		argv := make([]string, len(cfg.Prefix))
		// Only the arguments are substituted, never the command name.
		argv[0] = cfg.Prefix[0]
		for i := 1; i < len(cfg.Prefix); i++ {
			argv[i] = strings.ReplaceAll(cfg.Prefix[i], cfg.ReplaceStr, it)
		}
		jobs = append(jobs, Job{Argv: argv})
	}
	return jobs
}
