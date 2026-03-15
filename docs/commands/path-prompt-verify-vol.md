# PATH, PROMPT, VERIFY, VOL

---

## PATH

Displays or sets the executable search path.

### Syntax

```bat
PATH                 :: display current PATH
PATH ;               :: clear PATH
PATH dir1;dir2;...   :: set PATH
```

### Behaviour

`PATH` with no arguments displays the current value of the `PATH` environment variable.

`PATH ;` clears `PATH` to empty.

`PATH value` sets `PATH` to `value` and also updates the process-level `PATH` via `os.Setenv("PATH", value)`.

### Caveats

- On Unix, path entries are separated by `:` but cmd.exe uses `;`. go-msbatch stores and displays the value as-is; separating by `;` is intentional for compatibility with `.bat` scripts.
- Changes to `PATH` do affect child processes spawned by `os/exec` because the environment is propagated.

---

## PROMPT

Sets the command prompt string.

### Syntax

```bat
PROMPT [string]
PROMPT              :: reset to default ($P$G)
```

### Prompt codes

| Code | Expands to |
|------|-----------|
| `$$` | `$` |
| `$A` | `&` |
| `$B` | `\|` |
| `$C` | `(` |
| `$F` | `)` |
| `$G` | `>` |
| `$L` | `<` |
| `$Q` | `=` |
| `$S` | space |
| `$_` | newline |
| `$D` | current date |
| `$T` | current time |
| `$P` | current drive and path |
| `$N` | current drive letter |
| `$V` | Windows version string |
| `$E` | ESC character |
| `$H` | backspace |
| `$M` | empty string (UNC remote name — not available) |

```bat
PROMPT $P$G          :: C:\Users\Alice>
PROMPT [$T] $P$G     :: [14:32:01.00] C:\Users\Alice>
```

### Caveats

- `$N` (drive letter) returns the first character of the current working directory path. On Linux this may be `/` rather than a Windows drive letter.
- `$M` (UNC remote name) always expands to empty string since there is no UNC path support.
- The prompt is only used by the interactive REPL. It has no effect when running a script file.
- Changes to `PROMPT` are stored in the `PROMPT` environment variable.

---

## VERIFY

Toggles or displays the write-verify flag.

### Syntax

```bat
VERIFY              :: display current state
VERIFY ON
VERIFY OFF
```

### Behaviour

Stores state in the internal `__VERIFY__` environment variable.

```
VERIFY is OFF
VERIFY is ON
```

### Caveats

- **No actual verification is performed.** Real cmd.exe's `VERIFY ON` causes MS-DOS to verify that data is written correctly to disk after each write. go-msbatch stores the flag but does not act on it for any file operation.

---

## VOL

Displays the volume label and serial number of a drive.

### Syntax

```bat
VOL [drive:]
```

### Behaviour

Always prints:

```
Volume in drive X has no label.
Volume Serial Number is 0000-0000
```

The drive letter in the output reflects the argument if given, otherwise uses the current drive.

### Caveats

- **Hard-coded stub.** Real cmd.exe reads the actual volume label and serial from the filesystem. go-msbatch always returns a placeholder because volume label APIs are Windows-specific.
