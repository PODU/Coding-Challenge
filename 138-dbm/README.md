# `ccdbm` — a dbm-style key/value store (Coding Challenge #138)

A single-file key/value database in the tradition of Unix dbm and gdbm:
arbitrary byte keys and values, stored on disk with extendible hashing so
the file grows one bucket at a time and every lookup is one directory read
plus one page read.

Go, no dependencies.

## Build & run

```sh
go build -o ccdbm .
./ccdbm create test.db
./ccdbm put test.db greeting "Hello, world!"
./ccdbm put test.db language Go
./ccdbm get test.db greeting
./ccdbm list test.db
./ccdbm delete test.db language
./ccdbm stats test.db
./ccdbm reorganise test.db
```

```
created test.db (page size 4096, hash fnv1a)
Hello, world!
greeting
language
```

## Usage

```
ccdbm [flags] <command> <file> [args]
```

| Command | Meaning |
|---------|---------|
| `create <file>` | Create an empty database (fails if the file exists). |
| `put <file> <key> <value>` | Store or replace a value. |
| `get <file> <key>` | Print the value; exit 1 if the key is missing. |
| `delete <file> <key>` | Remove a key; exit 1 if it is missing. |
| `list <file>` | Print every key, one per line (`-values` adds `<TAB>value`). |
| `stats <file>` | Pages, depths, overflow, free list, fill ratio. |
| `reorganise <file>` | Rebuild the file compactly (`reorganize` also accepted). |
| `shell <file>` | Read gdbmtool-style commands from stdin. |
| `help`, `version` | |

| Flag | Meaning |
|------|---------|
| `-hex` | Keys and values are hex strings on input and output (for binary data). |
| `-ttl <duration>` | `put`: expire the key after e.g. `30s`, `1h`. |
| `-values` | `list`: print values too. |
| `-page-size <n>` | `create`: page size, power of two from 128 to 1048576 (default 4096). |
| `-hash <name>` | `create`: `fnv1a` (default), `djb2` or `murmur3`. |
| `-nosync` | Skip `fsync` after each write (much faster for bulk loads). |

Flags may go before or after the command name. Exit status: 0 ok, 1 key not
found, 2 error. `get`, `list` and `stats` open the file read-only.

### Binary data

```sh
ccdbm -hex put test.db 00ff cafebabe
ccdbm -hex get test.db 00ff          # cafebabe
```

### Batch mode

```sh
printf 'store greeting "Hello, world!"\nfetch greeting\nlist\ndelete greeting\n' | ccdbm shell test.db
```

Commands: `store`, `fetch`, `delete`, `list`, `keys`, `count`, `first`,
`next <key>`, `ttl <key> <duration>`, `stats`, `reorganize`, `help`, `quit`.
Arguments may be quoted and use `\n`, `\t`, `\\`, `\"` escapes; lines
starting with `#` are comments. Missing keys report `No such item found.` as
gdbmtool does.

### Expiry

```sh
ccdbm -ttl 10m put test.db session:42 '{"user":7}'
```

Expired keys disappear from `get`, `list` and iteration immediately and are
physically removed by the next `delete`, overwrite or `reorganise`.

## Library

```go
db, err := dbm.Create("test.db", dbm.Options{})
db.Put([]byte("k"), []byte("v"))
v, err := db.Get([]byte("k"))
db.PutTTL([]byte("tmp"), []byte("x"), time.Minute)
for k, _ := db.FirstKey(); k != nil; k, _ = db.NextKey(k) { ... }
db.Each(func(k, v []byte, expires uint64) bool { ...; return true })
db.Close()
res, err := dbm.Reorganise("test.db")
```

The Step 1 in-memory hash table and Step 2 in-memory extendible hash table
live in `internal/hashtable` and `internal/extendible`.

## Tests

```sh
go test ./...
go test -bench . ./internal/dbm
```

41 tests, all offline, including a 4 000-operation random model test that
reopens and reorganises the file mid-run. Coverage: hashfn 98 %, hashtable
100 %, extendible 95 %, dbm 83 %, cli 83 %.

## Limits

- One writer at a time. There is no cross-process locking; do not run two
  writers against the same file.
- Durability is "header last, then fsync". No write-ahead log; a crash inside
  a single page write can corrupt that page (reads detect and report it).
- The directory is capped at 2^24 slots; beyond that buckets chain instead
  of splitting. Keys and values have no size limit.
