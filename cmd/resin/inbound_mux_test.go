package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/model"
	"github.com/Resinat/Resin/internal/proxy"
	"github.com/Resinat/Resin/internal/service"
)

func newInboundMux(proxyToken string, forward, reverse, apiHandler, tokenActionHandler http.Handler) http.Handler {
	return newEndpointInboundMux(
		func() model.Endpoint { return service.NewDefaultEndpoint(0) },
		proxyToken,
		forward,
		reverse,
		apiHandler,
		tokenActionHandler,
	)
}

func tagHandler(tag string, status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Route", tag)
		w.WriteHeader(status)
	})
}

func TestInboundMux_PriorityForwardConnect(t *testing.T) {
	mux := newInboundMux(
		"tok",
		tagHandler("forward", http.StatusOK),
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	req := httptest.NewRequest(http.MethodConnect, "http://example.com:443", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Header().Get("X-Route") != "forward" {
		t.Fatalf("expected forward route, got %q", rec.Header().Get("X-Route"))
	}
}

func TestInboundMux_PriorityForwardAbsoluteURI(t *testing.T) {
	mux := newInboundMux(
		"tok",
		tagHandler("forward", http.StatusOK),
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/v1/ping", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Header().Get("X-Route") != "forward" {
		t.Fatalf("expected forward route, got %q", rec.Header().Get("X-Route"))
	}
}

func TestInboundMux_RoutesTokenAPINamespaceToTokenActionHandler(t *testing.T) {
	mux := newInboundMux(
		"tok",
		tagHandler("forward", http.StatusOK),
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	for _, path := range []string{"/tok/api", "/tok/api/v1/platforms"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Header().Get("X-Route") != "token-action" {
				t.Fatalf("expected token-action route, got %q", rec.Header().Get("X-Route"))
			}
		})
	}
}

func TestInboundMux_RoutesAPIForControlPlanePaths(t *testing.T) {
	mux := newInboundMux(
		"tok",
		tagHandler("forward", http.StatusOK),
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	cases := []string{
		"/",
		"/healthz",
		"/api",
		"/api/v1/system/info",
		"/ui",
		"/ui/",
		"/ui/platforms/demo",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Header().Get("X-Route") != "api" {
				t.Fatalf("expected api route, got %q", rec.Header().Get("X-Route"))
			}
		})
	}
}

func TestInboundMux_RoutesReverseForNonControlPlanePaths(t *testing.T) {
	mux := newInboundMux(
		"tok",
		tagHandler("forward", http.StatusOK),
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	cases := []string{
		"/tok/plat:acct/https/example.com/path",
		"/tok/plat/https/example.com/path",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Header().Get("X-Route") != "reverse" {
				t.Fatalf("expected reverse route, got %q", rec.Header().Get("X-Route"))
			}
		})
	}
}

func TestInboundMux_RejectsReverseWhenTokenMissingOrWrong(t *testing.T) {
	mux := newInboundMux(
		"tok",
		tagHandler("forward", http.StatusOK),
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	cases := []string{
		"/dashboard",
		"/wrong/plat:acct/https/example.com/path",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status: got %d, want %d", rec.Code, http.StatusForbidden)
			}
			if got := rec.Header().Get("X-Resin-Error"); got != "AUTH_FAILED" {
				t.Fatalf("X-Resin-Error: got %q, want %q", got, "AUTH_FAILED")
			}
			if got := rec.Header().Get("X-Route"); got != "" {
				t.Fatalf("expected no downstream handler route, got %q", got)
			}
		})
	}
}

func TestInboundMux_EmptyProxyToken_AllowsDummyTokenForTokenAction(t *testing.T) {
	mux := newInboundMux(
		"",
		tagHandler("forward", http.StatusOK),
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	req := httptest.NewRequest(http.MethodPost, "/any-dummy-token/api/v1/Default/actions/inherit-lease", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Header().Get("X-Route") != "token-action" {
		t.Fatalf("expected token-action route when proxy token empty, got %q", rec.Header().Get("X-Route"))
	}
}

func TestInboundMux_EmptyProxyToken_AllowsEmptyTokenSegmentForTokenAction(t *testing.T) {
	mux := newInboundMux(
		"",
		tagHandler("forward", http.StatusOK),
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	req := httptest.NewRequest(http.MethodPost, "//api/v1/Default/actions/inherit-lease", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Header().Get("X-Route") != "token-action" {
		t.Fatalf("expected token-action route with empty token segment, got %q", rec.Header().Get("X-Route"))
	}
}

func TestInboundMux_EmptyProxyToken_RoutesNonActionTokenNamespaceToTokenAction(t *testing.T) {
	mux := newInboundMux(
		"",
		tagHandler("forward", http.StatusOK),
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	req := httptest.NewRequest(http.MethodGet, "/any-token/api/v1/system/info", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Header().Get("X-Route") != "token-action" {
		t.Fatalf("expected token-action route for non-action token namespace, got %q", rec.Header().Get("X-Route"))
	}
}

func TestInboundMux_RoutesTokenInheritLeaseAction(t *testing.T) {
	mux := newInboundMux(
		"tok",
		tagHandler("forward", http.StatusOK),
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	req := httptest.NewRequest(http.MethodPost, "/tok/api/v1/Default/actions/inherit-lease", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Header().Get("X-Route") != "token-action" {
		t.Fatalf("expected token-action route, got %q", rec.Header().Get("X-Route"))
	}
}

func TestEndpointInboundMux_AppliesCapabilities(t *testing.T) {
	endpoint := model.Endpoint{
		AllowProxy:       true,
		AllowHTTPForward: false,
		AllowHTTPReverse: false,
	}
	mux := newEndpointInboundMux(
		func() model.Endpoint { return endpoint },
		"",
		tagHandler("forward", http.StatusOK),
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	t.Run("health remains available", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if rec.Header().Get("X-Route") != "api" {
			t.Fatalf("health route = %q, want api", rec.Header().Get("X-Route"))
		}
	})

	t.Run("management is hidden", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/endpoints", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("forward is forbidden", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://example.com/", nil))
		if rec.Code != http.StatusForbidden || rec.Header().Get("X-Resin-Error") != "ENDPOINT_CAPABILITY_DISABLED" {
			t.Fatalf("status/header = %d/%q", rec.Code, rec.Header().Get("X-Resin-Error"))
		}
	})

	t.Run("reverse is forbidden", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dummy/Default/https/example.com", nil))
		if rec.Code != http.StatusForbidden || rec.Header().Get("X-Resin-Error") != "ENDPOINT_CAPABILITY_DISABLED" {
			t.Fatalf("status/header = %d/%q", rec.Code, rec.Header().Get("X-Resin-Error"))
		}
	})

	endpoint.AllowManagement = true
	endpoint.AllowHTTPReverse = true
	t.Run("updated capabilities apply without rebuilding mux", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/endpoints", nil))
		if rec.Header().Get("X-Route") != "api" {
			t.Fatalf("management route = %q, want api", rec.Header().Get("X-Route"))
		}

		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dummy/Default/https/example.com", nil))
		if rec.Header().Get("X-Route") != "reverse" {
			t.Fatalf("reverse route = %q, want reverse", rec.Header().Get("X-Route"))
		}
	})
}

func TestEndpointInboundMux_InjectsProxyAuthInfoPolicy(t *testing.T) {
	seenRequired := false
	forward := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenRequired = proxy.InboundPolicyFromContext(r.Context()).RequireProxyAuthInfo
		w.WriteHeader(http.StatusNoContent)
	})
	mux := newEndpointInboundMux(
		func() model.Endpoint {
			return model.Endpoint{
				AllowProxy:           true,
				AllowHTTPForward:     true,
				RequireProxyAuthInfo: true,
			}
		},
		"",
		forward,
		tagHandler("reverse", http.StatusOK),
		tagHandler("api", http.StatusOK),
		tagHandler("token-action", http.StatusOK),
	)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://example.com/", nil))
	if rec.Code != http.StatusNoContent || !seenRequired {
		t.Fatalf("status/required = %d/%v, want 204/true", rec.Code, seenRequired)
	}
}

func TestEndpointInboundMux_HTTPForwardWithIPv6Account(t *testing.T) {
	upstreamHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") != "" {
			t.Error("proxy credentials reached the origin")
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			t.Errorf("read remote address: %v", err)
		}
		_, _ = io.WriteString(w, host)
	})
	newUpstream := func(tls bool) *httptest.Server {
		t.Helper()
		listener, err := net.Listen("tcp6", "[::1]:0")
		if err != nil {
			t.Skipf("IPv6 loopback unavailable: %v", err)
		}
		server := httptest.NewUnstartedServer(upstreamHandler)
		server.Listener.Close()
		server.Listener = listener
		if tls {
			server.StartTLS()
		} else {
			server.Start()
		}
		t.Cleanup(server.Close)
		return server
	}
	httpOrigin := newUpstream(false)
	httpsOrigin := newUpstream(true)
	ipv4Origin := httptest.NewServer(upstreamHandler)
	defer ipv4Origin.Close()

	const username = "fixed.alice"
	const password = "test:password"
	const token = "global-token"
	account := model.IPv6Account{Address: netip.MustParsePrefix("::1/128")}
	if err := account.SetPassword(password); err != nil {
		t.Fatal(err)
	}
	endpoint := service.NewDefaultEndpoint(0)
	endpoint.SOCKS5Accounts = map[string]model.IPv6Account{username: account}
	forward := proxy.NewForwardProxy(proxy.ForwardProxyConfig{
		ProxyToken: token, ProxyBypassRules: []string{"127.0.0.1"},
	})
	server := httptest.NewServer(newEndpointInboundMux(func() model.Endpoint { return endpoint }, token, forward, nil, nil, nil))
	defer server.Close()

	for _, tt := range []struct {
		name, target, password, source string
		status                         int
	}{
		{name: "HTTP", target: httpOrigin.URL, password: password, source: "::1", status: http.StatusOK},
		{name: "CONNECT", target: httpsOrigin.URL, password: password, source: "::1", status: http.StatusOK},
		{name: "IPv4 target", target: ipv4Origin.URL, password: password, status: http.StatusBadGateway},
		{name: "wrong password", target: httpOrigin.URL, password: "wrong", status: http.StatusForbidden},
		{name: "global token", target: ipv4Origin.URL, password: token, source: "127.0.0.1", status: http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			proxyURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			proxyURL.User = url.UserPassword(username, tt.password)
			transport := httpsOrigin.Client().Transport.(*http.Transport).Clone()
			defer transport.CloseIdleConnections()
			transport.Proxy = http.ProxyURL(proxyURL)
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			resp, err := client.Get(tt.target)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tt.status || (tt.source != "" && string(body) != tt.source) {
				t.Fatalf("status %d, body %q, want status %d and source %q", resp.StatusCode, body, tt.status, tt.source)
			}
		})
	}
}
