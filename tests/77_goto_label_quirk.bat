@echo off
goto target_with_spaces
echo skipped
: target_with_spaces
echo arrived
goto :second    ;
echo skipped2
:second
echo second-arrived
echo end
