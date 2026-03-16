@echo off
rem REN non-existent source — destination must not appear
ren nosrc_24_fail.txt dst_24_fail.txt
if exist dst_24_fail.txt (echo unexpected) else (echo ren-missing ok)
rem DEL non-existent file — should not crash, file stays absent
del nosuchfile_24_fail.txt
echo del-missing ok
rem XCOPY with no source — destination must not be created
xcopy nosrcdir_24_fail xcpydst_24_fail /s /e /i
if exist xcpydst_24_fail (echo unexpected) else (echo xcopy-missing ok)
