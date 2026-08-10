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

:: Voice: shimmer (female, 2026-08-06 — was ash, which is male). Bright and
:: cutting, which suits a deck boss shouting over jet noise on a handset;
:: fable was rejected earlier for being soft and storyteller-ish.
::
:: shimmer is shared with Al Dhafra Tower (OMAM, 251.1) — the one duplicate in
:: the stack. Every other female OpenAI voice is already spoken for: nova is
:: OMDM Tower (same process as Deckboss, so the worst possible collision),
:: coral is Marshal (the other carrier voice), sage is Command (everyone
:: monitors it). Al Dhafra is a land field a carrier pilot on 128.6 will not
:: have tuned, so it is the safest voice to double up on.
::
:: Delivery style and rate come from --voice-style-deckboss and speedDeckboss
:: in main.go — both gender-neutral, so no change needed there.
start "Deckboss" cmd /c "%~dp0atc.exe --airfield OMDM --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% --deckboss-freq 128.6 --deckboss-voice shimmer --no-atis --dashboard-port 6005 %MIZ_FLAG% --log-level %LOG%"
