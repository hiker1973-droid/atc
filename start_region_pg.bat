@echo off
:: vSFG-7 — Persian Gulf region launcher (dashboard-driven).
:: Starts every PG role: ATIS -> Towers (Minhad, Dhafra, Al Ain) -> Marshal ->
:: Command -> Deckboss. Does NOT start the launcher (it is already running when
:: the dashboard's Start Region button fires this). For the full cold-boot that
:: also opens the dashboard, use start_all.bat instead.
cd /d %~dp0

echo [vSFG-7] Launching PG ATIS...
call "%~dp0start_atis.bat"

echo [vSFG-7] Launching PG Towers...
call "%~dp0start_towers.bat"

echo [vSFG-7] Launching Marshal...
call "%~dp0start_marshal.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command.bat"

echo [vSFG-7] Launching Deckboss...
call "%~dp0start_deckboss.bat"

echo [vSFG-7] Persian Gulf region launched.
