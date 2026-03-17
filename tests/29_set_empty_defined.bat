@echo off
set MYVAR=123
if defined MYVAR echo IS DEFINED

set MYVAR=
if not defined MYVAR echo NOT DEFINED

set MYVAR=456
if defined MYVAR echo DEFINED AGAIN
