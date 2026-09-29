// Package cli implements the ccdbm command line.
package cli

import (
	"bufio"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"ccdbm/internal/dbm"
	"ccdbm/internal/hashfn"
)

// Version is reported by "ccdbm version".
const Version = "1.0.0"

// Exit codes.
const (
	ExitOK       = 0
	ExitNotFound = 1
	ExitError    = 2
)

const usageText = `ccdbm - a dbm-style key/value database

Usage:
  ccdbm [flags] <command> <file> [args]

Commands:
  create     <file>                 create an empty database
  put        <file> <key> <value>   store or replace a value
  get        <file> <key>           print the value for key
  delete     <file> <key>           remove key
  list       <file>                 print every key (one per line)
  stats      <file>                 print database statistics
  reorganise <file>                 rebuild the file compactly
  shell      <file>                 run gdbmtool-style commands from stdin
  help                              print this text
  version                           print the version

Flags:
  -hex             keys and values are hex strings (in and out)
  -ttl <duration>  put: expire the key after this long, e.g. 30s, 1h
  -values          list: print "key<TAB>value" instead of just keys
  -page-size <n>   create: page size in bytes (power of two, default 4096)
  -hash <name>     create: hash function (fnv1a, djb2, murmur3)
  -nosync          do not fsync after each write (faster, less durable)

Exit status: 0 ok, 1 key not found, 2 error.
`

type options struct {
	hex      bool
	ttl      time.Duration
	values   bool
	pageSize int
	hash     string
	noSync   bool
}

type env struct {
	opts   options
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// Run executes the command line and returns the process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	e := &env{stdin: stdin, stdout: stdout, stderr: stderr}
	fs := flag.NewFlagSet("ccdbm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&e.opts.hex, "hex", false, "")
	fs.DurationVar(&e.opts.ttl, "ttl", 0, "")
	fs.BoolVar(&e.opts.values, "values", false, "")
	fs.IntVar(&e.opts.pageSize, "page-size", dbm.DefaultPageSize, "")
	fs.StringVar(&e.opts.hash, "hash", "fnv1a", "")
	fs.BoolVar(&e.opts.noSync, "nosync", false, "")

	if err := fs.Parse(args); err != nil {
		return e.usageErr(err.Error())
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprint(stderr, usageText)
		return ExitError
	}
	cmd := strings.ToLower(rest[0])
	// flags may also follow the command name: ccdbm put -ttl 10s db k v
	if err := fs.Parse(rest[1:]); err != nil {
		return e.usageErr(err.Error())
	}
	rest = fs.Args()

	switch cmd {
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usageText)
		return ExitOK
	case "version":
		fmt.Fprintf(stdout, "ccdbm %s\n", Version)
		return ExitOK
	}
	if len(rest) == 0 {
		return e.usageErr(cmd + ": missing database file")
	}
	file := rest[0]
	rest = rest[1:]
	switch cmd {
	case "create":
		return e.create(file, rest)
	case "put", "store", "set":
		return e.put(file, rest)
	case "get", "fetch":
		return e.get(file, rest)
	case "delete", "del", "rm":
		return e.delete(file, rest)
	case "list", "keys":
		return e.list(file, rest)
	case "stats", "stat", "info":
		return e.stats(file, rest)
	case "reorganise", "reorganize", "reorg":
		return e.reorganise(file, rest)
	case "shell", "batch":
		return e.shell(file, rest)
	}
	return e.usageErr("unknown command " + cmd)
}

func (e *env) usageErr(msg string) int {
	fmt.Fprintf(e.stderr, "ccdbm: %s\n(run \"ccdbm help\" for usage)\n", msg)
	return ExitError
}

func (e *env) fail(err error) int {
	fmt.Fprintf(e.stderr, "ccdbm: %v\n", err)
	return ExitError
}

func (e *env) arity(cmd string, args []string, n int) bool {
	if len(args) != n {
		e.usageErr(fmt.Sprintf("%s: expected %d argument(s) after the file, got %d", cmd, n, len(args)))
		return false
	}
	return true
}

func (e *env) decode(s string) ([]byte, error) {
	if !e.opts.hex {
		return []byte(s), nil
	}
	b, err := hex.DecodeString(strings.TrimPrefix(strings.ToLower(s), "0x"))
	if err != nil {
		return nil, fmt.Errorf("bad hex %q: %w", s, err)
	}
	return b, nil
}

func (e *env) encode(b []byte) string {
	if e.opts.hex {
		return hex.EncodeToString(b)
	}
	return string(b)
}

func (e *env) open(file string, readOnly bool) (*dbm.DB, error) {
	return dbm.Open(file, dbm.Options{ReadOnly: readOnly, NoSync: e.opts.noSync})
}

func (e *env) create(file string, args []string) int {
	if !e.arity("create", args, 0) {
		return ExitError
	}
	id, err := hashfn.Parse(e.opts.hash)
	if err != nil {
		return e.fail(err)
	}
	db, err := dbm.Create(file, dbm.Options{PageSize: e.opts.pageSize, Hash: id, NoSync: e.opts.noSync})
	if err != nil {
		return e.fail(err)
	}
	if err := db.Close(); err != nil {
		return e.fail(err)
	}
	fmt.Fprintf(e.stdout, "created %s (page size %d, hash %s)\n", file, e.opts.pageSize, id)
	return ExitOK
}

func (e *env) put(file string, args []string) int {
	if !e.arity("put", args, 2) {
		return ExitError
	}
	k, err := e.decode(args[0])
	if err != nil {
		return e.fail(err)
	}
	v, err := e.decode(args[1])
	if err != nil {
		return e.fail(err)
	}
	db, err := e.open(file, false)
	if err != nil {
		return e.fail(err)
	}
	defer db.Close()
	if err := db.PutTTL(k, v, e.opts.ttl); err != nil {
		return e.fail(err)
	}
	return ExitOK
}

func (e *env) get(file string, args []string) int {
	if !e.arity("get", args, 1) {
		return ExitError
	}
	k, err := e.decode(args[0])
	if err != nil {
		return e.fail(err)
	}
	db, err := e.open(file, true)
	if err != nil {
		return e.fail(err)
	}
	defer db.Close()
	v, err := db.Get(k)
	if errors.Is(err, dbm.ErrNotFound) {
		fmt.Fprintf(e.stderr, "ccdbm: key not found: %s\n", e.encode(k))
		return ExitNotFound
	}
	if err != nil {
		return e.fail(err)
	}
	fmt.Fprintf(e.stdout, "%s\n", e.encode(v))
	return ExitOK
}

func (e *env) delete(file string, args []string) int {
	if !e.arity("delete", args, 1) {
		return ExitError
	}
	k, err := e.decode(args[0])
	if err != nil {
		return e.fail(err)
	}
	db, err := e.open(file, false)
	if err != nil {
		return e.fail(err)
	}
	defer db.Close()
	err = db.Delete(k)
	if errors.Is(err, dbm.ErrNotFound) {
		fmt.Fprintf(e.stderr, "ccdbm: key not found: %s\n", e.encode(k))
		return ExitNotFound
	}
	if err != nil {
		return e.fail(err)
	}
	return ExitOK
}

func (e *env) list(file string, args []string) int {
	if !e.arity("list", args, 0) {
		return ExitError
	}
	db, err := e.open(file, true)
	if err != nil {
		return e.fail(err)
	}
	defer db.Close()
	w := bufio.NewWriter(e.stdout)
	defer w.Flush()
	err = db.Each(func(k, v []byte, _ uint64) bool {
		if e.opts.values {
			fmt.Fprintf(w, "%s\t%s\n", e.encode(k), e.encode(v))
		} else {
			fmt.Fprintf(w, "%s\n", e.encode(k))
		}
		return true
	})
	if err != nil {
		return e.fail(err)
	}
	return ExitOK
}

func (e *env) stats(file string, args []string) int {
	if !e.arity("stats", args, 0) {
		return ExitError
	}
	db, err := e.open(file, true)
	if err != nil {
		return e.fail(err)
	}
	defer db.Close()
	st, err := db.Stats()
	if err != nil {
		return e.fail(err)
	}
	st.Write(e.stdout)
	return ExitOK
}

func (e *env) reorganise(file string, args []string) int {
	if !e.arity("reorganise", args, 0) {
		return ExitError
	}
	res, err := dbm.Reorganise(file)
	if err != nil {
		return e.fail(err)
	}
	fmt.Fprintf(e.stdout, "reorganised %s: %d -> %d bytes (%d keys kept, %d expired dropped, %d free + %d overflow pages reclaimed)\n",
		file, res.BytesBefore, res.BytesAfter, res.Keys, res.Expired, res.FreedPages, res.OverflowPages)
	return ExitOK
}
