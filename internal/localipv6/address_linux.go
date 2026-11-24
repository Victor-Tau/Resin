package localipv6

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/sagernet/netlink"
	"golang.org/x/sys/unix"
)

func onLinkPrefixes(index int) []netip.Prefix {
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V6, &netlink.Route{LinkIndex: index}, netlink.RT_FILTER_OIF)
	if err != nil {
		return nil // Interface addresses remain available when route enumeration is restricted.
	}
	var prefixes []netip.Prefix
	for _, route := range routes {
		if route.Dst == nil || len(route.Gw) != 0 || route.Type != unix.RTN_UNICAST {
			continue
		}
		prefix, err := netip.ParsePrefix(route.Dst.String())
		if err == nil {
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}

// Acquire installs one /128 and waits for duplicate-address detection.
// A non-nil release function removes an address created by this call.
func (m Mapping) Acquire() (func() error, error) {
	link, err := netlink.LinkByName(m.Interface)
	if err != nil {
		return nil, err
	}
	wanted := &netlink.Addr{IPNet: &net.IPNet{IP: net.IP(m.Address.AsSlice()), Mask: net.CIDRMask(128, 128)}}
	state := func() (bool, bool, error) {
		addresses, err := netlink.AddrList(link, netlink.FAMILY_V6)
		if err != nil {
			return false, false, err
		}
		for _, a := range addresses {
			if a.IP.Equal(wanted.IP) {
				if a.Flags&unix.IFA_F_DADFAILED != 0 {
					return true, false, fmt.Errorf("duplicate IPv6 address %s on %s", m.Address, m.Interface)
				}
				return true, a.Flags&unix.IFA_F_TENTATIVE == 0, nil
			}
		}
		return false, false, nil
	}
	found, ready, err := state()
	if err != nil || ready {
		return nil, err
	}
	var release func() error
	if !found {
		err := netlink.AddrAdd(link, wanted)
		if err == nil {
			release = func() error { return netlink.AddrDel(link, wanted) }
		} else if !errors.Is(err, unix.EEXIST) {
			return nil, fmt.Errorf("add %s to %s (requires CAP_NET_ADMIN): %w", m.Address, m.Interface, err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, ready, err = state()
		if ready {
			return release, nil
		}
		if err != nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err == nil {
		err = fmt.Errorf("IPv6 address %s on %s did not become ready", m.Address, m.Interface)
	}
	if release != nil {
		err = errors.Join(err, release())
	}
	return nil, err
}
