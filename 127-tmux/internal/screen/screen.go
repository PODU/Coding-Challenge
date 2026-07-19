// Package screen is a small terminal emulator: a grid of cells that shell
// output is written into. It handles the usual control characters and enough
// CSI sequences (cursor movement, erase, colours) for shells and line editors.
// Lines that scroll off the top are kept for copy mode.
package screen

type Cell struct {
	Rune  rune
	Style string // SGR parameters, e.g. "1;31"; "" is the default style
}

type Screen struct {
	w, h      int
	cells     [][]Cell
	cx, cy    int
	style     string
	scrollMax int
	scroll    [][]Cell // oldest first

	inEsc  bool   // inside an escape sequence
	escBuf []byte // escape sequence bytes after the ESC
}

// New returns a blank w x h screen (at least 1x1) with 1000 lines of
// scrollback.
func New(w, h int) *Screen {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	s := &Screen{w: w, h: h, scrollMax: 1000}
	s.alloc()
	return s
}

func (s *Screen) alloc() {
	s.cells = make([][]Cell, s.h)
	for y := range s.cells {
		s.cells[y] = s.blankRow()
	}
}

func (s *Screen) blankRow() []Cell {
	row := make([]Cell, s.w)
	for x := range row {
		row[x] = Cell{Rune: ' '}
	}
	return row
}

func (s *Screen) Size() (int, int) { return s.w, s.h }

func (s *Screen) Cursor() (int, int) { return s.cx, s.cy }

// Cell returns the cell at (x,y), or a blank if that is off the screen.
func (s *Screen) Cell(x, y int) Cell {
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return Cell{Rune: ' '}
	}
	return s.cells[y][x]
}

// Set puts r at (x,y), bypassing the cursor. Off-screen writes are dropped.
func (s *Screen) Set(x, y int, r rune, style string) {
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return
	}
	s.cells[y][x] = Cell{Rune: r, Style: style}
}

// Line returns the text of row y without trailing spaces.
func (s *Screen) Line(y int) string {
	if y < 0 || y >= s.h {
		return ""
	}
	rs := make([]rune, s.w)
	for x := 0; x < s.w; x++ {
		rs[x] = s.cells[y][x].Rune
	}
	end := s.w
	for end > 0 && rs[end-1] == ' ' {
		end--
	}
	return string(rs[:end])
}

// Scrollback returns the text of the scrolled-off lines, oldest first.
func (s *Screen) Scrollback() []string {
	out := make([]string, len(s.scroll))
	for i, row := range s.scroll {
		rs := make([]rune, len(row))
		for x, c := range row {
			rs[x] = c.Rune
		}
		out[i] = trimRight(string(rs))
	}
	return out
}

func (s *Screen) ScrollbackLen() int { return len(s.scroll) }

func (s *Screen) Write(p []byte) {
	for _, b := range p {
		s.feed(b)
	}
}

func (s *Screen) feed(b byte) {
	if s.inEsc {
		s.escBuf = append(s.escBuf, b)
		// A CSI sequence terminates on a final byte in range 0x40..0x7E.
		if len(s.escBuf) >= 2 && s.escBuf[0] == '[' {
			if b >= 0x40 && b <= 0x7e {
				s.handleCSI(s.escBuf[1:len(s.escBuf)-1], b)
				s.inEsc = false
				s.escBuf = nil
			}
			return
		}
		// Any other escape is two bytes long; drop it.
		if s.escBuf[0] != '[' {
			s.inEsc = false
			s.escBuf = nil
		}
		return
	}
	switch b {
	case 0x1b: // ESC
		s.inEsc = true
		s.escBuf = nil
	case '\r':
		s.cx = 0
	case '\n':
		s.newline()
	case '\b':
		if s.cx > 0 {
			s.cx--
		}
	case '\t':
		next := (s.cx/8 + 1) * 8
		if next >= s.w {
			next = s.w - 1
		}
		s.cx = next
	case 0x07: // BEL
	default:
		if b >= 0x20 {
			s.put(rune(b))
		}
	}
}

func (s *Screen) put(r rune) {
	if s.cx >= s.w {
		s.cx = 0
		s.newline()
	}
	s.cells[s.cy][s.cx] = Cell{Rune: r, Style: s.style}
	s.cx++
}

func (s *Screen) newline() {
	s.cy++
	if s.cy >= s.h {
		s.cy = s.h - 1
		s.scrollUp()
	}
}

func (s *Screen) scrollUp() {
	s.scroll = append(s.scroll, s.cells[0])
	if len(s.scroll) > s.scrollMax {
		s.scroll = s.scroll[len(s.scroll)-s.scrollMax:]
	}
	copy(s.cells, s.cells[1:])
	s.cells[s.h-1] = s.blankRow()
}

// handleCSI applies the CSI sequences we support; anything else is ignored.
// params excludes the '[' and the final byte.
func (s *Screen) handleCSI(params []byte, final byte) {
	nums := parseParams(params)
	arg := func(i, def int) int {
		if i < len(nums) && nums[i] > 0 {
			return nums[i]
		}
		return def
	}
	switch final {
	case 'H', 'f': // cursor position (1-based row;col)
		row := arg(0, 1) - 1
		col := arg(1, 1) - 1
		s.cy = clamp(row, 0, s.h-1)
		s.cx = clamp(col, 0, s.w-1)
	case 'A': // up
		s.cy = clamp(s.cy-arg(0, 1), 0, s.h-1)
	case 'B': // down
		s.cy = clamp(s.cy+arg(0, 1), 0, s.h-1)
	case 'C': // right
		s.cx = clamp(s.cx+arg(0, 1), 0, s.w-1)
	case 'D': // left
		s.cx = clamp(s.cx-arg(0, 1), 0, s.w-1)
	case 'G': // cursor to column
		s.cx = clamp(arg(0, 1)-1, 0, s.w-1)
	case 'd': // cursor to row
		s.cy = clamp(arg(0, 1)-1, 0, s.h-1)
	case 'J': // erase display
		s.eraseDisplay(arg(0, 0))
	case 'K': // erase line
		s.eraseLine(arg(0, 0))
	case 'm': // SGR
		s.style = string(params)
		if s.style == "0" {
			s.style = ""
		}
	}
}

func (s *Screen) eraseDisplay(mode int) {
	switch mode {
	case 0: // cursor to end
		s.eraseLine(0)
		for y := s.cy + 1; y < s.h; y++ {
			s.cells[y] = s.blankRow()
		}
	case 1: // start to cursor
		for y := 0; y < s.cy; y++ {
			s.cells[y] = s.blankRow()
		}
		s.eraseLine(1)
	case 2, 3: // whole screen
		for y := 0; y < s.h; y++ {
			s.cells[y] = s.blankRow()
		}
	}
}

func (s *Screen) eraseLine(mode int) {
	row := s.cells[s.cy]
	switch mode {
	case 0:
		for x := s.cx; x < s.w; x++ {
			row[x] = Cell{Rune: ' '}
		}
	case 1:
		for x := 0; x <= s.cx && x < s.w; x++ {
			row[x] = Cell{Rune: ' '}
		}
	case 2:
		for x := 0; x < s.w; x++ {
			row[x] = Cell{Rune: ' '}
		}
	}
}

// Resize changes the screen size, keeping whatever still fits from the
// top-left.
func (s *Screen) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	old := s.cells
	oldH := s.h
	s.w, s.h = w, h
	s.alloc()
	for y := 0; y < h && y < oldH; y++ {
		for x := 0; x < w && x < len(old[y]); x++ {
			s.cells[y][x] = old[y][x]
		}
	}
	s.cx = clamp(s.cx, 0, w-1)
	s.cy = clamp(s.cy, 0, h-1)
}

func parseParams(p []byte) []int {
	var nums []int
	cur := 0
	has := false
	for _, b := range p {
		if b >= '0' && b <= '9' {
			cur = cur*10 + int(b-'0')
			has = true
		} else if b == ';' {
			nums = append(nums, cur)
			cur, has = 0, false
		}
	}
	if has || len(nums) > 0 {
		nums = append(nums, cur)
	}
	return nums
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func trimRight(s string) string {
	end := len(s)
	for end > 0 && s[end-1] == ' ' {
		end--
	}
	return s[:end]
}
