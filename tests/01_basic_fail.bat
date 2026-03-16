@echo off
rem Undefined variable expands to empty string
echo UNDEF=%UNDEFINED_VAR_FAIL_ZZZ%END
rem Undefined variable makes IF DEFINED false
if defined UNDEFINED_VAR_FAIL_ZZZ (echo defined) else (echo not-defined)
rem Undefined in arithmetic defaults to 0
set /a result=%UNDEFINED_ARITH_FAIL%+5
echo arith-undef: %result%
