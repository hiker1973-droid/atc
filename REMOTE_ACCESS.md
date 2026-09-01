# Remote dashboard access — Cloudflare Tunnel + Access (Discord login)

Goal: expose the launcher dashboard to squadron members over the public
internet, gated by **Discord login** and restricted to members of the vSFG-7
Discord server. No inbound firewall holes, TLS handled by Cloudflare.

Two tiers of access, decided per identity:

| | Sees | Can change |
|---|---|---|
| **viewer** — anyone past the Discord gate | dashboard, `/fleet`, `/logs`, tower status, log tails | nothing |
| **operator** — listed in `--admins`, plus anyone on `--trusted-cidr` (the LAN) | everything a viewer sees | start / stop / restart roles, region start+stop, rescan, runway, weather |

```
browser ──HTTPS──▶ Cloudflare edge ──▶ Cloudflare Access (Discord OIDC)
                        │  (auth passes, stamps a signed JWT on the request)
                        ▼
                 Cloudflare Tunnel ──▶ cloudflared (on the rig) ──▶ 127.0.0.1:7000 launcher
                                                                     │  verifies the JWT,
                                                                     │  viewer or operator
                                                                     ├─▶ /tower/<port>/…  local tower dashboards
                                                                     └─▶ /rig/<name>/…    the other rigs' launchers
```

Why this shape: the launcher reverse-proxies both the tower dashboards
(`/tower/<port>/…`, allowlisted ports) and the other rigs' launchers
(`/rig/<name>/…`, allowlisted by `--fleet`), so the browser only ever talks to
one hostname. Everything else stays bound to the LAN.

---

## Part A — Launcher auth (DONE, in code)

`cmd/launcher/auth.go`. Off by default, so the LAN-only rigs are unaffected;
it switches on when `--access-aud` is passed.

```
launcher.exe ^
  --access-team vsfg7 ^
  --access-aud  <APPLICATION AUD TAG> ^
  --admins      you@example.com,someone@example.com ^
  --trusted-cidr 192.168.1.0/24
```

| Flag | Meaning |
|---|---|
| `--access-team` | the `<team>` in `<team>.cloudflareaccess.com` |
| `--access-aud` | the Access application's AUD tag. **Empty = auth off** |
| `--admins` | operators: Access email, Discord user id, or Discord username — comma separated, case-insensitive |
| `--trusted-cidr` | networks that get operator rights with no token (default `192.168.1.0/24`) |

Three things worth understanding before deploying it:

- **Point cloudflared at `127.0.0.1:7000`, and never add loopback to
  `--trusted-cidr`.** Tunnel traffic arrives *from* loopback. If loopback were
  trusted, every visitor through the tunnel would silently become an operator.
  The default CIDR deliberately excludes it, and there is a test pinning that.
- **The token is verified, not just read.** Access stamps
  `Cf-Access-Jwt-Assertion`; the launcher checks the RS256 signature against
  `https://<team>.cloudflareaccess.com/cdn-cgi/access/certs`, plus `aud`, `iss`
  and expiry. A forged header from the LAN is rejected.
- **Control actions are POST + same-origin.** They used to accept GET, which
  meant an `<img src="…/api/stop?name=X">` on any page a logged-in member
  visited could kick a role. `Sec-Fetch-Site` must be absent (curl) or
  `same-origin`.

On the LAN nothing changes: a browser on `192.168.1.x` is an operator without
logging into anything. Browse the box's **LAN IP**, not `localhost` — loopback
now needs a token.

## Part B — Cloudflare Tunnel (rig → Cloudflare)

Prereq: a domain already on Cloudflare. Pick a hostname, e.g. `atc.yourdomain.com`.
Use the **dashboard-managed (token) tunnel** — the older `cloudflared tunnel
login` cert flow failed on this rig (empty `cert.pem`) and is not worth retrying.

1. Get cloudflared: `winget install --id Cloudflare.cloudflared`, or drop
   `cloudflared.exe` next to the launcher.
2. Zero Trust → **Networks → Tunnels → Create a tunnel** → connector
   **Cloudflared** → name `vsfg7-atc` → Save.
3. On the install step, **Windows** tab, copy the token (the long `eyJ…` in the
   `cloudflared service install <TOKEN>` line), then run on the rig:
   `cloudflared.exe service install <TOKEN>`
4. Tunnel → **Public Hostnames** → Add: `atc.yourdomain.com` → HTTP →
   **`http://127.0.0.1:7000`** (see the loopback note in Part A).

At this point the hostname reaches the launcher but is **wide open** — the
launcher has no AUD to check yet. Do Parts C + D before sharing the URL.

## Part C — Discord OIDC shim (Cloudflare Worker)

Discord's OAuth2 is not OIDC-compliant, so Access cannot talk to it directly.
Deploy the community shim (`Erisa/discord-oidc-worker`), which also exposes a
`guilds` claim.

1. Discord Developer Portal → New Application → OAuth2 → copy **Client ID** +
   **Client Secret**; add redirect URI
   `https://<team>.cloudflareaccess.com/cdn-cgi/access/callback`.
   For guild restriction, create a **Bot** for the app, invite it to the vSFG-7
   server, copy the **Bot Token**.
2. Deploy:
   ```
   git clone https://github.com/Erisa/discord-oidc-worker && cd discord-oidc-worker
   npm install
   # create a KV namespace in the CF dashboard, put its id in wrangler.toml
   cp config.sample.json config.json   # clientId, clientSecret, redirectURL (the callback above)
   # for guild checks: "serversToCheckRolesFor": ["<vSFG-7 SERVER ID>"]
   npx wrangler secret put DISCORD_TOKEN     # the bot token
   npx wrangler deploy
   ```

## Part D — Cloudflare Access (the gate)

1. Zero Trust → **Settings → Authentication → Login methods → Add → OpenID Connect**:

   | Field | Value |
   |---|---|
   | Auth URL | `https://discord-oidc.YOURNAME.workers.dev/authorize/guilds` |
   | Token URL | `https://discord-oidc.YOURNAME.workers.dev/token` |
   | Certificate URL | `https://discord-oidc.YOURNAME.workers.dev/jwks.json` |
   | App ID / secret | the Discord client id + secret |
   | PKCE | Enabled |

   Use `/authorize/guilds` (not `/email`) so the `guilds` claim is populated.
   Save, then **Test** the login.
2. Zero Trust → **Access → Applications → Add → Self-hosted**:
   - Application domain `atc.yourdomain.com`; identity provider: the Discord
     OIDC only; policy Allow, restricted to the guild (OIDC claim
     `roles:<SERVER_ID>` present) or to an explicit identity list.
   - Session duration to taste (24h is reasonable).
3. **Copy the Application Audience (AUD) Tag** from the application's Overview
   tab — that is `--access-aud`. Restart the launcher with it.

## Part E — Verify

From the LAN, against the launcher's own IP. This is the matrix that was run
when the gate was built; every line is the expected result:

```
curl -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7000/api/health        # 403  loopback needs a token
curl -o /dev/null -w '%{http_code}\n' http://192.168.1.231:7000/api/health    # 200  LAN is trusted
curl -o /dev/null -w '%{http_code}\n' -H 'Cf-Access-Jwt-Assertion: forged.token.here' \
     http://192.168.1.231:7000/api/health                                     # 403  forged token
curl -o /dev/null -w '%{http_code}\n' 'http://192.168.1.231:7000/api/stop?name=Deckboss'   # 405  GET refused
curl -o /dev/null -w '%{http_code}\n' -X POST -H 'Sec-Fetch-Site: cross-site' \
     'http://192.168.1.231:7000/api/stop?name=Deckboss'                       # 403  cross-site refused
curl http://192.168.1.231:7000/api/me                                         # {"admin":true,"source":"lan"}
```

Then through the tunnel, in a fresh/incognito browser:

- `https://atc.yourdomain.com` bounces to Discord, then loads the dashboard.
- A member **not** in `--admins` sees a **VIEW ONLY** chip and no Start / Stop /
  ATIS / runway / weather controls.
- A non-member is denied by the Access policy and never reaches the launcher.

Kill switch: stop the `cloudflared` service and the public URL goes dark
instantly; the LAN dashboard is unaffected.

## Part F — Choosing a rig

The header has a **Rig** dropdown listing everything in `--fleet`. Selecting a
remote rig prefixes every call with `/rig/<name>/`, which this launcher proxies
onward — so one exposed hostname drives the whole fleet. That matters because
the exposed box (host, `.231`) is not where missions run (Training 1, `.220`).

Consequences worth being deliberate about:

- **An operator here is an operator on every rig in `--fleet`.** The remote
  launchers run auth-off on the LAN and see the proxy as a trusted local caller.
  Keep `--admins` tight. For a read-only public fleet view, list nobody in
  `--admins` and let the LAN keep control.
- This session's Access token and cookies are **stripped** before a request goes
  to another host.
- `/logs` and `/api/rig-log` already read every rig's logs — exposing this box
  exposes the fleet's mission logs, not just its own.

## Security notes

- The `/tower/` proxy is allowlisted to known dashboard ports; `/rig/` to names
  in `--fleet`. Neither can reach an arbitrary host or port.
- Never port-forward `:7000` (or 6001-6048) on the router — the tunnel is the
  only ingress and the rig's IP stays hidden.
- The tower dashboards still bind `0.0.0.0` with `Access-Control-Allow-Origin: *`
  (`cmd/atc/dashboard.go`). That is a LAN exposure the tunnel neither creates
  nor fixes; binding them to `127.0.0.1` is the follow-up.
- Run the launcher as a service. It has a habit of dying silently while the ATC
  roles keep serving — survivable on the LAN, an outage once people rely on the
  public URL.
- Rotate the Discord client secret / bot token if the Worker repo or KV leaks.

Sources: Cloudflare One generic-OIDC docs, Erisa/discord-oidc-worker README.
