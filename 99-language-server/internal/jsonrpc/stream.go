package jsonrpc

import (
	"bufio"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// DefaultMaxBody stops a bogus Content-Length from making us allocate
// gigabytes.
const DefaultMaxBody = 16 << 20 // 16 MiB

// Stream reads and writes framed messages. Writes may come from several
// goroutines; reads must come from one.
type Stream struct {
	r       *bufio.Reader
	w       io.Writer
	wmu     sync.Mutex
	MaxBody int
}

// NewStream returns a Stream reading from r and writing to w.
func NewStream(r io.Reader, w io.Writer) *Stream {
	return &Stream{r: bufio.NewReader(r), w: w, MaxBody: DefaultMaxBody}
}

// idPattern pulls the "id" out of a body that failed to parse, so the error
// reply can still go to the right request.
var idPattern = regexp.MustCompile(`"id"\s*:\s*("(?:[^"\\]|\\.)*"|-?\d+)`)

// Read reads one message. It returns io.EOF only between messages, and
// io.ErrUnexpectedEOF if the stream ends partway through one. A
// *FramingError or *ParseError leaves the stream at the next message.
func (s *Stream) Read() (*Message, error) {
	contentLength := -1
	var headerErr error
	sawHeader := false

	for {
		line, err := s.r.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if sawHeader || strings.TrimRight(line, "\r\n") != "" {
					return nil, io.ErrUnexpectedEOF
				}
				return nil, io.EOF
			}
			return nil, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		sawHeader = true

		colon := strings.IndexByte(trimmed, ':')
		if colon < 0 {
			// Keep reading to the blank line so the next Read starts clean.
			headerErr = &FramingError{Reason: "header line without colon: " + trimmed}
			continue
		}
		name := strings.TrimSpace(trimmed[:colon])
		value := strings.TrimSpace(trimmed[colon+1:])
		if strings.EqualFold(name, "Content-Length") {
			n, err := strconv.Atoi(value)
			if err != nil {
				headerErr = &FramingError{Reason: "invalid Content-Length: " + value}
				continue
			}
			contentLength = n
		}
	}

	if headerErr != nil {
		return nil, headerErr
	}
	if contentLength < 0 {
		return nil, &FramingError{Reason: "missing Content-Length header"}
	}
	if contentLength > s.MaxBody {
		return nil, &FramingError{Reason: "Content-Length " + strconv.Itoa(contentLength) + " exceeds cap"}
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(s.r, body); err != nil {
		if err == io.EOF {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, &ParseError{ID: recoverID(body), Err: err}
	}
	return &msg, nil
}

func recoverID(body []byte) json.RawMessage {
	m := idPattern.FindSubmatch(body)
	if m == nil {
		return nil
	}
	out := make([]byte, len(m[1]))
	copy(out, m[1])
	return out
}

// Write sends msg as one frame, filling in the jsonrpc version if it is empty.
func (s *Stream) Write(msg *Message) error {
	if msg.JSONRPC == "" {
		msg.JSONRPC = Version
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	s.wmu.Lock()
	defer s.wmu.Unlock()
	if _, err := io.WriteString(s.w, "Content-Length: "+strconv.Itoa(len(body))+"\r\n\r\n"); err != nil {
		return err
	}
	_, err = s.w.Write(body)
	return err
}
