@echo off
rem Test COPY continues on partial failure
echo good > goodfile_34.txt
mkdir copytest_dir2
copy goodfile_34.txt + nonexistent_34.txt copytest_dir2\
if exist copytest_dir2\goodfile_34.txt (echo good copied) else (echo good missing)
del goodfile_34.txt
rmdir /s /q copytest_dir2
