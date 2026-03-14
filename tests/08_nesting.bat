@echo off
set OUTER=outer
set INNER=inner

if "%OUTER%"=="outer" (
    echo Entering outer IF
    if "%INNER%"=="inner" (
        echo Entering inner IF
        for /L %%i in (1, 1, 2) do (
            echo Loop %%i inside nested IF
        )
    )
    echo Leaving outer IF
)
