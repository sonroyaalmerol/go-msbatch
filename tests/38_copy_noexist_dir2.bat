@echo off
rem Test COPY to non-existent directory - verify behavior
echo a > nec2_a.txt
copy nec2_a.txt nonexistent_dir2\
if exist nonexistent_dir2\nec2_a.txt (echo file in dir) else (echo file not in dir)
if exist nonexistent_dir2 (echo file created as name) else (echo no file with that name)
del nec2_a.txt 2>nul
del nonexistent_dir2 2>nul
