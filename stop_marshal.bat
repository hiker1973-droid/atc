@echo off
REM Stops only the Marshal-only atc.exe process. Tower / ATIS / other roles untouched.

powershell -NoProfile -Command "$m = Get-CimInstance Win32_Process -Filter \"name='atc.exe'\" | Where-Object { $_.CommandLine -like '*marshal-only*' }; if ($m) { foreach ($p in $m) { Stop-Process -Id $p.ProcessId -Force; Write-Host \"Killed Marshal PID $($p.ProcessId)\" } } else { Write-Host 'No marshal-only atc.exe running' }"
