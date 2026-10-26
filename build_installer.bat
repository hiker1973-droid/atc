@echo off
:: Builds dist\Readback-Setup.exe: the installer for a PC without the vSFG-7
:: setup (cmd\installer). It carries fresh builds of atc.exe and launcher.exe;
:: nothing in this folder's own atc.exe / launcher.exe is touched.
:: product_version.txt is the version customers see and updates compare.
cd /d %~dp0
for /f %%v in ('git describe --tags --always --dirty') do set VER=%%v
set /p PV=<product_version.txt
echo Building Readback-Setup %PV% (%VER%)
:: The #bug-reports webhook for Help > Report a problem's Send button, from a
:: file outside the repo (never committed). Without it the page only saves the zip.
set BW=
if exist "%USERPROFILE%\readback-bugreport-webhook.txt" set /p BW=<"%USERPROFILE%\readback-bugreport-webhook.txt"
set LDF=-X main.productVersion=%PV%
if defined BW (set "LDF=%LDF% -X main.bugWebhook=%BW%" & echo Report a problem: Send button included) else echo Report a problem: no webhook file, Save only
go build -o cmd\installer\payload\atc.exe .\cmd\atc || goto :fail
go build -ldflags "%LDF%" -o cmd\installer\payload\launcher.exe .\cmd\launcher || goto :fail
copy /y "cmd\launcher\help\Readback ATC *.pdf" cmd\installer\payload\ >nul || goto :fail
if not exist dist mkdir dist
:: -H windowsgui: the installer shows its own window (cmd\installer\gui.go), no console.
go build -ldflags "%LDF% -X main.version=%VER% -H windowsgui" -o dist\Readback-Setup.exe .\cmd\installer || goto :fail
echo Built dist\Readback-Setup.exe
exit /b 0
:fail
echo BUILD FAILED
exit /b 1
