package comm

// Config is a parsed command line.
type Config struct {
	suppress        [4]bool // suppress[n] is set by -n; index 0 is unused
	caseInsensitive bool
	checkOrder      bool // fail on the first unsorted line
	noCheckOrder    bool // don't check at all
	total           bool
	delimiter       string

	file1, file2 string // path or "-"
}
