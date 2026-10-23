@echo off
:: Builds dist\SkyEye-Setup.exe: the installer for a PC without the vSFG-7
:: setup (cmd\installer). It carries fresh builds of atc.exe and launcher.exe;
:: nothing in this folder's own atc.exe / launcher.exe is touched.
cd /d %~dp0
for /f %%v in ('git describe --tags --always --dirty') do set VER=%%v
echo Building SkyEye-Setup %VER%
go build -o cmd\installer\payload\atc.exe .\cmd\atc || goto :fail
go build -o cmd\installer\payload\launcher.exe .\cmd\launcher || goto :fail
if not exist dist mkdir dist
go build -ldflags "-X main.version=%VER%" -o dist\SkyEye-Setup.exe .\cmd\installer || goto :fail
echo Built dist\SkyEye-Setup.exe
exit /b 0
:fail
echo BUILD FAILED
exit /b 1
