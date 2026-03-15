# XCOPY

Extended file copy with recursive and filtering capabilities.

## Syntax

```bat
XCOPY source [destination] [flags]
```

## Flags

| Flag | Status | Meaning |
|------|--------|---------|
| `/S` | Implemented | Copy subdirectories (excluding empty) |
| `/E` | Implemented | Copy subdirectories including empty ones |
| `/I` | Implemented | Assume destination is a directory if ambiguous |
| `/Y` | Implemented | Suppress overwrite confirmation |
| `/-Y` | Implemented | Prompt before overwriting |
| `/P` | Implemented | Prompt before creating each file |
| `/Q` | Implemented | Quiet — do not display filenames as they are copied |
| `/F` | Implemented | Display full source and destination paths |
| `/L` | Implemented | List files only — do not copy |
| `/R` | Implemented | Overwrite read-only destination files |
| `/K` | Implemented | Keep read-only attribute on destination |
| `/H` | Implemented | Copy hidden and system files (dotfiles on Unix) |
| `/B` | Implemented | Copy as symbolic link (do not follow) |
| `/C` | Implemented | Continue copying even if errors occur |
| `/V` | Accepted, no effect | Verify writes |
| `/D[:date]` | Implemented | Copy only files newer than `date` (or newer than destination) |
| `/U` | Implemented | Copy only files that already exist at destination |
| `/T` | Implemented | Create directory structure only, no files |
| `/EXCLUDE:file[+file]` | Implemented | Skip files matching patterns in the given file(s) |
| `/A` | **Stub** | Copy only files with archive attribute set |
| `/M` | **Stub** | Same as `/A` but clears archive attribute |
| `/W` | Accepted, no effect | Prompt before starting |

## Per-file output

By default, XCOPY prints the relative source path (with backslashes) for each file copied:

```
xcopy_src\file1.txt
xcopy_src\sub\file2.txt
2 File(s) copied
```

`/Q` suppresses per-file output. `/F` prints full source and destination paths.

## EXCLUDE file format

`/EXCLUDE:patterns.txt` loads a list of patterns (one per line). Any source file whose absolute path contains a pattern substring is skipped.

```
/EXCLUDE:skip.txt+more_skip.txt
```

## Destination resolution

When the destination does not exist:

- If source is a single file and destination has no trailing `\`, XCOPY prompts:
  `Does <dest> specify a file name or directory name on the target (F = File, D = Directory)?`
  unless `/I` is given (treats it as a directory).
- If source is a directory or wildcard, destination is assumed to be a directory.

## Caveats

- **`/A` and `/M` (archive attribute) are stubs.** These flags require Windows-specific file attribute APIs not available cross-platform. They are accepted without error but have no effect.
- **`/V` (verify) has no effect.** No post-write verification is performed.
- **`/W` (wait for keypress before starting) has no effect.**
- **`/K` (keep attributes)**: on Unix, only the read-only (`0o444`) mode bit is considered. Windows attributes (archive, system, hidden) are not preserved because they do not exist on Unix filesystems.
- **Hidden file detection** uses the Unix dotfile convention (names starting with `.`). Files with the Windows `Hidden` attribute but non-dotfile names are not treated as hidden on Unix.
- **`/-Y` prompt**: if stdin is unavailable (nil or at EOF), the prompt defaults to `Y` (overwrite). Scripts relying on `/-Y` to protect files should ensure stdin is available.
- **`/D:date` format**: expects `MM-DD-YYYY`. Dates in other formats are not parsed.
- File permissions on destination are `0666` (subject to umask) by default. With `/K`, the read-only bit is preserved if set on the source.
- `ERRORLEVEL` values:
  - `0` — success, files were copied
  - `1` — no files were found to copy (not an error per se)
  - `2` — user cancelled via prompt
  - `4` — initialisation error
