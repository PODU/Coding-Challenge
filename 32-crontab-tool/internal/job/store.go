package job

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"crontab/internal/cron"
)

// Load reads a crontab file. Comments are skipped, except that "# name: X"
// names the job on the next line. A missing file means no jobs.
func Load(path string) ([]Job, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var jobs []Job
	var pendingName string
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		if strings.HasPrefix(text, "#") {
			if name, ok := parseNameDirective(text); ok {
				pendingName = name
			}
			continue
		}
		j, err := ParseLine(text)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		j.Name = pendingName
		pendingName = ""
		jobs = append(jobs, j)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

// ParseLine parses one "<expr> <command>" line.
func ParseLine(line string) (Job, error) {
	expr, command, err := splitExpr(line)
	if err != nil {
		return Job{}, err
	}
	sch, err := cron.Parse(expr)
	if err != nil {
		return Job{}, err
	}
	return Job{Schedule: sch, Expr: strings.TrimSpace(expr), Command: command}, nil
}

// Add appends a job to the crontab file, creating the file if needed. The file
// is left alone if the expression is invalid.
func Add(path, expr, command string) (Job, error) {
	sch, err := cron.Parse(expr)
	if err != nil {
		return Job{}, err
	}
	expr = strings.TrimSpace(expr)
	command = strings.TrimSpace(command)
	if command == "" {
		return Job{}, fmt.Errorf("command must not be empty")
	}

	if err := appendLine(path, expr+"\t"+command); err != nil {
		return Job{}, err
	}
	return Job{Schedule: sch, Expr: expr, Command: command}, nil
}

// appendLine adds line to the end of path, inserting a newline first if the
// file doesn't end with one.
func appendLine(path, line string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var b strings.Builder
	b.Write(existing)
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		b.WriteByte('\n')
	}
	b.WriteString(line)
	b.WriteByte('\n')
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// splitExpr splits a job line into the expression (five fields or an @macro)
// and the command, which is kept as written.
func splitExpr(line string) (expr, command string, err error) {
	trimmed := strings.TrimLeft(line, " \t")
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", "", fmt.Errorf("empty job line")
	}
	if strings.HasPrefix(fields[0], "@") {
		if len(fields) < 2 {
			return "", "", fmt.Errorf("macro job line %q is missing a command", line)
		}
		return fields[0], commandRemainder(trimmed, 1), nil
	}
	if len(fields) < 6 {
		return "", "", fmt.Errorf("expected 5 cron fields followed by a command, got %q", line)
	}
	return strings.Join(fields[:5], " "), commandRemainder(trimmed, 5), nil
}

// commandRemainder returns what follows the first n whitespace-separated
// fields of s, trimmed.
func commandRemainder(s string, n int) string {
	i := 0
	for ; n > 0; n-- {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		for i < len(s) && s[i] != ' ' && s[i] != '\t' {
			i++
		}
	}
	return strings.TrimSpace(s[i:])
}

func parseNameDirective(comment string) (string, bool) {
	body := strings.TrimSpace(strings.TrimPrefix(comment, "#"))
	const prefix = "name:"
	if strings.HasPrefix(strings.ToLower(body), prefix) {
		return strings.TrimSpace(body[len(prefix):]), true
	}
	return "", false
}
