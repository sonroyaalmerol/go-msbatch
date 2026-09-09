@echo off
:: Handles 3-9: open one on a file, duplicate it into stdout, and confirm the
:: shell's own stdout is restored afterwards.
echo routed 3>fd3_52.txt 1>&3
echo back-on-stdout

:: A block redirect keeps handle 3 open for every command inside it.
(
echo in-block 1>&3
) 3>fd3b_52.txt

echo --- fd3_52.txt
type fd3_52.txt
echo --- fd3b_52.txt
type fd3b_52.txt

:: Duplicating from a handle that was never opened is an error in cmd.
echo unreachable 1>&7

del fd3_52.txt fd3b_52.txt
