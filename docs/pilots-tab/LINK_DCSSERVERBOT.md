# Link this DCS server's DCSServerBot database to the SkyEye launcher

**For Claude Code running on a vSFG-7 DCS server** (Day .220, Night .221, Foothold .222).
Goal: the SkyEye launcher's **Pilots tab** and the nightly **Pilot Report** read player
statistics from DCSServerBot's PostgreSQL database. The launcher needs one environment
variable, `SKYEYE_DCSSB_DB`, holding a postgres URL for a **read-only** login.

The launcher code that uses it is `C:\SkyeyeATC\cmd\launcher\pilots.go`. It only runs
`SELECT` on `statistics`, `missions`, `players` and the greenieboard's `traps`.

## Ground rules

- **Do not** restart the launcher, press Update, or stop or start any role or DCS. The
  operator does that.
- **Do not** edit DCSServerBot's config, and do not restart DCSServerBot or PostgreSQL.
- **Ask the operator first** before changing PostgreSQL's network config (`pg_hba.conf`,
  `postgresql.conf`). This is only needed in the shared-database case (step 6).
- **Never** paste any password into your final report to the operator. Masking it as
  `***` is fine.
- **Everything is reversible:** the login can be dropped with
  `DROP OWNED BY skyeye_read; DROP ROLE skyeye_read;`, and the variable removed with
  `reg delete HKCU\Environment /v SKYEYE_DCSSB_DB /f`.

## 1. Find DCSServerBot and its database URL

1. Find the install folder. The running bot is a `python.exe` with `run.py` in its command
   line:
   ```powershell
   Get-CimInstance Win32_Process -Filter "Name='python.exe' OR Name='pythonw.exe'" |
     Where-Object CommandLine -match 'run\.py|DCSServerBot' | Select-Object ProcessId, CommandLine
   ```
   If nothing is running, look for `config\nodes.yaml` under `C:\DCSServerBot`,
   `%USERPROFILE%\DCSServerBot`, or `C:\Users\Administrator\DCSServerBot`.
2. Read `config\nodes.yaml`; older installs use `config\main.yaml`. Under the `database:`
   key there is a URL:
   ```yaml
   database:
     url: postgres://dcsserverbot:SECRET@127.0.0.1:5432/dcsserverbot
   ```
   Note the **user** (`dcsserverbot`), **host** (`127.0.0.1`), **port** (`5432`) and
   **database name** (`dcsserverbot`). Newer DCSServerBot versions write the literal word
   `SECRET` in place of the password and keep the real one elsewhere. You don't need it if
   you have the `postgres` superuser (step 3).
3. Decide which case this server is:
   - **Local:** the host is `127.0.0.1` or `localhost`. This server has its own database,
     so do steps 2–5 here.
   - **Shared:** the host is another machine's IP. All servers may share one database.
     Create the login (step 3) **once, on the database's own machine**, then on each
     server only do steps 4–5. Read step 6 first.

## 2. Find psql

```powershell
Get-ChildItem "C:\Program Files\PostgreSQL\*\bin\psql.exe" | Select-Object -Expand FullName
```
Use the newest one. If PostgreSQL isn't installed on this box, you're in the shared case:
create the login from the machine that has it.

## 3. Create the read-only login (once per database)

Connecting needs the PostgreSQL superuser (`postgres`) password, which was set when
PostgreSQL was installed. **Ask the operator for it.** Then pick a strong password for the
new login. Generate one; it must not contain `@ : / ? #`, so letters and digits only is
safest.

```powershell
$psql = "C:\Program Files\PostgreSQL\<ver>\bin\psql.exe"
$env:PGPASSWORD = "<postgres superuser password from the operator>"
& $psql -h 127.0.0.1 -U postgres -d <database name> -v ON_ERROR_STOP=1 -c @"
CREATE ROLE skyeye_read LOGIN PASSWORD '<new password>';
GRANT CONNECT ON DATABASE <database name> TO skyeye_read;
GRANT USAGE ON SCHEMA public TO skyeye_read;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO skyeye_read;
ALTER DEFAULT PRIVILEGES FOR ROLE <dcsserverbot user> IN SCHEMA public GRANT SELECT ON TABLES TO skyeye_read;
"@
Remove-Item Env:PGPASSWORD
```

- If `skyeye_read` **already exists** (another server's session made it, in the shared
  case), don't recreate it. Ask the operator for its password, or
  `ALTER ROLE skyeye_read PASSWORD '...'` and then update every server's variable.
- **If nobody knows the `postgres` password:** stop and tell the operator. The fallback is
  to put DCSServerBot's own database login in `SKYEYE_DCSSB_DB`. The launcher still only
  reads, but that login could write. That's the operator's call, not yours.

Check the login works and can read:
```powershell
$env:PGPASSWORD = "<new password>"
& $psql -h <db host> -U skyeye_read -d <database name> -c "SELECT count(*) AS slots, max(hop_on) AS newest FROM statistics;"
& $psql -h <db host> -U skyeye_read -d <database name> -c "SELECT count(*) AS traps FROM traps;"
Remove-Item Env:PGPASSWORD
```
The first query must return a row count and a recent `newest`. The second may fail with
"relation traps does not exist" if the greenieboard plugin isn't installed; that's fine,
the Pilots tab then just shows no traps.

## 4. Set the variable for the launcher's Windows user

The launcher reads `SKYEYE_*` variables from **HKCU\Environment of the user it runs as**.
Find that user:
```powershell
Get-CimInstance Win32_Process -Filter "Name='launcher.exe'" | ForEach-Object { (Invoke-CimMethod -InputObject $_ -MethodName GetOwner).User }
```
**As that user**, run:
```
setx SKYEYE_DCSSB_DB "postgres://skyeye_read:<new password>@<db host>:<port>/<database name>"
```
Expect `SUCCESS: Specified value was saved.` If you're running as a different user, tell
the operator to run the `setx` line themselves in a Command Prompt as the launcher's user.
Don't put the password in your report.

## 5. Hand over to the operator

The running launcher only picks up the variable when the operator presses **Update** on
this server in the dashboard. Tell the operator to do that. After they have, verify:
```powershell
(Invoke-WebRequest -UseBasicParsing "http://127.0.0.1:7000/api/pilots-raw?days=30").Content |
  ConvertFrom-Json | Select-Object configured, err, @{n='slots';e={$_.sessions.Count}}, @{n='traps';e={$_.traps.Count}}
```
Expected: `configured = True`, `err` empty, and `slots` greater than 0 if anyone flew in
the last 30 days. Then the **Pilots** tab (http://127.0.0.1:7000/pilots) shows this
server's pilots.

## 6. Shared database only: let the other servers reach it

**Ask the operator before doing any of this.** On the machine that runs PostgreSQL:
- `postgresql.conf` must have `listen_addresses` covering the LAN (e.g. `'*'`). Check with
  `SHOW listen_addresses;`.
- `pg_hba.conf` needs a line such as
  `host  <database name>  skyeye_read  192.168.1.0/24  scram-sha-256`.
- Applying either needs a PostgreSQL reload (`SELECT pg_reload_conf();`). The listen
  change needs a restart, which DCSServerBot must survive. That's why the operator decides
  when.
- The Windows firewall must allow TCP 5432 from 192.168.1.0/24.

From another server, test with the `psql` command in step 3, pointed at the database
machine's IP.

## Report back to the operator

Report these, without any password:
- DCSServerBot folder, database host, database name, and local or shared.
- Whether `skyeye_read` was created here, or already existed.
- The `slots` / `newest` from the step 3 check, and whether the `traps` table exists.
- Whether `SKYEYE_DCSSB_DB` was set, and for which Windows user.
- Anything left for the operator: pressing Update, the step 6 network changes, or a
  password they need to supply.
