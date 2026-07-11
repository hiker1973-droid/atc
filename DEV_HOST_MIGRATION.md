# Moving SkyeyeATC Dev from the VM to the Host

Goal: stand up a working SkyeyeATC dev + build environment directly on the host
machine, retiring the VM. This is a checklist you run **once** on the host.

> Repo: `https://github.com/hiker1973-droid/atc.git` · default branch `main`
> Working dir on this rig: `C:\SkyeyeATC`

---

## 0. Before you leave the VM (do NOT skip)

Anything not committed and pushed only exists on the VM. As of the last check the
VM's `main` was **ahead of origin by 1 commit**, plus several untracked local files.

```bash
# On the VM
git status                 # see what's uncommitted / untracked
git log origin/main..main  # see local commits not on origin
git fetch origin           # make sure nothing else landed from another box
git push origin main       # push your work up
```

**Untracked local-only files that git will NOT carry** (copy these to the host by
hand — USB / share / scp — or decide to commit them; see §5):

- `config.yml`
- `launcher.ps1`
- `check_env.bat`
- `stop_marshal.bat`
- `start_marshal_test.bat`  *(dev-only, local — intentionally never pushed)*
- kneeboard image(s): `*.jpg`, plus the `CA KNEEBOARD/` and `PG KNEEBOARD/`
  directories (gitignored — large binaries kept local; needed for map wiring)
- any SRS `*.cfg` / `*.lnk` (per-rig, gitignored)
- `.claude/settings.local.json` (per-machine Claude Code state — optional)

Also record the **current env-var values** from the VM so you can reproduce them
(§4): `SRS_EAM`, `SKYEYE_SRS`, `SKYEYE_TACVIEW`, `SKYEYE_MIZ`, `OPENAI_API_KEY`.

---

## 1. Host prerequisites

Install / verify on the host:

- [ ] **Go 1.26+** — `go version` (VM was on `go1.26.1`)
- [ ] **git** — `git --version`
- [ ] **SkyEye** cloned to `C:\Skyeye` — the build depends on it being there
- [ ] DCS World + SRS Server + Tacview (for actually *running* roles, not for building)

---

## 2. Clone the repo

```bash
cd /c            # or wherever you want it; C:\SkyeyeATC keeps paths matching the docs
git clone https://github.com/hiker1973-droid/atc.git SkyeyeATC
cd SkyeyeATC
```

If you use HTTPS you'll need a GitHub Personal Access Token (or the Git Credential
Manager) the first time you push.

Set your git identity on the host:

```bash
git config user.name  "hiker1973"
git config user.email "hiker1973@gmail.com"
```

---

## 3. Restore the local-only files (§0 list)

Copy the files/dirs you saved from the VM back into `C:\SkyeyeATC` at the same
relative paths. The critical ones for a working setup:

- [ ] `config.yml`, `launcher.ps1`, `check_env.bat`, `stop_marshal.bat`
- [ ] `CA KNEEBOARD/` and `PG KNEEBOARD/` (only if you'll wire/verify those maps)
- [ ] SRS `*.cfg` if this host will also run SRS client roles

`logs/` and `atis_cache/` are gitignored and **regenerate on first run** — don't
copy them.

---

## 4. Environment variables

All `start_*.bat` scripts fail fast if these are missing. Set with `setx`, then
open a **new** shell (setx doesn't affect the current one):

```bat
setx OPENAI_API_KEY  sk-proj-...
setx SRS_EAM         <rotate-per-mission>
setx SKYEYE_SRS      localhost:5008
setx SKYEYE_TACVIEW  localhost:42676
setx SKYEYE_MIZ      "C:\full\path\to\this-mission.miz"   REM optional; unset = newest .miz in miz-dir
```

Symptom of a missing var: a script aborts at the top with
`ERROR: <VAR> env var not set` (except `SKYEYE_MIZ`, which is optional).

Verify with `check_env.bat` (once restored) or just re-open a shell and echo them.

---

## 5. Decide what to commit (optional cleanup)

Some of the local-only files arguably *should* be tracked so the next machine move
is painless. Candidates to consider committing (get user sign-off first — this is
a prod repo):

- `config.yml`, `launcher.ps1`, `check_env.bat`, `stop_marshal.bat` — genuinely
  useful, not machine-specific → good candidates to track.
- `start_marshal_test.bat` — **keep local**, do not push (broadcasts test traffic
  on the carrier freq; dev-rig only).
- Kneeboard images / SRS cfg — leave gitignored (large binaries / per-rig).

If you commit any of these, do it on `main`, then push.

---

## 6. Build

```bat
build.bat
```

Produces `atc.exe` and `launcher.exe`. If `build.bat` fails with
"being used by another process", kill any running `atc.exe` / `launcher.exe` first.

Sanity check the binaries exist and run:

```bash
./atc.exe --help        # flags load
./launcher.exe --help
```

---

## 7. First run / verify

```
start_launcher.bat      # dashboard at http://localhost:7000/
start_atis.bat          # 5 ATIS stations
start_towers.bat        # OMDM/OMAM/OMAL, dashboards on 6001/6002/6003
```

or `start_all.bat` for the full stack (ATIS → towers → marshal → command →
deckboss → launcher).

Confirm: launcher dashboard loads, `logs/` starts filling, ATIS begins
broadcasting, no `level=error` / `SRS disconnected` at startup.

---

## 8. Git workflow going forward

The host is now your dev box. Origin gets pushes from more than one machine, so:

```bash
git fetch origin        # ALWAYS before you start / before you push
git status
# ... work ...
git add -p
git commit -m "..."
git push origin main    # only when the change is validated — this repo is prod-adjacent
```

Rules of thumb for this repo:
- **Fetch before pushing** — origin is not guaranteed to be canonical.
- **Don't push broken commits** — roles here hit production fast.
- Align on env-var / flag / schema changes before making them.

---

## Quick checklist

- [ ] VM work committed + pushed (incl. the 1 unpushed commit)
- [ ] Local-only files copied off the VM
- [ ] Go 1.26+, git, `C:\Skyeye` on the host
- [ ] Repo cloned, git identity set
- [ ] Local-only files restored
- [ ] Env vars set (new shell opened)
- [ ] `build.bat` produces both binaries
- [ ] Launcher + a role start clean, logs populate
