@echo off
set /a s=1>>4
echo s=%s%
if exist 4 (echo FILE4-CREATED & type 4)
set /a t=256>>2
echo t=%t%
echo mid
