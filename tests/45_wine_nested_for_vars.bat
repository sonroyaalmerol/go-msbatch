@echo off
echo Case 4a: Nested FOR with parent variable expansion
echo foo > file1.txt
echo bar > file2.txt
for %%f in (*.txt) do (
	echo In parent cycle: "%%~f"
	for %%x in ("%%~f") do (
		echo In child cycle: "%%~x"
	)
)
rm *.txt
echo.
echo Case 4b: FOR /R with variable path
mkdir Folder1
echo data > Folder1\inner.txt
for %%f in ("Folder1") do (
	for /r "%%~f" %%x in (*.txt) do (
		echo "%%~x"
	)
)
rm -rf Folder1
