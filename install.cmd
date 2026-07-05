@echo off
setlocal enabledelayedexpansion

echo Installing Goo AI CLI for Windows...

:: Execute PowerShell install script inline with Bypass execution policy
powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/kingjethro999/goo/main/install.ps1 | iex"

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo [ERROR] Installation failed. Please check your internet connection or run PowerShell manually:
    echo irm https://raw.githubusercontent.com/kingjethro999/goo/main/install.ps1 ^| iex
    exit /b %ERRORLEVEL%
)

endlocal
