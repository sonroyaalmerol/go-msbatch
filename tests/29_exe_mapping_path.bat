@echo off
rem This test verifies that MSBATCH_EXE_MAP works even with full paths.
rem It requires MSBATCH_EXE_MAP="ls.exe=ls" to be set in the environment.

C:\bin\ls.exe 29_exe_mapping_path.bat
