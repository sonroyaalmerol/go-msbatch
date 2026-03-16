@echo off
rem Division by zero returns 0 (go-msbatch documented behaviour)
set /a r=10/0
echo div0: %r%
rem Modulo by zero also returns 0
set /a r=7%%0
echo mod0: %r%
rem Undefined variable in arithmetic is treated as 0
set /a r=%UNDEF_ARITH_FAIL%+5
echo undef+5: %r%
