@echo off
rem && right side must NOT run when left side fails
echo seed > logic_fail_07.txt
del logic_fail_07.txt
type logic_fail_07.txt && echo should not print
echo after-and
rem || right side MUST run when left side fails
type logic_fail_07.txt || echo or-runs
echo done
