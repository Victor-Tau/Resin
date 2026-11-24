//go:build !linux

package localipv6

import (
	"fmt"
	"net/netip"
)

func onLinkPrefixes(int) []netip.Prefix { return nil }

func (m Mapping) Acquire() (func() error, error) {
	return nil, fmt.Errorf("automatic IPv6 address assignment requires Linux")
}
