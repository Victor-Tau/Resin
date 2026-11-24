package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/service"
)

func TestHandleListEndpointsPagination(t *testing.T) {
	srv, cp, _ := newControlPlaneTestServer(t)
	custom := []model.Endpoint{
		{ID: "endpoint-c", Port: 32003, AllowManagement: true},
		{ID: "endpoint-a", Port: 32001, AllowManagement: true},
		{ID: "endpoint-b", Port: 32002, AllowManagement: true},
	}
	for _, endpoint := range custom {
		if err := cp.Engine.InsertEndpoint(endpoint); err != nil {
			t.Fatalf("InsertEndpoint(%s): %v", endpoint.ID, err)
		}
	}

	firstRec := doJSONRequest(t, srv, http.MethodGet, "/api/v1/endpoints?limit=2&offset=0", nil, true)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first page status: got %d, want %d, body=%s", firstRec.Code, http.StatusOK, firstRec.Body.String())
	}
	var first PageResponse[service.EndpointResponse]
	if err := json.Unmarshal(firstRec.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if first.Total != 4 || first.Limit != 2 || first.Offset != 0 {
		t.Fatalf("first page metadata = total:%d limit:%d offset:%d, want 4/2/0", first.Total, first.Limit, first.Offset)
	}
	if len(first.Items) != 2 || first.Items[0].ID != service.DefaultEndpointID || first.Items[1].Port != 32001 {
		t.Fatalf("first page items = %+v, want default endpoint followed by port 32001", first.Items)
	}

	secondRec := doJSONRequest(t, srv, http.MethodGet, "/api/v1/endpoints?limit=2&offset=2", nil, true)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("second page status: got %d, want %d, body=%s", secondRec.Code, http.StatusOK, secondRec.Body.String())
	}
	var second PageResponse[service.EndpointResponse]
	if err := json.Unmarshal(secondRec.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if second.Total != 4 || second.Limit != 2 || second.Offset != 2 {
		t.Fatalf("second page metadata = total:%d limit:%d offset:%d, want 4/2/2", second.Total, second.Limit, second.Offset)
	}
	if len(second.Items) != 2 || second.Items[0].Port != 32002 || second.Items[1].Port != 32003 {
		t.Fatalf("second page items = %+v, want ports 32002 and 32003", second.Items)
	}
}

func TestHandleListEndpointsRejectsInvalidPagination(t *testing.T) {
	srv, _, _ := newControlPlaneTestServer(t)
	rec := doJSONRequest(t, srv, http.MethodGet, "/api/v1/endpoints?limit=-1", nil, true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertErrorCode(t, rec, "INVALID_ARGUMENT")
}

func TestHandleCreateDisabledEndpoint(t *testing.T) {
	srv, cp, _ := newControlPlaneTestServer(t)
	rec := doJSONRequest(t, srv, http.MethodPost, "/api/v1/endpoints", map[string]any{
		"port":    32009,
		"enabled": false,
	}, true)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var created service.EndpointResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Enabled || created.Status != "inactive" {
		t.Fatalf("created endpoint = %+v", created)
	}
	persisted, err := cp.Engine.GetEndpoint(created.ID)
	if err != nil {
		t.Fatalf("GetEndpoint: %v", err)
	}
	if persisted.Enabled {
		t.Fatal("disabled create state was not persisted")
	}
}

func TestHandleEndpointEnableAndDisableWithPatch(t *testing.T) {
	srv, cp, _ := newControlPlaneTestServer(t)
	created, err := cp.CreateEndpoint(service.CreateEndpointRequest{Port: 32010})
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}

	endpointPath := "/api/v1/endpoints/" + created.ID
	rec := doJSONRequest(t, srv, http.MethodPatch, endpointPath, map[string]any{"enabled": false}, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable status: got %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var disabled service.EndpointResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &disabled); err != nil {
		t.Fatalf("decode disable response: %v", err)
	}
	if disabled.Enabled || disabled.Status != "inactive" {
		t.Fatalf("disable response = %+v", disabled)
	}

	rec = doJSONRequest(t, srv, http.MethodPatch, endpointPath, map[string]any{"enabled": true}, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("start status: got %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var started service.EndpointResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode start response: %v", err)
	}
	if !started.Enabled {
		t.Fatalf("start response = %+v", started)
	}

	rec = doJSONRequest(t, srv, http.MethodPatch, "/api/v1/endpoints/default", map[string]any{"enabled": false}, true)
	if rec.Code != http.StatusConflict {
		t.Fatalf("disable default status: got %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	assertErrorCode(t, rec, "CONFLICT")

	rec = doJSONRequest(t, srv, http.MethodPatch, "/api/v1/endpoints/missing", map[string]any{"enabled": true}, true)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("start missing status: got %d, want %d, body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	assertErrorCode(t, rec, "NOT_FOUND")
}

func TestEndpointIPv6AccountsPersistWithoutExposingCredentials(t *testing.T) {
	srv, cp, _ := newControlPlaneTestServer(t)
	create := doJSONRequest(t, srv, http.MethodPost, "/api/v1/endpoints", map[string]any{
		"port": 2234, "enabled": false, "socks5_ipv6_interface": "eth0",
		"socks5_accounts": []map[string]string{{"username": "alice", "password": "secret-one", "address": "2001:db8:100:2101::5/64"}},
	}, true)
	if create.Code != 201 {
		t.Fatalf("create %d %s", create.Code, create.Body.String())
	}
	var response service.EndpointResponse
	if err := json.Unmarshal(create.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(create.Body.String(), "password") || strings.Contains(create.Body.String(), "secret-one") {
		t.Fatal("response exposed credentials")
	}
	path := "/api/v1/endpoints/" + response.ID
	rows := []map[string]string{{"username": "alice", "address": "2001:db8:100:2102::5/64"}, {"username": "bob", "password": "secret-two", "address": "2001:db8:100:2101::5/64"}}
	patch := doJSONRequest(t, srv, http.MethodPatch, path, map[string]any{"socks5_accounts": rows}, true)
	if patch.Code != 200 {
		t.Fatalf("patch %d %s", patch.Code, patch.Body.String())
	}
	saved, err := cp.Engine.GetEndpoint(response.ID)
	if err != nil {
		t.Fatal(err)
	}
	alice, bob := saved.SOCKS5Accounts["alice"], saved.SOCKS5Accounts["bob"]
	if !alice.Authenticate([]byte("secret-one")) || alice.Authenticate([]byte("secret-two")) || !bob.Authenticate([]byte("secret-two")) || alice.Address.String() != rows[0]["address"] {
		t.Fatal("stored account mapping or password mismatch")
	}
	rows[1]["address"] = rows[0]["address"]
	invalid := doJSONRequest(t, srv, http.MethodPatch, path, map[string]any{"socks5_accounts": rows}, true)
	if invalid.Code != 400 {
		t.Fatalf("duplicate source returned %d", invalid.Code)
	}
}
