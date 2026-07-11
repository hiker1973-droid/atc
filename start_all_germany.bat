@echo off
:: vSFG-7 — single-shot launcher for the Cold War Germany theatre.
:: Order: ATIS -> Towers (8 recovery bases) -> Command -> Dashboard
:: Set SKYEYE_MIZ to the Germany mission .miz before running.
:: NOTE: land-based theatre — NO carrier (Marshal/Deckboss) per the Germany
:: presets card ("LAND-BASED — NO CARRIER / MARSHAL / DECKBOSS").
:: Towers broadcast on VHF (COMM1 preset), ATIS on UHF in English + German.
cd /d %~dp0

echo [vSFG-7] Launching Germany ATIS...
call "%~dp0start_atis_germany.bat"

echo [vSFG-7] Launching Germany Towers...
call "%~dp0start_towers_germany.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command_germany.bat"

echo [vSFG-7] Launching Dashboard...
call "%~dp0start_launcher.bat"

echo [vSFG-7] Germany roles launched.
