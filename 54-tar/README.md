# mytar — a hand-rolled POSIX USTAR archiver (Coding Challenge #54)

`mytar` creates, lists and extracts **POSIX USTAR** tar archives. The 512-byte
header codec — field packing, octal encoding and the header checksum — is
implemented **by hand** in `internal/ustar`. The Go standard library
`archive/tar` is used **only in tests**, as an interop oracle; it never appears
in the production code path.

## Build

```sh
go build ./cmd/mytar      # produces ./mytar (or mytar.exe on Windows)
```

No third-party dependencies; standard library only.

## Usage

```
mytar -c|-x|-t [-v] [-f ARCHIVE] [FILE...]

  -c            create a new archive from FILE...
  -x            extract members from the archive (into the current directory)
  -t            list member names (with -v: long format)
  -f ARCHIVE    use ARCHIVE; "-" or omitted means stdout (create) / stdin (x,t)
  -v            verbose
```

Exactly one of `-c` / `-x` / `-t` must be given. Bundled GNU-style flags are
accepted (`-cvf`, `-xvf`, `-tf`, `-tfNAME`) as well as separate flags.

### Examples

```sh
mytar -cf archive.tar file1 file2 dir/     # create
mytar -tf archive.tar                      # list names
mytar -tvf archive.tar                     # list, long format
mytar -xf archive.tar                      # extract into CWD
cat archive.tar | mytar -t                 # stream from stdin
mytar -cf - dir/ | gzip > dir.tar.gz       # stream to stdout
```

### Exit codes

- `0` success
- `1` runtime error (I/O, corrupt archive, unsafe path)
- `2` usage error

Errors are written to **stderr** prefixed `mytar:`; list output goes to stdout
only, so `mytar -tf` stays cleanly pipeable.

## What it supports

- USTAR 512-byte block framing with zero-padding and the two trailing zero
  blocks that mark end-of-archive.
- Regular files, empty files (header only, no body blocks), directories
  (`typeflag '5'`), and symbolic links (`typeflag '2'`).
- Hard links on read (`typeflag '1'`).
- **Long names**: first the USTAR `prefix`/`name` split at a `/` boundary (up to
  255 bytes); if that still does not fit, a **GNU `L`** long-name extension
  record (synthetic `././@LongLink` entry) carrying the full path. The reader
  understands both.
- Octal numeric fields encoded as zero-padded, NUL-terminated ASCII, with the
  historical `printf("%06o\0 ", sum)` checksum. On read, both the unsigned and
  the legacy **signed** checksum are accepted.
- Mode and mtime are restored on extract (chmod/chtimes failures are warnings,
  not fatal).

## Safety

Extraction is hardened against the classic "tar-slip" traversal class
(`sanitizePath` in `internal/cmd/paths.go`): member names that are absolute,
carry a drive/volume, or resolve outside the destination via `..` are refused.
Symlink members whose target would escape the destination are likewise refused.
On create, leading `/` and the volume name are stripped (matching `tar`'s
"removing leading '/'" behaviour) so archives are relocatable.

## Architecture

```
cmd/mytar/main.go        CLI: flag parsing/normalisation, mode dispatch, exit codes
internal/cmd/            operations (the only layer that touches disk)
  create.go  extract.go  list.go  paths.go  owner_*.go
internal/archive/        512-byte block framing
  writer.go  reader.go
internal/ustar/          byte-level USTAR codec (no filesystem, no archive/tar)
  header.go  octal.go  checksum.go  names.go  const.go
```

Lower layers know nothing about the filesystem, keeping the codec pure and
unit-testable.

## Tests

```sh
go test ./...
```

Coverage:

- **Header unit tests** — exact field offsets, checksum value, octal
  encode/decode boundaries, base-256 rejection, signed-checksum acceptance,
  tamper detection, prefix-split round trip.
- **Framing round trip** — varied body sizes, empty file, directory, exact
  512-byte body, GNU long name; verifies block alignment and the two trailing
  zero blocks.
- **Create→extract round trip** — a nested temp tree (nested dirs, empty file,
  empty dir, >100-char name, optional symlink) archived then extracted to a
  second temp dir and compared byte-for-byte (and mode on non-Windows).
- **Interop with `archive/tar`** — both directions: stdlib reads a
  mytar-produced archive (including a GNU long name) with matching metadata, and
  mytar lists/extracts a stdlib-produced USTAR archive.
- **Safety** — crafted archives with `../evil`, an absolute path and an escaping
  symlink are refused; `sanitizePath` unit cases.
- **CLI** — `parseArgs` bundling, mode exclusivity, glued/missing `-f`.

## Caveats

- **Read scope is USTAR/GNU-longname.** PAX extended headers (`x`/`g`) and GNU
  base-256 numeric fields are out of scope: they are detected and rejected with
  a clear error rather than mis-parsed. (Go's `archive/tar` emits PAX for long
  names by default, so the stdlib-reads-mytar interop test relies on mytar's own
  GNU `L` output, and the mytar-reads-stdlib test pins `tar.FormatUSTAR`.)
- **Owner info.** UID/GID are read from `syscall.Stat_t` on POSIX hosts; on
  Windows they default to `0` and uname/gname to `root`.
- **Symlinks on Windows** require privilege; the symlink round-trip test skips
  itself if `os.Symlink` is unavailable.
- Devices, FIFOs and sockets are skipped on create (not part of the core).
