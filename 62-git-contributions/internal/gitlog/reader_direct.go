package gitlog

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gitgraph/internal/model"
)

// DirectReader is the fallback for when git isn't installed. It reads the HEAD
// reflog (.git/logs/HEAD), which lists every commit HEAD has pointed at. That
// misses history that was fetched rather than made locally, and other
// branches, but it is good enough for a simple linear repo.
type DirectReader struct{}

// gitDir returns the git directory for a working tree, following the
// "gitdir: <path>" file that worktrees and submodules use.
func gitDir(repoPath string) (string, error) {
	p := filepath.Join(repoPath, ".git")
	info, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return p, nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	if rest, ok := strings.CutPrefix(line, "gitdir:"); ok {
		gd := strings.TrimSpace(rest)
		if !filepath.IsAbs(gd) {
			gd = filepath.Join(repoPath, gd)
		}
		return gd, nil
	}
	return p, nil
}

// Read returns the commits in the HEAD reflog made on or after since. A commit
// can appear in the reflog several times; it is returned once.
func (DirectReader) Read(repoPath string, since time.Time, opts ReadOptions) ([]model.Commit, error) {
	gd, err := gitDir(repoPath)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(gd, "logs", "HEAD"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]struct{})
	var commits []model.Commit
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		c, ok := parseReflogLine(sc.Text())
		if !ok {
			continue
		}
		if _, dup := seen[c.SHA]; dup {
			continue
		}
		if !since.IsZero() && c.When.Before(since) {
			continue
		}
		seen[c.SHA] = struct{}{}
		commits = append(commits, c)
	}
	return commits, sc.Err()
}

// parseReflogLine parses one reflog entry:
//
//	<old> <new> Name <email> <unixtime> <tz>\t<message>
func parseReflogLine(line string) (model.Commit, bool) {
	tab := strings.IndexByte(line, '\t')
	if tab < 0 {
		return model.Commit{}, false
	}
	head := line[:tab]

	sp1 := strings.IndexByte(head, ' ')
	if sp1 < 0 {
		return model.Commit{}, false
	}
	rest := head[sp1+1:]
	sp2 := strings.IndexByte(rest, ' ')
	if sp2 < 0 {
		return model.Commit{}, false
	}
	newSHA := rest[:sp2]
	ident := rest[sp2+1:]

	gt := strings.LastIndexByte(ident, '>')
	if gt < 0 {
		return model.Commit{}, false
	}
	who := ident[:gt+1]
	timePart := strings.TrimSpace(ident[gt+1:])

	name, email := splitIdentity(who)

	tf := strings.Fields(timePart)
	if len(tf) < 1 {
		return model.Commit{}, false
	}
	unix, err := strconv.ParseInt(tf[0], 10, 64)
	if err != nil {
		return model.Commit{}, false
	}
	loc := time.UTC
	if len(tf) >= 2 {
		if l := parseTZOffset(tf[1]); l != nil {
			loc = l
		}
	}
	return model.Commit{
		SHA:         newSHA,
		AuthorName:  name,
		AuthorEmail: email,
		When:        time.Unix(unix, 0).In(loc),
	}, true
}

func splitIdentity(s string) (name, email string) {
	lt := strings.IndexByte(s, '<')
	gt := strings.LastIndexByte(s, '>')
	if lt < 0 || gt < 0 || gt < lt {
		return strings.TrimSpace(s), ""
	}
	return strings.TrimSpace(s[:lt]), s[lt+1 : gt]
}

// parseTZOffset turns an offset like "+0530" into a fixed zone, or nil.
func parseTZOffset(s string) *time.Location {
	if len(s) != 5 || (s[0] != '+' && s[0] != '-') {
		return nil
	}
	h, err1 := strconv.Atoi(s[1:3])
	m, err2 := strconv.Atoi(s[3:5])
	if err1 != nil || err2 != nil {
		return nil
	}
	off := (h*60 + m) * 60
	if s[0] == '-' {
		off = -off
	}
	return time.FixedZone(s, off)
}
