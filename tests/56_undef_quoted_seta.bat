@echo off
set S=hello
echo [%S:~1,2%]
echo [%U:~1,2%]
echo [%U:o=x%]
echo [%U%]
echo [%U:~%]
echo [%~9x]
set /a "q=1<<4"
echo q=%q%
set /a "h=0x10+0xff"
echo h=%h%
set /a "o=021+1"
echo o=%o%
set /a "sh=256>>2"
echo sh=%sh%
