# MOVE and REN / RENAME

---

## MOVE

Moves or renames files and directories.

### Syntax

```bat
MOVE [/Y | /-Y] source destination
```

### Flags

| Flag | Status | Meaning |
|------|--------|---------|
| `/Y` | Accepted, no effect | Suppress overwrite confirmation |
| `/-Y` | Accepted, no effect | Prompt before overwriting |

### Behaviour

```bat
MOVE old.txt new.txt           :: rename file
MOVE file.txt C:\archive\      :: move file into directory
MOVE C:\src\*.txt C:\dst\      :: move multiple files
```

Uses `os.Rename()` internally. When source and destination are on different filesystems, the move falls back to copy-then-delete.

### Caveats

- **`/Y` and `/-Y` are ignored** — go-msbatch never prompts before overwriting. If the destination exists it is silently replaced.
- Moving a directory across filesystems is not supported (only same-filesystem renames work for directories).
- Glob expansion in the source is supported.
- `os.Rename()` on Linux is atomic within a filesystem but not across filesystems. Partial failures on cross-device moves are possible.

---

## REN / RENAME

Renames files using optional glob patterns.

### Syntax

```bat
REN source newname
RENAME source newname
```

`newname` is a **filename only** (no path component). The file stays in the same directory.

### Glob pattern support

Wildcards in `source` expand to multiple files. In `newname`, `*` preserves the corresponding part of the original name, and `?` preserves a single character.

```bat
REN *.txt *.bak         :: rename all .txt to .bak
REN report_?.doc old_?.doc
```

### Caveats

- `newname` cannot contain a directory path. `REN file.txt dir\file.txt` is not valid; use `MOVE` instead.
- Glob pattern substitution (`*` in destination) is implemented: the `*` in the newname replaces the stem matched by `*` in the source. Complex patterns with multiple wildcards may not produce the exact result as cmd.exe.
- If `source` matches no files, an error is printed and `ERRORLEVEL` is set to `1`.
