@echo off
echo Test FOR with variable expansion in set:
set A=1 2 3
for %%a in (%A%) do echo Item: %%a

echo Test FOR with comma delimiter:
set B=alpha,beta,gamma
for %%a in (%B%) do echo Comma: %%a

echo Test FOR with semicolon delimiter:
set C=one;two;three
for %%a in (%C%) do echo Semi: %%a

echo Test FOR with mixed delimiters:
set D=1 2,3;4
for %%a in (%D%) do echo Mixed: %%a

echo Test FOR with quoted string (should be single item):
set E="hello world"
for %%a in (%E%) do echo Quoted: %%a

echo Test FOR with nested loops:
set X=a b
set Y=1 2
for %%i in (%X%) do for %%j in (%Y%) do echo %%i%%j
