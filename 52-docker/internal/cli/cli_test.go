package cli

import (
	"reflect"
	"testing"

	"ccrun/internal/container"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    container.Spec
		wantErr bool
	}{
		{
			name: "image and command",
			args: []string{"run", "ubuntu", "/bin/bash"},
			want: container.Spec{Image: "ubuntu", Command: "/bin/bash"},
		},
		{
			name: "image only uses config entrypoint",
			args: []string{"run", "alpine"},
			want: container.Spec{Image: "alpine"},
		},
		{
			name: "image command and args",
			args: []string{"run", "ubuntu", "/bin/echo", "hello", "world"},
			want: container.Spec{Image: "ubuntu", Command: "/bin/echo", Args: []string{"hello", "world"}},
		},
		{
			name: "rootfs mode positional is command",
			args: []string{"run", "--rootfs", "./alpine", "/bin/busybox", "sh"},
			want: container.Spec{Rootfs: "./alpine", Command: "/bin/busybox", Args: []string{"sh"}},
		},
		{
			name: "flags with equals form",
			args: []string{"run", "--memory=100m", "--cpus=1.5", "--hostname=box", "alpine", "sh"},
			want: container.Spec{Image: "alpine", Command: "sh", Memory: "100m", CPUs: "1.5", Hostname: "box"},
		},
		{
			name: "tag flag",
			args: []string{"run", "--tag", "22.04", "ubuntu"},
			want: container.Spec{Image: "ubuntu", Tag: "22.04"},
		},
		{
			name: "double dash separator ends ccrun flags",
			args: []string{"run", "--rootfs", "./root", "--", "/bin/sh", "-c", "echo hi"},
			want: container.Spec{Rootfs: "./root", Command: "/bin/sh", Args: []string{"-c", "echo hi"}},
		},
		{
			name: "first positional ends flag parsing",
			args: []string{"run", "alpine", "myprog", "--not-a-ccrun-flag"},
			want: container.Spec{Image: "alpine", Command: "myprog", Args: []string{"--not-a-ccrun-flag"}},
		},
		{name: "no args", args: nil, wantErr: true},
		{name: "wrong subcommand", args: []string{"start", "alpine"}, wantErr: true},
		{name: "run with nothing", args: []string{"run"}, wantErr: true},
		{name: "unknown flag", args: []string{"run", "--frob", "x", "alpine"}, wantErr: true},
		{name: "flag missing value", args: []string{"run", "--memory"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%v) = %+v, want error", tt.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%v) unexpected error: %v", tt.args, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse(%v)\n got = %+v\nwant = %+v", tt.args, got, tt.want)
			}
		})
	}
}
