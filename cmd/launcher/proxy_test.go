package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// The rig picker sends every call to /rig/<name>/… . That path reaches another
// box on the LAN, so it carries the same operator gate as the local actions.
func TestRigProxyGate(t *testing.T) {
	fleetRigs = []Rig{{Name: "training1", Host: "192.168.1.220", Port: 7000}}
	t.Cleanup(func() { fleetRigs = nil })

	operator := identity{Name: "wingman", Admin: true, Source: "access"}
	viewer := identity{Name: "rookie", Admin: false, Source: "access"}

	cases := []struct {
		name   string
		method string
		path   string
		id     identity
		want   int
	}{
		{"unknown rig", "GET", "/rig/nowhere/api/roles", operator, http.StatusNotFound},
		{"no path", "GET", "/rig/training1", operator, http.StatusBadRequest},
		{"viewer cannot stop a remote role", "POST", "/rig/training1/api/stop?name=x", viewer, http.StatusForbidden},
		{"cross-site post refused", "POST", "/rig/training1/api/stop?name=x", operator, http.StatusForbidden},
		// A viewer's GET is not method-gated — it goes to the rig. That path is
		// covered hermetically by TestRigProxyForwardsWithoutCredentials rather
		// than here, so the suite never depends on a rig being up.
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := withIdentity(req(tc.method, tc.path, "192.168.1.44:5555"), tc.id)
			if tc.name == "cross-site post refused" {
				r.Header.Set("Sec-Fetch-Site", "cross-site")
			}
			w := httptest.NewRecorder()
			handleRigProxy(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d, want %d (body %q)", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

// A viewer's GET reaches the remote rig; the 503 above is the unreachable rig,
// not the gate. Confirm the request really is forwarded, and that this
// session's Access credentials are not forwarded with it.
func TestRigProxyForwardsWithoutCredentials(t *testing.T) {
	var got *http.Request
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		w.Write([]byte(`[]`))
	}))
	defer upstream.Close()

	host, port := splitHostPortForTest(t, upstream.Listener.Addr().String())
	fleetRigs = []Rig{{Name: "training1", Host: host, Port: port}}
	t.Cleanup(func() { fleetRigs = nil })

	r := withIdentity(req("GET", "/rig/training1/api/roles", "192.168.1.44:5555"),
		identity{Name: "rookie", Admin: false, Source: "access"})
	r.Header.Set(accessJWTHeader, "a.real.token")
	r.AddCookie(&http.Cookie{Name: accessJWTCookie, Value: "a.real.token"})

	w := httptest.NewRecorder()
	handleRigProxy(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (body %q)", w.Code, w.Body.String())
	}
	if got == nil {
		t.Fatal("request never reached the rig")
	}
	if got.URL.Path != "/api/roles" {
		t.Fatalf("rig saw path %q, want /api/roles", got.URL.Path)
	}
	if got.Header.Get(accessJWTHeader) != "" || got.Header.Get("Cookie") != "" {
		t.Fatalf("Access credentials leaked to the rig: %v", got.Header)
	}
}

func TestIsSelfMatchesTheLocalLauncher(t *testing.T) {
	listen := *flagListen
	*flagListen = ":7000"
	t.Cleanup(func() { *flagListen = listen })

	if !isSelf(Rig{Name: "host", Host: "127.0.0.1", Port: 7000}) {
		t.Fatal("loopback on the listen port should be this launcher")
	}
	if isSelf(Rig{Name: "training1", Host: "192.168.1.220", Port: 7000}) {
		t.Fatal("another box was reported as this launcher")
	}
	if isSelf(Rig{Name: "host", Host: "127.0.0.1", Port: 7001}) {
		t.Fatal("a different port is a different launcher")
	}
}

func splitHostPortForTest(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("port %q: %v", portStr, err)
	}
	return host, port
}
