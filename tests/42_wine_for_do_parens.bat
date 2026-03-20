@echo off
echo Case 1: FOR with IF block after DO (no parens)
echo foo > test1.txt
echo bar > test2.txt
for %%a in (*.txt) do if #==# (
	echo File: "%%~a"
)
rm *.txt
echo.
echo Case 1b: FOR with parenthesized IF after DO
echo foo > test1.txt
echo bar > test2.txt
for %%a in (*.txt) do (if #==# (
	echo File: "%%~a"
))
rm *.txt
echo.
echo Case 1c: FOR with simple command after DO
echo foo > test1.txt
echo bar > test2.txt
for %%a in (*.txt) do if #==# echo Simple: "%%~a"
rm *.txt
