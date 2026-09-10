@echo off
set /a t1=12>>9
echo t1=%t1%
if exist 9 (echo F9)
set /a t2=7>>9
echo t2=%t2%
set /a t3=x2>>9
echo t3=%t3%
set /a t4=2>>z4.txt
echo t4=%t4%
if exist z4.txt (echo FZ4 & type z4.txt)
echo a1<<b
echo done1
