@echo off

:: Test backtick in filename - backticks are regular characters
echo Test backtick in filename:
echo test content > file`with`backtick.txt
type file`with`backtick.txt
del /q file`with`backtick.txt

:: Test gawk with backtick in filename
echo more data > data`file.lst
gawk "{print}" data`file.lst
del /q data`file.lst
