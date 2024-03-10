// Package container describes what to run and, on Linux, runs it.
package container

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"ccrun/internal/registry"
)

// DefaultCPUPeriod is the cpu.max period in microseconds.
const DefaultCPUPeriod int64 = 100000

// ParseMemory parses a size like "100m", "1g", "512k" or a byte count.
func ParseMemory(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty memory value")
	}
	mult := int64(1)
	switch last := s[len(s)-1]; last {
	case 'k', 'K':
		mult, s = 1<<10, s[:len(s)-1]
	case 'm', 'M':
		mult, s = 1<<20, s[:len(s)-1]
	case 'g', 'G':
		mult, s = 1<<30, s[:len(s)-1]
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid memory value %q", s)
	}
	return n * mult, nil
}

// ParseCPUQuota turns a --cpus value such as "1.5" into a cpu.max quota.
func ParseCPUQuota(s string, period int64) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty cpus value")
	}
	cpus, err := strconv.ParseFloat(s, 64)
	if err != nil || cpus <= 0 {
		return 0, fmt.Errorf("invalid cpus value %q", s)
	}
	return int64(cpus * float64(period)), nil
}

// ErrUnsupported is what Run returns outside Linux.
var ErrUnsupported = errors.New("container execution is only supported on Linux")

// DefaultHostname is the container hostname when none is given.
const DefaultHostname = "ccrun"

// ReexecArg as the first argument tells ccrun it is the child process that Run
// started inside the new namespaces.
const ReexecArg = "__ccrun_child__"

// Spec is a parsed `run` command line.
type Spec struct {
	Image string // e.g. "ubuntu:22.04"; empty with --rootfs
	// Rootfs is an already unpacked filesystem to use instead of pulling.
	Rootfs string
	// Command defaults to the image's Entrypoint and Cmd.
	Command  string
	Args     []string
	Memory   string // e.g. "100m"; empty means no limit
	CPUs     string // e.g. "1.5"; empty means no limit
	Hostname string
	Tag      string // overrides the tag in Image
}

// Resolved is the process to start, after applying Spec to the image config.
type Resolved struct {
	Argv       []string
	Env        []string
	WorkingDir string
	Hostname   string
}

// Resolve works out the process to run the way Docker does: a command given
// on the command line replaces the image's Entrypoint and Cmd; env and working
// directory come from the image.
func Resolve(spec Spec, cfg registry.ImageConfig) Resolved {
	r := Resolved{
		WorkingDir: cfg.Config.WorkingDir,
		Hostname:   spec.Hostname,
	}
	if r.WorkingDir == "" {
		r.WorkingDir = "/"
	}
	if r.Hostname == "" {
		r.Hostname = DefaultHostname
	}

	// Copy so the caller can't modify cfg through r.
	if len(cfg.Config.Env) > 0 {
		r.Env = append(r.Env, cfg.Config.Env...)
	}

	if spec.Command != "" {
		r.Argv = append([]string{spec.Command}, spec.Args...)
	} else {
		r.Argv = append(r.Argv, cfg.Config.Entrypoint...)
		r.Argv = append(r.Argv, cfg.Config.Cmd...)
	}
	return r
}
