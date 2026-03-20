@echo off
rem Test COPY with multiple files using wildcard
echo file1 > copytest_a.txt
echo file2 > copytest_b.txt
mkdir copytest_dir
copy copytest_*.txt copytest_dir\
if exist copytest_dir\copytest_a.txt (echo a ok) else (echo a fail)
if exist copytest_dir\copytest_b.txt (echo b ok) else (echo b fail)
del copytest_a.txt copytest_b.txt
rmdir /s /q copytest_dir
