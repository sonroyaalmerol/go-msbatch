@echo off
:: Setup: create source files
echo Hello> ca_a.txt
echo World> ca_b.txt
echo Extra> ca_c.txt

:: Form 1: embedded + with explicit destination
copy ca_a.txt+ca_b.txt ca_out1.txt
type ca_out1.txt

:: Form 2: standalone + tokens with explicit destination
copy ca_a.txt + ca_b.txt ca_out2.txt
type ca_out2.txt

:: Form 3: embedded + with no destination (append into first source)
copy ca_a.txt+ca_c.txt
type ca_a.txt

:: Cleanup
del ca_a.txt ca_b.txt ca_c.txt ca_out1.txt ca_out2.txt
