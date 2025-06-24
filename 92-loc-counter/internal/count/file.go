package count

import (
	"bufio"
	"io"
	"os"

	"loc/internal/lang"
)

// FileCount holds one file's counts. Lines == Blank+Comment+Code.
type FileCount struct {
	Path    string
	Lang    string
	Lines   int
	Blank   int
	Comment int
	Code    int
}

const maxLineBytes = 16 * 1024 * 1024 // allow very long lines, e.g. minified JS

// CountFile opens path and classifies it using def.
func CountFile(path string, def *lang.LanguageDef) (FileCount, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileCount{Path: path, Lang: def.Name}, err
	}
	defer f.Close()
	fc, err := CountReader(f, def)
	fc.Path = path
	fc.Lang = def.Name
	return fc, err
}

// CountReader counts the lines read from r. CRLF endings and a final line with
// no newline are handled by bufio.Scanner.
func CountReader(r io.Reader, def *lang.LanguageDef) (FileCount, error) {
	var fc FileCount
	st := &scanState{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	for sc.Scan() {
		fc.Lines++
		switch classify(def, sc.Text(), st) {
		case kindBlank:
			fc.Blank++
		case kindComment:
			fc.Comment++
		case kindCode:
			fc.Code++
		}
	}
	return fc, sc.Err()
}
