package scanner

import (
	"fmt"
	"net"
	"strings"
)

// ParseHosts expands a comma-separated target list into hosts, dropping
// duplicates but otherwise keeping the order given. Each item can be:
//
//	"localhost"          -> ["localhost"]                 (plain host or IP, as-is)
//	"localhost,127.0.0.1"-> ["localhost", "127.0.0.1"]
//	"192.168.1.*"        -> 192.168.1.1 .. 192.168.1.254  (IPv4 wildcard, last octet)
//	"192.168.1.0/30"     -> usable host addresses in the CIDR block
//
// CIDR blocks bigger than a /24 are an error unless allowLarge is set, so a
// typo can't turn into a mass scan.
func ParseHosts(spec string, allowLarge bool) ([]string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, fmt.Errorf("empty host specification")
	}

	var hosts []string
	seen := make(map[string]struct{})
	add := func(h string) {
		if _, ok := seen[h]; ok {
			return
		}
		seen[h] = struct{}{}
		hosts = append(hosts, h)
	}

	for _, raw := range strings.Split(spec, ",") {
		tok := strings.TrimSpace(raw)
		if tok == "" {
			return nil, fmt.Errorf("empty host token in %q", spec)
		}

		expanded := []string{tok}
		var err error
		switch {
		case strings.Contains(tok, "/"):
			expanded, err = expandCIDR(tok, allowLarge)
		case strings.Contains(tok, "*"):
			expanded, err = expandWildcard(tok)
		}
		if err != nil {
			return nil, err
		}
		for _, h := range expanded {
			add(h)
		}
	}

	return hosts, nil
}

// expandWildcard turns "a.b.c.*" into a.b.c.1 through a.b.c.254.
func expandWildcard(tok string) ([]string, error) {
	octets := strings.Split(tok, ".")
	if len(octets) != 4 || octets[3] != "*" {
		return nil, fmt.Errorf("invalid wildcard %q: expected a.b.c.* (IPv4, last octet only)", tok)
	}
	prefix := strings.Join(octets[:3], ".")
	if net.ParseIP(prefix+".0") == nil {
		return nil, fmt.Errorf("invalid wildcard %q: %q is not a valid IPv4 prefix", tok, prefix)
	}
	hosts := make([]string, 0, 254)
	for i := 1; i <= 254; i++ {
		hosts = append(hosts, fmt.Sprintf("%s.%d", prefix, i))
	}
	return hosts, nil
}

// expandCIDR lists the host addresses in an IPv4 CIDR block, leaving out the
// network and broadcast addresses except for /31 and /32.
func expandCIDR(tok string, allowLarge bool) ([]string, error) {
	ip, ipNet, err := net.ParseCIDR(tok)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %q: %v", tok, err)
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("invalid CIDR %q: only IPv4 CIDR blocks are supported", tok)
	}

	ones, bits := ipNet.Mask.Size()
	total := 1 << uint(bits-ones)
	if total > 256 && !allowLarge {
		return nil, fmt.Errorf("CIDR %q expands to %d addresses (larger than /24); pass -allow-large to scan it", tok, total)
	}

	var all []string
	for cur := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(cur); cur = nextIP(cur) {
		all = append(all, cur.String())
	}

	if ones <= 30 && len(all) >= 2 {
		all = all[1 : len(all)-1]
	}
	return all, nil
}

func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
}
