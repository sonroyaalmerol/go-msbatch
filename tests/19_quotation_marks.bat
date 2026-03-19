@echo off

:: ============================================================
:: Test file documenting Windows CMD quotation mark behavior
:: ============================================================

echo === Regular commands ===

:: Double quotes ARE string delimiters
echo Double quote string: "hello world"

:: Single quotes are REGULAR characters (not string delimiters)
echo Single quote chars: 'hello world'

:: Backticks are REGULAR characters (not string delimiters)
echo Backtick chars: `hello world`

echo.
echo === IF comparisons ===

:: Both styles work - quotes are just part of the string being compared
set MODE=prod
if '%MODE%'=='prod' echo Single quote comparison works
if "%MODE%"=="prod" echo Double quote comparison works

:: Mismatch - these should NOT print
set MODE=dev
if '%MODE%'=='prod' echo SHOULD NOT PRINT 1
if "%MODE%"=="prod" echo SHOULD NOT PRINT 2

echo.
echo === FOR /F without usebackq ===

:: Double quotes = literal string
for /F %%i in ("test string") do echo Double quoted string: %%i

:: Single quotes = execute command
for /F %%i in ('echo cmd result') do echo Single quoted command: %%i

:: Backticks = regular characters (part of filename)
echo file content > test`file.txt
for /F %%i in (test`file.txt) do echo Backtick filename: %%i
rm test`file.txt

echo.
echo === FOR /F with usebackq ===

:: Double quotes = literal string (same as without usebackq)
for /F "usebackq" %%i in ("test string 2") do echo usebackq double: %%i

:: Single quotes = literal string (CHANGED with usebackq!)
for /F "usebackq" %%i in ('literal string') do echo usebackq single: %%i

:: Backticks = execute command (CHANGED with usebackq!)
for /F "usebackq" %%i in (`echo backtick cmd`) do echo usebackq backtick: %%i

echo.
echo === Filenames with quotes ===

:: Single quote in filename - valid on Linux
echo content > file'with'quotes.txt
type file'with'quotes.txt
rm file'with'quotes.txt

:: Backtick in filename - valid on Linux
echo content > file`with`backtick.txt
type file`with`backtick.txt
rm file`with`backtick.txt

echo.
echo Done with quotation tests
