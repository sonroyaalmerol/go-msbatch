@echo off

:: Test 1: Create a zip file
echo Test 1: Create zip file
echo test content > testfile.txt
pkzip test.zip testfile.txt
if exist test.zip (
    echo PKZIP created archive successfully
) else (
    echo PKZIP failed to create archive
)
rm testfile.txt

:: Test 2: Extract zip file
echo Test 2: Extract zip file
pkunzip test.zip
if exist testfile.txt (
    echo PKUNZIP extracted successfully
    type testfile.txt
) else (
    echo PKUNZIP failed to extract
)
rm testfile.txt test.zip

:: Test 3: Create with recurse flag
echo Test 3: Create with recurse
mkdir subdir
echo sub content > subdir\subfile.txt
pkzip -r recurse.zip *.txt
if exist recurse.zip (
    echo PKZIP -r created archive successfully
) else (
    echo PKZIP -r failed
)
rm -rf subdir recurse.zip

:: Test 4: Extract to output directory
echo Test 4: Extract to output directory
echo output test > output_test.txt
pkzip output.zip output_test.txt
rm output_test.txt
mkdir output_dir
pkunzip output.zip output_dir\
if exist output_dir\output_test.txt (
    echo PKUNZIP to output_dir succeeded
) else (
    echo PKUNZIP to output_dir failed
)
rm -rf output_dir output.zip

echo Done with PKZIP tests
