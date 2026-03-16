@echo off
rem Shift past all provided args — %1 becomes empty
rem Test runner provides A B C as %1 %2 %3
shift
shift
shift
echo [%1]
rem Extra shift on already-empty args — still empty
shift
echo [%1]
