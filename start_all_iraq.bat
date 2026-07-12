@echo off
:: vSFG-7 — single-shot launcher for the Iraq theatre.
:: Order: ATIS -> Towers (9 recovery bases) -> Command -> Dashboard
:: Set SKYEYE_MIZ to the Iraq mission .miz before running.
:: NOTE: land-based theatre — NO carrier (Marshal/Deckboss). Towers broadcast
:: on UHF (COMM1 preset / DCS default), ATIS on VHF-band 230.x in English +
:: Arabic. Only Al Asad / Al Sahra / Al Salam / Balad carry an ATIS station.
cd /d %~dp0

echo [vSFG-7] Launching Iraq ATIS...
call "%~dp0start_atis_iraq.bat"

echo [vSFG-7] Launching Iraq Towers...
call "%~dp0start_towers_iraq.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command_iraq.bat"

echo [vSFG-7] Launching Dashboard...
call "%~dp0start_launcher.bat"

echo [vSFG-7] Iraq roles launched.
