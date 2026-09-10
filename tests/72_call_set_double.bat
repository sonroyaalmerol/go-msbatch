@echo off
set VAR=first
call set VAR=%%VAR%%-second
echo VAR=%VAR%
call echo CALLED=%%VAR%%
set NESTED=deep
call call echo DBL=%%NESTED%%
echo end
