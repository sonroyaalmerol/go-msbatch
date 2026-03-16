@echo off
rem FOR /D on a non-existent directory — no iterations
for /D %%d in (nosuchdir_20_fail\*) do echo should not print
echo no-dir ok
rem FOR /R on a non-existent root — no iterations
for /R nosuchdir_20_fail %%f in (*.txt) do echo should not print
echo no-root ok
rem FOR /D matching no subdirectories (empty dir)
mkdir empty_20_fail
for /D %%d in (empty_20_fail\*) do echo should not print
echo no-subdirs ok
rmdir empty_20_fail
