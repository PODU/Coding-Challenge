// Command gitgraph draws a GitHub-style contributions heatmap from local git
// repositories.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gitgraph/internal/aggregate"
	"gitgraph/internal/cache"
	"gitgraph/internal/calendar"
	"gitgraph/internal/discovery"
	"gitgraph/internal/gitlog"
	"gitgraph/internal/model"
	"gitgraph/internal/render/svghtml"
	"gitgraph/internal/render/terminal"
)

const usage = `gitgraph - GitHub-style git contributions graph (offline)

Usage:
  gitgraph [flags] [PATH ...]

PATH    One or more repo paths or root directories to scan (default ".").

Range
  --days N         Analyse the previous N days (default 30).
  --since DATE     Explicit start date YYYY-MM-DD (overrides --days).
  --until DATE     Explicit end date YYYY-MM-DD (default today).
  --tz NAME        IANA timezone for day bucketing (default local).
  --first-day D    Week starts on sun|mon (default sun).

Authors
  --author STR     Filter to commits whose name OR email contains STR.
  --me             Filter to each repo's git user.email (default).
  --all-authors    Aggregate every author.
  --include-merges Count merge commits (default off).

Discovery
  --nested         Descend into nested repositories.
  --all-branches   Include commits from all branches.

Output
  --format FMT     terminal | svg | html (default terminal).
  --out FILE       Write svg/html to FILE (default stdout).
  --no-color       Disable ANSI colour.
  --color          Force ANSI colour even when piped.
  --stats-only     Print stats only, skip the grid.
  --leaderboard    Show contributor leaderboard (implies --all-authors).

Cache
  --no-cache       Ignore and do not write the cache.
  --refresh        Force a full recompute, then update the cache.

Misc
  --version, --help
`

const versionString = "gitgraph 1.0.0"

type config struct {
	paths        []string
	days         int
	since, until string
	tz           string
	firstDay     string
	author       string
	me           bool
	allAuthors   bool
	includeMerge bool
	nested       bool
	allBranches  bool
	format       string
	out          string
	noColor      bool
	forceColor   bool
	statsOnly    bool
	leaderboard  bool
	noCache      bool
	refresh      bool
}

func main() {
	os.Exit(run(os.Args[1:]))
}

// run runs gitgraph and returns the exit code.
func run(args []string) int {
	cfg, code, done := parseArgs(args)
	if done {
		return code
	}

	loc, err := resolveTZ(cfg.tz)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitgraph:", err)
		return 1
	}
	firstDay, err := resolveFirstDay(cfg.firstDay)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitgraph:", err)
		return 2
	}
	start, end, err := resolveWindow(cfg, loc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitgraph:", err)
		return 2
	}

	repos, err := discovery.Find(cfg.paths, cfg.nested)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitgraph: discovery:", err)
		return 1
	}
	if len(repos) == 0 {
		fmt.Fprintln(os.Stderr, "gitgraph: no git repositories found under", strings.Join(cfg.paths, ", "))
		return 1
	}

	reader := gitlog.Default()
	since := start.Time(loc)

	var (
		cachePath string
		store     *cache.Cache
	)
	if !cfg.noCache {
		if p, perr := cache.DefaultPath(); perr == nil {
			cachePath = p
			store = cache.Load(cachePath)
		}
	}

	readOpts := gitlog.ReadOptions{IncludeMerges: cfg.includeMerge, AllBranches: cfg.allBranches}

	var allCommits []model.Commit
	perRepo := make(map[string][]model.Commit)

	for _, repo := range repos {
		commits, rerr := readRepo(reader, store, repo.Path, since, readOpts, cfg.refresh)
		if rerr != nil {
			fmt.Fprintf(os.Stderr, "gitgraph: %s: %v\n", repo.Name, rerr)
			continue
		}
		if store != nil {
			newest := ""
			if len(commits) > 0 {
				newest = commits[0].SHA
			}
			store.Put(repo.Path, commits, newest, since)
		}
		perRepo[repo.Path] = commits
		allCommits = append(allCommits, commits...)
	}

	if store != nil && cachePath != "" {
		if serr := store.Save(cachePath); serr != nil {
			fmt.Fprintln(os.Stderr, "gitgraph: warning: could not write cache:", serr)
		}
	}

	allCommits = aggregate.Dedup(allCommits)

	authorFilter := cfg.author
	if cfg.leaderboard {
		cfg.allAuthors = true
	}
	if !cfg.allAuthors && authorFilter == "" {
		// --me is the default: use the first configured user.email we find.
		authorFilter = firstNonEmptyEmail(repos)
		if authorFilter == "" {
			fmt.Fprintln(os.Stderr, "gitgraph: warning: could not determine git user.email; showing all authors")
		}
	}

	filtered := aggregate.Filter(allCommits, authorFilter)
	counts := aggregate.Bucket(filtered, loc, start, end)
	stats := aggregate.ComputeStats(counts, start, end)
	cal := calendar.Build(counts, start, end, firstDay)

	title := buildTitle(cfg, repos, authorFilter, stats)

	if err := emit(cfg, cal, stats, title); err != nil {
		fmt.Fprintln(os.Stderr, "gitgraph:", err)
		return 1
	}

	if cfg.leaderboard {
		board := aggregate.Leaderboard(allCommits, loc, start, end)
		_ = terminal.RenderLeaderboard(os.Stdout, board)
	}

	return 0
}

// readRepo returns a repo's commits. With a usable cache entry only the
// commits since the cached head are read from git.
func readRepo(reader gitlog.Reader, store *cache.Cache, path string, since time.Time, opts gitlog.ReadOptions, refresh bool) ([]model.Commit, error) {
	if store != nil && !refresh {
		if cached, lastSHA, ok := store.Get(path, since); ok {
			if er, isExec := reader.(gitlog.ExecReader); isExec {
				fresh, err := er.ReadRange(path, lastSHA, opts)
				if err == nil {
					return cache.Merge(fresh, cached), nil
				}
				// On any error, fall back to a full read.
			}
		}
	}
	return reader.Read(path, since, opts)
}

func emit(cfg config, cal model.Calendar, stats model.Stats, title string) error {
	switch cfg.format {
	case "", "terminal":
		return terminal.Render(os.Stdout, cal, stats, terminal.Options{
			Color: decideColor(cfg), Title: title, StatsOnly: cfg.statsOnly,
		})
	case "svg":
		return writeOut(cfg.out, svghtml.SVG(cal))
	case "html":
		return writeOut(cfg.out, svghtml.HTML(cal, stats, title))
	default:
		return fmt.Errorf("unknown --format %q (want terminal|svg|html)", cfg.format)
	}
}

func writeOut(path, content string) error {
	if path == "" || path == "-" {
		_, err := os.Stdout.WriteString(content)
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func decideColor(cfg config) bool {
	if cfg.forceColor {
		return true
	}
	if cfg.noColor {
		return false
	}
	if fi, err := os.Stdout.Stat(); err == nil {
		return fi.Mode()&os.ModeCharDevice != 0
	}
	return false
}

func buildTitle(cfg config, repos []discovery.Repo, author string, stats model.Stats) string {
	scope := author
	if cfg.allAuthors || author == "" {
		scope = "all authors"
	}
	repoLabel := fmt.Sprintf("%d repos", len(repos))
	if len(repos) == 1 {
		repoLabel = repos[0].Name
	}
	return fmt.Sprintf("Contributions: %s | %s | %d days", scope, repoLabel, stats.Days)
}

func firstNonEmptyEmail(repos []discovery.Repo) string {
	for _, r := range repos {
		if e := gitlog.UserEmail(r.Path); e != "" {
			return e
		}
	}
	return ""
}

func resolveTZ(name string) (*time.Location, error) {
	if name == "" || strings.EqualFold(name, "local") {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("unknown timezone %q: %w", name, err)
	}
	return loc, nil
}

func resolveFirstDay(s string) (time.Weekday, error) {
	switch strings.ToLower(s) {
	case "", "sun", "sunday":
		return time.Sunday, nil
	case "mon", "monday":
		return time.Monday, nil
	default:
		return time.Sunday, fmt.Errorf("invalid --first-day %q (want sun|mon)", s)
	}
}

func resolveWindow(cfg config, loc *time.Location) (start, end model.Date, err error) {
	if cfg.until != "" {
		end, err = model.ParseDate(cfg.until)
		if err != nil {
			return
		}
	} else {
		end = model.DateIn(time.Now(), loc)
	}
	if cfg.since != "" {
		start, err = model.ParseDate(cfg.since)
		if err != nil {
			return
		}
	} else {
		if cfg.days < 1 {
			cfg.days = 1
		}
		start = end.AddDays(-(cfg.days - 1))
	}
	if start.After(end) {
		err = fmt.Errorf("start date %s is after end date %s", start, end)
	}
	return
}

// parseArgs accepts --flag, --flag=value and --flag value, plus paths. When
// done is true the caller should exit with code (after --help, --version or a
// usage error).
func parseArgs(args []string) (cfg config, code int, done bool) {
	cfg = config{days: 30, format: "terminal", firstDay: "sun"}
	stringFlags := map[string]*string{
		"since":     &cfg.since,
		"until":     &cfg.until,
		"tz":        &cfg.tz,
		"first-day": &cfg.firstDay,
		"author":    &cfg.author,
		"format":    &cfg.format,
		"out":       &cfg.out,
	}
	i := 0
	next := func(name string) (string, bool) {
		if i+1 >= len(args) {
			fmt.Fprintf(os.Stderr, "gitgraph: flag %s needs a value\n", name)
			return "", false
		}
		i++
		return args[i], true
	}
	for ; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") || a == "-" {
			cfg.paths = append(cfg.paths, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		val := ""
		hasVal := false
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			val, hasVal = name[eq+1:], true
			name = name[:eq]
		}
		getVal := func() (string, bool) {
			if hasVal {
				return val, true
			}
			return next("--" + name)
		}
		if dst, ok := stringFlags[name]; ok {
			v, ok := getVal()
			if !ok {
				return cfg, 2, true
			}
			*dst = v
			continue
		}
		switch name {
		case "help", "h":
			fmt.Print(usage)
			return cfg, 0, true
		case "version", "V":
			fmt.Println(versionString)
			return cfg, 0, true
		case "days":
			v, ok := getVal()
			if !ok {
				return cfg, 2, true
			}
			n := 0
			if _, e := fmt.Sscanf(v, "%d", &n); e != nil || n < 1 {
				fmt.Fprintf(os.Stderr, "gitgraph: invalid --days %q\n", v)
				return cfg, 2, true
			}
			cfg.days = n
		case "me":
			cfg.me = true
		case "all-authors":
			cfg.allAuthors = true
		case "include-merges":
			cfg.includeMerge = true
		case "nested":
			cfg.nested = true
		case "all-branches":
			cfg.allBranches = true
		case "no-color", "no-colour":
			cfg.noColor = true
		case "color", "colour":
			cfg.forceColor = true
		case "stats-only":
			cfg.statsOnly = true
		case "leaderboard":
			cfg.leaderboard = true
		case "no-cache":
			cfg.noCache = true
		case "refresh":
			cfg.refresh = true
		default:
			fmt.Fprintf(os.Stderr, "gitgraph: unknown flag --%s\n", name)
			return cfg, 2, true
		}
	}
	if len(cfg.paths) == 0 {
		cfg.paths = []string{"."}
	}
	for j, p := range cfg.paths {
		if abs, e := filepath.Abs(p); e == nil {
			cfg.paths[j] = abs
		}
	}
	sort.Strings(cfg.paths)
	return cfg, 0, false
}
