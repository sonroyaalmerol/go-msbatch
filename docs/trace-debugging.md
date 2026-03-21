# Trace Debugging

When debugging complex batch projects with multiple files, subroutine calls, and file I/O, msbatch provides a trace mode that shows execution flow in real-time.

## Usage

```bash
msbatch --trace script.bat           # Basic trace
msbatch --trace-verbose script.bat   # Verbose (includes SET, ERRORLEVEL)
msbatch /TRACE script.bat            # CMD-style flag
msbatch /TRACE:V script.bat          # CMD-style verbose
```

### Trace Output File

By default, trace output goes to stderr. You can redirect it to a file:

```bash
msbatch --trace script.bat 2>trace.log
```

Or use the dedicated flag:

```bash
msbatch --trace-file=trace.log script.bat
msbatch --trace-file trace.log script.bat
```

## Trace Output

### File Entry

Each batch file is marked with brackets:

```
[main.bat]
   1: echo off
   2: call helper.bat
CALL helper.bat
  [helper.bat]
     1: echo Starting helper
```

### Commands

Each command shows its line number:

```
[script.bat]
   1: echo off
   2: echo Hello
Hello
   3: set FOO=bar
```

### Subroutine Calls

`CALL :label` shows entry and return:

```
   5: call :process arg1
CALL :process arg1
     8: :process
     9: echo Processing %1
Processing arg1
    10: exit /b 0
  EXIT /B 0
RETURN
   6: echo Done
```

### GOTO

```
   7: goto :end
GOTO :end
  12: :end
```

### Redirections

File I/O operations are traced:

```
   3: echo data > output.txt
> output.txt
   4: echo more >> output.txt
>> output.txt
   5: set /p val=< input.txt
< input.txt
```

### File Deletion

```
   8: del temp.txt
DEL temp.txt
```

## Verbose Mode

`--trace-verbose` adds additional details:

```
[script.bat]
   3: set FOO=bar
  SET FOO=bar
   4: somecommand
  ERRORLEVEL=0
   5: exit /b 1
  ERRORLEVEL=1
  EXIT /B 1
```

## Example: Multi-File Debugging

Given a project with `main.bat` calling `writer.bat` and `reader.bat`:

```
[main.bat]
   1: echo off
   2: call writer.bat
CALL writer.bat
  [writer.bat]
     2: echo data from writer
  > shared.txt
     3: echo more data
  >> shared.txt
   3: call reader.bat
CALL reader.bat
  [reader.bat]
     2: set /p line1=
  < shared.txt
     3: echo Read: %line1%
Read: data
     4: del shared.txt
  DEL shared.txt
   4: echo All done
All done
```

The indentation makes it easy to see:
- Which file each command runs in
- When subroutines and external batch files are entered/exited
- What files are being written, read, or deleted
- The flow of execution through the entire project
