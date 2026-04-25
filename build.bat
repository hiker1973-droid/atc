@echo off
REM vsfg7-atc build script
REM Run from C:\SkyeyeATC
REM Requires Go 1.26+ and SkyEye cloned at C:\Skyeye

echo Building vsfg7-atc...
go build -o atc.exe ./cmd/atc
if %ERRORLEVEL% neq 0 (
    echo atc.exe build failed.
    exit /b %ERRORLEVEL%
)
echo Building launcher...
go build -o launcher.exe ./cmd/launcher
if %ERRORLEVEL% neq 0 (
    echo launcher.exe build failed.
    exit /b %ERRORLEVEL%
)
echo Build successful -- atc.exe and launcher.exe ready.