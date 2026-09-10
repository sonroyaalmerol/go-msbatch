@echo off
set VENV=C:\proj\env\sgenv
if not ["%CONDA_DEFAULT_ENV%"]==["%VENV%"] ( echo NEEDS ACTIVATION )
set A=x
set B=x
if "%A%"=="%B%" ( echo PLAIN-EQ )
if ["%A%"]==["%B%"] ( echo BRACKET-EQ )
if ["%A%"]==["y"] ( echo WRONG-NE )
set C=y
if ["%A%"]==["%C%"] ( echo WRONG-NE ) else ( echo ELSE-RAN )
if /I ["force"]==[force] ( echo WRONG-MIXED ) else ( echo MIXED-ELSE )
if /I [force]==[force] ( echo UNQUOTED-MATCH )
echo done
