package trace

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// rawProber listens for ICMP on a raw socket and sends either UDP datagrams or
// ICMP echo requests.
type rawProber struct {
	cfg  Config
	recv *icmp.PacketConn // also used for sending in ICMP mode
	send *ipv4.PacketConn // UDP mode only
	udp  net.PacketConn   // kept so Close can close it
	id   int              // ICMP echo identifier
	buf  []byte
}

// NewProber opens the sockets for a live trace. The raw ICMP socket needs
// Administrator or root.
func NewProber(cfg Config) (Prober, error) {
	recv, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, wrapPrivilegeError(err)
	}

	p := &rawProber{
		cfg:  cfg,
		recv: recv,
		id:   os.Getpid() & 0xffff,
		buf:  make([]byte, 1500),
	}

	if cfg.Mode == ModeUDP {
		c, err := net.ListenPacket("udp4", "0.0.0.0:0")
		if err != nil {
			recv.Close()
			return nil, fmt.Errorf("open UDP send socket: %w", err)
		}
		p.udp = c
		p.send = ipv4.NewPacketConn(c)
	}
	return p, nil
}

func (p *rawProber) Probe(ttl, seq int) (Reply, error) {
	start := time.Now()
	deadline := start.Add(p.cfg.Timeout)
	dstPort := DestPort(p.cfg.BasePort, ttl)

	if err := p.sendProbe(ttl, seq, dstPort); err != nil {
		return Reply{}, err
	}

	if err := p.recv.SetReadDeadline(deadline); err != nil {
		return Reply{}, err
	}

	for {
		n, peer, err := p.recv.ReadFrom(p.buf)
		if err != nil {
			var nerr net.Error
			if errors.As(err, &nerr) && nerr.Timeout() {
				return Reply{TimedOut: true}, nil
			}
			return Reply{}, err
		}

		// The raw socket sees all ICMP traffic, so skip anything that isn't a
		// reply to this probe.
		parsed, perr := ParseICMP(p.buf[:n])
		if perr != nil || !parsed.Matches(p.cfg.Mode, p.id, seq, dstPort) {
			continue
		}

		return Reply{
			From:        ipFromAddr(peer),
			RTT:         time.Since(start),
			DestReached: parsed.DestinationReached(p.cfg.Mode),
		}, nil
	}
}

func (p *rawProber) sendProbe(ttl, seq, dstPort int) error {
	if p.cfg.Mode == ModeICMP {
		if err := p.recv.IPv4PacketConn().SetTTL(ttl); err != nil {
			return err
		}
		msg := BuildEchoRequest(p.id, seq, p.cfg.PacketSize)
		b, err := msg.Marshal(nil)
		if err != nil {
			return err
		}
		_, err = p.recv.WriteTo(b, &net.IPAddr{IP: p.cfg.Dest})
		return err
	}

	if err := p.send.SetTTL(ttl); err != nil {
		return err
	}
	payload := BuildPayload(p.cfg.PacketSize)
	_, err := p.send.WriteTo(payload, nil, &net.UDPAddr{IP: p.cfg.Dest, Port: dstPort})
	return err
}

func (p *rawProber) Close() error {
	if p.udp != nil {
		p.udp.Close()
	}
	return p.recv.Close()
}

func ipFromAddr(a net.Addr) net.IP {
	switch v := a.(type) {
	case *net.IPAddr:
		return v.IP
	case *net.UDPAddr:
		return v.IP
	default:
		return nil
	}
}

func wrapPrivilegeError(err error) error {
	if errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("opening raw ICMP socket requires elevated privileges "+
			"(run as Administrator on Windows / root on Unix): %w", err)
	}
	return fmt.Errorf("open raw ICMP socket: %w", err)
}
