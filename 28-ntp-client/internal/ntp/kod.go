package ntp

import (
	"errors"
	"fmt"
)

// Errors for responses that fail validation.
var (
	ErrLeapNotInSync = errors.New("ntp: server clock not synchronised (leap indicator 3)")
	ErrBadMode       = errors.New("ntp: response is not in server mode")
	ErrBadVersion    = errors.New("ntp: unsupported protocol version")
	ErrBadStratum    = errors.New("ntp: unusable stratum")
	// ErrOriginateMismatch means the reply isn't for our request: it is stale
	// or spoofed.
	ErrOriginateMismatch = errors.New("ntp: originate timestamp does not match request")
)

// KissOfDeathError means the server sent a Kiss-o'-Death (stratum 0) packet.
// Code is the kiss code, e.g. DENY or RATE.
type KissOfDeathError struct {
	Code string
}

func (e *KissOfDeathError) Error() string {
	return fmt.Sprintf("ntp: kiss-o'-death from server: %q", e.Code)
}

// IsKissOfDeath reports whether p is a Kiss-o'-Death packet.
func (p *Packet) IsKissOfDeath() bool {
	return p.Stratum == KissOfDeathStr
}

// KissCode returns the kiss code, which is carried in the Reference ID.
func (p *Packet) KissCode() string {
	return p.ReferenceIDString()
}

// Validate checks that p is a usable server reply to our request. originate is
// the transmit timestamp we sent, which the server must echo back.
func (p *Packet) Validate(originate Timestamp) error {
	if p.IsKissOfDeath() {
		return &KissOfDeathError{Code: p.KissCode()}
	}
	if p.LeapIndicator() == LeapNotInSync {
		return ErrLeapNotInSync
	}
	if p.Mode() != ModeServer {
		return fmt.Errorf("%w: mode %d", ErrBadMode, p.Mode())
	}
	if v := p.Version(); v != Version3 && v != Version4 {
		return fmt.Errorf("%w: version %d", ErrBadVersion, v)
	}
	if p.Stratum > MaxStratum {
		return fmt.Errorf("%w: stratum %d", ErrBadStratum, p.Stratum)
	}
	if p.OriginateTime != originate {
		return ErrOriginateMismatch
	}
	return nil
}
