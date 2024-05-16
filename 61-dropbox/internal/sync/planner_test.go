package sync

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

// fixed clock so conflict names are deterministic.
var fixedNow = time.Date(2026, 6, 26, 13, 5, 9, 0, time.UTC)

const conflictTS = "2026-06-26 13-05-09"

func e(hash string) Entry { return Entry{Hash: hash} }
func r(hash string, v int64) Entry {
	return Entry{Hash: hash, Version: v}
}

func TestPlanTable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		local  Snapshot
		remote Snapshot
		base   Snapshot
		want   []Action
	}{
		{
			name:   "unchanged everywhere -> no-op",
			local:  Snapshot{"a": e("h1")},
			remote: Snapshot{"a": r("h1", 1)},
			base:   Snapshot{"a": e("h1")},
			want:   nil,
		},
		{
			name:   "new local only -> upload",
			local:  Snapshot{"a": e("h1")},
			remote: Snapshot{},
			base:   Snapshot{},
			want:   []Action{{Kind: Upload, Path: "a", RemoteVersion: 0}},
		},
		{
			name:   "new remote only -> download",
			local:  Snapshot{},
			remote: Snapshot{"a": r("h1", 1)},
			base:   Snapshot{},
			want:   []Action{{Kind: Download, Path: "a", RemoteVersion: 1}},
		},
		{
			name:   "local modified -> upload",
			local:  Snapshot{"a": e("h2")},
			remote: Snapshot{"a": r("h1", 3)},
			base:   Snapshot{"a": e("h1")},
			want:   []Action{{Kind: Upload, Path: "a", RemoteVersion: 3}},
		},
		{
			name:   "remote modified -> download",
			local:  Snapshot{"a": e("h1")},
			remote: Snapshot{"a": r("h2", 4)},
			base:   Snapshot{"a": e("h1")},
			want:   []Action{{Kind: Download, Path: "a", RemoteVersion: 4}},
		},
		{
			name:   "local deleted, remote unchanged -> delete remote",
			local:  Snapshot{},
			remote: Snapshot{"a": r("h1", 2)},
			base:   Snapshot{"a": e("h1")},
			want:   []Action{{Kind: DeleteRemote, Path: "a", RemoteVersion: 2}},
		},
		{
			name:   "remote deleted, local unchanged -> delete local",
			local:  Snapshot{"a": e("h1")},
			remote: Snapshot{},
			base:   Snapshot{"a": e("h1")},
			want:   []Action{{Kind: DeleteLocal, Path: "a"}},
		},
		{
			name:   "both changed, same hash -> no-op (converged)",
			local:  Snapshot{"a": e("h2")},
			remote: Snapshot{"a": r("h2", 5)},
			base:   Snapshot{"a": e("h1")},
			want:   nil,
		},
		{
			name:   "both changed, different hash -> conflict",
			local:  Snapshot{"a.txt": e("hL")},
			remote: Snapshot{"a.txt": r("hR", 7)},
			base:   Snapshot{"a.txt": e("h0")},
			want: []Action{{
				Kind: Conflict, Path: "a.txt", RemoteVersion: 7,
				ConflictPath: "a (conflicted copy " + conflictTS + ").txt",
			}},
		},
		{
			name:   "both new, different hash -> conflict (no base)",
			local:  Snapshot{"a.txt": e("hL")},
			remote: Snapshot{"a.txt": r("hR", 1)},
			base:   Snapshot{},
			want: []Action{{
				Kind: Conflict, Path: "a.txt", RemoteVersion: 1,
				ConflictPath: "a (conflicted copy " + conflictTS + ").txt",
			}},
		},
		{
			name:   "both new, same hash -> no-op",
			local:  Snapshot{"a": e("h1")},
			remote: Snapshot{"a": r("h1", 1)},
			base:   Snapshot{},
			want:   nil,
		},
		{
			name:   "local deleted, remote changed -> download (restore)",
			local:  Snapshot{},
			remote: Snapshot{"a": r("h2", 9)},
			base:   Snapshot{"a": e("h1")},
			want:   []Action{{Kind: Download, Path: "a", RemoteVersion: 9}},
		},
		{
			name:   "remote deleted, local changed -> upload (resurrect)",
			local:  Snapshot{"a": e("h2")},
			remote: Snapshot{},
			base:   Snapshot{"a": e("h1")},
			want:   []Action{{Kind: Upload, Path: "a", RemoteVersion: 0}},
		},
		{
			name:   "both deleted -> no-op",
			local:  Snapshot{},
			remote: Snapshot{},
			base:   Snapshot{"a": e("h1")},
			want:   nil,
		},
		{
			name:   "conflict name in nested dir",
			local:  Snapshot{"d/sub/f.md": e("hL")},
			remote: Snapshot{"d/sub/f.md": r("hR", 2)},
			base:   Snapshot{"d/sub/f.md": e("h0")},
			want: []Action{{
				Kind: Conflict, Path: "d/sub/f.md", RemoteVersion: 2,
				ConflictPath: "d/sub/f (conflicted copy " + conflictTS + ").md",
			}},
		},
		{
			name:   "multiple mixed actions, sorted by path",
			local:  Snapshot{"b": e("nb"), "c": e("h1")},
			remote: Snapshot{"a": r("ra", 1), "c": r("h1", 1)},
			base:   Snapshot{"c": e("h1")},
			want: []Action{
				{Kind: Download, Path: "a", RemoteVersion: 1},
				{Kind: Upload, Path: "b", RemoteVersion: 0},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Plan(tc.local, tc.remote, tc.base, fixedNow)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Plan() =\n  %+v\nwant\n  %+v", got, tc.want)
			}
		})
	}
}

func TestPlanIdempotent(t *testing.T) {
	t.Parallel()
	local := Snapshot{"a": e("h2"), "x": e("hx")}
	remote := Snapshot{"a": r("h1", 1), "y": r("hy", 1)}
	base := Snapshot{"a": e("h1")}
	p1 := Plan(local, remote, base, fixedNow)
	p2 := Plan(local, remote, base, fixedNow)
	if !reflect.DeepEqual(p1, p2) {
		t.Fatalf("planning twice differs:\n%+v\n%+v", p1, p2)
	}
}

// TestPlanFixedPoint applies a non-conflicting plan to the snapshots, recomputes
// the baseline the way the executor does, and asserts the next Plan is empty —
// i.e. sync converges.
func TestPlanFixedPoint(t *testing.T) {
	t.Parallel()
	rows := []struct {
		name                string
		local, remote, base Snapshot
	}{
		{"new local", Snapshot{"a": e("h1")}, Snapshot{}, Snapshot{}},
		{"new remote", Snapshot{}, Snapshot{"a": r("h1", 1)}, Snapshot{}},
		{"local modified", Snapshot{"a": e("h2")}, Snapshot{"a": r("h1", 1)}, Snapshot{"a": e("h1")}},
		{"remote modified", Snapshot{"a": e("h1")}, Snapshot{"a": r("h2", 2)}, Snapshot{"a": e("h1")}},
		{"local deleted", Snapshot{}, Snapshot{"a": r("h1", 1)}, Snapshot{"a": e("h1")}},
		{"remote deleted", Snapshot{"a": e("h1")}, Snapshot{}, Snapshot{"a": e("h1")}},
		{"resurrect", Snapshot{"a": e("h2")}, Snapshot{}, Snapshot{"a": e("h1")}},
		{"restore", Snapshot{}, Snapshot{"a": r("h2", 2)}, Snapshot{"a": e("h1")}},
		{"mixed", Snapshot{"b": e("nb"), "c": e("h1")}, Snapshot{"a": r("ra", 1), "c": r("h1", 1)}, Snapshot{"c": e("h1")}},
	}
	for _, row := range rows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			local := clone(row.local)
			remote := clone(row.remote)
			plan := Plan(local, remote, row.base, fixedNow)
			for _, a := range plan {
				if a.Kind == Conflict {
					t.Fatalf("unexpected conflict in %q", row.name)
				}
				apply(local, remote, a)
			}
			base := convergedBaseline(local, remote)
			if next := Plan(local, remote, base, fixedNow); len(next) != 0 {
				t.Fatalf("not a fixed point; second plan: %+v", next)
			}
		})
	}
}

func clone(s Snapshot) Snapshot {
	out := make(Snapshot, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}

// apply mutates local/remote as the executor would for a successful action.
func apply(local, remote Snapshot, a Action) {
	switch a.Kind {
	case Upload:
		l := local[a.Path]
		remote[a.Path] = Entry{Hash: l.Hash, Size: l.Size, Version: a.RemoteVersion + 1}
	case Download:
		r := remote[a.Path]
		local[a.Path] = Entry{Hash: r.Hash, Size: r.Size}
	case DeleteLocal:
		delete(local, a.Path)
	case DeleteRemote:
		delete(remote, a.Path)
	}
}

// convergedBaseline mirrors Syncer.saveBaseline.
func convergedBaseline(local, remote Snapshot) Snapshot {
	base := Snapshot{}
	for p, m := range local {
		if r, ok := remote[p]; ok && r.Hash == m.Hash {
			base[p] = Entry{Hash: m.Hash, Size: m.Size}
		}
	}
	return base
}

func TestKindString(t *testing.T) {
	t.Parallel()
	kinds := []Kind{Upload, Download, DeleteLocal, DeleteRemote, Conflict}
	want := []string{"upload", "download", "delete-local", "delete-remote", "conflict"}
	for i, k := range kinds {
		if k.String() != want[i] {
			t.Errorf("Kind(%d).String() = %s, want %s", k, k.String(), want[i])
		}
	}
}

// guard against accidental dependence on map iteration order.
func TestPlanDeterministicOrder(t *testing.T) {
	t.Parallel()
	local := Snapshot{"z": e("1"), "a": e("2"), "m": e("3")}
	got := Plan(local, Snapshot{}, Snapshot{}, fixedNow)
	paths := make([]string, len(got))
	for i, a := range got {
		paths[i] = a.Path
	}
	if !sort.StringsAreSorted(paths) {
		t.Fatalf("actions not sorted by path: %v", paths)
	}
}
