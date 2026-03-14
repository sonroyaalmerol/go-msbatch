@echo off
set VAL=1
if "%VAL%"=="1" (
    echo IF branch works
) else (
    echo ELSE branch works (error)
)

if not "%VAL%"=="2" echo IF NOT works

goto :SKIP
echo This should be skipped
:SKIP
echo After GOTO SKIP

call :SUBROUTINE Arg1 Arg2
echo Back from SUBROUTINE
goto :EOF

:SUBROUTINE
echo Inside SUBROUTINE with %1 and %2
exit /b
