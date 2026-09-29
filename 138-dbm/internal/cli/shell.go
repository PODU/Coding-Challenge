package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
	"time"

	"ccdbm/internal/dbm"
)

const shellHelp = `Commands (gdbmtool style):
  store <key> <value>     store or replace a value
  fetch <key>             print the value for key
  delete <key>            remove key
  list                    print every key and value
  keys                    print every key
  count                   print the number of keys
  first                   print the first key
  next <key>              print the key after key
  ttl <key> <duration>    store nothing; set an expiry on an existing key
  stats                   print database statistics
  reorganize              rebuild the file compactly
  help                    print this text
  quit                    exit
Arguments may be quoted ("like this" or 'this') and use \n \t \\ \" escapes.
Lines starting with # are comments.
`

// shell reads one command per line from stdin and executes it against the
// database, in the manner of gdbmtool. Errors are reported and the loop
// continues; the exit status is non-zero only if the database itself
// cannot be opened.
func (e *env) shell(file string, args []string) int {
	if !e.arity("shell", args, 0) {
		return ExitError
	}
	db, err := e.open(file, false)
	if err != nil {
		return e.fail(err)
	}
	defer func() {
		if db != nil {
			db.Close()
		}
	}()
	sc := bufio.NewScanner(e.stdin)
	sc.Buffer(make([]byte, 1<<20), 1<<30)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		toks, err := tokenize(line)
		if err != nil {
			fmt.Fprintf(e.stderr, "ccdbm: %v\n", err)
			continue
		}
		if len(toks) == 0 {
			continue
		}
		cmd, args := strings.ToLower(toks[0]), toks[1:]
		switch cmd {
		case "quit", "q", "exit":
			return ExitOK
		case "reorganize", "reorganise", "reorg":
			db.Close()
			db = nil
			res, err := dbm.Reorganise(file)
			if err != nil {
				fmt.Fprintf(e.stderr, "ccdbm: %v\n", err)
				return ExitError
			}
			fmt.Fprintf(e.stdout, "reorganised: %d -> %d bytes\n", res.BytesBefore, res.BytesAfter)
			if db, err = e.open(file, false); err != nil {
				fmt.Fprintf(e.stderr, "ccdbm: %v\n", err)
				return ExitError
			}
			continue
		}
		if err := e.shellCmd(db, cmd, args); err != nil {
			fmt.Fprintf(e.stderr, "%v\n", err)
		}
	}
	if err := sc.Err(); err != nil {
		return e.fail(err)
	}
	return ExitOK
}

func (e *env) shellCmd(db *dbm.DB, cmd string, args []string) error {
	need := func(n int) error {
		if len(args) != n {
			return fmt.Errorf("%s: expected %d argument(s), got %d", cmd, n, len(args))
		}
		return nil
	}
	notFound := errors.New("No such item found.")
	switch cmd {
	case "help", "?":
		fmt.Fprint(e.stdout, shellHelp)
	case "store", "put", "set":
		if err := need(2); err != nil {
			return err
		}
		k, err := e.decode(args[0])
		if err != nil {
			return err
		}
		v, err := e.decode(args[1])
		if err != nil {
			return err
		}
		return db.PutTTL(k, v, e.opts.ttl)
	case "fetch", "get":
		if err := need(1); err != nil {
			return err
		}
		k, err := e.decode(args[0])
		if err != nil {
			return err
		}
		v, err := db.Get(k)
		if errors.Is(err, dbm.ErrNotFound) {
			return notFound
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(e.stdout, "%s\n", e.encode(v))
	case "delete", "del", "rm":
		if err := need(1); err != nil {
			return err
		}
		k, err := e.decode(args[0])
		if err != nil {
			return err
		}
		err = db.Delete(k)
		if errors.Is(err, dbm.ErrNotFound) {
			return notFound
		}
		return err
	case "list":
		return db.Each(func(k, v []byte, _ uint64) bool {
			fmt.Fprintf(e.stdout, "%s %s\n", e.encode(k), e.encode(v))
			return true
		})
	case "keys":
		return db.Each(func(k, _ []byte, _ uint64) bool {
			fmt.Fprintf(e.stdout, "%s\n", e.encode(k))
			return true
		})
	case "count":
		fmt.Fprintf(e.stdout, "There are %d items in the database.\n", db.Len())
	case "first":
		k, err := db.FirstKey()
		if err != nil {
			return err
		}
		if k == nil {
			return notFound
		}
		fmt.Fprintf(e.stdout, "%s\n", e.encode(k))
	case "next":
		if err := need(1); err != nil {
			return err
		}
		k, err := e.decode(args[0])
		if err != nil {
			return err
		}
		nk, err := db.NextKey(k)
		if err != nil {
			return err
		}
		if nk == nil {
			return notFound
		}
		fmt.Fprintf(e.stdout, "%s\n", e.encode(nk))
	case "ttl":
		if err := need(2); err != nil {
			return err
		}
		k, err := e.decode(args[0])
		if err != nil {
			return err
		}
		d, err := time.ParseDuration(args[1])
		if err != nil {
			return err
		}
		v, err := db.Get(k)
		if errors.Is(err, dbm.ErrNotFound) {
			return notFound
		}
		if err != nil {
			return err
		}
		return db.PutTTL(k, v, d)
	case "stats", "status":
		st, err := db.Stats()
		if err != nil {
			return err
		}
		st.Write(e.stdout)
	default:
		return fmt.Errorf("unknown command %q (try \"help\")", cmd)
	}
	return nil
}

// tokenize splits a shell line into words, honouring double and single
// quotes and backslash escapes inside double quotes.
func tokenize(line string) ([]string, error) {
	var toks []string
	var cur strings.Builder
	inTok := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"' || c == '\'':
			inTok = true
			q := c
			i++
			for ; i < len(line) && line[i] != q; i++ {
				if q == '"' && line[i] == '\\' && i+1 < len(line) {
					i++
					switch line[i] {
					case 'n':
						cur.WriteByte('\n')
					case 't':
						cur.WriteByte('\t')
					case 'r':
						cur.WriteByte('\r')
					case '0':
						cur.WriteByte(0)
					default:
						cur.WriteByte(line[i])
					}
					continue
				}
				cur.WriteByte(line[i])
			}
			if i >= len(line) {
				return nil, fmt.Errorf("unterminated %c quote", q)
			}
		case c == '\\' && i+1 < len(line):
			inTok = true
			i++
			cur.WriteByte(line[i])
		case c == ' ' || c == '\t':
			if inTok {
				toks = append(toks, cur.String())
				cur.Reset()
				inTok = false
			}
		default:
			inTok = true
			cur.WriteByte(c)
		}
	}
	if inTok {
		toks = append(toks, cur.String())
	}
	return toks, nil
}
