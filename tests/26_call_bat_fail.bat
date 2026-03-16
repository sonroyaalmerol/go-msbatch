@echo off
rem CALL non-existent batch file — errorlevel must be non-zero
call nosuchscript_26_fail.bat
if errorlevel 1 (echo call-missing ok) else (echo call-missing wrong)
rem EXIT /B sets errorlevel in caller
echo @echo off > helper_fail_26.bat
echo exit /b 42 >> helper_fail_26.bat
call helper_fail_26.bat
echo errorlevel: %ERRORLEVEL%
del helper_fail_26.bat
rem CALL returns and execution continues — verify we get here
echo after-call
