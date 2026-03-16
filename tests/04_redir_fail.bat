@echo off
rem > overwrites — second write must replace first
echo first > redir_fail_04.txt
echo second > redir_fail_04.txt
type redir_fail_04.txt
del redir_fail_04.txt
rem >> on non-existent file creates it
echo created >> redir_new_04.txt
if exist redir_new_04.txt (echo append-create ok) else (echo append-create failed)
del redir_new_04.txt
