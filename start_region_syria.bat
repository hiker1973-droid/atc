@echo off
:: vSFG-7 - Syria (Eastern Med) region launcher (dashboard-driven).
:: Starts ATIS (English + Arabic) -> Towers (8 recovery bases) -> Command.
:: Does NOT start the launcher (already running when the dashboard fires this).
::
:: CARRIER OPS ARE RUN ON THIS MAP: CVN-72 "ABE", TACAN 72X / ICLS 12 / Link 4
:: 336.000. Marshal 306.300 (since 2026-09-19), Deckboss 128.600 (shared with the
:: mission's own AI carrier ATC, operator ruling 2026-09-02), Paddles 128.300
:: (operator 2026-10-18: "Foothold can use deckboss, Marshall and paddles"; the
:: same frequency as the Black Sea, not the card's LSO 128.100).
::
:: SRS on the Foothold VM is :5002, not the Training rig's :5008.
cd /d %~dp0

echo [vSFG-7] Launching Syria ATIS...
call "%~dp0start_atis_syria.bat"

echo [vSFG-7] Launching Syria Towers...
call "%~dp0start_towers_syria.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command_syria.bat"

echo [vSFG-7] Launching Marshal (306.300)...
call "%~dp0start_marshal_syria.bat"

echo [vSFG-7] Launching Deckboss (128.600)...
call "%~dp0start_deckboss_syria.bat"

echo [vSFG-7] Launching Paddles (128.300)...
call "%~dp0start_lso_syria.bat"

echo [vSFG-7] Launching GCI Overlord (266.000)...
:: In the region since 2026-10-21 (skyeye.exe is off the servers).
call "%~dp0start_gci_syria.bat"

echo [vSFG-7] Syria region launched.
