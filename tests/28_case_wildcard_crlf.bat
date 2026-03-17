@echo off
rem test CRLF line continuation
echo line1 ^
line2 ^
line3

rem create a test file to check case-insensitive execution and if exist wildcards
mkdir test_case_dir
echo echo I am a batch file > test_case_dir\CaseTest.bat
echo dummy > test_case_dir\MyFile.txt

rem call batch file with wrong case
call test_case_dir\casetest.bat

rem if exist wildcards
if exist test_case_dir\*.txt echo Found TXT wildcard

rem clean up
rmdir /s /q test_case_dir
