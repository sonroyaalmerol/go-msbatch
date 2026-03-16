@echo off
rem IF false — ELSE branch must run
if "a"=="b" (echo wrong) else (echo else taken)
rem IF NOT with a true condition — also falls to ELSE
if not "x"=="x" (echo wrong) else (echo if-not works)
rem GOTO skips over intermediate label
goto :END
:MIDDLE
echo should not print
:END
echo end reached
