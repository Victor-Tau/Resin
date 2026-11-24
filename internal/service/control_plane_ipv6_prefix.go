package service

import (
	"errors"
	"net/netip"
	"strings"
	"unicode/utf8"

	"github.com/Resinat/Resin/internal/localipv6"
	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/state"
	"github.com/google/uuid"
)

type IPv6PrefixInput struct {
	Label     string `json:"label"`
	Interface string `json:"interface"`
	Prefix    string `json:"prefix"`
}

func (s *ControlPlaneService) ListIPv6Prefixes() ([]model.IPv6Prefix, error) {
	if s == nil || s.Engine == nil {
		return nil, internal("IPv6 configuration service is not initialized", nil)
	}
	s.endpointMu.RLock()
	defer s.endpointMu.RUnlock()

	prefixes, err := s.Engine.ListIPv6Prefixes()
	if err != nil {
		return nil, internal("list IPv6 prefixes", err)
	}
	return prefixes, nil
}

func (s *ControlPlaneService) SaveIPv6Prefix(id string, input IPv6PrefixInput) (model.IPv6Prefix, error) {
	var prefix model.IPv6Prefix
	if s == nil || s.Engine == nil {
		return prefix, internal("IPv6 configuration service is not initialized", nil)
	}
	label := strings.TrimSpace(input.Label)
	if label == "" || utf8.RuneCountInString(label) > 60 {
		return prefix, invalidArg("IPv6 prefix label must be 1–60 characters")
	}
	mapping, err := localipv6.ParseMapping(strings.TrimSpace(input.Interface), strings.TrimSpace(input.Prefix))
	if err != nil {
		return prefix, invalidArg(err.Error())
	}
	network := netip.PrefixFrom(mapping.Address, 64).Masked()

	s.endpointMu.Lock()
	defer s.endpointMu.Unlock()

	if id == "" {
		prefix.ID = uuid.NewString()
	} else {
		prefix, err = s.Engine.GetIPv6Prefix(id)
		if errors.Is(err, state.ErrNotFound) {
			return prefix, notFound("IPv6 prefix not found")
		}
		if err != nil {
			return prefix, internal("read IPv6 prefix", err)
		}
		if prefix.ProxyCount > 0 && (prefix.Prefix != network || prefix.Interface != mapping.Interface) {
			return prefix, conflict("only the label can change while proxies use this IPv6 prefix")
		}
	}
	prefix.Label, prefix.Interface, prefix.Prefix = label, mapping.Interface, network
	if err := s.Engine.SaveIPv6Prefix(prefix); err != nil {
		if errors.Is(err, state.ErrConflict) {
			return prefix, conflict("IPv6 prefix already saved on this interface")
		}
		return prefix, internal("save IPv6 prefix", err)
	}
	return prefix, nil
}

func (s *ControlPlaneService) DeleteIPv6Prefix(id string) error {
	if s == nil || s.Engine == nil {
		return internal("IPv6 configuration service is not initialized", nil)
	}
	s.endpointMu.Lock()
	defer s.endpointMu.Unlock()

	if err := s.Engine.DeleteIPv6Prefix(id); err != nil {
		if errors.Is(err, state.ErrNotFound) {
			return notFound("IPv6 prefix not found")
		}
		return internal("delete IPv6 prefix", err)
	}
	return nil
}
