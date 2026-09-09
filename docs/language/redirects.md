# Redirection & Pipes

## Standard File Descriptors

| FD  | Name                      |
| --- | ------------------------- |
| 0   | stdin                     |
| 1   | stdout                    |
| 2   | stderr                    |
| 3–9 | extra redirection handles |

Descriptors 0, 1, and 2 are the standard streams. Handles 3 through 9 can be opened on files and duplicated like cmd: `3>file`, `1>&3`, `3>&1`. Duplicating a handle that was never opened prints cmd's error (`The handle could not be duplicated during redirection of handle N.`) and skips the command, matching cmd.exe.

## Redirection Operators

| Operator   | Meaning                              |
| ---------- | ------------------------------------ |
| `> file`   | Redirect stdout to `file` (truncate) |
| `>> file`  | Redirect stdout to `file` (append)   |
| `< file`   | Read stdin from `file`               |
| `2> file`  | Redirect stderr to `file` (truncate) |
| `2>> file` | Redirect stderr to `file` (append)   |
| `>&2`      | Redirect stdout to stderr            |
| `2>&1`     | Redirect stderr to stdout            |
| `<&0`      | (stdin from stdin — no-op)           |

```bat
ECHO hello > out.txt
DIR >> log.txt
TYPE missing.txt 2> err.txt
DIR 2>&1 > all.txt
```

**Caveats:**

- `2>&1 > file` and `> file 2>&1` behave differently (as in real CMD) - order matters because the redirection is applied left-to-right.
- Whitespace before a redirection operator is part of the command's text, exactly as in cmd: `ECHO text > file` writes `text ` with the trailing space, and `ECHO text>file` writes it without.
- Redirections apply to the entire command including its pipeline stage.

## Pipes

```bat
command1 | command2
DIR /B | SORT
ECHO hello world | FIND "world"
```

The left and right commands run **concurrently in goroutines**. An `os.Pipe()` connects the left side's stdout to the right side's stdin.

A pipeline can be chained:

```bat
DIR /B | SORT | FIND ".txt"
```

**Caveats:**

- Environment changes inside either side of a pipe do **not** propagate back to the parent script. Each side gets a snapshot of the environment at the time the pipe is set up.
- Stderr is not piped — it flows through to the parent's stderr on both sides. Use `2>&1` before the `|` to merge stderr into the pipe.
- Real cmd.exe also runs both sides concurrently; the behaviour here is intentionally consistent.

## Combining Redirections with Pipes

```bat
command1 2>&1 | command2
```

Merges stderr into stdout before piping. Redirect operators on `command2` apply to the right side of the pipe only.

## Command Separators & Conditional Execution

These are not strictly redirects but are parsed alongside them:

| Operator | Meaning                                               |
| -------- | ----------------------------------------------------- |
| `&`      | Run right side unconditionally after left             |
| `&&`     | Run right side only if left succeeds (`ERRORLEVEL 0`) |
| `\|\|`   | Run right side only if left fails (`ERRORLEVEL != 0`) |

```bat
MKDIR out && ECHO created
DEL file.txt || ECHO delete failed
ECHO a & ECHO b & ECHO c
```

**Caveat:** `&&` and `||` check whether `ERRORLEVEL` is `0` or non-zero. They do not distinguish between different non-zero exit codes.
