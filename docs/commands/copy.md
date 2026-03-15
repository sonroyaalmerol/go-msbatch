# COPY

Copies one or more files to a destination, with optional concatenation.

## Syntax

```bat
COPY [/Y | /-Y] [/B | /A] [/V] source destination
COPY [/Y | /-Y] src1+src2[+...] destination   :: concatenate
```

## Flags

| Flag | Status | Meaning |
|------|--------|---------|
| `/Y` | Accepted, no effect | Suppress overwrite confirmation |
| `/-Y` | Accepted, no effect | Prompt before overwriting |
| `/B` | Accepted, no effect | Binary mode |
| `/A` | Accepted, no effect | ASCII mode (stop at Ctrl-Z) |
| `/V` | Accepted, no effect | Verify writes |

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

Files are concatenated in order. The destination is created (or overwritten).

## COPYCMD environment variable

If `COPYCMD` contains `/Y`, overwrite prompts are suppressed globally.

## Caveats

- **`/Y` and `/-Y` are accepted but ignored.** go-msbatch never prompts before overwriting. To prevent accidental overwrites, use `IF EXIST` before `COPY`.
- **`/B` and `/A` have no effect.** All files are copied as raw byte streams. Ctrl-Z is not treated as an end-of-file marker even with `/A`.
- **`/V` (verify) has no effect.** No checksum or re-read verification is performed after copying.
- Files are always created with mode `0666` (subject to umask on Unix). Original file permissions and attributes are **not preserved**.
- Glob expansion in the source is supported (`*.txt`). If no files match, the pattern is passed as-is and the copy fails with a "file not found" error.
- `COPY` does not support copying directories; use `XCOPY` or `ROBOCOPY` for that.
- `ERRORLEVEL` is set to `0` on success, `1` on failure. A partial failure (some files copied, others not) may leave `ERRORLEVEL` as `0`.
