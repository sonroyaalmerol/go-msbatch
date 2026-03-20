@echo off
rem Test COPY multiple files to existing directory
rem This tests the case where os.Stat(dst) succeeds
echo a > multi_a.txt
echo b > multi_b.txt
echo c > multi_c.txt
mkdir multi_dir
copy multi_a.txt multi_dir\
copy multi_b.txt multi_dir\
copy multi_c.txt multi_dir\
if exist multi_dir\multi_a.txt (echo a ok) else (echo a fail)
if exist multi_dir\multi_b.txt (echo b ok) else (echo b fail)
if exist multi_dir\multi_c.txt (echo c ok) else (echo c fail)
del multi_a.txt multi_b.txt multi_c.txt
rmdir /s /q multi_dir
