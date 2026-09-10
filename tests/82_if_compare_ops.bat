@echo off
if "a" == "a" echo q-eq
if /i "ABC" == "abc" echo ci-eq
if not "a" == "b" echo ne
if "a" == "b" (echo then) else echo else-side
if 1 EQU 1 echo num-eq
if 1 NEQ 2 echo num-ne
if 10 LSS 9 echo str-lt
if 2 GEQ 2 echo ge
if defined PATH echo path-defined
echo end
