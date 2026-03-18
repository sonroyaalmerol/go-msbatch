@echo off
echo Original args: %1 %2 %3
echo All args (%%*): %*

echo Shifting...
shift
echo After 1st shift: %1 %2 %3
echo All args (%%*): %*

echo Shifting from /1...
shift /1
echo After shift /1: %1 %2 %3
echo All args (%%*): %*

echo Shifting from /2...
shift /2
echo After shift /2: %1 %2 %3
echo All args (%%*): %*

echo Shifting in a loop...
FOR /L %%G IN (1,1,2) DO shift
echo After loop: %1 %2 %3
echo All args (%%*): %*
