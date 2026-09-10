@echo off
echo line-one> pp_in.txt
set /p X=<pp_in.txt
echo X=[%X%]
set /p "Y=Prompt: " < pp_in.txt
echo Y=[%Y%]
echo end
