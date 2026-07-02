@echo off
:: vSFG-7 — single-shot launcher for the Caucasus (Black Sea) theatre.
:: Order: ATIS -> Towers (Batumi, Kobuleti, Senaki, Kutaisi) -> Command -> Dashboard
:: Set SKYEYE_MIZ to the Caucasus mission .miz before running.
:: NOTE: carrier ops (Marshal/Deckboss) are NOT launched here yet — the Black Sea
:: kneeboard assigns different carrier freqs (Deckboss 306.2, Airboss 306.3,
:: Marshall 306.1, LSO 128.6) than PG. Decide the role->freq mapping, then add
:: start_marshal / start_deckboss with the CA --marshal-freq / --deckboss-freq.
cd /d %~dp0

echo [vSFG-7] Launching Caucasus ATIS...
call "%~dp0start_atis_caucasus.bat"

echo [vSFG-7] Launching Caucasus Towers...
call "%~dp0start_towers_caucasus.bat"

echo [vSFG-7] Launching Command...
call "%~dp0start_command_caucasus.bat"

echo [vSFG-7] Launching Dashboard...
call "%~dp0start_launcher.bat"

echo [vSFG-7] Caucasus roles launched.
