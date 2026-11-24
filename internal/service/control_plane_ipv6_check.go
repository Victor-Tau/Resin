package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/Resinat/Resin/internal/localipv6"
	"github.com/Resinat/Resin/internal/probe"
)

type IPv6PrefixCheck struct {
	Address  string `json:"address"`
	Assigned bool   `json:"assigned"`
	Egress   bool   `json:"egress"`
	Stage    string `json:"stage,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (s *ControlPlaneService) CheckIPv6Prefix(ctx context.Context, iface, cidr string) (result IPv6PrefixCheck, err error) {
	mapping, problem := localipv6.ParseMapping(strings.TrimSpace(iface), strings.TrimSpace(cidr))
	if problem != nil {
		return result, invalidArg(problem.Error())
	}
	if !s.ipv6CheckMu.TryLock() {
		return result, conflict("another IPv6 prefix check is running")
	}
	defer s.ipv6CheckMu.Unlock()

	network := netip.PrefixFrom(mapping.Address, 64).Masked()
	for range 3 {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		address := network.Addr().As16()
		if _, err := rand.Read(address[8:]); err != nil {
			return result, internal("generate IPv6 test address", err)
		}
		mapping.Address = netip.AddrFrom16(address)
		if mapping.Address == network.Addr() {
			continue
		}
		result.Address = mapping.Address.String()
		release, err := mapping.Acquire()
		if err != nil {
			result.Stage, result.Error = "assign", err.Error()
			return result, nil
		}
		if release == nil {
			continue // An existing address cannot prove that Resin can install one.
		}
		defer func() {
			if err := release(); err != nil {
				result.Stage, result.Error = "cleanup", err.Error()
			}
		}()
		result.Assigned = true
		if err := verifyIPv6Egress(ctx, mapping.Address); err != nil {
			result.Stage, result.Error = "egress", err.Error()
		} else {
			result.Egress = true
		}
		return result, nil
	}
	result.Stage, result.Error = "assign", "could not find an unused test address"
	return result, nil
}

func verifyIPv6Egress(ctx context.Context, address netip.Addr) error {
	dialer := net.Dialer{LocalAddr: &net.TCPAddr{IP: address.AsSlice()}}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, target string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp6", target)
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   8 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probe.EgressTraceURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IPv6 test server returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return err
	}
	ip, _, err := probe.ParseCloudflareTrace(body)
	if err != nil {
		return err
	}
	if ip != address {
		return fmt.Errorf("test server saw %s instead of %s", ip, address)
	}
	return nil
}
