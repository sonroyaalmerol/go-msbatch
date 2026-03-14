@echo off
echo hello > source_19.txt
copy source_19.txt dest_19.txt
if exist dest_19.txt (echo copy ok) else (echo copy failed)
move dest_19.txt moved_19.txt
if exist moved_19.txt (echo move ok) else (echo move failed)
if exist dest_19.txt (echo dest exists) else (echo dest gone)
del source_19.txt moved_19.txt
if exist source_19.txt (echo src exists) else (echo src gone)
if exist moved_19.txt (echo moved exists) else (echo moved gone)
