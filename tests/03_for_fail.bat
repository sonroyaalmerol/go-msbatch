@echo off
rem FOR /L with start > end (positive step) — zero iterations
for /L %%n in (5,1,1) do echo should not print
echo zero-iter ok
rem FOR /L with start < end (negative step) — zero iterations
for /L %%n in (1,-1,5) do echo should not print
echo neg-step ok
rem FOR over empty set — zero iterations
for %%x in () do echo should not print
echo empty-set ok
