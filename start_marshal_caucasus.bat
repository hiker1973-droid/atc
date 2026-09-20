@echo off
cd /d C:\SkyeyeATC
if not defined SKYEYE_SRS (
    echo ERROR: SKYEYE_SRS env var not set. Run: setx SKYEYE_SRS ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
set SRS=%SKYEYE_SRS%
if not defined SKYEYE_TACVIEW (
    echo ERROR: SKYEYE_TACVIEW env var not set. Run: setx SKYEYE_TACVIEW ^<host:port^>  then open a new cmd.
    pause
    exit /b 1
)
set TACVIEW=%SKYEYE_TACVIEW%
if not defined SRS_EAM (
    echo ERROR: SRS_EAM env var not set. Run: setx SRS_EAM ^<password^>  then open a new cmd.
    pause
    exit /b 1
)
set EAM=%SRS_EAM%
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"

:: Caucasus / Black Sea Marshal -- CVN-72.
::
:: 306.300 = Hornet COMM1 CH3 "CVN-72 AI MARSHALL" -- that IS this process.
:: Operator ruling 2026-09-20: "306.3 is our SkyeyeATC Marshall; 306.2 is when we
:: have a live human Marshall." The card's "AI" label means the robot controller,
:: i.e. us -- it is not the DCS mission's own AI. So SkyEye transmits on 306.300 and
:: leaves 306.200 clear for a squadron member controlling by voice.
::
:: The 2026-09-19 move to 306.200 read those labels backwards and is reverted here.
:: History: 306.100 (2026-09-15) -> 306.300 (2026-09-18) -> 306.200 (2026-09-19)
:: -> 306.300 (2026-09-20, this ruling). Do not "fix" this back to 306.200.
::
:: --airfield UGSB (Batumi): Marshal is carrier-only and registers no tower, so
:: this sets the weather/divert context and the magnetic variation BRC is
:: spoken with (+7.5 E, DimOn 2020s value, 2026-09-19). Log goes to atc-marshal.log -- --marshal-only wins the
:: log-slug switch in cmd/atc/main.go.
::
:: Voice coral and dashboard 6004 are shared with the PG and Syria Marshals by
:: design: only one map runs at a time.
start "Marshal (Caucasus)" cmd /c "%~dp0atc.exe --marshal-only --airfield UGSB --marshal-freq 306.3 --marshal-voice coral --srs-addr %SRS% --tacview-addr %TACVIEW% --eam-password %EAM% --dashboard-port 6004 %MIZ_FLAG% --log-level %LOG%"
