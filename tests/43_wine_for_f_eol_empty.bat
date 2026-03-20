@echo off
echo Case 2: FOR /F with EOL option and empty lines
echo Line1 > File.txt
echo. >> File.txt
echo Line2 >> File.txt
echo. >> File.txt
echo Line3 >> File.txt
for /f "usebackq eol=: tokens=1" %%a in ("File.txt") do (
	echo "%%~a"
)
rm File.txt
