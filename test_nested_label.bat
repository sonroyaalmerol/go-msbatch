@echo off
if 1==1 (
    echo Inside IF
    :MYLABEL
    echo Reached Label
)
goto MYLABEL
echo FAILED
exit /b 1
