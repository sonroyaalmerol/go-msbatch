@echo off
set RESULT=
for %%a in (outer) do (
  for %%a in (inner) do set RESULT=[%%a]
  echo MID=%%a
)
echo RESULT=%RESULT%
echo end
