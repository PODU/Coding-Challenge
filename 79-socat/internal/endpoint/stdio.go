package endpoint

import "io"

// stdio joins separate read and write streams, so each half can be closed on
// its own.
type stdio struct {
	r io.Reader
	w io.Writer
}

// NewStdio builds a STDIO endpoint. Tests pass buffers instead of os.Stdin and
// os.Stdout.
func NewStdio(r io.Reader, w io.Writer) Endpoint {
	return &stdio{r: r, w: w}
}

func (s *stdio) Read(p []byte) (int, error)  { return s.r.Read(p) }
func (s *stdio) Write(p []byte) (int, error) { return s.w.Write(p) }

func (s *stdio) Close() error {
	rerr := s.CloseRead()
	werr := s.CloseWrite()
	if rerr != nil {
		return rerr
	}
	return werr
}

func (s *stdio) CloseRead() error {
	if c, ok := s.r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

func (s *stdio) CloseWrite() error {
	if c, ok := s.w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
