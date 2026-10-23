@echo off
:: Publishes a Readback ATC release: builds Readback-Setup.exe and puts it on the
:: public releases repo, where every installed copy's daily check finds it.
::   1. Raise the number in product_version.txt (e.g. 0.1.0 -> 0.2.0).
::   2. Write what changed in docs\releases\<version>.md (shown in the update offer).
::   3. Commit, then run this.
cd /d %~dp0
set REPO=hiker1973-droid/readback-atc-releases
set /p PV=<product_version.txt
if not exist docs\releases\%PV%.md (
    echo Write docs\releases\%PV%.md first: it is the "what's new" customers see.
    exit /b 1
)
git diff --quiet HEAD -- || (
    echo Commit your changes first, so the release is exactly a commit.
    exit /b 1
)
call "%~dp0build_installer.bat" || exit /b 1
gh release create v%PV% dist\Readback-Setup.exe "cmd\launcher\help\Readback ATC Installation Guide.pdf" "cmd\launcher\help\Readback ATC User Manual.pdf" --repo %REPO% --title "Readback ATC %PV%" --notes-file docs\releases\%PV%.md || goto :fail
git tag -a readback-v%PV% -m "Readback ATC %PV%" && git push origin readback-v%PV%
echo Published Readback ATC %PV%.
exit /b 0
:fail
echo PUBLISH FAILED
exit /b 1
