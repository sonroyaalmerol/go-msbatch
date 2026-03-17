@echo off
echo START
call :sub1
echo BACK1
call:sub2
echo BACK2
goto:eof
echo FAIL_TOP

:sub1
echo INSIDE SUB1
goto :eof
echo FAIL_SUB1

:sub2
echo INSIDE SUB2
call :eof
echo FAIL_SUB2
exit /b 0
