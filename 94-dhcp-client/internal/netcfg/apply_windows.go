//go:build windows

// Package netcfg configures the host interface with a leased address. Only
// --live --apply uses it; tests never do.
package netcfg

import (
	"fmt"
	"net"
	"os/exec"
)

// Apply assigns the leased address statically using netsh. Requires admin.
func Apply(iface string, ip, mask, gateway net.IP) error {
	args := []string{"interface", "ip", "set", "address",
		fmt.Sprintf("name=%s", iface), "static", ip.String(), mask.String()}
	if gateway != nil {
		args = append(args, gateway.String())
	}
	if out, err := exec.Command("netsh", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("netsh set address: %w: %s", err, out)
	}
	return nil
}

// Release reverts the interface to DHCP-managed addressing.
func Release(iface string) error {
	out, err := exec.Command("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=%s", iface), "dhcp").CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh release: %w: %s", err, out)
	}
	return nil
}
