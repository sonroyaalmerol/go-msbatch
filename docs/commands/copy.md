# COPY

Copies one or more files to a destination, with optional concatenation.

## Syntax

```bat
COPY [/Y | /-Y] [/B | /A] [/V] source destination
COPY [/Y | /-Y] src1+src2[+...] destination   :: concatenate
```

## Flags

| Flag  | Status              | Meaning                          |
| ----- | ------------------- | -------------------------------- |
| `/Y`  | Accepted, no effect | Suppress overwrite confirmation  |
| `/-Y` | Accepted, no effect | Prompt before overwriting        |
| `/B`  | Implemented         | Binary mode (no Ctrl-Z handling) |
| `/A`  | Implemented         | Text mode (stop at Ctrl-Z)       |
| `/V`  | Accepted, no effect | Verify writes                    |

## Simple copy

```bat
COPY src.txt dst.txt
COPY *.txt backup\
COPY file.txt C:\archive\
```

When `destination` is an existing directory, the file is copied into that directory keeping the original filename.

When `destination` does not exist, it is created as a new file with that name.

## Concatenation with `+`

Multiple source files can be joined into one destination file:

```bat
COPY header.txt+body.txt+footer.txt combined.txt
COPY part1.txt + part2.txt result.txt    :: spaces around + are allowed
```

Files are concatenated in order. The destination is created (or overwritten). Each copied source name is echoed, one final Ctrl-Z byte terminates the output in text mode (`/A` is the default), and `/B` copies raw bytes with no Ctrl-Z. A missing source is skipped silently as long as at least one source resolves.

## COPYCMD environment variable

If `COPYCMD` contains `/Y`, overwrite prompts are suppressed globally.

## Caveats

- **`/Y` and `/-Y` are accepted but ignored.** go-msbatch never prompts before overwriting. To prevent accidental overwrites, use `IF EXIST` before `COPY`.
- **`/B` and `/A` are implemented for concatenation and multi-source copies.** With `/A` (the default) each source is truncated at its first Ctrl-Z byte and the combined output ends with one Ctrl-Z; `/B` copies raw bytes and adds no marker. Single-file copies are byte-for-byte regardless of mode.
- **`/V` (verify) has no effect.** No checksum or re-read verification is performed after copying.
- Files are always created with mode `0666` (subject to umask on Unix). Original file permissions and attributes are **not preserved**.
- Glob expansion in the source is supported (`*.txt`). If no files match, the pattern is echoed followed by `The system cannot find the file specified.`, and `0 file(s) copied.` is reported for wildcard sources.
- `COPY` does not support copying directories; use `XCOPY` or `ROBOCOPY` for that.
- `ERRORLEVEL` is set to `0` on success, `1` on failure. A partial failure (some files copied, others not) may leave `ERRORLEVEL` as `0`.
