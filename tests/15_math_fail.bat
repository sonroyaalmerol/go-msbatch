@echo off
rem Division by zero — returns 0 per go-msbatch spec
set /a r=10/0
echo [%r%]
rem Modulo by zero — returns 0
set /a r=9%%0
echo [%r%]
rem Unary minus via subtraction
set /a r=0-5
echo [%r%]
rem Chained division
set /a r=100/10/2
echo [%r%]
