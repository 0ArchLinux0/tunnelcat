@echo off
REM install-windows.bat - install tunnelcat on Windows, with no
REM install wizard. Just unzip, add to PATH, run.
REM
REM Usage (from PowerShell or cmd):
REM   iwr -Uri https://github.com/0ArchLinux0/tunnelcat/releases/latest/download/windows-amd64.zip -OutFile tunnelcat.zip
REM   Expand-Archive tunnelcat.zip
REM   cd windows-amd64
REM   .\tunnelcat.exe --version
REM
REM Or, to install to C:\Program Files\tunnelcat\ and add to PATH:
REM   .\install-windows.bat
REM
REM What it does:
REM   1. Detects architecture (currently amd64 only).
REM   2. Creates C:\Program Files\tunnelcat\.
REM   3. Copies tunnelcat.exe there.
REM   4. Adds C:\Program Files\tunnelcat\ to the user PATH.
REM
REM After install, open a NEW PowerShell window (so PATH reloads)
REM and run `tunnelcat --version`.

setlocal

if not "%PROCESSOR_ARCHITECTURE%"=="AMD64" (
    echo Error: only AMD64 Windows is supported in this installer.
    echo For ARM64, see the project README.
    exit /b 1
)

set "INSTALL_DIR=%ProgramFiles%\tunnelcat"
set "BIN_NAME=tunnelcat.exe"

if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"

REM Copy the binary.
copy /Y "%BIN_NAME" "%INSTALL_DIR%\%BIN_NAME%" >nul
echo Installed %INSTALL_DIR%\%BIN_NAME%

REM Add to user PATH (HKCU\Environment).
set "OLDPATH=%PATH%"
for /f "tokens=2*" %%A in ('reg query "HKCU\Environment" /v PATH 2^>nul') do set "OLDPATH=%%B"
echo %OLDPATH% | findstr /I /C:"%INSTALL_DIR%" >nul
if errorlevel 1 (
    set "NEWPATH=%OLDPATH%;%INSTALL_DIR%"
    reg add "HKCU\Environment" /v PATH /t REG_EXPAND_SZ /d "%NEWPATH%" /f >nul
    echo Added %INSTALL_DIR% to your user PATH.
    echo Open a NEW PowerShell window for the new PATH to take effect.
) else (
    echo %INSTALL_DIR% is already in your PATH.
)

echo.
echo To verify: open a new PowerShell window and run:
echo   tunnelcat --version

endlocal
