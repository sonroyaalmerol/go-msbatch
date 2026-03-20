@echo off
rem Test COPY single file to non-existent directory (no trailing slash)
rem Windows CMD: copies to a FILE with that name, not a directory
echo single > single_src.txt
copy /y single_src.txt noexist_single_file
if exist noexist_single_file (echo file created) else (echo file not created)
if exist noexist_single_file\single_src.txt (echo dir created) else (echo not a dir)
del single_src.txt
del noexist_single_file
