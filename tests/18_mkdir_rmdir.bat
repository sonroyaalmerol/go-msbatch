@echo off
mkdir testdir_18
if exist testdir_18 (echo created) else (echo failed)
rmdir testdir_18
if exist testdir_18 (echo exists) else (echo removed)
mkdir testdir_18\sub
if exist testdir_18\sub (echo subdir ok) else (echo subdir fail)
rmdir /s /q testdir_18
if exist testdir_18 (echo still exists) else (echo tree removed)
