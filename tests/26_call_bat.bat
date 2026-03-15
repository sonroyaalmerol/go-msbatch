@echo off
rem Create a helper batch file, call it, verify output and env propagation.
echo main start

echo @echo off > helper_26.bat
echo echo helper says %%1 >> helper_26.bat
echo set HELPER_RAN=yes >> helper_26.bat

call helper_26.bat hello
echo helper returned: %ERRORLEVEL%
if "%HELPER_RAN%"=="yes" (echo env propagated) else (echo env not propagated)

rem Direct invocation (no CALL) — also runs in-process.
echo @echo off > direct_26.bat
echo echo direct ran >> direct_26.bat

direct_26.bat
echo after direct

del helper_26.bat
del direct_26.bat
