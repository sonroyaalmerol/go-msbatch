# Trace Debugging

When debugging complex batch projects with multiple files, subroutine calls, and file I/O, msbatch provides a trace mode that shows execution flow in real-time.

## Usage

### Command-Line Flags

```bash
msbatch --trace script.bat           # Basic trace
msbatch --trace-verbose script.bat   # Verbose (includes SET, ERRORLEVEL)
msbatch /TRACE script.bat            # CMD-style flag
msbatch /TRACE:V script.bat          # CMD-style verbose
```

### Environment Variables

You can also enable tracing via environment variables:

```bash
export MSBATCH_TRACE=on              # or: 1, true
export MSBATCH_TRACE=verbose         # or: 2
export MSBATCH_TRACE_FILE=trace.log  # redirect trace to file
msbatch script.bat
```

| Variable | Values | Description |
|----------|--------|-------------|
| `MSBATCH_TRACE` | `0`, `off`, `false` | Disable tracing |
| `MSBATCH_TRACE` | `1`, `on`, `true` | Basic trace mode |
| `MSBATCH_TRACE` | `2`, `verbose` | Verbose trace mode |
| `MSBATCH_TRACE_FILE` | `<path>` | Write trace output to file (auto-enables trace) |

**Note:** Command-line flags take precedence over environment variables.

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

## Interactive Debugger

For hands-on debugging, msbatch provides an interactive debugger that lets you set breakpoints, step through code, and inspect variables.

### Starting the Debugger

```bash
# Break at REM BREAK / :: BREAK comments
msbatch --debug script.bat

# Single-step through every statement
msbatch --step script.bat

# CMD-style flags also work
msbatch /DEBUG script.bat
msbatch /STEP script.bat
```

### Environment Variables

```bash
export MSBATCH_BREAKPOINTS=on    # or: 1, true
export MSBATCH_STEP=on           # or: 1, true
msbatch script.bat
```

| Variable | Values | Description |
|----------|--------|-------------|
| `MSBATCH_BREAKPOINTS` | `0`, `off`, `false` | Disable breakpoint debugger |
| `MSBATCH_BREAKPOINTS` | `1`, `on`, `true` | Break at breakpoints |
| `MSBATCH_STEP` | `0`, `off`, `false` | Disable step mode |
| `MSBATCH_STEP` | `1`, `on`, `true` | Single-step mode |

**Note:** Command-line flags take precedence over environment variables.

### Setting Breakpoints

Add `REM BREAK` or `:: BREAK` comments in your batch file:

```batch
@echo off
set VAR1=Hello
REM BREAK          <- execution pauses here
set VAR2=World
:: BREAK           <- this also works
echo %VAR1% %VAR2%
```

### Debugger Commands

When execution pauses, you can use these commands:

| Command | Description |
|---------|-------------|
| `c`, `continue` | Continue execution until next breakpoint |
| `s`, `step` | Execute one line and stay in step mode |
| `n`, `next` | Execute one line, then continue |
| `q`, `quit` | Exit the script immediately |
| `v`, `vars` | Show all environment variables |
| `p <var>` | Print value of a specific variable |
| `b <line>` | Add breakpoint at line number |
| `d <line>` | Delete breakpoint at line number |
| `l`, `list` | List all breakpoints |
| `h`, `help` | Show help |

### Example Session

```batch
# script.bat
@echo off
set COUNT=0
REM BREAK
set /a COUNT+=1
echo Count is %COUNT%
```

```bash
$ msbatch --debug script.bat

[DEBUG] Breakpoint at script.bat:4
  4: REM BREAK

(debug) v
Environment variables:
  COUNT=0

(debug) c
  5: set /a COUNT+=1

(debug) p COUNT
  COUNT=1

(debug) c
Count is 1
```
