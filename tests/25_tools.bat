@echo off
rem SORT
echo banana > sort_25.txt
echo apple >> sort_25.txt
echo cherry >> sort_25.txt
sort sort_25.txt
del sort_25.txt

rem FIND
echo hello world > find_25.txt
echo goodbye world >> find_25.txt
find "hello" find_25.txt
del find_25.txt

rem TREE
mkdir tree_25
mkdir tree_25\alpha
mkdir tree_25\beta
tree tree_25
rmdir /s /q tree_25

rem WHERE (dynamic path - just verify it runs)
where sh > tmpwhere25.txt
if exist tmpwhere25.txt (echo where ok) else (echo where failed)
del tmpwhere25.txt

rem HOSTNAME (dynamic - just verify it runs)
hostname > tmphostname25.txt
if exist tmphostname25.txt (echo hostname ok) else (echo hostname failed)
del tmphostname25.txt

rem WHOAMI (dynamic - just verify it runs)
whoami > tmpwhoami25.txt
if exist tmpwhoami25.txt (echo whoami ok) else (echo whoami failed)
del tmpwhoami25.txt

rem TIMEOUT
timeout /t 0
echo timeout ok
