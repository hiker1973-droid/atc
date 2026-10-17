@echo off
:: vSFG-7 — Caucasus (Black Sea) region launcher (dashboard-driven).
:: Starts ATIS -> Towers (Batumi, Kobuleti, Senaki, Kutaisi, Tbilisi) -> Command ->
:: Marshal (306.300) -> Deckboss (128.600).
:: Does NOT start the launcher (already running when the dashboard fires this).
:: For the full cold-boot that also opens the dashboard, use start_all_caucasus.bat.
::
:: Carrier freqs by operator ruling: Marshal 306.300 (2026-09-20, COMM1 CH3
:: "CVN-72 AI MARSHALL" -- the AI Marshal on the card is this bot; 306.200 stays
:: clear for a live human Marshal). Deckboss stays 128.600 as in every theatre
:: (2026-09-15). start_marshal_caucasus.bat is the authority on the Marshal freq
:: and carries the full history -- check there before trusting this line. Do not
:: start the PG start_marshal.bat / start_deckboss.bat on a Black Sea mission.
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

echo [vSFG-7] Caucasus region launched.
