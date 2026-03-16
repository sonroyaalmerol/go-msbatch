@echo off
rem Outer IF false — entire nested block (inner IF + FOR) must be skipped
if "a"=="b" (
    echo outer wrong
    if "x"=="x" (
        echo inner wrong
    )
    for /L %%i in (1,1,3) do echo loop wrong
)
echo outer-false ok
rem Inner IF false — outer runs but inner body skipped
set OUTER=yes
set INNER=no
if "%OUTER%"=="yes" (
    echo outer-true
    if "%INNER%"=="yes" (
        echo inner wrong
    ) else (
        echo inner-else
    )
)
