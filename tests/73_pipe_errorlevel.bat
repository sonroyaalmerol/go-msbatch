@echo off
echo hello | find "hello"
echo EL1=%errorlevel%
echo hello | find "xyz"
echo EL2=%errorlevel%
echo xyz | find /v "hello" >nul
echo EL3=%errorlevel%
echo end
