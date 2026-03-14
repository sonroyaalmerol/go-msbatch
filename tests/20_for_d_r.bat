@echo off
:: Setup
mkdir ford_test\aaa
mkdir ford_test\bbb
echo x > ford_test\aaa\file1.txt
echo y > ford_test\bbb\file2.txt

:: FOR /D - iterate directories matching pattern
echo Dirs:
for /d %%d in (ford_test\*) do echo %%d

:: FOR /R - recurse, collect all .txt files
echo Files:
for /r ford_test %%f in (*.txt) do echo %%f

:: Cleanup
rmdir /s /q ford_test
