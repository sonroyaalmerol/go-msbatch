@echo off
echo Testing Path Mapping
:: On Linux, this should be mapped to something like /mnt/c/Windows
:: We use a path that might not exist but we want to see if 'exist' behaves consistently if mapped.
if exist C:\Windows (
    echo C:\Windows exists ^(mapped^)
) else (
    echo C:\Windows does not exist ^(mapped^)
)

set TEST_PATH=C:\Foo\Bar
echo TEST_PATH is %TEST_PATH%
