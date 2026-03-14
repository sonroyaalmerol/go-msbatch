@echo off
echo Testing FOR /L:
for /L %%n in (1, 1, 3) do echo Step %%n

echo Testing FOR file globbing:
:: Creating dummy files
echo foo > file1.test
echo bar > file2.test
for %%f in (*.test) do echo Found %%f
rm *.test
