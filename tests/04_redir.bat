@echo off
echo Redirection Test > out.txt
echo Second Line >> out.txt
cat < out.txt
del /q out.txt

echo Pipe Test | cat
