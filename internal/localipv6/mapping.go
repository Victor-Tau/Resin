// Package localipv6 manages source addresses for fixed SOCKS5 endpoints.
package localipv6

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strings"
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

// ParseProxyMapping accepts a host address and the legacy /64 notation.
func ParseProxyMapping(iface, address string) (Mapping, error) {
	address = strings.TrimSpace(address)
	if !strings.Contains(address, "/") {
		address += "/64"
	}
	mapping, err := ParseMapping(iface, address)
	if err != nil {
		return Mapping{}, fmt.Errorf("select an interface and a public IPv6 address")
	}
	return mapping, nil
}

type Interface struct {
	Name       string      `json:"name"`
	Prefixes   []string    `json:"prefixes"`
	Candidates []Candidate `json:"candidates"`
}

type Candidate struct {
	Prefix netip.Prefix `json:"prefix"`
	Parent netip.Prefix `json:"parent"`
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
		var networks []netip.Prefix
		for _, address := range addresses {
			p, err := netip.ParsePrefix(address.String())
			if err == nil && globalUnicast.Contains(p.Addr()) {
				info.Prefixes = append(info.Prefixes, p.Masked().String())
				networks = append(networks, p)
			}
		}
		networks = append(networks, onLinkPrefixes(link.Index)...)
		info.Candidates = candidatesForNetworks(networks)
		slices.Sort(info.Prefixes)
		info.Prefixes = slices.Compact(info.Prefixes)
		result = append(result, info)
	}
	return result, nil
}

func candidatesForNetworks(networks []netip.Prefix) []Candidate {
	byPrefix := make(map[netip.Prefix]netip.Prefix)
	for _, network := range networks {
		parent := network.Masked()
		if parent.Bits() < globalUnicast.Bits() || parent.Bits() > 64 || !globalUnicast.Contains(parent.Addr()) {
			continue
		}
		candidateCount := 1
		first := netip.PrefixFrom(network.Addr(), 64).Masked()
		// Expand /56 and narrower networks completely. For larger networks
		// offer the /64 containing the interface address or route start.
		if parent.Bits() >= 56 {
			candidateCount = 1 << (64 - parent.Bits())
			first = netip.PrefixFrom(parent.Addr(), 64)
		}
		addr := first.Addr().As16()
		start := binary.BigEndian.Uint64(addr[:8])
		for i := 0; i < candidateCount; i++ {
			binary.BigEndian.PutUint64(addr[:8], start+uint64(i))
			prefix := netip.PrefixFrom(netip.AddrFrom16(addr), 64)
			if previous, exists := byPrefix[prefix]; !exists || parent.Bits() > previous.Bits() {
				byPrefix[prefix] = parent
			}
		}
	}
	result := make([]Candidate, 0, len(byPrefix))
	for prefix, parent := range byPrefix {
		result = append(result, Candidate{Prefix: prefix, Parent: parent})
	}
	slices.SortFunc(result, func(a, b Candidate) int { return a.Prefix.Addr().Compare(b.Prefix.Addr()) })
	return result
}
