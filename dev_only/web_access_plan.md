# Web-interface access plan (deferred)

User ask 2026-05-29: "make this a web interface, give people access via
browser." Already a web interface (launcher serves `ui.html` at `:7000`,
tower dashboards serve JSON at `:6001`–`:6003`). What's needed is
*remote access*. Deferred — user wrote "we will start in a while."

## Current state of network bindings

Confirmed live on Training 1 via `netstat -ano`:

| Process | Bind | Reachable from LAN today? |
|---|---|---|
| `launcher.exe` | `0.0.0.0:7000` | yes if Windows Firewall allows |
| `atc.exe` OMDM dashboard | `0.0.0.0:6001` | yes if Windows Firewall allows |
| `atc.exe` OMAM dashboard | `0.0.0.0:6002` | yes if Windows Firewall allows |
| `atc.exe` OMAL dashboard | `0.0.0.0:6003` | yes if Windows Firewall allows |
| `atc.exe` Marshal dashboard | `0.0.0.0:6004` | yes if Windows Firewall allows |
| `atc.exe` Deckboss dashboard | `0.0.0.0:6005` | yes if Windows Firewall allows |

Bind code:
- launcher: `cmd/launcher/main.go:35` — `flag.String("listen", ":7000", ...)` (Go's `:7000` = all interfaces).
- towers/marshal/deckboss: `cmd/atc/dashboard.go:184` — `fmt.Sprintf("0.0.0.0:%d", ds.port)`.

CORS is already wide open — `Access-Control-Allow-Origin: *` set on every dashboard endpoint (`cmd/atc/dashboard.go:163`). No browser-side blocker.

So the *only* thing keeping squadron members from loading `http://<rig-IP>:7000/` today is the Windows Firewall inbound rules. No code change is needed for LAN access.

## Options we put on the table

### Option A — Squadron LAN only
- Open Windows Firewall inbound on TCP 7000 (and optionally 6001-6005 if operators want to deep-link into a single role's dashboard).
- Share `http://<rig-LAN-IP>:7000/` in the squadron channel.
- **Risk:** anyone reachable on the LAN can hit the Start/Stop/Restart Role buttons — `cmd/launcher/main.go` exposes `/api/start`, `/api/stop`, `/api/restart`, `/api/runway`, `/api/miz-weather` with no auth.
- **Effort:** firewall command only. Zero code.

### Option B — Internet, no auth
- Port-forward 7000 on the squadron router OR put Cloudflare Tunnel / ngrok in front.
- Same no-auth risk as A but the attack surface is the entire internet — random scanners will find it.
- **Not recommended** without at least Option D's auth layer.
- **Effort:** router config or tunnel setup, no code.

### Option C — Internet with auth (recommended for public)
- Add login middleware in `cmd/launcher/main.go` — simplest is a shared password issued via `setx SKYEYE_DASH_PASS ...`.
- Cookie/JWT for session, sent on every fetch. Middleware checks before forwarding to `/api/*` and the static `ui.html`.
- Read-only endpoints (`/status`, log feed) could remain open or also gated, operator choice.
- **Effort:** ~150 LOC in launcher + a small JS login page. One commit.
- **Caveats:** real password hashing (bcrypt), not constant-time on plain compare. TLS strongly recommended (Cloudflare Tunnel gives it for free; bare port-forward would need a reverse proxy with a cert).

### Option D — LAN now, internet/auth later (the user's likely path)
- Option A today (firewall open, share LAN URL).
- Option C as a follow-up before any internet exposure.
- Acceptable while squadron use stays on the squadron LAN / VPN.

## Open questions before kicking this off

1. **Network topology** — is Training 1 on a squadron LAN, a VPS / cloud host, or a home-NAT box? Determines whether "LAN access" is what people actually want or if internet is implied.
2. **Who clicks Start/Stop?** — if non-operators see the dashboard, the Controls panel buttons need to be either hidden or gated, since accidental clicks during a live mission would kick a role.
3. **TLS** — bare HTTP across the public internet ships passwords in cleartext. Tunnel or reverse proxy with a cert is the realistic path; both add a moving part.
4. **Read-only public mirror?** — alternative shape: a read-only dashboard at a public URL (status/log only, no Controls), full dashboard stays LAN-only. Reduces auth requirements.

## Implementation notes (when we restart)

- The launcher's `/api/*` handlers all currently bypass any check — adding middleware wraps `mux.Handle` rather than each handler.
- Read-only-vs-write split exists naturally: `/status` and `/ws/log` are read; everything under `/api/` is write. A middleware that gates `/api/*` and leaves the rest open is the smallest auth shape.
- Tower dashboards (6001-6005) also expose `/runway` and `/weather` POST endpoints — those need the same gating if exposed.
- `setx SKYEYE_DASH_PASS ...` matches the existing env-var pattern (`OPENAI_API_KEY`, `SRS_EAM`, `SKYEYE_SRS`, `SKYEYE_TACVIEW`). Re-use the four-var precedent.

## Decision log

- 2026-05-29: options A/B/C/D presented. User: "make notes. we will start in a while." → this file.
- 2026-07-09: **resumed.** Chosen: public internet + full control + **Cloudflare Access with Discord login** (edge auth, restricted to the vSFG-7 guild), fronted by Cloudflare Tunnel. Launcher now reverse-proxies the tower dashboards under `/tower/<port>/…` (allowlisted) so only `:7000` is exposed and the panels work remotely. Full runbook: **`REMOTE_ACCESS.md`** at repo root. Note discovered during setup: Discord is **not** a Cloudflare-native OIDC IdP — needs the `Erisa/discord-oidc-worker` shim (Discord OAuth2 isn't OIDC-compliant).
