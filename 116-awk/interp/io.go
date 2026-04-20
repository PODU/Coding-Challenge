package interp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"goawk/ast"
)

// nextMainRecord returns the next record from the ARGV file sequence (or
// stdin). It updates FILENAME and resets FNR on file boundaries but does not
// touch NR/FNR counters (the caller does).
func (p *Interp) nextMainRecord() (string, bool) {
	for {
		if p.curReader != nil {
			rec, ok, err := p.curReader.next(p.rs)
			if err != nil {
				p.fatalf("read error: %v", err)
			}
			if ok {
				return rec, true
			}
			if p.curCloser != nil {
				p.curCloser.Close()
			}
			p.curReader = nil
			p.curCloser = nil
			continue
		}
		if !p.openNextInput() {
			return "", false
		}
	}
}

// openNextInput opens the next input file/stdin. Returns false when input is
// exhausted.
func (p *Interp) openNextInput() bool {
	argc := int(p.getVar("ARGC").num())
	argv := p.resolveArray("ARGV")
	for p.argIndex < argc {
		arg := argv[strconv.Itoa(p.argIndex)].str(p.convfmt)
		p.argIndex++
		if arg == "" {
			continue
		}
		if name, val, ok := splitAssignment(arg); ok {
			p.SetVarString(name, unescape(val))
			continue
		}
		p.processedAnyArg = true
		p.filename = arg
		p.fnr = 0
		if arg == "-" {
			p.curReader = newRecordReader(p.stdin)
			p.curCloser = nil
			return true
		}
		f, err := os.Open(arg)
		if err != nil {
			p.fatalf("can't open file %s", arg)
		}
		p.curReader = newRecordReader(f)
		p.curCloser = f
		return true
	}
	if !p.processedAnyArg && !p.usedStdin {
		p.usedStdin = true
		p.processedAnyArg = true
		p.fnr = 0
		p.curReader = newRecordReader(p.stdin)
		p.curCloser = nil
		return true
	}
	return false
}

func splitAssignment(s string) (name, val string, ok bool) {
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return "", "", false
	}
	name = s[:eq]
	if !isIdentifier(name) {
		return "", "", false
	}
	return name, s[eq+1:], true
}

func isIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			continue
		}
		if i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}

func (p *Interp) evalGetline(g *ast.Getline) value {
	switch g.Mode {
	case ast.GetlineMain:
		rec, ok := p.nextMainRecord()
		if !ok {
			return num(0)
		}
		p.nr++
		p.fnr++
		if g.Target != nil {
			p.assignTo(g.Target, strnum(rec))
		} else {
			p.setRecord(rec)
		}
		return num(1)
	case ast.GetlineFile:
		name := p.eval(g.Source).str(p.convfmt)
		in := p.getInputFile(name)
		if in == nil {
			return num(-1)
		}
		rec, ok, err := in.reader.next(p.rs)
		if err != nil {
			return num(-1)
		}
		if !ok {
			return num(0)
		}
		if g.Target != nil {
			p.assignTo(g.Target, strnum(rec))
		} else {
			p.setRecord(rec)
		}
		return num(1)
	case ast.GetlineCmd:
		cmd := p.eval(g.Source).str(p.convfmt)
		in := p.getInputCommand(cmd)
		if in == nil {
			return num(-1)
		}
		rec, ok, err := in.reader.next(p.rs)
		if err != nil {
			return num(-1)
		}
		if !ok {
			return num(0)
		}
		p.nr++
		if g.Target != nil {
			p.assignTo(g.Target, strnum(rec))
		} else {
			p.setRecord(rec)
		}
		return num(1)
	}
	return num(-1)
}

type inStream struct {
	reader *recordReader
	closer io.Closer
	cmd    *exec.Cmd
}

func (s *inStream) Close() error {
	var err error
	if s.closer != nil {
		err = s.closer.Close()
	}
	if s.cmd != nil {
		s.cmd.Wait()
	}
	return err
}

func (p *Interp) getInputFile(name string) *inStream {
	if s, ok := p.inFiles[name]; ok {
		return s
	}
	var r io.Reader
	var closer io.Closer
	if name == "-" || name == "/dev/stdin" {
		r = p.stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return nil
		}
		r = f
		closer = f
	}
	s := &inStream{reader: newRecordReader(r), closer: closer}
	p.inFiles[name] = s
	return s
}

func (p *Interp) getInputCommand(cmd string) *inStream {
	if s, ok := p.inFiles["cmd:"+cmd]; ok {
		return s
	}
	p.output.Flush()
	c := shellCommand(cmd)
	c.Stderr = p.errout
	out, err := c.StdoutPipe()
	if err != nil {
		return nil
	}
	if err := c.Start(); err != nil {
		return nil
	}
	s := &inStream{reader: newRecordReader(out), cmd: c}
	p.inFiles["cmd:"+cmd] = s
	return s
}

type outStream struct {
	w      io.Writer
	buf    *bufio.Writer
	closer io.Closer
	cmd    *exec.Cmd
}

func (s *outStream) Close() error {
	if s.buf != nil {
		s.buf.Flush()
	}
	var err error
	if s.closer != nil {
		err = s.closer.Close()
	}
	if s.cmd != nil {
		s.cmd.Wait()
	}
	return err
}

func (p *Interp) redirectWrite(r *ast.Redirect, s string) {
	target := p.eval(r.Target).str(p.convfmt)
	out := p.getOutputStream(r.Mode, target)
	out.buf.WriteString(s)
}

func (p *Interp) getOutputStream(mode, target string) *outStream {
	key := mode + ":" + target
	if mode == ">" {
		key = ">:" + target // ">" and ">>" share the same file handle
	}
	if mode == ">>" {
		key = ">:" + target
	}
	if s, ok := p.outFiles[key]; ok {
		return s
	}
	var s *outStream
	switch mode {
	case "|":
		p.output.Flush()
		c := shellCommand(target)
		c.Stdout = p.stdoutW
		c.Stderr = p.errout
		in, err := c.StdinPipe()
		if err != nil {
			p.fatalf("can't open pipe to %q", target)
		}
		if err := c.Start(); err != nil {
			p.fatalf("can't run %q", target)
		}
		s = &outStream{closer: in, buf: bufio.NewWriter(in), cmd: c}
	default:
		flags := os.O_CREATE | os.O_WRONLY
		if mode == ">>" {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		var w io.Writer
		var closer io.Closer
		switch target {
		case "/dev/stdout":
			w = p.output
		case "/dev/stderr":
			w = p.errout
		default:
			f, err := os.OpenFile(target, flags, 0644)
			if err != nil {
				p.fatalf("can't redirect to %s", target)
			}
			w = f
			closer = f
		}
		s = &outStream{w: w, buf: bufio.NewWriter(w), closer: closer}
	}
	p.outFiles[key] = s
	return s
}

func (p *Interp) bClose(args []ast.Expr) value {
	name := p.arg(args, 0).str(p.convfmt)
	found := false
	for _, key := range []string{">:" + name, "|:" + name} {
		if s, ok := p.outFiles[key]; ok {
			s.Close()
			delete(p.outFiles, key)
			found = true
		}
	}
	if s, ok := p.inFiles[name]; ok {
		s.Close()
		delete(p.inFiles, name)
		found = true
	}
	if s, ok := p.inFiles["cmd:"+name]; ok {
		s.Close()
		delete(p.inFiles, "cmd:"+name)
		found = true
	}
	if found {
		return num(0)
	}
	return num(-1)
}

func (p *Interp) bFflush(args []ast.Expr) value {
	p.output.Flush()
	for _, s := range p.outFiles {
		if s.buf != nil {
			s.buf.Flush()
		}
	}
	return num(0)
}

func (p *Interp) bSystem(args []ast.Expr) value {
	cmd := p.arg(args, 0).str(p.convfmt)
	p.output.Flush()
	for _, s := range p.outFiles {
		if s.buf != nil {
			s.buf.Flush()
		}
	}
	c := shellCommand(cmd)
	c.Stdout = p.stdoutW
	c.Stderr = p.errout
	c.Stdin = os.Stdin
	err := c.Run()
	if err == nil {
		return num(0)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return num(float64(ee.ExitCode()))
	}
	return num(-1)
}

func shellCommand(cmd string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		if sh, err := exec.LookPath("sh"); err == nil {
			return exec.Command(sh, "-c", cmd)
		}
		return exec.Command("cmd", "/C", cmd)
	}
	return exec.Command("sh", "-c", cmd)
}

func (p *Interp) callUser(n *ast.Call) value {
	fn := p.prog.Functions[n.Name]
	if len(n.Args) > len(fn.Params) {
		p.fatalf("function %s called with %d args, expects %d", n.Name, len(n.Args), len(fn.Params))
	}
	fr := &frame{
		scalars: map[string]value{},
		arrays:  map[string]map[string]value{},
		isLocal: map[string]bool{},
	}
	for i, param := range fn.Params {
		fr.isLocal[param] = true
		if i < len(n.Args) {
			if fn.Arrays[i] {
				fr.arrays[param] = p.resolveArrayArg(n.Args[i])
			} else {
				fr.scalars[param] = p.eval(n.Args[i])
			}
		} else {
			if fn.Arrays[i] {
				fr.arrays[param] = map[string]value{}
			} else {
				fr.scalars[param] = uninit
			}
		}
	}
	p.frames = append(p.frames, fr)
	p.returnValue = uninit
	p.execBlock(fn.Body)
	rv := p.returnValue
	p.frames = p.frames[:len(p.frames)-1]
	p.returnValue = uninit
	return rv
}

// resolveArrayArg returns the array map referenced by an argument passed to an
// array parameter (must be a bare variable name).
func (p *Interp) resolveArrayArg(e ast.Expr) map[string]value {
	v, ok := e.(*ast.Var)
	if !ok {
		p.fatalf("expected array argument")
	}
	return p.resolveArray(v.Name)
}

// unescape processes backslash escapes for -v and command-line assignments.
func unescape(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		case '/':
			b.WriteByte('/')
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

var _ = fmt.Sprintf
