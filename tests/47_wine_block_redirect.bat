@echo off
echo Case 6a: Global redirection after block
if #==# (
	echo Text 1...
	echo Text 2...
)> "Test.bug"
echo Contents of Test.bug:
type Test.bug
rm Test.bug
echo.
echo Case 6b: Per-command redirection (normal)
if #==# (
	echo Text 1...> "Test.bug"
	echo Text 2...> "Test.bug"
)
echo Contents of Test.bug:
type Test.bug
rm Test.bug
