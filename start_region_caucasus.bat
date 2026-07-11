@echo off
:: vSFG-7 — Caucasus (Black Sea) region launcher (dashboard-driven).
:: Starts ATIS -> Towers (Batumi, Kobuleti, Senaki, Kutaisi) -> Command.
:: Does NOT start the launcher (already running when the dashboard fires this).
:: Land/sea theatre — carrier roles (Marshal/Deckboss) not wired for CA yet.
:: For the full cold-boot that also opens the dashboard, use start_all_caucasus.bat.
cd /d %~dp0

echo [vSFG-7] Launching Caucasus ATIS...
call "%~dp0start_atis_caucasus.bat"

echo [vSFG-7] Launching Caucasus Towers...
call "%~dp0start_towers_caucasus.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command_caucasus.bat"

echo [vSFG-7] Caucasus region launched.
