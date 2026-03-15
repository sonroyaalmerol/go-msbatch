# MKDIR / MD and RMDIR / RD

---

## MKDIR / MD

Creates one or more directories.

### Syntax

```bat
MKDIR path [path ...]
MD path [path ...]
```

### Behaviour

Uses `os.MkdirAll()` which creates the full directory tree including any missing intermediate directories. This is equivalent to `mkdir -p` on Unix.

```bat
MKDIR newdir
MKDIR C:\projects\myapp\src    :: creates all three levels if needed
MD logs tmp cache
```

### Caveats

- **All `/` flags are silently ignored.** No flags are defined for `MKDIR` in cmd.exe anyway, so this only matters if a script accidentally passes flags.
- Arguments beginning with `/` are skipped, not treated as directory names.
- If the directory already exists, no error is raised (unlike POSIX `mkdir` without `-p`). This matches cmd.exe's behaviour.
- On Unix, directories are created with mode `0777` (subject to umask). File permissions from the Windows side are not considered.

---

## RMDIR / RD

Removes directories.

### Syntax

```bat
RMDIR [/S] [/Q] path [path ...]
RD [/S] [/Q] path [path ...]
```

### Flags

| Flag | Meaning |
|------|---------|
| `/S` | Remove the entire directory tree recursively |
| `/Q` | Quiet — do not prompt for confirmation |

### Behaviour

Without `/S`, removes only empty directories. With `/S`, recursively removes all contents.

```bat
RMDIR emptydir
RMDIR /S /Q build
```

### Caveats

- **`/Q` has no real effect** because go-msbatch never prompts for confirmation. The flag is parsed and accepted for compatibility but is functionally a no-op.
- Without `/S`, attempting to remove a non-empty directory returns an error (matching cmd.exe).
- Attempting to remove the current working directory will fail on most OSes.
- Glob expansion in the path argument (e.g. `RMDIR /S /Q tmp*`) is applied before the command runs, so multiple directories can be removed at once.
