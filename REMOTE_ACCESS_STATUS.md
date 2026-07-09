# Remote access — setup status / resume here (2026-07-09)

Goal: expose the launcher dashboard publicly, gated by **Discord login**
(restricted to the vSFG-7 guild) via **Cloudflare Tunnel + Cloudflare Access**.
Full runbook: `REMOTE_ACCESS.md`.

## DONE
- **Launcher reverse-proxy** — `/tower/<port>/…` (allowlisted 6001-6005 / 6011-6014).
  UI fetches tower data through it, so only `:7000` needs exposing. Committed +
  pushed (`4f7fbd7`). Verified: `/tower/6013/status` OK, `/tower/9999` → 403.
- **cloudflared** downloaded to `C:\SkyeyeATC\cloudflared.exe` (v2026.7.x,
  standalone — no install needed).

## BLOCKED / IN PROGRESS — the tunnel
- Tried `cloudflared tunnel login` (cert method). It **failed**: cloudflared on
  the rig couldn't fetch the cert (`error="Failed to fetch resource"`), the
  browser-download fallback produced an **empty cert.pem**. Cert method abandoned.
- **Pivoted to the dashboard-managed (token) tunnel** — cleaner, no cert.pem / no
  config.yml, and installs cloudflared as a **service** (also fixes the process
  getting reaped between sessions).

## RESUME HERE  ← next action
1. Cloudflare **Zero Trust → Networks → Tunnels → Create a tunnel** → connector
   **Cloudflared** → name `vsfg7-atc` → Save.
2. On the install step, **Windows** tab: copy the **token** (the long `eyJ…`
   string in the `cloudflared service install <TOKEN>` command). Skip the
   download — cloudflared is already on the rig.
3. Install the connector on the rig (Claude can run this given the token):
   `C:\SkyeyeATC\cloudflared.exe service install <TOKEN>`
4. In the tunnel's **Public Hostnames** → Add: `atc.<yourdomain>` → HTTP →
   `localhost:7000`.
5. Then the **auth gate** (Part C+D of `REMOTE_ACCESS.md`): deploy the
   `Erisa/discord-oidc-worker` shim, add it as an OIDC login method in Access,
   and create an Access application for `atc.<yourdomain>` restricted to the
   vSFG-7 Discord guild.

## Rig process state (may need a bounce next session)
- **Launcher** (`:7000`) keeps getting reaped between sessions — restart with
  `start_launcher.bat` (or the dashboard-managed cloudflared service makes this a
  non-issue once auth is on). Check: is `:7000` listening?
- **Caucasus** towers up (Batumi 260 / Senaki 261 / Kobuleti 262 / Kutaisi 263),
  EN+Russian ATIS. **Persian Gulf left DOWN** (mission is Caucasus).
- Remember: launch multi-tower sets with staggering (SRS drops a simultaneous
  registration — see the SRS gotcha memory).

## Git
- All code pushed through `4f7fbd7`. This status file + any doc tweaks are the
  only local-only changes.
