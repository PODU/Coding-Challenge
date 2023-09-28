package ntp

import (
	"encoding/binary"
	"errors"
	"testing"
)

// refID packs a 4-char ASCII code into a Reference ID value.
func refID(code string) uint32 {
	var b [4]byte
	copy(b[:], code)
	return binary.BigEndian.Uint32(b[:])
}

// makePacket builds a server-mode packet for validation tests.
func makePacket(li, vn, mode, stratum uint8) *Packet {
	return &Packet{
		LIVNMode: packLIVNMode(li, vn, mode),
		Stratum:  stratum,
	}
}

func TestKissOfDeath(t *testing.T) {
	for _, code := range []string{"DENY", "RSTR", "RATE"} {
		p := makePacket(0, 4, 4, 0) // stratum 0 = KoD
		p.ReferenceID = refID(code)

		if !p.IsKissOfDeath() {
			t.Errorf("%s: IsKissOfDeath = false, want true", code)
		}
		if got := p.KissCode(); got != code {
			t.Errorf("KissCode = %q, want %q", got, code)
		}

		err := p.Validate(0)
		var kod *KissOfDeathError
		if !errors.As(err, &kod) {
			t.Fatalf("%s: Validate err = %v, want *KissOfDeathError", code, err)
		}
		if kod.Code != code {
			t.Errorf("KoD code = %q, want %q", kod.Code, code)
		}
	}
}

func TestLeapNotInSync(t *testing.T) {
	p := makePacket(LeapNotInSync, 4, 4, 2)
	if err := p.Validate(0); !errors.Is(err, ErrLeapNotInSync) {
		t.Fatalf("Validate err = %v, want ErrLeapNotInSync", err)
	}
}

func TestValidateModeVersionStratum(t *testing.T) {
	cases := []struct {
		name              string
		li, vn, mode, str uint8
		wantErr           error
	}{
		{"ok-v4", 0, 4, 4, 2, nil},
		{"ok-v3", 0, 3, 4, 2, nil},
		{"bad-mode-client", 0, 4, 3, 2, ErrBadMode},
		{"bad-mode-zero", 0, 4, 0, 2, ErrBadMode},
		{"bad-version-2", 0, 2, 4, 2, ErrBadVersion},
		{"bad-version-5", 0, 5, 4, 2, ErrBadVersion},
		{"bad-stratum-16", 0, 4, 4, 16, ErrBadStratum},
		{"bad-stratum-200", 0, 4, 4, 200, ErrBadStratum},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := makePacket(c.li, c.vn, c.mode, c.str)
			err := p.Validate(0) // originate 0 matches default
			if c.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate err = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("Validate err = %v, want %v", err, c.wantErr)
			}
		})
	}
}

func TestOriginateMismatch(t *testing.T) {
	p := makePacket(0, 4, 4, 2)
	p.OriginateTime = 0x1111111100000000

	if err := p.Validate(0x2222222200000000); !errors.Is(err, ErrOriginateMismatch) {
		t.Fatalf("Validate err = %v, want ErrOriginateMismatch", err)
	}
	if err := p.Validate(0x1111111100000000); err != nil {
		t.Fatalf("Validate (matching originate) err = %v, want nil", err)
	}
}
