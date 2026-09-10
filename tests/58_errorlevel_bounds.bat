@echo off
call :fail2
if errorlevel 0 echo lvl0-yes
if errorlevel 1 echo lvl1-yes
if errorlevel 2 echo lvl2-yes
if errorlevel 3 echo lvl3-yes
if not errorlevel 3 echo not-lvl3
call :fail0
if errorlevel 0 echo zero-lvl0-yes
if errorlevel 1 echo zero-lvl1-yes
echo mid
ver >nul
if errorlevel 1 echo ver-failed
if not errorlevel 1 echo ver-ok
echo end
exit /b 0
:fail2
exit /b 2
:fail0
exit /b 0
