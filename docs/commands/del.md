# DEL / ERASE

Deletes files.

## Syntax

```bat
DEL [/S] [/Q] [/F] [/A[:attr]] pattern [pattern ...]
ERASE [/S] [/Q] [/F] [/A[:attr]] pattern [pattern ...]
```

## Flags

| Flag | Status | Meaning |
|------|--------|---------|
| `/S` | Implemented | Delete matching files in all subdirectories |
| `/Q` | Accepted, no effect | Quiet — suppress confirmation prompts |
| `/F` | Accepted, no effect | Force deletion of read-only files |
| `/A[:attr]` | **Not implemented** | Filter by file attributes |

## Behaviour

Deletes files matching each `pattern`. Glob wildcards (`*`, `?`) are expanded.

```bat
DEL temp.txt
DEL *.log
DEL /S /Q *.tmp
```

With `/S`, the pattern is matched in the current directory and all subdirectories.

## Caveats

- **`/A` attribute filter is not implemented.** Flags like `/A:H` (hidden), `/A:R` (read-only), `/A:-H` (not hidden) are accepted by the parser but silently ignored — all matching files are deleted regardless of attributes.
- **`/Q` has no effect** — go-msbatch never prompts for confirmation when deleting files.
- **`/F` has no effect** — read-only files may or may not be deletable depending on the OS and file permissions. go-msbatch attempts the deletion but does not explicitly clear the read-only bit first.
- Deleting a directory path (not a file) is silently skipped.
- If a pattern matches no files, no error is raised and `ERRORLEVEL` remains `0`. Real cmd.exe prints a "Could Not Find" message and may set `ERRORLEVEL` to `1` in some cases.
- Glob expansion is performed by Go's `filepath.Glob`; `[…]` bracket patterns are supported by the Go glob engine but not by cmd.exe.
