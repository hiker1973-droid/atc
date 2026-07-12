@echo off
:: vSFG-7 — Iraq region launcher (dashboard-driven).
:: Starts ATIS (English + Arabic) -> Towers (9 recovery bases) -> Command.
:: Does NOT start the launcher (already running when the dashboard fires this).
:: Land-based theatre — no Marshal/Deckboss.
:: For the full cold-boot that also opens the dashboard, use start_all_iraq.bat.
cd /d %~dp0

echo [vSFG-7] Launching Iraq ATIS...
call "%~dp0start_atis_iraq.bat"

echo [vSFG-7] Launching Iraq Towers...
call "%~dp0start_towers_iraq.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command_iraq.bat"

echo [vSFG-7] Iraq region launched.
