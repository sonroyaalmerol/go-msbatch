# FOR Loops

All `FOR` variants iterate over a set and execute a command for each element. The loop variable is a **single ASCII letter** (A–Z, a–z).

In a script (`*.bat`/`*.cmd`), the variable is written as `%%V`. In interactive mode, use `%V`.

---

## FOR %%V IN (set) DO — File List

```bat
FOR %%V IN (*.txt) DO ECHO %%V
FOR %%V IN (a.txt b.txt c.txt) DO ECHO %%V
```

`set` is a space- or comma-separated list of file patterns. Each pattern is glob-expanded; matched files are iterated. Non-matching patterns are passed as-is (not skipped).

**Caveats:**

- Only `*` and `?` glob wildcards are supported. `[…]` bracket ranges are not.
- The set is expanded once before the loop starts, so files created by the loop body are not included in the iteration.
- If a pattern matches no files, the literal pattern string is passed to the body as if it were a filename.

---

## FOR /L %%V IN (start,step,end) DO — Numeric Range

```bat
FOR /L %%i IN (1,1,10) DO ECHO %%i
FOR /L %%i IN (10,-1,1) DO ECHO %%i   :: countdown
FOR /L %%i IN (0,2,8) DO ECHO %%i     :: 0 2 4 6 8
```

Iterates `%%V` from `start` to `end` inclusive, incrementing by `step`.

- If `step > 0` and `start > end`, the loop does not execute.
- If `step < 0` and `start < end`, the loop does not execute.
- If `step == 0`, the loop runs once with `%%V = start`.

**Caveat:** Unlike real cmd.exe, a zero step does not hang the interpreter — it runs exactly once and exits.

---

## FOR /D %%V IN (pattern) DO — Directories Only

```bat
FOR /D %%d IN (*) DO ECHO %%d
FOR /D %%d IN (C:\Users\*) DO ECHO %%d
```

Like the plain `FOR`, but only directories (not files) are matched and iterated.

**Caveat:** The pattern is matched against the host OS filesystem. Hidden directories (dotfiles on Unix, or `Hidden` attribute on Windows) are included unless filtered manually.

---

## FOR /R [root] %%V IN (pattern) DO — Recursive

```bat
FOR /R . %%f IN (*.log) DO DEL %%f
FOR /R C:\data %%f IN (*.csv) DO ECHO %%f
```

Walks the directory tree rooted at `root` (default: current directory) and matches `pattern` in each subdirectory.

**Caveat:** `root` is optional; if omitted the current directory is used. Unlike real cmd.exe, the pattern must be a filename pattern (not a bare directory). Pattern `*.*` and `*` both match all files.

---

## FOR /F ["options"] %%V IN (input) DO — Field Parsing

Parses text line-by-line and splits each line into fields.

```bat
FOR /F %%L IN (file.txt) DO ECHO %%L
FOR /F "tokens=1,2" %%A IN (file.txt) DO ECHO %%A %%B
FOR /F "delims=," %%A IN (data.csv) DO ECHO %%A
```

### Input sources

| Syntax | Meaning |
|--------|---------|
| `(file.txt)` | Lines from `file.txt` |
| `("literal string")` | Lines from the literal string |
| `('command')` | Lines from the output of `command` |

With `usebackq` option, the quoting is swapped:

| Syntax (usebackq) | Meaning |
|--------------------|---------|
| `("file.txt")` | Lines from `file.txt` |
| `(`literal string`)` | Lines from the literal string |
| `(\`command\`)` | Lines from command output |

### Options string

| Option | Default | Meaning |
|--------|---------|---------|
| `eol=c` | `;` | Skip lines whose first (non-whitespace?) character is `c` |
| `skip=n` | `0` | Skip the first `n` lines |
| `delims=chars` | space + tab | Field delimiter characters |
| `tokens=spec` | `1` | Which fields to capture (see below) |
| `usebackq` | off | Swap quoting rules (see above) |

### tokens= specification

| Spec | Meaning |
|------|---------|
| `1` | First field only → `%%A` |
| `1,2` | Fields 1 and 2 → `%%A`, `%%B` |
| `1-3` | Fields 1 through 3 → `%%A`, `%%B`, `%%C` |
| `1*` | Field 1 and the remainder of the line → `%%A`, `%%B` |
| `*` | Entire line as a single token → `%%A` |

Each additional requested token gets the next letter after the loop variable. If the loop variable is `%%A` and `tokens=1,2`, the second token is in `%%B`.

**Caveats:**

- The `eol` character skips the line only when it is the first character of the line (after leading whitespace is stripped per `delims`).
- `skip=n` counts raw lines before any `eol` filtering.
- If a line has fewer fields than `tokens` requests, the extra loop variables are set to empty.
- Command execution in `'command'` runs via the host shell (`/bin/sh` on Unix) when the command does not resolve to a `.bat`/`.cmd` file. This may behave differently from cmd.exe's `cmd /C command` invocation.
- `usebackq` backtick syntax (`` ` ``) is fully supported.

---

## FOR Variable Tilde Modifiers

Inside the loop body, tilde modifiers can be applied to the loop variable (same set as for positional parameters):

```bat
FOR %%f IN (*.txt) DO (
    ECHO %%~ff    :: full path
    ECHO %%~nf    :: name without extension
    ECHO %%~xf    :: extension
    ECHO %%~zf    :: file size
    ECHO %%~tf    :: last modified time
)
```

**Caveat:** `%%~sf` (short 8.3 name) returns the normal path on Unix since 8.3 names are a Windows NTFS concept.
