@echo off
:: vSFG-7 -- Kola region launcher (dashboard-driven).
:: Operation WHITE NIGHT. Starts ATIS -> Tower (the field in play: Bodo for mission 1) ->
:: Command -> Marshal (306.300) -> Deckboss (128.600).
:: Does NOT start the launcher (already running when the dashboard fires this).
:: For the full cold-boot that also opens the dashboard, use start_all_kola.bat.
:: Set SKYEYE_MIZ to the WHITE NIGHT mission .miz before running.
cd /d %~dp0

echo [vSFG-7] Launching Kola ATIS...
call "%~dp0start_atis_kola.bat"

echo [vSFG-7] Launching Kola Towers...
call "%~dp0start_towers_kola.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command_kola.bat"

echo [vSFG-7] Launching Marshal (306.300)...
call "%~dp0start_marshal_kola.bat"

echo [vSFG-7] Launching Deckboss (128.600)...
call "%~dp0start_deckboss_kola.bat"

echo [vSFG-7] Kola region launched.
