@echo off
echo Testing tokens and delims:
for /F "tokens=2,3 delims=," %%a in ("part1,part2,part3,part4") do (
    echo Token 2: %%a
    echo Token 3: %%b
)

echo Testing usebackq with backquoted command:
:: Mocking a command output via echo piped to cat or similar
for /F "usebackq tokens=1,2" %%i in (`echo hello world`) do (
    echo I: %%i
    echo J: %%j
)

echo Testing file parsing with custom delims:
echo key:value > test_config.txt
for /F "tokens=1,2 delims=:" %%k in (test_config.txt) do (
    echo KEY=%%k
    echo VALUE=%%l
)
rm test_config.txt
