@echo off
:: vSFG-7 - Syria (Eastern Med) region launcher (dashboard-driven).
:: Starts ATIS (English + Arabic) -> Towers (8 recovery bases) -> Command.
:: Does NOT start the launcher (already running when the dashboard fires this).
::
:: CARRIER OPS ARE RUN ON THIS MAP: CVN-72 "ABE", TACAN 72X / ICLS 12 / Link 4
:: 336.000. Marshal 306.100, Deckboss 306.200, LSO 128.100 -- the mission's own
:: AI carrier ATC sits on 128.600. Those roles are NOT started here yet; see
:: SYRIA_PLAN.md section 3.
::
:: SRS on the Foothold VM is :5002, not the Training rig's :5008.
cd /d %~dp0

echo [vSFG-7] Launching Syria ATIS...
call "%~dp0start_atis_syria.bat"

echo [vSFG-7] Launching Syria Towers...
call "%~dp0start_towers_syria.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command_syria.bat"

echo [vSFG-7] Syria region launched.
