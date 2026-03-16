@echo off
rem FOR /F on a non-existent file — loop body never runs
for /F %%a in (nosuchfile_16_fail.txt) do echo should not print
echo missing-file ok
rem FOR /F skipping lines that don't match (skip= option)
for /F "skip=5" %%a in ("only one line") do echo should not print
echo skip-all ok
