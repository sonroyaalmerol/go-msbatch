@echo off
echo This is a long ^
command that spans ^
multiple lines.

set X=1
if "%X%"=="1" ^
echo X is 1 ^(inline with caret^)

echo Caret at end ^
of line ^
is continued.
