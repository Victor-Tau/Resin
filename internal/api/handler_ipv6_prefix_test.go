package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/service"
)

func TestIPv6PrefixCheckReportsAssignmentFailureSeparately(t *testing.T) {
	srv, _, _ := newControlPlaneTestServer(t)
	const path = "/api/v1/ipv6-prefixes/check"
	input := map[string]string{"interface": "nonexistent-test-interface", "prefix": "2001:db8:1::/56"}
	got := doJSONRequest(t, srv, http.MethodPost, path, input, true)
	if got.Code != http.StatusBadRequest {
		t.Fatalf("invalid prefix returned %d: %s", got.Code, got.Body.String())
	}
	input["prefix"] = "2001:db8:1::/64"
	got = doJSONRequest(t, srv, http.MethodPost, path, input, true)
	var result service.IPv6PrefixCheck
	if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &result) != nil ||
		result.Assigned || result.Egress || result.Stage != "assign" || result.Address == "" || result.Error == "" {
		t.Fatalf("unexpected assignment check result: %d %s", got.Code, got.Body.String())
	}
}

func TestIPv6PrefixManagementAndProxySelection(t *testing.T) {
	srv, cp, _ := newControlPlaneTestServer(t)
	const prefixPath = "/api/v1/ipv6-prefixes"
	const proxyPath = "/api/v1/ipv6-configs"

	prefix := service.IPv6PrefixInput{Label: " Office ", Interface: "eth0", Prefix: "2001:db8:1234:5601::10/64"}
	rec := doJSONRequest(t, srv, http.MethodPost, prefixPath, prefix, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create prefix status %d: %s", rec.Code, rec.Body.String())
	}
	var saved model.IPv6Prefix
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil || saved.ID == "" ||
		saved.Label != "Office" || saved.Prefix.String() != "2001:db8:1234:5601::/64" {
		t.Fatalf("unexpected saved prefix: %+v, %v", saved, err)
	}

	rec = doJSONRequest(t, srv, http.MethodPost, prefixPath, prefix, true)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate prefix status %d: %s", rec.Code, rec.Body.String())
	}

	proxy := service.IPv6ConfigInput{
		PrefixID:      saved.ID,
		SOCKS5Account: service.SOCKS5Account{Username: "alice", Password: "test1234", Address: "2001:db8:1234:5602::10/64"},
	}
	rec = doJSONRequest(t, srv, http.MethodPost, proxyPath, map[string]any{"items": []service.IPv6ConfigInput{proxy}}, true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range proxy status %d: %s", rec.Code, rec.Body.String())
	}
	proxy.Address = "2001:db8:1234:5601::10"
	rec = doJSONRequest(t, srv, http.MethodPost, proxyPath, map[string]any{"items": []service.IPv6ConfigInput{proxy}}, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create proxy status %d: %s", rec.Code, rec.Body.String())
	}
	var proxies []service.IPv6ConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &proxies); err != nil || len(proxies) != 1 ||
		proxies[0].Interface != "eth0" || proxies[0].PrefixID != saved.ID {
		t.Fatalf("proxy should take its interface from the prefix: %s, %v", rec.Body.String(), err)
	}

	itemPath := prefixPath + "/" + saved.ID
	prefix.Label = "Work"
	rec = doJSONRequest(t, srv, http.MethodPut, itemPath, prefix, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("rename prefix status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doJSONRequest(t, srv, http.MethodGet, prefixPath, nil, true)
	var listed []model.IPv6Prefix
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || len(listed) != 1 ||
		listed[0].Label != "Work" || listed[0].ProxyCount != 1 {
		t.Fatalf("expected renamed prefix and one proxy: %s, %v", rec.Body.String(), err)
	}
	prefix.Prefix = "2001:db8:1234:5602::/64"
	rec = doJSONRequest(t, srv, http.MethodPut, itemPath, prefix, true)
	if rec.Code != http.StatusConflict {
		t.Fatalf("changing an in-use range returned %d: %s", rec.Code, rec.Body.String())
	}

	rec = doJSONRequest(t, srv, http.MethodDelete, itemPath, nil, true)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete prefix status %d: %s", rec.Code, rec.Body.String())
	}
	configs, err := cp.Engine.ListIPv6Configs()
	if err != nil || len(configs) != 1 || configs[0].PrefixID != "" || configs[0].Account.Address.Addr().String() != proxy.Address ||
		configs[0].Account.Interface != "eth0" || !configs[0].Account.Authenticate([]byte(proxy.Password)) {
		t.Fatalf("proxy should keep its address, interface and credentials: %v", err)
	}
}
