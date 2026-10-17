@echo off
:: vSFG-7 — single-shot launcher for the Caucasus (Black Sea) theatre.
:: Order: ATIS -> Towers (Batumi, Kobuleti, Senaki, Kutaisi, Tbilisi) -> Command ->
:: Marshal (306.300) -> Deckboss (128.600) -> Dashboard
:: Set SKYEYE_MIZ to the Caucasus mission .miz before running.
:: Carrier freqs by operator ruling 2026-09-15 — see CAUCASUS_PLAN.md section 4.
:: Marshal moved to 306.300 by the 2026-09-20 ruling; start_marshal_caucasus.bat
:: is the authority and carries the history. Do not trust this line over that one.
cd /d %~dp0

echo [vSFG-7] Launching Caucasus ATIS...
call "%~dp0start_atis_caucasus.bat"

echo [vSFG-7] Launching Caucasus Towers...
call "%~dp0start_towers_caucasus.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command_caucasus.bat"

echo [vSFG-7] Launching Marshal (306.300)...
call "%~dp0start_marshal_caucasus.bat"

echo [vSFG-7] Launching Deckboss (128.600)...
call "%~dp0start_deckboss_caucasus.bat"

echo [vSFG-7] Launching Paddles (128.300)...
call "%~dp0start_lso_caucasus.bat"

echo [vSFG-7] Launching Dashboard...
call "%~dp0start_launcher.bat"

echo [vSFG-7] Caucasus roles launched.
