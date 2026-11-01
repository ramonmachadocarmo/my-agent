@echo off
rem Installs my-agent (double-click, or run from cmd). Arguments are passed to
rem install.ps1, e.g.:  install.cmd -Uninstall
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1" %*
if errorlevel 1 (
  echo.
  pause
  exit /b 1
)
rem keep the window open when started by double-click
echo %cmdcmdline% | find /i "%~0" >nul && pause
exit /b 0
