# Remote dashboard access — Cloudflare Tunnel + Access (Discord login)

Goal: expose the launcher dashboard (`:7000`) to squadron members over the
public internet, gated by **Discord login** and restricted to members of the
vSFG-7 Discord server. No inbound firewall holes, TLS handled by Cloudflare.

```
browser ──HTTPS──▶ Cloudflare edge ──▶ Cloudflare Access (Discord OIDC)
                        │  (auth passes)
                        ▼
                 Cloudflare Tunnel ──▶ cloudflared (on the rig) ──▶ localhost:7000 (launcher)
                                                                       └─▶ /tower/<port>/… proxied to the tower dashboards
```

Why this shape: the launcher now **reverse-proxies** every tower endpoint under
`/tower/<port>/…` (allowlisted ports 6001-6005, 6011-6014), so the browser only
ever talks to `:7000`. Only that one hostname is exposed through the tunnel; the
per-tower dashboards stay bound to localhost. Auth is entirely at the Cloudflare
edge — there is **no** password or login code in the launcher.

---

## Part A — Launcher proxy (DONE, in code)

Already implemented (`cmd/launcher/main.go` `handleTowerProxy` + `ui.html` now
fetches `/tower/<port>/…` instead of `http://localhost:<port>`). Nothing to do
here beyond running the current `launcher.exe`. Verify locally:

```
curl http://localhost:7000/tower/6013/status      # → Senaki JSON
curl http://localhost:7000/tower/9999/status      # → 403 (allowlist)
```

## Part B — Cloudflare Tunnel (rig → Cloudflare)

Prereq: a domain already on Cloudflare (you have one). Pick a hostname, e.g.
`atc.yourdomain.com`.

1. Install cloudflared on the rig:
   `winget install --id Cloudflare.cloudflared`  (or download the Windows .msi).
2. Authenticate: `cloudflared tunnel login`  (opens browser, pick your domain).
3. Create the tunnel: `cloudflared tunnel create vsfg7-atc`  (note the tunnel UUID / creds file it writes).
4. Route DNS: `cloudflared tunnel route dns vsfg7-atc atc.yourdomain.com`
5. Config file `C:\Users\Administrator\.cloudflared\config.yml`:
   ```yaml
   tunnel: vsfg7-atc
   credentials-file: C:\Users\Administrator\.cloudflared\<TUNNEL-UUID>.json
   ingress:
     - hostname: atc.yourdomain.com
       service: http://localhost:7000
     - service: http_status:404
   ```
6. Run it: `cloudflared tunnel run vsfg7-atc`  (test), then install as a service
   so it survives reboot: `cloudflared service install`.

At this point `https://atc.yourdomain.com` reaches the launcher — but it is
**wide open**. Do Parts C + D before sharing the URL.

## Part C — Discord OIDC shim (Cloudflare Worker)

Discord's OAuth2 is **not** OIDC-compliant, so Cloudflare Access can't talk to
it directly. Deploy the community Worker shim (Erisa/discord-oidc-worker) that
turns Discord OAuth into OIDC and exposes a `guilds` claim.

1. Discord Developer Portal (https://discord.com/developers/applications):
   - New Application → OAuth2 → copy **Client ID** + **Client Secret**.
   - Add redirect URI: `https://YOURTEAM.cloudflareaccess.com/cdn-cgi/access/callback`
     (`YOURTEAM` = your Cloudflare Zero Trust team subdomain).
   - For guild restriction: create a **Bot** for the app, invite it to the
     vSFG-7 server, and copy the **Bot Token**.
2. Deploy the Worker:
   ```
   git clone https://github.com/Erisa/discord-oidc-worker && cd discord-oidc-worker
   npm install
   # create a KV namespace in the CF dashboard, put its id in wrangler.toml
   cp config.sample.json config.json   # set clientId, clientSecret, redirectURL (the callback above)
   # for guild checks: set "serversToCheckRolesFor": ["<vSFG-7 SERVER ID>"]
   npx wrangler secret put DISCORD_TOKEN     # paste the bot token
   npx wrangler deploy
   ```
   Worker ends up at `https://discord-oidc.YOURNAME.workers.dev`.

## Part D — Cloudflare Access (the gate)

1. Zero Trust dashboard → **Settings → Authentication → Login methods → Add new
   → OpenID Connect**:
   | Field | Value |
   |---|---|
   | Auth URL | `https://discord-oidc.YOURNAME.workers.dev/authorize/guilds` |
   | Token URL | `https://discord-oidc.YOURNAME.workers.dev/token` |
   | Certificate URL | `https://discord-oidc.YOURNAME.workers.dev/jwks.json` |
   | App ID | Discord Client ID |
   | Client secret | Discord Client Secret |
   | PKCE | Enabled |
   - Use `/authorize/guilds` (not `/email`) so the `guilds` claim is populated.
   - Save → **Test** the login.
2. Zero Trust → **Access → Applications → Add → Self-hosted**:
   - Application domain: `atc.yourdomain.com`.
   - Identity providers: the Discord OIDC you just added (only).
   - **Policy** (Allow), restrict to the squadron one of two ways:
     - **By guild** — add the server id as an OIDC claim `roles:<SERVER_ID>` in
       the IdP config, then policy rule: OIDC claim `roles:<SERVER_ID>` is
       present; or
     - **By identity** — allow-list specific Discord emails / user IDs.
   - Session duration to taste (e.g. 24h).

## Part E — Verify & operate

- Open `https://atc.yourdomain.com` in a fresh/incognito browser → Cloudflare
  bounces you to Discord login → after auth the dashboard loads and all panels
  populate (they flow through the launcher proxy).
- A non-member (not in the server / not allow-listed) should be **denied** by
  the Access policy.
- Rollback / kill switch: stop the `cloudflared` service (`cloudflared service
  uninstall` or stop the Windows service) → the public URL goes dark instantly;
  the local `:7000` dashboard is unaffected.

## Security notes

- Everyone who passes Access gets **full control** (Start/Stop/runway/weather) —
  that was the chosen access level. Anyone you let through Discord can kick a
  role mid-mission, so keep the guild/allow-list tight.
- The `/tower/` proxy is allowlisted to known dashboard ports only — it can't be
  used to reach arbitrary localhost services.
- Never port-forward `:7000` (or 6001-6014) directly on the router — the tunnel
  is the only ingress; the rig's IP stays hidden.
- Rotate the Discord client secret / bot token if the Worker repo or KV is ever
  exposed.

Sources: Cloudflare One generic-OIDC docs, Erisa/discord-oidc-worker README.
