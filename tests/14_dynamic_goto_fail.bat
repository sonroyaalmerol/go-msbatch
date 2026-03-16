@echo off
rem GOTO with variable skips intervening labels
set SKIP=LANDING
goto %SKIP%
:BYPASSED
echo should not print
:LANDING
echo landed
rem Chained GOTO via variable
set NEXT=FINAL
goto %NEXT%
echo should not print
:FINAL
echo final
