@echo off
set /a z=09, q=5
echo q=%q%
set /a w=1+
echo w=%w%
set /a v=1+abc
echo v=%v%
set /a t=0x10
echo t=%t%
echo EL1=%errorlevel%
set /a u=09
echo EL2=%errorlevel%
set /a r=1+
echo EL3=%errorlevel%
echo.done y
echo.z
echo. done
echo.  spaced
