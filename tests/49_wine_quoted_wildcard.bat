@echo off
echo Case 8a: FOR with unquoted wildcard
echo foo > test1.txt
echo bar > test2.txt
for %%f in (*.txt) do (
	echo "%%~f"
)
rm *.txt
echo.
echo Case 8b: FOR with quoted wildcard
echo foo > test1.txt
echo bar > test2.txt
for %%f in ("*.txt") do (echo "%%~f")
rm *.txt
echo.
echo Case 8c: FOR with quoted path wildcard
mkdir Dir
echo foo > Dir\test1.txt
echo bar > Dir\test2.txt
for %%f in ("Dir\*.txt") do (echo "%%~f")
rm -rf Dir
