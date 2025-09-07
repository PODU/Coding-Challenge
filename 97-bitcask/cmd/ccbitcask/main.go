// Command ccbitcask is a small CLI over the bitcask key-value store.
//
//	ccbitcask -db <dir> set    <key> <value>   # Put; prints OK
//	ccbitcask -db <dir> get    <key>           # prints value, or "(nil)"
//	ccbitcask -db <dir> delete <key>           # append tombstone; prints OK
//	ccbitcask -db <dir> list                   # one key per line
//	ccbitcask -db <dir> merge                  # run compaction
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	bitcask "ccbitcask"
)

func main() {
	os.Exit(run())
}

func run() int {
	db := flag.String("db", "./database", "database directory")
	sync := flag.Bool("sync", false, "fsync after every write")
	maxfile := flag.Int64("maxfile", bitcask.DefaultMaxFileSize, "active-file rotation threshold in bytes")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		return 1
	}
	cmd := args[0]

	opts := bitcask.Options{
		Dir:          *db,
		MaxFileSize:  *maxfile,
		SyncOnPut:    *sync,
		VerifyOnRead: true,
		ReadOnly:     cmd == "get" || cmd == "list",
	}

	b, err := bitcask.Open(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	defer b.Close()

	switch cmd {
	case "set":
		if len(args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: ccbitcask -db <dir> set <key> <value>")
			return 1
		}
		if err := b.Put([]byte(args[1]), []byte(args[2])); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println("OK")
		return 0

	case "get":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: ccbitcask -db <dir> get <key>")
			return 1
		}
		v, err := b.Get([]byte(args[1]))
		if errors.Is(err, bitcask.ErrKeyNotFound) {
			fmt.Println("(nil)")
			return 2
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Printf("%s\n", v)
		return 0

	case "delete":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: ccbitcask -db <dir> delete <key>")
			return 1
		}
		if err := b.Delete([]byte(args[1])); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println("OK")
		return 0

	case "list":
		for _, k := range b.Keys() {
			fmt.Printf("%s\n", k)
		}
		return 0

	case "merge":
		if err := b.Merge(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println("OK")
		return 0

	default:
		fmt.Fprintln(os.Stderr, "unknown command:", cmd)
		usage()
		return 1
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ccbitcask - a Bitcask key-value store

Usage:
  ccbitcask -db <dir> set    <key> <value>
  ccbitcask -db <dir> get    <key>
  ccbitcask -db <dir> delete <key>
  ccbitcask -db <dir> list
  ccbitcask -db <dir> merge

Flags:
  -db <dir>        database directory (default ./database)
  -sync            fsync after every write
  -maxfile <n>     active-file rotation threshold in bytes
`)
}
