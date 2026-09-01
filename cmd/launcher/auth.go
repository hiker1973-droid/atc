// Authentication and authorization for the launcher.
//
// The launcher was built for a trusted LAN and had no auth at all. Exposing it
// publicly (Cloudflare Tunnel — see REMOTE_ACCESS.md) splits callers in two:
//
//	viewer   — any squadron member who passed the Cloudflare Access gate.
//	           Reads status, fleet, alerts and logs. Cannot change anything.
//	operator — an identity listed in --admins, plus anyone on --trusted-cidr.
//	           Everything a viewer can do, plus start/stop/restart, region
//	           control, rescan, and the towers' runway/weather POSTs.
//
// Identity comes from the JWT Cloudflare Access stamps on every proxied
// request. The token is *verified* against the team's JWKS rather than merely
// read: the header is trivially forgeable by anything that can reach :7000
// directly, so an unverified read would hand operator rights to the whole LAN.
package main

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	flagAccessTeam = flag.String("access-team", "",
		"Cloudflare Access team name (the <team> in <team>.cloudflareaccess.com)")
	flagAccessAUD = flag.String("access-aud", "",
		"Cloudflare Access application AUD tag. Empty = auth OFF (the historical LAN-only posture)")
	flagAdmins = flag.String("admins", "",
		"Identities allowed to run control actions: comma-separated Access emails and/or Discord user ids")
	flagTrustedCIDR = flag.String("trusted-cidr", "192.168.1.0/24",
		"Networks granted operator rights without an Access token. Loopback is deliberately excluded: "+
			"cloudflared connects from 127.0.0.1, so trusting it would make every tunnel visitor an operator")
)

const (
	accessJWTHeader = "Cf-Access-Jwt-Assertion"
	accessJWTCookie = "CF_Authorization"
	clockLeeway     = 60 * time.Second
)

// identity is who the current request is acting as. Source distinguishes
// "open" (auth disabled), "lan" (trusted network, no token) and "access"
// (a verified Cloudflare Access token).
type identity struct {
	Name   string `json:"name"`
	Admin  bool   `json:"admin"`
	Source string `json:"source"`
}

var (
	authOn      bool
	adminSet    map[string]bool
	trustedNets []*net.IPNet
	verifier    *accessVerifier
)

// initAuth wires up the auth config from flags. Auth stays off unless
// --access-aud is given, so the other rigs' launchers keep working unchanged.
func initAuth() error {
	adminSet = map[string]bool{}
	for _, a := range strings.Split(*flagAdmins, ",") {
		if a = strings.ToLower(strings.TrimSpace(a)); a != "" {
			adminSet[a] = true
		}
	}
	trustedNets = nil
	for _, c := range strings.Split(*flagTrustedCIDR, ",") {
		if c = strings.TrimSpace(c); c == "" {
			continue
		}
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			return fmt.Errorf("bad --trusted-cidr %q: %w", c, err)
		}
		trustedNets = append(trustedNets, n)
	}

	if *flagAccessAUD == "" {
		authOn = false
		return nil
	}
	if *flagAccessTeam == "" {
		return errors.New("--access-aud needs --access-team as well")
	}
	team := strings.TrimSuffix(strings.TrimPrefix(*flagAccessTeam, "https://"), ".cloudflareaccess.com")
	verifier = &accessVerifier{
		issuer:   "https://" + team + ".cloudflareaccess.com",
		certsURL: "https://" + team + ".cloudflareaccess.com/cdn-cgi/access/certs",
		aud:      *flagAccessAUD,
		keys:     map[string]*rsa.PublicKey{},
	}
	authOn = true
	return nil
}

// authSummary is the one-line startup banner describing the posture.
func authSummary() string {
	if !authOn {
		return "auth OFF — every caller is an operator (LAN-only posture; set --access-aud to gate)"
	}
	nets := make([]string, 0, len(trustedNets))
	for _, n := range trustedNets {
		nets = append(nets, n.String())
	}
	return fmt.Sprintf("auth ON — Access team %s, %d operator(s), trusted %s",
		*flagAccessTeam, len(adminSet), strings.Join(nets, ","))
}

// ── Request identity ──────────────────────────────────────────────────────────

type ctxKey int

const identityKey ctxKey = 0

// identify resolves the caller. A token always wins over network trust, so an
// operator can test the gate from the LAN and see exactly what a member sees.
func identify(r *http.Request) (identity, error) {
	if !authOn {
		return identity{Name: "anonymous", Admin: true, Source: "open"}, nil
	}
	if tok := accessToken(r); tok != "" {
		cl, err := verifier.verify(tok)
		if err != nil {
			return identity{}, fmt.Errorf("access token rejected: %w", err)
		}
		name := cl.Email
		if name == "" {
			name = cl.Sub
		}
		return identity{Name: name, Admin: isAdmin(cl), Source: "access"}, nil
	}
	if ip := clientIP(r); ip != nil && trusted(ip) {
		return identity{Name: "lan:" + ip.String(), Admin: true, Source: "lan"}, nil
	}
	return identity{}, errors.New("no Cloudflare Access token")
}

func accessToken(r *http.Request) string {
	if h := r.Header.Get(accessJWTHeader); h != "" {
		return h
	}
	if c, err := r.Cookie(accessJWTCookie); err == nil {
		return c.Value
	}
	return ""
}

// clientIP reads the transport peer only. X-Forwarded-For is deliberately
// ignored — anything that can reach the port can set it, and forging a trusted
// source address would be a free upgrade to operator.
func clientIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(host)
}

func trusted(ip net.IP) bool {
	for _, n := range trustedNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func isAdmin(cl *accessClaims) bool {
	for _, name := range cl.names() {
		if adminSet[strings.ToLower(name)] {
			return true
		}
	}
	return false
}

// withAuth resolves identity once per request and rejects anyone who is
// neither on a trusted network nor carrying a valid Access token.
func withAuth(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := identify(r)
		if err != nil {
			http.Error(w, "forbidden: "+err.Error(), http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey, id)))
	})
}

func who(r *http.Request) identity {
	if id, ok := r.Context().Value(identityKey).(identity); ok {
		return id
	}
	return identity{}
}

// controlAllowed is the gate on every state-changing endpoint. Three checks:
//
//	POST only    — these used to be GETs, which any <img src> could fire.
//	same-origin  — Sec-Fetch-Site blocks a cross-site post from a page a
//	               logged-in member happens to be visiting. An empty value means
//	               a non-browser client (curl, ops scripts) and passes.
//	operator     — the identity resolved by withAuth must be an admin.
func controlAllowed(r *http.Request) (int, error) {
	if r.Method != http.MethodPost {
		return http.StatusMethodNotAllowed, errors.New("POST required for control actions")
	}
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin", "none":
	default:
		return http.StatusForbidden, errors.New("cross-site control request refused")
	}
	if id := who(r); !id.Admin {
		return http.StatusForbidden, fmt.Errorf("%s is a viewer, not an operator", id.Name)
	}
	return http.StatusOK, nil
}

func requireControl(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if code, err := controlAllowed(r); err != nil {
			http.Error(w, err.Error(), code)
			return
		}
		h(w, r)
	}
}

// handleMe lets the UI decide whether to render the control surface. It is a
// hint for the page, never the enforcement — that is controlAllowed.
func handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, who(r))
}

// ── Cloudflare Access token verification ─────────────────────────────────────

type accessVerifier struct {
	issuer   string
	certsURL string
	aud      string

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	lastFetch time.Time
}

var jwksClient = &http.Client{Timeout: 10 * time.Second}

type audList []string

func (a *audList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*a = audList{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

func (a audList) has(s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

type accessClaims struct {
	Aud    audList        `json:"aud"`
	Iss    string         `json:"iss"`
	Sub    string         `json:"sub"`
	Email  string         `json:"email"`
	Exp    int64          `json:"exp"`
	Nbf    int64          `json:"nbf"`
	Custom map[string]any `json:"custom"`
}

// names is every handle this identity could be listed under in --admins: the
// Access email, the subject, and the Discord username the OIDC shim passes
// through as a custom claim.
func (c *accessClaims) names() []string {
	out := []string{}
	if c.Email != "" {
		out = append(out, c.Email)
	}
	if c.Sub != "" {
		out = append(out, c.Sub)
	}
	for _, k := range []string{"preferred_username", "id", "username"} {
		if s, ok := c.Custom[k].(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func (v *accessVerifier) verify(tok string) (*accessClaims, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return nil, errors.New("malformed JWT")
	}
	var hdr struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &hdr); err != nil {
		return nil, fmt.Errorf("bad JWT header: %w", err)
	}
	if hdr.Alg != "RS256" {
		return nil, fmt.Errorf("unsupported signing alg %q", hdr.Alg)
	}
	pub, err := v.key(hdr.Kid)
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("bad signature encoding")
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); err != nil {
		return nil, errors.New("signature does not verify")
	}

	var cl accessClaims
	if err := decodeSegment(parts[1], &cl); err != nil {
		return nil, fmt.Errorf("bad JWT claims: %w", err)
	}
	now := time.Now()
	if cl.Exp == 0 || now.After(time.Unix(cl.Exp, 0).Add(clockLeeway)) {
		return nil, errors.New("token expired")
	}
	if cl.Nbf != 0 && now.Before(time.Unix(cl.Nbf, 0).Add(-clockLeeway)) {
		return nil, errors.New("token not valid yet")
	}
	if cl.Iss != v.issuer {
		return nil, fmt.Errorf("token issued by %q, want %q", cl.Iss, v.issuer)
	}
	if !cl.Aud.has(v.aud) {
		return nil, errors.New("token is for a different Access application")
	}
	return &cl, nil
}

func decodeSegment(seg string, v any) error {
	raw, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// key returns the signing key for kid, refreshing the JWKS at most once a
// minute so an unknown-kid flood cannot turn into a request flood at Cloudflare.
func (v *accessVerifier) key(kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	k, ok := v.keys[kid]
	fresh := time.Since(v.lastFetch) < time.Minute
	v.mu.Unlock()
	if ok {
		return k, nil
	}
	if fresh {
		return nil, fmt.Errorf("unknown signing key %q", kid)
	}
	if err := v.refresh(); err != nil {
		return nil, err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown signing key %q", kid)
}

func (v *accessVerifier) refresh() error {
	v.mu.Lock()
	v.lastFetch = time.Now() // stamp the attempt, so failures back off too
	v.mu.Unlock()

	resp, err := jwksClient.Get(v.certsURL)
	if err != nil {
		return fmt.Errorf("fetch Access certs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch Access certs: %s", resp.Status)
	}
	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("parse Access certs: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		nb, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eb, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil || len(eb) == 0 || len(eb) > 8 {
			continue
		}
		e := 0
		for _, b := range eb {
			e = e<<8 | int(b)
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}
	}
	if len(keys) == 0 {
		return errors.New("no usable RSA keys in the Access JWKS")
	}
	v.mu.Lock()
	v.keys = keys
	v.mu.Unlock()
	return nil
}
