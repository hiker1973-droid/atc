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
:: RED side, ON DEMAND (operator 2026-10-22, Day, BFM Venom vs Raider: "a red Command
:: 280.00 ... Red will be using mozdok airfield", "Mozdok tower is 266", "tonight only
:: and maybe used in the future but not all the time"). NOT called by
:: start_region_caucasus.bat: start it from the dashboard when red flies. An Update
:: stops it with the rest of the Caucasus region and does NOT bring it back.
::
:: --coalition red: registers and transmits on SRS as red (coalition 1).
:: Red Command 280.0 (BFM mission: Hornet COMM2 CH4, Viper COMM1 CH3), Lisa (operator
:: 2026-10-22 "do lisa for red command"; was Gideon; OpenAI fallback alloy): check-in, radio check,
:: fence in/out, RTB and recovery handoff to the RED fields only; no tasking, ranges,
:: tankers, JTAC, GCI or Marshal - those are blue's.
::
:: The red fields are the towers set to Red with the Side switch on their dashboard
:: card (sides.json, operator 2026-10-23). Mozdok Tower now runs with the Caucasus
:: towers, blue for the Training mission: for a BFM night set Mozdok to Red, which
:: restarts it red on 266.0 handing departures to Red Command 280.0.
start "vSFG-7 Red Command (Caucasus)" cmd /c "%~dp0atc.exe --command-only --coalition red --map caucasus --command-freq 280.0 --command-name Red-Command --command-voice alloy --elevenlabs-command-voice bG3bxTbEPR7EZE4M0kcC --handoff-marshal-freq 0 --srs-addr %SRS% --eam-password %EAM% --tacview-addr %TACVIEW% %MIZ_FLAG% --pprof-port 7776 --log-level %LOG%"
