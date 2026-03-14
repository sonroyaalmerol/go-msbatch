@echo off
set TARGET=MYLABEL
goto %TARGET%
echo This skipped
:MYLABEL
echo Reached MYLABEL
call echo External Call Works
