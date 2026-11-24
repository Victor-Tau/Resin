package localipv6

import (
	"net/netip"
	"testing"
)

func TestCandidatesExpand56AndPreferMoreSpecificNetwork(t *testing.T) {
	broader := netip.MustParsePrefix("2001:db8:1:1e83::2/56")
	exact := netip.MustParsePrefix("2001:db8:1:1e83::/64")
	candidates := candidatesForNetworks([]netip.Prefix{
		broader, broader.Masked(), exact,
		netip.MustParsePrefix("2001:db8:3::2/128"),
		netip.MustParsePrefix("fe80::1/64"),
	})
	if len(candidates) != 256 {
		t.Fatalf("expected 256 candidates, got %d", len(candidates))
	}
	if candidates[0].Prefix.String() != "2001:db8:1:1e00::/64" ||
		candidates[255].Prefix.String() != "2001:db8:1:1eff::/64" {
		t.Fatalf("unexpected first or last candidate: %v, %v", candidates[0], candidates[255])
	}
	if candidates[0x83] != (Candidate{Prefix: exact, Parent: exact}) {
		t.Fatalf("expected the more specific network as parent: %v", candidates[0x83])
	}
}

func TestCandidatesDoNotEnumerateBroadNetwork(t *testing.T) {
	candidates := candidatesForNetworks([]netip.Prefix{netip.MustParsePrefix("2001:db8:1:abcd::2/48")})
	if len(candidates) != 1 || candidates[0].Prefix.String() != "2001:db8:1:abcd::/64" ||
		candidates[0].Parent.String() != "2001:db8:1::/48" {
		t.Fatalf("unexpected broad network candidates: %v", candidates)
	}
}
