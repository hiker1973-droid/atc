@echo off
cd /d C:\SkyeyeATC
if not defined SKYEYE_SRS (
    echo ERROR: SKYEYE_SRS env var not set. Run: setx SKYEYE_SRS ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
set SRS=%SKYEYE_SRS%
if not defined SRS_EAM (
    echo ERROR: SRS_EAM env var not set. Run: setx SRS_EAM ^<password^>  then open a new cmd.
    pause
    exit /b 1
)
set EAM=%SRS_EAM%
if not defined SKYEYE_TACVIEW (
    echo ERROR: SKYEYE_TACVIEW env var not set. Run: setx SKYEYE_TACVIEW ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
set TACVIEW=%SKYEYE_TACVIEW%
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"

:: Syria / Eastern Med Deckboss -- 128.600, matching the squadron kneeboard
:: (user ruling 2026-09-02). This SUPERSEDES the 306.200 in SYRIA_PLAN.md
:: section 3, which is now stale for this role.
::
:: Known and accepted: 128.600 is SHARED with the mission's own CVN-72 AI ATC
:: (moved off Foothold's stock 272.000), so we transmit alongside the DCS AI
:: controller rather than on a clear channel. The kneeboard is the authority --
:: pilots were tuned to 128.600 and heard nothing while we sat on 306.200.
::
:: LSO 128.100 is on the squadron card but atc.exe has no LSO role -- Deckboss
:: and Marshal only ever hand off to it verbally. Nothing to launch for it.
::
:: --airfield LCRA (Akrotiri): --deckboss-freq makes this a deckboss-only
:: instance and the tower srsLoop is skipped (cmd/atc/main.go), so no duplicate
:: Akrotiri Tower registers on 252.000. It does set the log slug, so Deckboss
:: events land in atc-lcra.log alongside Akrotiri Tower -- filter on
:: Deckboss/128.6 when monitoring, same caveat as PG's atc-omdm.log.
::
:: Voice shimmer, same as PG Deckboss: only one map runs at a time.
start "Deckboss (Syria)" cmd /c "%~dp0atc.exe --airfield LCRA --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --deckboss-freq 128.6 --deckboss-voice shimmer --no-atis --dashboard-port 6005 %MIZ_FLAG% --log-level %LOG%"
