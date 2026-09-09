@echo off
:: Inside CALL :label, %0/%1/%* must be the subroutine's own arguments, and the
:: caller's must be restored on return. The runner invokes this file with A B C.
call :sub one two three
echo after-star=%*
goto :eof

:sub
echo label0=%0
echo arg1=%1
echo star=%*
goto :eof
