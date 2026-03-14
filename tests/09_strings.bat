@echo off
set STR=HelloWorld
echo STR is %STR%
echo Slicing first 5: %STR:~0,5%
echo Slicing last 5: %STR:~-5%
echo Slicing mid: %STR:~2,3%
echo Slicing till end: %STR:~5%

set REPLACE=The quick brown fox
echo REPLACE is %REPLACE%
echo Replace fox with cat: %REPLACE:fox=cat%
echo Replace space with underscore: %REPLACE: =_%
echo Wildcard replace: %REPLACE:*quick =%
