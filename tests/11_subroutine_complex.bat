@echo off
set GLOBAL_VAR=Global
echo Initial GLOBAL_VAR is %GLOBAL_VAR%

call :SUBROUTINE
echo After subroutine, GLOBAL_VAR is %GLOBAL_VAR%
echo After subroutine, LOCAL_VAR is %LOCAL_VAR%
goto :EOF

:SUBROUTINE
setlocal
set GLOBAL_VAR=Overridden
set LOCAL_VAR=Local
echo Inside subroutine, GLOBAL_VAR is %GLOBAL_VAR%
echo Inside subroutine, LOCAL_VAR is %LOCAL_VAR%
endlocal
goto :EOF
