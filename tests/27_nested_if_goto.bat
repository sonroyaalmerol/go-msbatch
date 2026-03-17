@echo off
set "D=x"
if /I "%D%" NEQ "f" (if /I "%D%" NEQ "b" (if /I "%D%" NEQ "a" (if /I "%D%" NEQ "x" (goto ERROR_INPUTS))))
echo Success x
set "D=y"
if /I "%D%" NEQ "f" (if /I "%D%" NEQ "b" (if /I "%D%" NEQ "a" (if /I "%D%" NEQ "x" (goto ERROR_INPUTS))))
echo Fail y
goto EOF

:ERROR_INPUTS
echo Error Inputs Reached

:EOF
