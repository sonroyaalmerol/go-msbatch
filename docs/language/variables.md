# Variables & Expansion

## Environment Variables

Variables are case-insensitive; all names are stored and looked up in upper-case.

```bat
SET FOO=hello
ECHO %FOO%        :: hello
ECHO %foo%        :: hello (same variable)
```

`SET` with no arguments lists all variables. `SET prefix` lists all variables whose names start with `prefix`.

## Percent Expansion (Phase 1)

Performed before lexing. The entire line is scanned for `%…%` patterns.

### Literal percent

```bat
ECHO %%     :: single %
```

### Positional parameters

| Syntax | Value |
|--------|-------|
| `%0` | Script path (or empty in interactive mode) |
| `%1`–`%9` | Command-line arguments 1–9 |
| `%*` | All arguments from `%1` onwards, space-joined |

`SHIFT` rotates parameters: after `SHIFT`, `%1` becomes what was `%2`, etc. `%0` is never shifted.

### Variable reference

```bat
SET NAME=World
ECHO Hello %NAME%!    :: Hello World!
```

Referencing an undefined variable expands to empty string (not an error).

### Substring slicing `%VAR:~start,len%`

```bat
SET STR=Hello World
ECHO %STR:~6%       :: World        (from offset 6 to end)
ECHO %STR:~0,5%     :: Hello        (first 5 chars)
ECHO %STR:~-5%      :: World        (last 5 chars)
ECHO %STR:~-5,3%    :: Wor          (3 chars from 5th-from-end)
```

- Negative `start` counts from the end.
- If `len` is omitted, the slice extends to the end.
- If `len` is negative, it trims that many characters from the right.

**Caveat:** If `start` is beyond the string length the result is empty. No error is raised.

### String substitution `%VAR:old=new%`

```bat
SET MSG=foo bar foo
ECHO %MSG:foo=baz%   :: baz bar baz   (all occurrences, case-insensitive)
ECHO %MSG:foo=%      :: " bar "       (delete occurrences)
```

The match is case-insensitive. All non-overlapping occurrences are replaced.

**Caveat:** Unlike PowerShell's `-replace`, there is no regex support — the `old` value is treated as a literal string.

### Tilde modifiers `%~[mods]n`

Applied to a positional parameter `%0`–`%9`.

| Modifier | Meaning |
|----------|---------|
| `f` | Fully qualified path |
| `d` | Drive letter only (e.g. `C:`) |
| `p` | Path only (directory, including trailing `\`) |
| `n` | Filename without extension |
| `x` | Extension (including leading `.`) |
| `s` | Expand to short (8.3) path (approximated — returns the same path on Unix) |
| `e` | Extension (alias for `x`) |
| `a` | File attributes string (e.g. `--a------`) |
| `t` | Last-modified timestamp |
| `z` | File size in bytes |
| `$PATHVAR:` | Search for the file in the directories listed in `PATHVAR` |

Combinations are allowed: `%~dpn1` → drive + path + name of `%1`.

**Caveats:**

- `%~s` (short 8.3 name) is not available on Unix; returns the full path unchanged.
- `%~a`, `%~t`, `%~z` require the file to exist. If it does not, an empty string (or an error string) is returned.
- `%~$PATHVAR:n` only works when `PATHVAR` is defined and contains valid search paths. If the file is not found in any listed directory, the result is empty.

## Delayed Expansion (Phase 5)

Enabled with `SETLOCAL ENABLEDELAYEDEXPANSION`. Uses `!VAR!` syntax instead of `%VAR%`.

```bat
SETLOCAL ENABLEDELAYEDEXPANSION
SET COUNT=0
FOR /L %%i IN (1,1,5) DO (
    SET /A COUNT+=1
    ECHO !COUNT!    :: reads the UPDATED value each iteration
)
```

Without delayed expansion, `%COUNT%` inside the loop body would be expanded once when the block is parsed — before any iterations run — yielding `0` every time.

`^!` produces a literal `!` when delayed expansion is active.

**Caveats:**

- `SETLOCAL ENABLEDELAYEDEXPANSION` must appear before use; there is no command-line flag to pre-enable it.
- `DISABLEDELAYEDEXPANSION` is not supported as an explicit SETLOCAL argument. Delayed expansion is controlled per-scope: `ENDLOCAL` reverts to the state of the enclosing scope.
- `!` inside a `FOR /F` command string is expanded before the command runs, which can interfere with shell metacharacters in the command.

## SETLOCAL / ENDLOCAL Scoping

`SETLOCAL` pushes the current variable snapshot onto a stack. `ENDLOCAL` pops it, discarding all variable changes made since the last `SETLOCAL`.

```bat
SET X=outer
SETLOCAL
SET X=inner
ECHO %X%       :: inner
ENDLOCAL
ECHO %X%       :: outer
```

**Caveats:**

- `SETLOCAL` scopes are not automatically closed when a subroutine returns via `EXIT /B`. Scripts that use `SETLOCAL` inside a `:label` subroutine should call `ENDLOCAL` before `EXIT /B`, or the scope stack will accumulate.
- Real cmd.exe implicitly closes unclosed `SETLOCAL` scopes at the end of a batch file. go-msbatch does the same only for the top-level batch invocation; nested `CALL` returns do not auto-close inner scopes.

## Special Variables

| Variable | Set by | Meaning |
|----------|--------|---------|
| `ERRORLEVEL` | Every command | Exit code of the last command |
| `PATH` | `PATH` command or `SET PATH=…` | Executable search path |
| `PROMPT` | `PROMPT` command | Prompt string template |
| `COPYCMD` | User | Default `/Y` or `/-Y` for `COPY`/`XCOPY` |
| `__VERIFY__` | `VERIFY` command | Internal verify state (`ON`/`OFF`) |

**Caveat:** `ERRORLEVEL` is set after every command. `IF ERRORLEVEL n` tests whether the current `ERRORLEVEL` is **greater than or equal to** `n`, not equal to it — this matches cmd.exe but surprises many users.
