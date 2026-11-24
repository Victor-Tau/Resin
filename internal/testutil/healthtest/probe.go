package healthtest

import (
	"net/netip"

	"github.com/Resinat/Resin/internal/node"
)

// ProbeEgressOK supplies a verified egress sample for a test node.
func ProbeEgressOK(pool interface {
	GetEntry(node.Hash) (*node.NodeEntry, bool)
	ApplyFeedback(node.Hash, uint64, node.Feedback) bool
}, hash node.Hash) {
	entry, ok := pool.GetEntry(hash)
	if !ok {
		panic("missing test node")
	}
	ip := entry.GetEgressIP()
	if !ip.IsValid() {
		ip = netip.MustParseAddr("2001:db8::1")
	}
	success := true
	pool.ApplyFeedback(hash, entry.RoutingVersion(), node.Feedback{Result: &success, Egress: true, IP: &ip})
}
