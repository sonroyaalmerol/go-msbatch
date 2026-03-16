@echo off
rem ASSOC for unknown extension returns nothing / not defined
assoc .nosuchext_23_fail
echo assoc-done
rem FTYPE for unknown type returns nothing / not defined
ftype nosuchtype_23_fail
echo ftype-done
rem PATH with no arg returns current PATH (just verify it doesn't crash)
path > tmppath_23_fail.txt
if exist tmppath_23_fail.txt (echo path-ok) else (echo path-failed)
del tmppath_23_fail.txt
