@echo off
rem Test SETLOCAL/ENDLOCAL: env var isolation (NOT cwd restoration - Windows CMD doesn't restore cwd)

rem --- env var isolation ---
set OUTER50=before
setlocal
set INNER50=inside
echo inner: %INNER50%
endlocal
if defined INNER50 (echo inner-leaked) else (echo inner-gone)
echo outer: %OUTER50%

rem --- cwd NOT restored on ENDLOCAL (Windows CMD behavior) ---
mkdir setlocal_subdir_50
setlocal
cd setlocal_subdir_50
echo hello > sentinel50.txt
if exist sentinel50.txt (echo cwd-changed) else (echo cwd-not-changed)
endlocal
rem After ENDLOCAL, cwd is STILL in subdir (not restored)
if exist sentinel50.txt (echo cwd-not-restored) else (echo cwd-restored)
if exist ..\setlocal_subdir_50\sentinel50.txt (echo file-in-subdir) else (echo file-missing)
cd ..
rmdir /s /q setlocal_subdir_50

rem --- nested SETLOCAL ---
set A50=outer
setlocal
set A50=middle
setlocal
set A50=inner
echo nested-inner: %A50%
endlocal
echo nested-middle: %A50%
endlocal
echo nested-outer: %A50%
