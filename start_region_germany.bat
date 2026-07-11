@echo off
:: vSFG-7 — Cold War Germany region launcher (dashboard-driven).
:: Starts ATIS (English + German) -> Towers (8 recovery bases) -> Command.
:: Does NOT start the launcher (already running when the dashboard fires this).
:: Land-based theatre — no Marshal/Deckboss.
:: For the full cold-boot that also opens the dashboard, use start_all_germany.bat.
cd /d %~dp0

echo [vSFG-7] Launching Germany ATIS...
call "%~dp0start_atis_germany.bat"

echo [vSFG-7] Launching Germany Towers...
call "%~dp0start_towers_germany.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command_germany.bat"

echo [vSFG-7] Germany region launched.
