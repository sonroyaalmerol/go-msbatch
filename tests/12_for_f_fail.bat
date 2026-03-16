@echo off
rem Requesting a token beyond what exists — loop body skipped
for /F "tokens=5" %%a in ("one two three") do echo should not print
echo too-few-tokens ok
rem Empty string — loop body skipped
for /F %%a in ("") do echo should not print
echo empty-string ok
rem String with only delimiters — no tokens
for /F "delims=," %%a in (",,,") do echo should not print
echo only-delims ok
