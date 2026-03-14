@echo off
echo Success && echo This should print
ls /nonexistent || echo This should print
true && echo Logical AND works
false || echo Logical OR works
