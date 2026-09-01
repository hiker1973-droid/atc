# Remote access — setup status / resume here (2026-09-01)

Goal: expose the launcher dashboard publicly, gated by **Discord login**
(restricted to the vSFG-7 guild) via **Cloudflare Tunnel + Cloudflare Access**,
with squadron members read-only and a named operator list keeping control.
Full runbook: `REMOTE_ACCESS.md`.

Decisions taken 2026-09-01: **split roles** (viewer / operator), exposed rig is
**host `.231`**, ingress is **Cloudflare Tunnel + Discord OIDC**.

## DONE
- **Launcher reverse-proxy** — `/tower/<port>/…` (allowlisted 6001-6048). Older
  work, pushed as `4f7fbd7`.
- **Launcher auth** (`cmd/launcher/auth.go`, new) — Cloudflare Access JWT
  verification against the team JWKS (signature, `aud`, `iss`, expiry), viewer /
  operator split, `--access-team` / `--access-aud` / `--admins` /
  `--trusted-cidr`. Off unless `--access-aud` is set, so the other rigs are
  untouched. `/api/me` reports the caller's tier.
- **Control actions hardened** — start / stop / restart / start-region /
  stop-region / rescan and the proxied tower `runway` + `weather` POSTs are now
  operator-only, POST-only, same-origin-only. They previously accepted GET from
  anyone who could reach the port.
- **Rig picker** — `/rig/<name>/…` proxy plus a header dropdown, so the one
  exposed launcher can drive any rig in `--fleet` (the missions run on
  Training 1, not on the exposed box). Access credentials are stripped before
  the hop.
- **Viewer UI** — non-operators get a VIEW ONLY chip and no control buttons.
- **Squadron patch** in the header (`/logo.png`, embedded in the binary).
- **Tests** — `cmd/launcher/auth_test.go`, `proxy_test.go`. Cover token forgery,
  expiry, wrong-audience, wrong-issuer, `alg:none`, claim tampering, the
  loopback-is-not-trusted rule, the control gate, and the proxy gate.
- **Verified on the LAN** against a real launcher — the curl matrix in
  `REMOTE_ACCESS.md` Part E, plus a live `/rig/training1/api/roles` hop.

## RESUME HERE  ← next action
Everything left is in the Cloudflare and Discord consoles (needs your accounts):

1. **Tunnel** — `REMOTE_ACCESS.md` Part B. cloudflared is **not** on `.231`
   (the July download was on another rig and is gone). Install it, create the
   `vsfg7-atc` tunnel, install the connector with the token, and point the
   public hostname at **`http://127.0.0.1:7000`**.
2. **Discord OIDC worker** — Part C.
3. **Access application + policy**, then copy its **AUD tag** — Part D.
4. **Restart the launcher with the auth flags** (Part A) and re-run the Part E
   matrix through the tunnel.
5. **Install the launcher as a Windows service** so the public URL does not go
   dark when it does its silent-death trick.

## Rig state (2026-09-01)
- Launchers up on training1 `.220`, dev `.221`, foothold `.222`. **Host `.231`
  has no launcher running** — start it before exposing anything.
- Host is on `main`; the auth work above is uncommitted local change.

## Follow-up, not blocking
- Tower dashboards bind `0.0.0.0` with `Access-Control-Allow-Origin: *`
  (`cmd/atc/dashboard.go`). Pre-existing LAN exposure; bind to `127.0.0.1`,
  the launcher proxy is the only consumer.
