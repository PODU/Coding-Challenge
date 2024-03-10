package container

import (
	"reflect"
	"runtime"
	"testing"

	"ccrun/internal/registry"
)

func cfgWith(env, entrypoint, cmd []string, workdir string) registry.ImageConfig {
	var c registry.ImageConfig
	c.Config.Env = env
	c.Config.Entrypoint = entrypoint
	c.Config.Cmd = cmd
	c.Config.WorkingDir = workdir
	return c
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name string
		spec Spec
		cfg  registry.ImageConfig
		want Resolved
	}{
		{
			name: "cli command overrides image entrypoint and cmd",
			spec: Spec{Command: "/bin/ls", Args: []string{"-l"}},
			cfg:  cfgWith([]string{"PATH=/bin"}, []string{"/entry"}, []string{"/bin/sh"}, "/work"),
			want: Resolved{
				Argv:       []string{"/bin/ls", "-l"},
				Env:        []string{"PATH=/bin"},
				WorkingDir: "/work",
				Hostname:   DefaultHostname,
			},
		},
		{
			name: "no cli command uses entrypoint plus cmd",
			spec: Spec{},
			cfg:  cfgWith(nil, []string{"/entry"}, []string{"arg1"}, ""),
			want: Resolved{
				Argv:       []string{"/entry", "arg1"},
				WorkingDir: "/",
				Hostname:   DefaultHostname,
			},
		},
		{
			name: "hostname override honoured",
			spec: Spec{Command: "sh", Hostname: "box"},
			cfg:  cfgWith(nil, nil, nil, ""),
			want: Resolved{
				Argv:       []string{"sh"},
				WorkingDir: "/",
				Hostname:   "box",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Resolve(tt.spec, tt.cfg)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Resolve()\n got = %+v\nwant = %+v", got, tt.want)
			}
		})
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"100m", 100 << 20, false},
		{"1g", 1 << 30, false},
		{"512k", 512 << 10, false},
		{"1048576", 1048576, false},
		{"100M", 100 << 20, false},
		{"", 0, true},
		{"abc", 0, true},
		{"-5m", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseMemory(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseMemory(%q) = %d, want error", tt.in, got)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("ParseMemory(%q) = %d, %v; want %d", tt.in, got, err, tt.want)
		}
	}
}

func TestParseCPUQuota(t *testing.T) {
	if q, err := ParseCPUQuota("1.5", DefaultCPUPeriod); err != nil || q != 150000 {
		t.Errorf("ParseCPUQuota(1.5) = %d, %v; want 150000", q, err)
	}
	if _, err := ParseCPUQuota("0", DefaultCPUPeriod); err == nil {
		t.Errorf("ParseCPUQuota(0) want error")
	}
	if _, err := ParseCPUQuota("x", DefaultCPUPeriod); err == nil {
		t.Errorf("ParseCPUQuota(x) want error")
	}
}

func TestRunUnsupportedOnNonLinux(t *testing.T) {
	// On the Windows build host Run must return ErrUnsupported, never panic.
	// (On Linux this test is skipped because Run would attempt a real clone.)
	if runtime.GOOS == "linux" {
		t.Skip("Run performs real isolation on Linux")
	}
	code, err := Run(Spec{Command: "/bin/true"}, ".", registry.ImageConfig{})
	if err != ErrUnsupported {
		t.Fatalf("Run err = %v, want ErrUnsupported", err)
	}
	if code == 0 {
		t.Errorf("Run code = 0, want non-zero on unsupported host")
	}
}
