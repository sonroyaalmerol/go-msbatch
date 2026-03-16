@echo off
rem FIND with string not present — exits with errorlevel 1, no match output
echo hello world > find_fail_25.txt
find "nosuchstring_xyz" find_fail_25.txt
if errorlevel 1 (echo find-nomatch ok) else (echo find-nomatch wrong)
del find_fail_25.txt
rem SORT empty file — no output, no crash
type nul > sort_empty_25.txt
sort sort_empty_25.txt
echo sort-empty ok
del sort_empty_25.txt
rem WHERE for something unlikely to exist
where nosuchprog_xyz_25 > tmpwhere_25.txt 2>&1
if errorlevel 1 (echo where-missing ok) else (echo where-missing wrong)
del tmpwhere_25.txt
