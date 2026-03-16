@echo off
rem SETLOCAL/ENDLOCAL — variable set inside must not leak out
setlocal
set SCOPED_FAIL=inside
echo inside: %SCOPED_FAIL%
endlocal
if defined SCOPED_FAIL (echo scope-leaked) else (echo scope-ok)
rem Nested SETLOCAL — inner scope isolated from outer
setlocal
set OUTER_VAR=outer
setlocal
set OUTER_VAR=inner
echo inner-override: %OUTER_VAR%
endlocal
echo after-inner-endlocal: %OUTER_VAR%
endlocal
