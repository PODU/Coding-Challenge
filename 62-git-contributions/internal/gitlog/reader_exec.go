package gitlog

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"gitgraph/internal/model"
)

// ASCII unit and record separators can't appear in names or emails, so they
// split the output safely.
const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
	logFormat = "--pretty=format:%H" + fieldSep + "%an" + fieldSep + "%ae" + fieldSep + "%aI" + recordSep
)

// ExecReader reads history by running git log.
type ExecReader struct{}

func (ExecReader) Read(repoPath string, since time.Time, opts ReadOptions) ([]model.Commit, error) {
	args := []string{"-C", repoPath, "log", logFormat}
	if !since.IsZero() {
		args = append(args, "--since="+since.Format(time.RFC3339))
	}
	if !opts.IncludeMerges {
		args = append(args, "--no-merges")
	}
	if opts.AllBranches {
		args = append(args, "--all")
	}

	cmd := exec.Command("git", args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// git log fails on a repository with no commits yet. That just means
		// there is nothing to count.
		msg := stderr.String()
		if strings.Contains(msg, "does not have any commits yet") ||
			strings.Contains(msg, "bad default revision") ||
			strings.Contains(msg, "ambiguous argument 'HEAD'") {
			return nil, nil
		}
		return nil, fmt.Errorf("git log in %s: %w: %s", repoPath, err, strings.TrimSpace(msg))
	}
	return ParseLog(out.Bytes())
}

// ReadRange reads the commits in lastSHA..HEAD. It fails if the repo doesn't
// know lastSHA (after a rebase, say), in which case callers do a full Read.
func (ExecReader) ReadRange(repoPath, lastSHA string, opts ReadOptions) ([]model.Commit, error) {
	if lastSHA == "" {
		return nil, fmt.Errorf("empty lastSHA")
	}
	args := []string{"-C", repoPath, "log", logFormat, lastSHA + "..HEAD"}
	if !opts.IncludeMerges {
		args = append(args, "--no-merges")
	}
	if opts.AllBranches {
		args = append(args, "--all")
	}
	cmd := exec.Command("git", args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git log range in %s: %w: %s", repoPath, err, strings.TrimSpace(stderr.String()))
	}
	return ParseLog(out.Bytes())
}

// ParseLog parses git log output in ExecReader's format.
func ParseLog(data []byte) ([]model.Commit, error) {
	var commits []model.Commit
	for _, rec := range bytes.Split(data, []byte(recordSep)) {
		rec = bytes.Trim(rec, "\r\n")
		if len(rec) == 0 {
			continue
		}
		fields := strings.Split(string(rec), fieldSep)
		if len(fields) != 4 {
			return nil, fmt.Errorf("malformed git log record: %q", string(rec))
		}
		when, err := time.Parse(time.RFC3339, fields[3])
		if err != nil {
			return nil, fmt.Errorf("parsing commit date %q: %w", fields[3], err)
		}
		commits = append(commits, model.Commit{
			SHA:         fields[0],
			AuthorName:  fields[1],
			AuthorEmail: fields[2],
			When:        when,
		})
	}
	return commits, nil
}
