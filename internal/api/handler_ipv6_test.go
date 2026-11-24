package api

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/service"
)

func TestIPv6ConfigsBatchAndBindings(t *testing.T) {
	srv, cp, _ := newControlPlaneTestServer(t)
	endpoint := model.Endpoint{ID: "custom", Port: 2234, AllowProxy: true, AllowSOCKS5: true}
	if err := cp.Engine.InsertEndpoint(endpoint); err != nil {
		t.Fatal(err)
	}
	const path = "/api/v1/ipv6-configs"
	items := []service.IPv6ConfigInput{
		{
			Interface: "eth0", EndpointIDs: []string{endpoint.ID, service.DefaultEndpointID},
			SOCKS5Account: service.SOCKS5Account{Username: "alice", Password: "test1234", Address: "2001:db8:100:2101::5/64"},
		},
		{
			Interface:     "eth0",
			SOCKS5Account: service.SOCKS5Account{Username: "bob", Password: "test5678", Address: "2001:db8:100:2101::6/64"},
		},
	}
	rec := doJSONRequest(t, srv, http.MethodPost, path, map[string]any{"items": items}, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "password") || strings.Contains(rec.Body.String(), "test1234") {
		t.Fatal("create response exposed credentials")
	}
	var created []service.IPv6ConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || len(created) != 2 {
		t.Fatalf("invalid batch response: %v, %v", created, err)
	}

	alice := items[0]
	alice.Password = ""
	alice.EndpointIDs = []string{endpoint.ID}
	alicePath := path + "/" + created[0].ID
	rec = doJSONRequest(t, srv, http.MethodPut, alicePath, alice, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status %d: %s", rec.Code, rec.Body.String())
	}
	configs, err := cp.Engine.ListIPv6Configs()
	if err != nil || len(configs) != 2 ||
		!slices.Equal(configs[0].EndpointIDs, []string{endpoint.ID}) ||
		!configs[0].Account.Authenticate([]byte(items[0].Password)) {
		t.Fatalf("updated bindings or preserved password are invalid: %v", err)
	}

	rec = doJSONRequest(t, srv, http.MethodDelete, "/api/v1/endpoints/"+endpoint.ID, nil, true)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete endpoint status %d: %s", rec.Code, rec.Body.String())
	}
	rec = doJSONRequest(t, srv, http.MethodGet, path, nil, true)
	var listed []service.IPv6ConfigResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || len(listed) != 2 ||
		listed[0].EndpointIDs == nil || len(listed[0].EndpointIDs) != 0 {
		t.Fatalf("expected both configurations to remain after unbinding: %s, %v", rec.Body.String(), err)
	}

	rec = doJSONRequest(t, srv, http.MethodDelete, alicePath, nil, true)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete configuration status %d: %s", rec.Code, rec.Body.String())
	}
}
