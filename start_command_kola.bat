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
if defined SKYEYE_TACVIEW (
    set TACVIEW_FLAG=--tacview-addr %SKYEYE_TACVIEW%
) else (
    echo NOTE: SKYEYE_TACVIEW not set. Command will run without proactive tower handoff.
    set TACVIEW_FLAG=
)
set LOG=info
set GOMAXPROCS=2
set GOGC=50
set GOMEMLIMIT=256MiB
:: Boot weather follows the loaded mission. The DCS hook writes current_mission.txt on every
:: load (its setx re-pin of SKYEYE_MIZ can't run: os.execute is a stub in DCS hooks, 2026-11-21).
:: Falls back to SKYEYE_MIZ when the file or the .miz it names is missing.
set "CUR_MIZ="
if exist "%~dp0current_mission.txt" set /p CUR_MIZ=<"%~dp0current_mission.txt"
if defined CUR_MIZ if exist "%CUR_MIZ%" set "SKYEYE_MIZ=%CUR_MIZ%"
set MIZ_FLAG=
if defined SKYEYE_MIZ set MIZ_FLAG=--miz-path "%SKYEYE_MIZ%"
:: Command 282.0 is shared across theatres; --map kola makes the proactive handoff
:: scan the Kola field(s) in play instead of the PG ones.
:: --handoff-marshal-freq 306.3: our Marshal (start_marshal_kola.bat). Without the
:: flag the default happens to be 306.3 too, but Syria shipped relying on a default
:: and handed pilots to the wrong Marshal -- so it is stated here.
:: ASSASSIN 1-1 (61.000 FM, North-Lapland: targets 1,2,7) + ASSASSIN 1-2 (62.000 FM, South-Lapland: 3,4,5,6) = the AI target assigner
:: (operator 2026-11-14, "they just assign targets"): --jtac-assign-only names the nets Assassin and drops the 9-line / clearance / BDA flow
:: (cmd\atc\jtac_assign.go). Kola Training only. Targets: cmd\atc\kola_targets.go, generated from the Kola Training mission
:: (KOLA\Kola Training\_build\gen_hitman_go_ko.py). Another Kola mission (White Night) gets "negative tasking".
:: Tankers (2026-11-15, operator: "do not see Texaco, Shell, Mauler in Kola"): Texaco 1-1/2-1/3-1 252.1-3 MPRS basket, Shell 1-1/2-1/3-1 253.1-3 boom,
:: Mauler 1-1 252.6 S-3B recovery tanker (angels/KIAS are fallbacks; Tacview gives the live values).
:: On/off per theatre: launcher Settings > Roles (not an env var).
start "vSFG-7 Command (Kola)" cmd /c "%~dp0atc.exe --command-only --map kola --command-gci-name Darkstar --command-gci-freq 264.0 --command-freq 282.0 --command-name vSFG-7-Command --command-voice onyx --jtac-assign-only --jtac-freq 61.0 --jtac-voice ash --jtac-sector North-Lapland --jtac2-freq 62.0 --jtac2-sector South-Lapland --tanker-net 252.1/Texaco-1-1/24/280/basket/eric --tanker-net 252.2/Texaco-2-1/24/280/basket/hank --tanker-net 252.3/Texaco-3-1/24/280/basket/brian --tanker-net 253.1/Shell-1-1/20/295/boom/charlie --tanker-net 253.2/Shell-2-1/20/295/boom/daniel --tanker-net 253.3/Shell-3-1/20/295/boom/adam --tanker-net 252.6/Mauler-1-1/8/250/basket/gideon --handoff-marshal-freq 306.3 --srs-addr %SRS% --eam-password %EAM% %TACVIEW_FLAG% %MIZ_FLAG% --pprof-port 7770 --log-level %LOG%"
