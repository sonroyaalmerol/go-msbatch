@echo off
set STR=hello
rem Replace with no match — original string returned unchanged
echo no-match: %STR:xyz=abc%
rem Slice beyond length — empty
echo oob: [%STR:~100%]
rem Negative offset larger than length — full string
echo neg-oob: %STR:~-100%
rem Undefined variable — empty expansion
echo undef: [%UNDEF_STR_FAIL%]
rem Replace all occurrences of a substring
set SRC=aabbaabb
echo replace-all: %SRC:aa=x%
