# Software KVM: Coding Challenge #142

`cckvm` shares one keyboard, mouse and clipboard across several computers.
It speaks the Barrier 1.6 wire protocol, so it works with genuine Barrier
clients and servers as well as with itself.

Source: https://codingchallenges.substack.com/p/coding-challenge-142-software-kvm

## Build & test

```sh
go build -o cckvm ./cmd/cckvm     # cckvm.exe on Windows
go vet ./...
go test ./...
CCKVM_NATIVE_TEST=1 go test ./internal/platform/   # optional: touches the real desktop
```

Requires Go 1.24+. Standard library only, no cgo.

## Usage

Describe the screens and how their edges connect, in Barrier's config
format. Links go one way, so list both directions:

```
section: screens
    desktop:
    laptop:
    tablet:
end
section: links
    desktop:
        right = laptop
        down  = tablet
    laptop:
        left = desktop
    tablet:
        up = desktop
end
section: options
    relativeMouseMoves = false
end
```

On the machine with the keyboard and mouse:

```sh
cckvm server -c kvm.conf -n desktop
```

On each other machine (or start a genuine Barrier client named `laptop`
and point it at the server):

```sh
cckvm client -n laptop desktop.local
```

Push the mouse off the right edge of `desktop` and it appears on
`laptop`. Push it back to return. Copy on one machine and paste on
another. Scroll Lock keeps the cursor on the current screen, and holding a
mouse button stops it from leaving the screen mid-drag.

| Option | |
|---|---|
| `-n NAME` | screen name, defaults to the host name |
| `-a ADDR` | server listen address (default `:24800`) |
| `--relative` | send DMRM deltas instead of DMMV positions |
| `--tls` | encrypt. The server prints its certificate fingerprint; clients must also pass `--fingerprint SHA256:…` (`cckvm fingerprint` prints it) |
| `--no-reconnect` | client exits when the connection drops |
| `--backend log --size WxH` | print input instead of injecting it (works on any OS) |
| `-v` | debug logging |

## Platform support

Native input capture and injection works on **Windows**: low-level hooks,
`SendInput` and the clipboard, all in pure Go. On other operating systems
use `--backend log`. It runs the full protocol and prints what it receives,
which is enough to check interoperability with a Barrier peer. A new
desktop backend only has to implement `platform.Screen`.
