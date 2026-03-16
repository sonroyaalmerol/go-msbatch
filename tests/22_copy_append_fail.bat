@echo off
rem COPY of non-existent source — destination must not be created
copy nosrcfile_22_fail.txt dst_22_fail.txt
if exist dst_22_fail.txt (echo unexpected) else (echo src-missing ok)
rem COPY overwrite — destination gets new content, not appended
echo original > cow_22.txt
echo overwrite > cow_src_22.txt
copy cow_src_22.txt cow_22.txt
type cow_22.txt
del cow_22.txt
del cow_src_22.txt
