@echo off
rem REN
echo test > ren_src_24.txt
ren ren_src_24.txt ren_dst_24.txt
if exist ren_dst_24.txt (echo ren ok) else (echo ren failed)
if exist ren_src_24.txt (echo src exists) else (echo src gone)
del ren_dst_24.txt

rem MKLINK
echo target > mklink_target_24.txt
mklink mklink_link_24.txt mklink_target_24.txt
if exist mklink_link_24.txt (echo mklink ok) else (echo mklink failed)
del mklink_link_24.txt
del mklink_target_24.txt

rem MORE
echo hello > more_24.txt
echo world >> more_24.txt
more more_24.txt
del more_24.txt

rem XCOPY
mkdir xcopy_src_24
mkdir xcopy_src_24\sub
echo file1 > xcopy_src_24\file1.txt
echo file2 > xcopy_src_24\sub\file2.txt
xcopy xcopy_src_24 xcopy_dst_24 /s /e /i
if exist xcopy_dst_24\file1.txt (echo xcopy ok) else (echo xcopy failed)
if exist xcopy_dst_24\sub\file2.txt (echo xcopy sub ok) else (echo xcopy sub failed)
rmdir /s /q xcopy_src_24
rmdir /s /q xcopy_dst_24
