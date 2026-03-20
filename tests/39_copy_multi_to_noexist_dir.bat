@echo off
rem Test COPY multiple files to non-existent destination (no trailing slash)
rem Windows CMD: creates a FILE with concatenated contents (NOT a directory)
echo a > m1.txt
echo b > m2.txt
echo c > m3.txt
copy /y *.txt noexist_multi_file
if exist noexist_multi_file (echo file created) else (echo file not created)
if exist noexist_multi_file\m1.txt (echo is dir) else (echo is file)
type noexist_multi_file
del m1.txt m2.txt m3.txt noexist_multi_file
