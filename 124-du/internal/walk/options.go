package walk

// Unit is the unit sizes are printed in.
type Unit int

const (
	UnitBlock  Unit = iota // 512 bytes, the POSIX default
	UnitK                  // 1024 bytes
	UnitM                  // 1024*1024 bytes
	UnitHuman              // K/M/G suffixes
	UnitCustom             // Options.BlockSize
)

// Options are the parsed command-line options. Sizes are always counted in
// bytes and only converted to Unit when printed.
type Options struct {
	Apparent bool // logical size instead of allocated blocks
	Inodes   bool // count entries instead of bytes

	All       bool  // -a: include individual files
	Summarize bool  // -s: only the per-argument total (depth 0)
	Total     bool  // -c: append a grand total line
	MaxDepth  int   // -d: print up to N levels; -1 == unlimited
	Threshold int64 // -t: >= SIZE (positive) or <= |SIZE| (negative); 0 == off

	Dereference   bool     // -L: follow symlinks, count target
	OneFileSystem bool     // -x: stay on the start filesystem
	Excludes      []string // --exclude globs (matched against name and path)

	Unit      Unit
	BlockSize int64 // used when Unit == UnitCustom

	JSON bool
	Sort string
}

// Entry is one output row. The walker emits children before their parent.
type Entry struct {
	Path  string
	Size  int64
	Depth int
	IsDir bool
}
