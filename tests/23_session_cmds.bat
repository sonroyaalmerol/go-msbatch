@echo off
rem BREAK
break
echo break ok

rem PATH
path C:\TestPath23
path

rem VERIFY
verify off
verify
verify on
verify

rem ASSOC
assoc .test23=TestFile23
assoc .test23

rem FTYPE
ftype TestFile23=notepad.exe
ftype TestFile23

rem DATE (dynamic output - just verify it runs)
date > tmpdate23.txt
if exist tmpdate23.txt (echo date ok) else (echo date failed)
del tmpdate23.txt

rem TIME (dynamic output - just verify it runs)
time > tmptime23.txt
if exist tmptime23.txt (echo time ok) else (echo time failed)
del tmptime23.txt

rem PROMPT
prompt MyPrompt$G
echo prompt set
prompt $P$G
echo prompt restored
