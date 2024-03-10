# ccrun — a minimal container runtime

Coding Challenge #52 ("Build your own Docker"). `ccrun` is a tiny `docker run`:
it pulls a public Docker Hub image, unpacks its layers into a root filesystem,
and (on Linux) launches a process inside Linux namespaces with `pivot_root` and
cgroup limits, wiring stdio and propagating the exit code.

## Two halves: portable core + Linux-only runtime

The codebase is split so the parts that *can* be exercised on any OS are
exercised on any OS:

| Layer | Package | Portable? | Tested on Windows |
|-------|---------|-----------|-------------------|
| CLI parsing | `internal/cli` | yes | yes |
| Image-reference / URL building | `internal/registry` (`ref.go`) | yes | yes |
| Bearer-token auth flow | `internal/registry` (`auth.go`) | yes | yes |
| Manifest / index / config parsing | `internal/registry` (`manifest.go`) | yes | yes |
| Pull orchestration (token → manifest → config → layers) | `internal/registry` (`client.go`) | yes | yes (httptest, no network) |
| Layer extraction (gzip+tar, stacking, whiteouts, anti-traversal) | `internal/imagefs` | yes | yes |
| Config/CLI merge, memory/CPU parsing | `internal/container` (`spec.go`) | yes | yes |
| Namespaces, `pivot_root`, `/proc`, UID/GID maps, cgroups, exec | `internal/container` (`run_linux.go`) | **Linux only** | compiled-checked (cross-build) |
| Non-Linux stub returning a clear error | `internal/container` (`run_other.go`) | yes | yes |

The Linux runtime lives behind `//go:build linux`; on every other OS
`container.Run` returns `ErrUnsupported` ("container execution is only supported
on Linux") with no panic.

## Build & test

```sh
# Portable suite — passes on the Windows build host (and Linux):
go build ./...
go vet ./...
go test ./...

# Verify the Linux-only runtime compiles (from any host):
GOOS=linux GOARCH=amd64 go build ./...
GOOS=linux GOARCH=amd64 go vet ./...
```

There are **no third-party dependencies** — only the Go standard library
(`net/http`, `encoding/json`, `archive/tar`, `compress/gzip`,
`net/http/httptest`, `syscall`). Unit tests use `httptest` and in-memory tar
archives, so `go test ./...` performs no network or privileged operations.

## Usage

```sh
ccrun run [--rootfs DIR] [--memory BYTES] [--cpus N] [--hostname NAME] [--tag TAG] <IMAGE|COMMAND> [ARGS...]
```

| Flag | Default | Meaning |
|------|---------|---------|
| `--rootfs <dir>` | (pull image) | Run inside an existing extracted rootfs instead of pulling |
| `--memory <bytes>` | unlimited | cgroup memory limit (e.g. `100m`, `1g`) |
| `--cpus <n>` | unlimited | cgroup CPU quota (e.g. `1.5`) |
| `--hostname <name>` | `ccrun` | UTS-namespace hostname |
| `--tag <tag>` | from ref / `latest` | Override the image tag |

Examples (the `run` steps require Linux):

```sh
ccrun run ubuntu /bin/bash              # pull ubuntu:latest and run bash
ccrun run --tag 3.20 alpine sh          # pull a specific tag
ccrun run --rootfs ./alpine /bin/busybox sh   # run in a pre-extracted rootfs
ccrun run --memory 100m alpine sh       # apply a cgroup memory limit
```

`ccrun` exits with the container process's exit code; its own errors (bad
arguments, pull failure, running on a non-Linux host) exit non-zero with a
diagnostic on stderr.

## Running the container steps (Linux)

The image pull and layer extraction work anywhere, but the isolation steps need
Linux. Copy the static binary into a Linux VM (or build there):

```sh
GOOS=linux GOARCH=amd64 go build -o ccrun ./cmd/ccrun
# on the Linux host:
./ccrun run alpine /bin/sh
```

Rootless operation uses a user namespace mapping container UID 0 to the invoking
host user, so no root is required for the namespace/pivot steps. cgroup limits
require a writable cgroup v2 hierarchy; when delegation is unavailable `ccrun`
prints a warning and continues without the limit rather than failing.

## What is and isn't covered

Implemented: Registry V2 token-auth pull, multi-arch index → `linux/amd64`
selection, Schema 2 + OCI media types, sha256 blob verification, layer stacking
with whiteouts and zip-slip protection, config (`Env`/`WorkingDir`/`Entrypoint`/
`Cmd`) application, UTS/PID/mount/user namespaces, `pivot_root` (with `chroot`
fallback), `/proc` mount, UID/GID maps, and best-effort cgroup v2 memory/CPU
limits.

Out of scope (challenge "going further"): network namespaces, an image cache
keyed by digest, overlayfs, and a full PID 1 init/signal-reaper.
