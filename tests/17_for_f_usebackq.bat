@echo off

:: Test 1: usebackq with backticks - executes command
echo Test 1: usebackq with backtick command
for /F "usebackq tokens=1,2" %%i in (`echo hello world`) do (
    echo CmdOut: %%i %%j
)

:: Test 2: usebackq with single quotes - literal string (NOT command)
echo Test 2: usebackq with single quote string
for /F "usebackq tokens=1,2 delims=," %%i in ('part1,part2') do (
    echo SingleQuote: %%i %%j
)

:: Test 3: usebackq with double quotes - reads FILE
echo Test 3: usebackq with double quote file
echo partA,partB > tempfile.txt
for /F "usebackq tokens=1,2 delims=," %%i in ("tempfile.txt") do (
    echo DoubleQuoteFile: %%i %%j
)
rm tempfile.txt

:: Test 4: WITHOUT usebackq - single quotes execute command
echo Test 4: without usebackq, single quote executes command
for /F "tokens=1,2" %%i in ('echo cmd output') do (
    echo NoUsebackqCmd: %%i %%j
)

:: Test 5: WITHOUT usebackq - double quotes are literal string
echo Test 5: without usebackq, double quote is string
for /F "tokens=1,2 delims=," %%i in ("item1,item2") do (
    echo NoUsebackqString: %%i %%j
)
