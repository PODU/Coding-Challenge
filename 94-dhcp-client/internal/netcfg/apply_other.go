//go:build !windows

// Package netcfg configures the host interface with a leased address. Only
// --live --apply uses it; tests never do.
package netcfg

import (
	"fmt"
	"net"
	"os/exec"
)

// Apply assigns the leased address using iproute2. Requires root.
func Apply(iface string, ip, mask, gateway net.IP) error {
	prefix, _ := net.IPMask(mask.To4()).Size()
	cidr := fmt.Sprintf("%s/%d", ip.String(), prefix)
	if out, err := exec.Command("ip", "addr", "add", cidr, "dev", iface).CombinedOutput(); err != nil {
		return fmt.Errorf("ip addr add: %w: %s", err, out)
	}
	if gateway != nil {
		if out, err := exec.Command("ip", "route", "add", "default", "via", gateway.String(), "dev", iface).CombinedOutput(); err != nil {
			return fmt.Errorf("ip route add default: %w: %s", err, out)
		}
	}
	return nil
}

// Release removes addresses from the interface.
func Release(iface string) error {
	if out, err := exec.Command("ip", "addr", "flush", "dev", iface).CombinedOutput(); err != nil {
		return fmt.Errorf("ip addr flush: %w: %s", err, out)
	}
	return nil
}
