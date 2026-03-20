@echo off
echo Case 5a: FOR /F without quotes on existing file
echo Line1 > File.txt
echo Line2 >> File.txt
for /f %%a in (File.txt) do (
	echo "%%~a"
)
rm File.txt
echo.
echo Case 5b: FOR /F with usebackq and quotes
echo Line1 > File.txt
echo Line2 >> File.txt
for /f "usebackq" %%a in ("File.txt") do (
	echo "%%~a"
)
rm File.txt
