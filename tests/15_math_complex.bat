@echo off
set /A RESULT=1 + 2 * 3
echo 1 + 2 * 3 = %RESULT%
set /A RESULT=(1 + 2) * 3
echo ^(1 + 2^) * 3 = %RESULT%
set /A RESULT=10 / 3
echo 10 / 3 = %RESULT%
:: In batch, % must be escaped as %%
set /A RESULT=10 %% 3
echo 10 %% 3 = %RESULT%

set /A VAL=10
set /A VAL+=5
echo VAL += 5: %VAL%
set /A VAL*=2
echo VAL *= 2: %VAL%

:: Multiple expressions
set /A A=1, B=2, C=A+B
echo A=%A% B=%B% C=%C%
