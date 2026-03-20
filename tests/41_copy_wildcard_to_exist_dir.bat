@echo off
rem Test COPY wildcard to existing directory
echo w1 > w1.txt
echo w2 > w2.txt
mkdir exist_dir
copy /y w*.txt exist_dir
if exist exist_dir\w1.txt (echo w1 ok) else (echo w1 fail)
if exist exist_dir\w2.txt (echo w2 ok) else (echo w2 fail)
del w1.txt w2.txt
rmdir /s /q exist_dir
