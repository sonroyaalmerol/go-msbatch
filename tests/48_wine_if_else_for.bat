@echo off
echo Case 7a: IF/ELSE with FOR inside
echo data > file1.txt
echo data > file2.txt
if #==# (
	echo IF condition.
	for %%m in (*.txt) do (
		echo FOR cycle: "%%~m"
	)
) else (
	echo ELSE condition.
)
rm *.txt
echo.
echo Case 7b: Simple IF/ELSE without FOR
if #==# (
	echo Simple IF.
) else (
	echo Simple ELSE.
)
