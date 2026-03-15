# Control Flow

## IF

```bat
IF [NOT] condition command
IF [NOT] condition (compound) ELSE (compound)
```

### Condition types

| Syntax | True when |
|--------|-----------|
| `EXIST path` | `path` exists on the filesystem |
| `DEFINED varname` | `varname` is set to any value (including empty) |
| `ERRORLEVEL n` | Last exit code is **≥ n** |
| `CMDEXTVERSION n` | Command extension version is **≥ n** (always 2) |
| `str1 == str2` | Strings are equal (case-sensitive unless `/I`) |
| `str1 EQU str2` | Strings or integers are equal |
| `str1 NEQ str2` | Not equal |
| `str1 LSS str2` | Less than |
| `str1 LEQ str2` | Less than or equal |
| `str1 GTR str2` | Greater than |
| `str1 GEQ str2` | Greater than or equal |

`/I` flag makes string comparisons case-insensitive and must appear immediately after `IF`:

```bat
IF /I "%INPUT%"=="yes" ECHO confirmed
```

`NOT` negates the condition:

```bat
IF NOT EXIST output.txt ECHO missing
```

### ELSE clause

```bat
IF EXIST file.txt (
    ECHO found
) ELSE (
    ECHO not found
)
```

**Caveat:** The `ELSE` keyword must appear on the **same line** as the closing `)` of the IF branch (or on the same line as the command if no parentheses are used). This matches cmd.exe behaviour.

### EQU / NEQ / LSS / LEQ / GTR / GEQ

When both operands look like integers, comparison is numeric. Otherwise it falls back to string comparison.

**Caveats:**

- `IF ERRORLEVEL n` tests `ERRORLEVEL >= n`, not `== n`. Use `IF %ERRORLEVEL%==n` for exact equality.
- `IF DEFINED` does not distinguish between a variable set to an empty string and an unset variable — both return false. To test for an empty value use `IF "%VAR%"==""`.
- `IF EXIST` follows the host OS's filesystem conventions. On case-sensitive Linux, `IF EXIST File.TXT` will not find `file.txt`.

---

## GOTO

```bat
GOTO label
GOTO :EOF
GOTO %VARIABLE%
```

Jumps execution to `:label` in the current script. Labels are matched case-insensitively.

`:EOF` is a special pseudo-label that jumps to the end of the script (equivalent to `EXIT /B`).

**Caveats:**

- `GOTO` searches the script **from the beginning** for the label — it does not scan forward from the current position. This matches cmd.exe.
- `GOTO` inside a `FOR` loop body terminates the loop immediately.
- Jumping into a `FOR` loop body from outside is not supported and produces undefined behaviour.

---

## CALL

### Subroutine call

```bat
CALL :label [arg1 arg2 ...]
```

Saves the current position and jumps to `:label`. On `EXIT /B` or reaching end-of-file the call returns and execution resumes after the `CALL` line. Arguments become `%1`–`%9` inside the subroutine.

```bat
CALL :greet Alice
ECHO back
GOTO :EOF

:greet
ECHO Hello %1
EXIT /B
```

### External batch call

```bat
CALL script.bat [args]
CALL script       :: .bat or .cmd extension auto-appended
```

Runs another batch file **in-process**, sharing the calling script's environment. Variable changes in the called script are visible to the caller.

Searching order: current directory first, then each directory in `PATH`.

**Caveats:**

- Unlike cmd.exe, a direct invocation without `CALL` (e.g. `other.bat`) also runs in-process and also returns to the caller. In real cmd.exe, a direct invocation without `CALL` terminates the calling script when the child returns. go-msbatch treats both forms identically — both return.
- `CALL` to a non-batch external command is dispatched to `os/exec`. Environment changes made by that process are **not** visible to the caller.
- `CALL` cannot call a label in a different script file; only labels in the current script are reachable.

---

## EXIT

```bat
EXIT [/B] [exitcode]
```

| Form | Behaviour |
|------|-----------|
| `EXIT` | Terminates the entire interpreter session |
| `EXIT /B` | Returns from the current batch file or subroutine |
| `EXIT /B n` | Returns with `ERRORLEVEL` set to `n` |
| `EXIT n` | Terminates the session with exit code `n` |

**Caveat:** `EXIT` (without `/B`) propagates through all active `CALL` frames and terminates the process. This matches cmd.exe. Use `EXIT /B` when you only want to return from the current script.

---

## SHIFT

```bat
SHIFT [/n]
```

Removes `%1` and shifts all remaining arguments left: `%2`→`%1`, `%3`→`%2`, etc.

`%0` (script name) is never shifted.

**Caveat:** The `/n` form (shift starting from argument n) is **not implemented**. Only the no-argument form is supported, which always shifts from position 1.

---

## Labels

```bat
:label_name
```

A line whose first non-whitespace character is `:` defines a label. Labels are stripped from output and never executed as commands.

`::` is treated as a comment (a label with an empty name that can never be targeted by `GOTO`).

**Caveat:** Unlike `REM`, `::` inside a parenthesised block can cause parsing errors in real cmd.exe in some edge cases. go-msbatch accepts `::` anywhere a label is valid.

---

## Compound Statements (Blocks)

Parentheses group multiple commands for use with `IF`, `ELSE`, and `FOR`:

```bat
IF EXIST file.txt (
    ECHO found
    DEL file.txt
)
```

**Caveat:** The entire block is read as a single logical line by cmd.exe. Percent expansion happens once at parse time for the whole block. Use `!delayed!` variables when you need per-line expansion inside a block.
