// Package endpoint opens the stream behind a parsed address.
package endpoint

import (
	"fmt"
	"io"
	"net"
	"os"

	"ccsocat/internal/addr"
)

// Endpoint is what every address kind opens to.
type Endpoint = io.ReadWriteCloser

// Role says which side of the relay an endpoint is on. FILE uses it to decide
// between reading and writing.
type Role int

const (
	RoleLeft  Role = iota // first address (data source in -u mode)
	RoleRight             // second address (data sink in -u mode)
)

// Open opens a non-listening address. Listening ones go through Listen.
func Open(spec *addr.AddressSpec, role Role) (Endpoint, error) {
	switch spec.Kind {
	case addr.KindSTDIO:
		return NewStdio(os.Stdin, os.Stdout), nil

	case addr.KindTCP, addr.KindUDP:
		conn, err := net.Dial(spec.Net, net.JoinHostPort(spec.Host(), spec.Port()))
		if err != nil {
			return nil, fmt.Errorf("connect %s: %w", spec.Raw, err)
		}
		return conn, nil

	case addr.KindFile:
		return openFile(spec, role)

	case addr.KindExec:
		return newExec(spec.Command())

	case addr.KindTCPListen, addr.KindUDPListen:
		return nil, fmt.Errorf("%s is a listening address; use Listen", spec.Kind)

	default:
		return nil, fmt.Errorf("unsupported address kind %s", spec.Kind)
	}
}

// openFile opens the file for writing if any write option is given or it is
// the right-hand address, and read-only otherwise.
func openFile(spec *addr.AddressSpec, role Role) (Endpoint, error) {
	path := spec.Path()
	create := spec.HasOption("create") || spec.HasOption("creat")
	appendF := spec.HasOption("append")
	rdonly := spec.HasOption("rdonly")
	rdwr := spec.HasOption("rdwr")

	write := create || appendF || spec.HasOption("wronly") || rdwr ||
		(role == RoleRight && !rdonly)

	flags := os.O_RDONLY
	if write {
		if rdwr {
			flags = os.O_RDWR
		} else {
			flags = os.O_WRONLY
		}
		if create {
			flags |= os.O_CREATE
		}
		if appendF {
			flags |= os.O_APPEND | os.O_CREATE
		}
		if spec.HasOption("trunc") {
			flags |= os.O_TRUNC
		}
	}

	f, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", spec.Raw, err)
	}
	return f, nil
}
