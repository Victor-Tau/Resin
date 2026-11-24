// Package localipv6 manages source addresses for fixed SOCKS5 endpoints.
package localipv6

import (
	"fmt"
	"net"
	"net/netip"
	"slices"
)

type Mapping struct {
	Interface string
	Address   netip.Addr
}

var globalUnicast = netip.MustParsePrefix("2000::/3")

// ParseMapping accepts an address within the administrator's chosen /64.
// Only the individual address is installed, leaving subnet routes untouched.
func ParseMapping(iface, cidr string) (Mapping, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if iface == "" || err != nil || prefix.Bits() != 64 || !globalUnicast.Contains(prefix.Addr()) {
		return Mapping{}, fmt.Errorf("select an interface and a public IPv6 address with /64")
	}
	return Mapping{Interface: iface, Address: prefix.Addr()}, nil
}

type Interface struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

func Interfaces() ([]Interface, error) {
	links, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	result := make([]Interface, 0, len(links))
	for _, link := range links {
		if link.Flags&net.FlagLoopback != 0 || link.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, err := link.Addrs()
		if err != nil {
			return nil, err
		}
		info := Interface{Name: link.Name, Prefixes: []string{}}
		for _, address := range addresses {
			p, err := netip.ParsePrefix(address.String())
			if err == nil && globalUnicast.Contains(p.Addr()) {
				info.Prefixes = append(info.Prefixes, p.Masked().String())
			}
		}
		slices.Sort(info.Prefixes)
		info.Prefixes = slices.Compact(info.Prefixes)
		result = append(result, info)
	}
	return result, nil
}
