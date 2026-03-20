@echo off
rem Test COPY multiple files with wildcard to existing directory
echo a > wc_1.cmd
echo b > wc_2.cmd
echo c > wc_3.cmd
mkdir wc_dir
copy *.cmd wc_dir\
if exist wc_dir\wc_1.cmd (echo 1 ok) else (echo 1 fail)
if exist wc_dir\wc_2.cmd (echo 2 ok) else (echo 2 fail)
if exist wc_dir\wc_3.cmd (echo 3 ok) else (echo 3 fail)
del wc_1.cmd wc_2.cmd wc_3.cmd
rmdir /s /q wc_dir
