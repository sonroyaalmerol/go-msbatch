@echo off
:: %0 = tests/21_tilde_modifiers.bat (relative path used by the test runner)
echo name=%~n0
echo ext=%~x0
echo nx=%~nx0
:: %~dp0 is machine-dependent, so assert the contract instead of the literal:
:: it must be absolute and must rejoin with %~nx0 to name this very file.
if exist "%~dp0%~nx0" (echo dp-resolves=yes) else (echo dp-resolves=no)
