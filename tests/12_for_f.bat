@echo off
echo Testing FOR /F with string:
for /F "tokens=1" %%i in ("Hello World") do echo %%i

echo Testing FOR /F with multiple lines:
:: In my implementation, literal string is currently treated as one line
:: and we take the first word.
for /F %%a in ("Line1 Line2") do echo First word: %%a
