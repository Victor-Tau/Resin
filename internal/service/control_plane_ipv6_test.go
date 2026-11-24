package service

import "testing"

func TestIPv6ConfigBatchRollsBackOnEndpointFailure(t *testing.T) {
	cp, runtime := newEndpointTestService(t, nil)
	endpoint, err := cp.CreateEndpoint(CreateEndpointRequest{Port: 2234})
	if err != nil {
		t.Fatal(err)
	}
	input := IPv6ConfigInput{
		Interface: "eth0", EndpointIDs: []string{DefaultEndpointID, endpoint.ID},
		SOCKS5Account: SOCKS5Account{Username: "alice", Password: "test1234", Address: "2001:db8:100:2101::5/64"},
	}

	runtime.failPort = endpoint.Port
	if _, err := cp.CreateIPv6Configs([]IPv6ConfigInput{input}); err == nil {
		t.Fatal("expected endpoint activation to fail")
	}
	configs, err := cp.ListIPv6Configs()
	if err != nil || len(configs) != 0 {
		t.Fatalf("failed batch was persisted: %v, %v", configs, err)
	}
	if len(runtime.endpoints[DefaultEndpointID].SOCKS5Accounts) != 0 {
		t.Fatal("failed batch left credentials active on the default endpoint")
	}

	runtime.failPort = 0
	if _, err := cp.CreateIPv6Configs([]IPv6ConfigInput{input}); err != nil {
		t.Fatal(err)
	}
	for _, id := range input.EndpointIDs {
		account := runtime.endpoints[id].SOCKS5Accounts[input.Username]
		if account.Interface != input.Interface || account.Address.String() != input.Address {
			t.Fatalf("endpoint %s did not receive the new address", id)
		}
	}
	if _, err := cp.UpdateEndpoint(endpoint.ID, []byte(`{"allow_socks5":false}`)); err != nil {
		t.Fatal(err)
	}
	if len(runtime.endpoints[endpoint.ID].SOCKS5Accounts) != 0 {
		t.Fatal("disabled SOCKS5 retained active credentials")
	}
	if _, err := cp.UpdateEndpoint(endpoint.ID, []byte(`{"allow_socks5":true}`)); err != nil {
		t.Fatal(err)
	}
	if len(runtime.endpoints[endpoint.ID].SOCKS5Accounts) != 1 {
		t.Fatal("enabling SOCKS5 did not restore its binding")
	}
}
