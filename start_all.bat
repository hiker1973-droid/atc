@echo off
:: vSFG-7 — single-shot launcher for Training 1 live roles.
:: Order: ATIS -> Towers (Minhad, Dhafra, Al Ain) -> Marshal -> Command -> Deckboss
:: Each child script spawns its own cmd window via `start`, so this script returns quickly.
cd /d %~dp0

echo [vSFG-7] Launching ATIS...
call "%~dp0start_atis.bat"

echo [vSFG-7] Launching Towers...
call "%~dp0start_towers.bat"

echo [vSFG-7] Launching Marshal...
call "%~dp0start_marshal.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command.bat"

echo [vSFG-7] Launching Deckboss...
call "%~dp0start_deckboss.bat"

echo [vSFG-7] All roles launched.
