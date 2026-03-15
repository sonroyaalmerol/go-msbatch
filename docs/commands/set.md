# SET

Displays, sets, or removes environment variables. Also evaluates arithmetic (`/A`) and reads user input (`/P`).

## Syntax

```bat
SET                        :: list all variables
SET prefix                 :: list variables starting with prefix
SET name=value             :: assign value
SET name=                  :: delete variable (assign empty)
SET /A expression          :: evaluate arithmetic expression
SET /P name=prompt         :: read value from stdin
```

## Display

`SET` with no arguments lists every variable in the current environment, one per line in `name=value` form.

`SET prefix` lists all variables whose names begin with `prefix` (case-insensitive):

```bat
SET PATH
SET SYS
```

## Assignment

```bat
SET GREETING=Hello World
ECHO %GREETING%    :: Hello World
```

Variable names are case-insensitive; stored in upper-case internally.

To delete a variable, assign it an empty value:

```bat
SET TEMP_VAR=
```

## Arithmetic — /A

See [arithmetic.md](../language/arithmetic.md) for full details.

```bat
SET /A X=10*3+2     :: X=32
SET /A X+=5         :: X=37
SET /A A=1, B=2, C=A+B
```

## Prompt — /P

```bat
SET /P NAME=Enter your name:
ECHO Hello %NAME%
```

Displays the prompt string, reads one line from stdin, and stores it in `NAME` (without the trailing newline).

**Caveats:**

- If stdin is redirected from a file or pipe, the prompt string is still printed to stdout but the value is read silently from stdin.
- If stdin returns EOF immediately (e.g., empty pipe), the variable is set to an empty string.

## Caveats

- `SET name=value` includes everything to the right of the first `=` as the value, including leading and trailing spaces. `SET X= hello ` sets `X` to ` hello ` (with spaces).
- `SET` does not support quoted variable names. `SET "X=value"` (with surrounding quotes) is interpreted by cmd.exe as stripping the outer quotes; go-msbatch may not handle this identically.
- `ERRORLEVEL` is special: it is set automatically after every command. Manually setting `SET ERRORLEVEL=0` overrides the automatic value only until the next command runs.
