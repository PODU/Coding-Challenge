package report

import (
	"testing"

	"du/internal/walk"
)

func TestHuman(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1023, "1023"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1048576, "1.0M"},
		{1073741824, "1.0G"},
		{1099511627776, "1.0T"},
	}
	for _, c := range cases {
		if got := Human(c.in); got != c.want {
			t.Errorf("Human(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatSizeUnits(t *testing.T) {
	cases := []struct {
		name string
		opts walk.Options
		in   int64
		want string
	}{
		{"blocks-512", walk.Options{Unit: walk.UnitBlock}, 1024, "2"},
		{"blocks-512-round-up", walk.Options{Unit: walk.UnitBlock}, 1000, "2"},
		{"kib", walk.Options{Unit: walk.UnitK}, 2048, "2"},
		{"kib-round-up", walk.Options{Unit: walk.UnitK}, 2049, "3"},
		{"mib", walk.Options{Unit: walk.UnitM}, 1048576, "1"},
		{"custom-1", walk.Options{Unit: walk.UnitCustom, BlockSize: 1}, 6000, "6000"},
		{"custom-1k", walk.Options{Unit: walk.UnitCustom, BlockSize: 1024}, 4096, "4"},
		{"human", walk.Options{Unit: walk.UnitHuman}, 1536, "1.5K"},
		{"inodes", walk.Options{Inodes: true}, 7, "7"},
		{"zero-blocks", walk.Options{Unit: walk.UnitBlock}, 0, "0"},
	}
	for _, c := range cases {
		if got := FormatSize(c.in, c.opts); got != c.want {
			t.Errorf("%s: FormatSize(%d) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}
