@echo off
(echo one:two:three
echo ;semi
echo alpha beta gamma
echo last) > ff_in.txt
for /f "tokens=1,2 delims=:" %%a in (ff_in.txt) do echo T12=[%%a][%%b]
for /f "tokens=*" %%a in (ff_in.txt) do echo TSTAR=[%%a]
for /f "skip=2" %%a in (ff_in.txt) do echo SKIP=[%%a]
for /f "eol=;" %%a in (ff_in.txt) do echo EOL=[%%a]
for /f "tokens=1,*" %%a in ("x y z") do echo T1REST=[%%a]+[%%b]
for /f "usebackq delims=" %%a in ("ff_in.txt") do echo UB=[%%a]
echo end
