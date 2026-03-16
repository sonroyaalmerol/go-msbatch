@echo off
rem %1 = "A" (plain string, no path separators or extension)
echo ext=[%~x1]
echo name=[%~n1]
rem %2 = "B"
echo ext2=[%~x2]
