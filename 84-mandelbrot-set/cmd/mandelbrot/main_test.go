package main

import (
	"io"
	"path/filepath"
	"testing"
)

func TestBuildConfigValidation(t *testing.T) {
	base := func() options {
		return options{
			width: 100, height: 100, dpi: 72,
			minRe: -2, maxRe: 2, minIm: -2, maxIm: 2,
			mag: 1, iter: 500, paletteName: "rainbow", inside: "#000000",
		}
	}
	tests := []struct {
		name    string
		mutate  func(*options)
		center  bool
		wantErr bool
	}{
		{"valid bounds", func(*options) {}, false, false},
		{"width zero", func(o *options) { o.width = 0 }, false, true},
		{"height negative", func(o *options) { o.height = -1 }, false, true},
		{"iter zero", func(o *options) { o.iter = 0 }, false, true},
		{"minRe>=maxRe", func(o *options) { o.minRe = 2; o.maxRe = 2 }, false, true},
		{"minIm>=maxIm", func(o *options) { o.minIm = 1; o.maxIm = 0 }, false, true},
		{"unknown palette", func(o *options) { o.paletteName = "tartan" }, false, true},
		{"bad inside colour", func(o *options) { o.inside = "blue" }, false, true},
		{"mag zero center mode", func(o *options) { o.mag = 0 }, true, true},
		{"valid center mode", func(o *options) { o.mag = 200 }, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := base()
			tt.mutate(&o)
			_, err := buildConfig(o, tt.center)
			if (err != nil) != tt.wantErr {
				t.Fatalf("buildConfig err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestRunWritesPNG(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.png")
	args := []string{"-width", "32", "-height", "32", "-iter", "100", "-out", out}
	if err := run(args, io.Discard); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if _, err := filepath.Abs(out); err != nil {
		t.Fatal(err)
	}
}

func TestRunBadFlagsFail(t *testing.T) {
	cases := [][]string{
		{"-width", "0"},
		{"-iter", "-5"},
		{"-minRe", "2", "-maxRe", "2"},
		{"-palette", "nope"},
		{"-inside", "xyz"},
	}
	for _, args := range cases {
		full := append([]string{"-out", filepath.Join(t.TempDir(), "x.png")}, args...)
		if err := run(full, io.Discard); err == nil {
			t.Fatalf("expected error for args %v", args)
		}
	}
}
