package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// testAuth puts the package globals into the "gated" posture initAuth would
// build, but with a key we hold so tokens can be minted offline.
func testAuth(t *testing.T, admins ...string) (*rsa.PrivateKey, *accessVerifier) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	v := &accessVerifier{
		issuer:   "https://vsfg7.cloudflareaccess.com",
		certsURL: "https://vsfg7.cloudflareaccess.com/cdn-cgi/access/certs",
		aud:      "AUD-TAG",
		keys:     map[string]*rsa.PublicKey{"kid1": &key.PublicKey},
		// Recent fetch: an unknown kid must fail closed rather than hit the network.
		lastFetch: time.Now(),
	}
	adminSet = map[string]bool{}
	for _, a := range admins {
		adminSet[strings.ToLower(a)] = true
	}
	_, lan, _ := net.ParseCIDR("192.168.1.0/24")
	trustedNets = []*net.IPNet{lan}
	verifier = v
	authOn = true
	t.Cleanup(func() { authOn = false; verifier = nil; adminSet = nil; trustedNets = nil })
	return key, v
}

func mintToken(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	seg := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return base64.RawURLEncoding.EncodeToString(b)
	}
	head := seg(map[string]string{"alg": "RS256", "kid": kid, "typ": "JWT"})
	body := seg(claims)
	sum := sha256.Sum256([]byte(head + "." + body))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return head + "." + body + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func goodClaims() map[string]any {
	return map[string]any{
		"aud":   []string{"AUD-TAG"},
		"iss":   "https://vsfg7.cloudflareaccess.com",
		"sub":   "discord-1234",
		"email": "Wingman@vsfg7.example",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"nbf":   time.Now().Add(-time.Minute).Unix(),
	}
}

func req(method, target, remote string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	r.RemoteAddr = remote
	return r
}

// ── token verification ────────────────────────────────────────────────────────

func TestVerifyAcceptsWellFormedToken(t *testing.T) {
	key, v := testAuth(t)
	cl, err := v.verify(mintToken(t, key, "kid1", goodClaims()))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if cl.Email != "Wingman@vsfg7.example" || cl.Sub != "discord-1234" {
		t.Fatalf("claims not parsed: %+v", cl)
	}
}

func TestVerifyRejectsBadTokens(t *testing.T) {
	key, v := testAuth(t)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	mutate := func(f func(map[string]any)) string {
		c := goodClaims()
		f(c)
		return mintToken(t, key, "kid1", c)
	}

	cases := []struct {
		name  string
		token string
	}{
		{"expired", mutate(func(c map[string]any) { c["exp"] = time.Now().Add(-2 * time.Hour).Unix() })},
		{"no exp", mutate(func(c map[string]any) { delete(c, "exp") })},
		{"not yet valid", mutate(func(c map[string]any) { c["nbf"] = time.Now().Add(time.Hour).Unix() })},
		{"wrong audience", mutate(func(c map[string]any) { c["aud"] = []string{"SOMEONE-ELSES-APP"} })},
		{"wrong issuer", mutate(func(c map[string]any) { c["iss"] = "https://evil.cloudflareaccess.com" })},
		{"unknown key id", mintToken(t, key, "kid-nope", goodClaims())},
		{"signed by another key", mintToken(t, otherKey, "kid1", goodClaims())},
		{"malformed", "not.a.jwt.at.all"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := v.verify(tc.token); err == nil {
				t.Fatal("expected rejection, token was accepted")
			}
		})
	}
}

// A token whose signature was stripped and alg downgraded must not pass — the
// classic "alg: none" forgery.
func TestVerifyRejectsAlgNone(t *testing.T) {
	_, v := testAuth(t)
	enc := func(v any) string {
		b, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	tok := enc(map[string]string{"alg": "none", "kid": "kid1"}) + "." + enc(goodClaims()) + "."
	if _, err := v.verify(tok); err == nil {
		t.Fatal("alg=none token was accepted")
	}
}

func TestVerifyRejectsTamperedClaims(t *testing.T) {
	key, v := testAuth(t)
	tok := mintToken(t, key, "kid1", goodClaims())
	parts := strings.Split(tok, ".")
	forged := goodClaims()
	forged["email"] = "attacker@example.com"
	b, _ := json.Marshal(forged)
	parts[1] = base64.RawURLEncoding.EncodeToString(b)
	if _, err := v.verify(strings.Join(parts, ".")); err == nil {
		t.Fatal("claims were swapped under the original signature and still verified")
	}
}

// ── identity resolution ───────────────────────────────────────────────────────

func TestIdentifyTrustsLANButNotLoopback(t *testing.T) {
	testAuth(t)

	id, err := identify(req("GET", "/api/roles", "192.168.1.44:5555"))
	if err != nil {
		t.Fatalf("LAN caller rejected: %v", err)
	}
	if !id.Admin || id.Source != "lan" {
		t.Fatalf("LAN caller should be a trusted operator, got %+v", id)
	}

	// cloudflared terminates the tunnel on 127.0.0.1. If loopback were trusted,
	// every visitor arriving through the tunnel would inherit operator rights.
	if _, err := identify(req("GET", "/api/roles", "127.0.0.1:5555")); err == nil {
		t.Fatal("loopback without a token was trusted — tunnel traffic would be admin")
	}

	if _, err := identify(req("GET", "/api/roles", "203.0.113.9:5555")); err == nil {
		t.Fatal("internet caller without a token was allowed")
	}
}

func TestIdentifyTokenBeatsNetworkTrust(t *testing.T) {
	key, _ := testAuth(t) // no admins configured
	r := req("GET", "/api/roles", "192.168.1.44:5555")
	r.Header.Set(accessJWTHeader, mintToken(t, key, "kid1", goodClaims()))

	id, err := identify(r)
	if err != nil {
		t.Fatalf("identify: %v", err)
	}
	if id.Source != "access" {
		t.Fatalf("token should win over LAN trust, got source %q", id.Source)
	}
	if id.Admin {
		t.Fatal("un-listed identity became an operator by being on the LAN")
	}
}

func TestIdentifyRejectsForgedHeaderFromLAN(t *testing.T) {
	testAuth(t)
	r := req("GET", "/api/roles", "192.168.1.44:5555")
	r.Header.Set(accessJWTHeader, "garbage.token.here")
	if _, err := identify(r); err == nil {
		t.Fatal("an unverifiable token was accepted")
	}
}

func TestIdentifyReadsCookie(t *testing.T) {
	key, _ := testAuth(t, "wingman@vsfg7.example")
	r := req("GET", "/api/roles", "203.0.113.9:5555")
	r.AddCookie(&http.Cookie{Name: accessJWTCookie, Value: mintToken(t, key, "kid1", goodClaims())})
	id, err := identify(r)
	if err != nil {
		t.Fatalf("identify: %v", err)
	}
	if !id.Admin {
		t.Fatalf("listed operator not recognised from cookie: %+v", id)
	}
}

func TestAuthOffKeepsEveryoneOperator(t *testing.T) {
	authOn = false
	id, err := identify(req("GET", "/api/roles", "203.0.113.9:5555"))
	if err != nil || !id.Admin || id.Source != "open" {
		t.Fatalf("auth-off posture changed: %+v err=%v", id, err)
	}
}

func TestIsAdminMatchesAnyHandleCaseInsensitively(t *testing.T) {
	testAuth(t, "WINGMAN@vsfg7.example")
	if !isAdmin(&accessClaims{Email: "wingman@vsfg7.example"}) {
		t.Fatal("email match should be case-insensitive")
	}

	testAuth(t, "discord-1234")
	if !isAdmin(&accessClaims{Sub: "discord-1234"}) {
		t.Fatal("subject should be usable in --admins")
	}

	testAuth(t, "viking")
	if !isAdmin(&accessClaims{Custom: map[string]any{"preferred_username": "Viking"}}) {
		t.Fatal("discord username claim should be usable in --admins")
	}
	if isAdmin(&accessClaims{Email: "someone@else.example"}) {
		t.Fatal("unlisted identity granted operator rights")
	}
}

// ── control gate ──────────────────────────────────────────────────────────────

func withIdentity(r *http.Request, id identity) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), identityKey, id))
}

func TestControlAllowed(t *testing.T) {
	operator := identity{Name: "wingman", Admin: true, Source: "access"}
	viewer := identity{Name: "rookie", Admin: false, Source: "access"}

	cases := []struct {
		name    string
		method  string
		site    string
		id      identity
		wantErr bool
		code    int
	}{
		{"operator posts same-origin", "POST", "same-origin", operator, false, http.StatusOK},
		{"non-browser client", "POST", "", operator, false, http.StatusOK},
		{"GET is refused", "GET", "same-origin", operator, true, http.StatusMethodNotAllowed},
		{"cross-site is refused", "POST", "cross-site", operator, true, http.StatusForbidden},
		{"same-site is refused", "POST", "same-site", operator, true, http.StatusForbidden},
		{"viewer is refused", "POST", "same-origin", viewer, true, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := withIdentity(req(tc.method, "/api/stop?name=x", "192.168.1.44:5555"), tc.id)
			if tc.site != "" {
				r.Header.Set("Sec-Fetch-Site", tc.site)
			}
			code, err := controlAllowed(r)
			if tc.wantErr != (err != nil) {
				t.Fatalf("err=%v, wanted error: %v", err, tc.wantErr)
			}
			if code != tc.code {
				t.Fatalf("status %d, want %d", code, tc.code)
			}
		})
	}
}

// The old GET shape is what an <img src> could fire from any page a logged-in
// member visits; requireControl must turn that into a 405 without running the
// handler.
func TestRequireControlBlocksDriveByGET(t *testing.T) {
	ran := false
	h := requireControl(func(http.ResponseWriter, *http.Request) { ran = true })

	w := httptest.NewRecorder()
	h(w, withIdentity(req("GET", "/api/stop-region?region=syria", "192.168.1.44:5555"),
		identity{Name: "wingman", Admin: true}))

	if ran {
		t.Fatal("handler ran for a drive-by GET")
	}
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d, want 405", w.Code)
	}
}

func TestWithAuthRejectsUnauthenticated(t *testing.T) {
	testAuth(t)
	served := false
	h := withAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served = true }))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req("GET", "/fleet", "203.0.113.9:5555"))
	if served {
		t.Fatal("unauthenticated request reached the handler")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", w.Code)
	}
}

func TestWithAuthPassesIdentityThrough(t *testing.T) {
	testAuth(t)
	var got identity
	h := withAuth(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = who(r) }))
	h.ServeHTTP(httptest.NewRecorder(), req("GET", "/fleet", "192.168.1.44:5555"))
	if got.Source != "lan" || !got.Admin {
		t.Fatalf("identity not carried into the handler: %+v", got)
	}
}

func TestAudienceAcceptsBareString(t *testing.T) {
	var a audList
	if err := json.Unmarshal([]byte(`"AUD-TAG"`), &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !a.has("AUD-TAG") || a.has("other") {
		t.Fatalf("bare-string aud mishandled: %v", a)
	}
}

func TestInitAuthRejectsBadConfig(t *testing.T) {
	aud, team, cidr, admins := *flagAccessAUD, *flagAccessTeam, *flagTrustedCIDR, *flagAdmins
	t.Cleanup(func() {
		*flagAccessAUD, *flagAccessTeam, *flagTrustedCIDR, *flagAdmins = aud, team, cidr, admins
		authOn = false
	})

	*flagAccessAUD, *flagAccessTeam, *flagTrustedCIDR, *flagAdmins = "AUD", "", "192.168.1.0/24", ""
	if err := initAuth(); err == nil {
		t.Fatal("--access-aud without --access-team should be rejected")
	}

	*flagAccessTeam, *flagTrustedCIDR = "vsfg7", "not-a-cidr"
	if err := initAuth(); err == nil {
		t.Fatal("a malformed --trusted-cidr should be rejected")
	}

	*flagTrustedCIDR = "192.168.1.0/24"
	if err := initAuth(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	if !authOn || verifier.issuer != "https://vsfg7.cloudflareaccess.com" {
		t.Fatalf("verifier not configured: %+v", verifier)
	}
}
