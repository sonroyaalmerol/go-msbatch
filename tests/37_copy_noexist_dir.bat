@echo off
rem Test COPY to non-existent directory
echo a > nec_a.txt
copy nec_a.txt nonexistent_dir\
if exist nonexistent_dir\nec_a.txt (echo unexpected success) else (echo expected failure)
del nec_a.txt
