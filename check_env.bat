@echo off
REM SkyeyeATC env-var sanity check. Run this in a fresh cmd window.

echo === SkyeyeATC env vars ===
if defined OPENAI_API_KEY (
    echo OPENAI_API_KEY = %OPENAI_API_KEY:~0,12%... [length OK]
) else (
    echo OPENAI_API_KEY = ^<NOT SET^>  -- run: setx OPENAI_API_KEY ^<key^>
)
if defined SRS_EAM      (echo SRS_EAM        = %SRS_EAM%)        else (echo SRS_EAM        = ^<NOT SET^>  -- run: setx SRS_EAM ^<password^>)
if defined SKYEYE_SRS   (echo SKYEYE_SRS     = %SKYEYE_SRS%)     else (echo SKYEYE_SRS     = ^<NOT SET^>  -- run: setx SKYEYE_SRS ^<host:port^>)
if defined SKYEYE_TACVIEW (echo SKYEYE_TACVIEW = %SKYEYE_TACVIEW%) else (echo SKYEYE_TACVIEW = ^<NOT SET^>  -- run: setx SKYEYE_TACVIEW ^<host:port^>)

echo.
echo === SRS reachability ===
if defined SKYEYE_SRS (
    for /f "tokens=1,2 delims=:" %%a in ("%SKYEYE_SRS%") do (
        powershell -NoProfile -Command "try { $c = New-Object Net.Sockets.TcpClient; $c.Connect('%%a', %%b); 'SRS %%a:%%b - REACHABLE' } catch { 'SRS %%a:%%b - UNREACHABLE: ' + $_.Exception.Message }"
    )
) else (
    echo Skipped: SKYEYE_SRS not set.
)

echo.
echo === Tacview reachability ===
if defined SKYEYE_TACVIEW (
    for /f "tokens=1,2 delims=:" %%a in ("%SKYEYE_TACVIEW%") do (
        powershell -NoProfile -Command "try { $c = New-Object Net.Sockets.TcpClient; $c.Connect('%%a', %%b); 'Tacview %%a:%%b - REACHABLE' } catch { 'Tacview %%a:%%b - UNREACHABLE: ' + $_.Exception.Message }"
    )
) else (
    echo Skipped: SKYEYE_TACVIEW not set.
)

echo.
pause
