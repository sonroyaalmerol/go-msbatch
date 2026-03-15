# ROBOCOPY

Robust file copy with mirroring, filtering, and retry logic.

## Syntax

```bat
ROBOCOPY source destination [file [file ...]] [flags]
```

`file` is one or more filename patterns (default: `*.*`).

## Copy behaviour flags

| Flag | Status | Meaning |
|------|--------|---------|
| `/S` | Implemented | Copy subdirectories (excluding empty) |
| `/E` | Implemented | Copy subdirectories including empty ones |
| `/MIR` | Implemented | Mirror source → destination (implies `/E` + `/PURGE`) |
| `/PURGE` | Implemented | Delete destination files not in source |
| `/MOV` | Implemented | Move files (delete source after copy) |
| `/MOVE` | Implemented | Move files and directories |
| `/SL` | Implemented | Copy symbolic links as links (not targets) |
| `/CREATE` | Implemented | Create directory tree only (zero-length files) |
| `/LEV:n` | Implemented | Copy only the top `n` levels of the tree |
| `/L` | Implemented | List only — do not copy |

## File selection flags

| Flag | Status | Meaning |
|------|--------|---------|
| `/XF pattern [...]` | Implemented | Exclude files matching patterns |
| `/XD pattern [...]` | Implemented | Exclude directories matching patterns |
| `/XO` | Implemented | Exclude older files |
| `/XN` | Implemented | Exclude newer files |
| `/XC` | Implemented | Exclude changed files (same size/time, different content — approximated by size+time) |
| `/XL` | Implemented | Exclude lonely files (in source only) |
| `/XX` | Implemented | Exclude extra files (in destination only) |
| `/IS` | Implemented | Include same files |
| `/XJ` | Implemented | Exclude junction points |
| `/XJD` | Implemented | Exclude junction points for directories |
| `/XJF` | Implemented | Exclude junction points for files |
| `/MAX:n` | Implemented | Exclude files larger than `n` bytes |
| `/MIN:n` | Implemented | Exclude files smaller than `n` bytes |
| `/MAXAGE:n` | Implemented | Exclude files older than `n` days (or absolute date) |
| `/MINAGE:n` | Implemented | Exclude files newer than `n` days |
| `/FFT` | Implemented | Assume FAT file times (2-second granularity) |
| `/A` | **Stub** | Copy only archive files |
| `/M` | **Stub** | Copy archive files and reset archive flag |
| `/IA:flags` | Accepted, no effect | Include only files with attributes |
| `/XA:flags` | Accepted, no effect | Exclude files with attributes |

## Multithreading

| Flag | Status | Meaning |
|------|--------|---------|
| `/MT` | Implemented | Use 8 threads (default when no count given) |
| `/MT:n` | Implemented | Use `n` threads (1–128) |

`/MT` parallelises **file copies within each directory** using a goroutine pool capped at `n` workers. Directory recursion itself is always serial (one directory at a time), matching the real Robocopy behaviour.

The output writer and statistics counters are internally protected with locks when `/MT` is active, so the summary and log file remain consistent regardless of thread count.

## Retry flags

| Flag | Status | Meaning |
|------|--------|---------|
| `/R:n` | Implemented | Number of retries on failed copies (default 1) |
| `/W:n` | Implemented | Wait time in seconds between retries (default 3) |

## Attribute modification

| Flag | Status | Meaning |
|------|--------|---------|
| `/A+:flags` | Implemented (Unix-limited) | Add file attributes after copy |
| `/A-:flags` | Implemented (Unix-limited) | Remove file attributes after copy |

Only the `R` (read-only) flag has an effect on Unix. `A`, `S`, `H`, `C`, `N`, `E`, `T` are accepted but ignored.

## Output / logging flags

| Flag | Status | Meaning |
|------|--------|---------|
| `/LOG:file` | Implemented | Write output to log file (overwrite) |
| `/LOG+:file` | Implemented | Write output to log file (append) |
| `/TEE` | Implemented | Output to both log and stdout |
| `/NFL` | Implemented | No file list in output |
| `/NDL` | Implemented | No directory list in output |
| `/NJH` | Implemented | No job header |
| `/NJS` | Implemented | No job summary |
| `/FP` | Implemented | Include full path in output |
| `/NS` | Implemented | No file sizes |
| `/NC` | Implemented | No file classes |
| `/TS` | Implemented | Include file timestamps |
| `/V` | Implemented | Verbose — show skipped files |

## Accepted but non-functional flags

The following flags are parsed and accepted without error but have no effect:

`/B` `/COMPRESS` `/J` `/Z` `/ZB` `/SEC` `/SECFIX` `/DST` `/COPYALL` `/NOCOPY`
`/NODCOPY` `/IM` `/PF` `/256` `/UNICODE` `/BYTES` `/DEBUG` `/ETA` `/TIMFIX`
`/NOSD` `/NODD` `/QUIT` `/IF` `/COPY:` `/DCOPY:` `/IPG:`
`/MON:` `/MOT:` `/RH:` `/JOB:` `/SAVE:` `/LFSM` `/MAXLAD:` `/MINLAD:`
`/SD:` `/DD:` `/UNILOG` `/UNILOG+:`

## Exit codes

ROBOCOPY uses bitwise exit codes (unlike most commands which use 0/1):

| Bit | Value | Meaning |
|-----|-------|---------|
| 0 | 1 | One or more files were copied |
| 1 | 2 | Extra files or directories found in destination |
| 2 | 4 | Mismatched files found |
| 3 | 8 | Some files or directories could not be copied |
| 4 | 16 | Fatal error |

A value of `0` means no files were copied and no failures. These values can be combined: `ERRORLEVEL 3` means files were copied AND extras were found.

**Caveat:** Scripts that check `IF ERRORLEVEL 1 ECHO error` will trigger on a successful copy (since bit 0 is set). Check specific bits or use exact equality.

## Summary output

By default, ROBOCOPY prints a job header and summary:

```
-------------------------------------------------------------------------------
   ROBOCOPY     ::     Robust File Copy for Windows
-------------------------------------------------------------------------------

  Source : C:\src\
  Dest   : C:\dst\
  Files  : *.*

  Options : /S /FFT /R:1 /W:3

------------------------------------------------------------------------------

                   Total    Copied   Skipped  Mismatch    FAILED    Extras
    Dirs :             3         3         0         0         0         0
   Files :             5         5         0         0         0         0
   Bytes :          1.0k      1.0k         0         0         0         0

   Speed :               12345 Bytes/sec.
   Speed :               0.123 MegaBytes/min.
```

Use `/NJH /NJS` to suppress header and summary for cleaner scripted output.

## Caveats

- **`/A` and `/M`** (archive attribute) require Windows-specific attribute APIs; they are accepted but have no effect on Unix.
- **`/MT[:n]`** parallelises file copies within each directory. The thread count applies per-directory; overall parallelism can therefore exceed `n` when directories are deeply nested and multiple levels are active simultaneously.
- **`/JOB:` and `/SAVE:`** (job files) are not implemented.
- **ACL/security copy flags** (`/SEC`, `/SECFIX`, `/COPY:O`, `/COPY:S`, `/SECFIX`, `/EFSRAW`) are accepted but ACL operations require Windows APIs.
- **Junction detection** (`/XJ`, `/XJD`, `/XJF`): on Unix a "junction" is approximated by any symlink pointing to a directory.
- **`/FFT`** (FAT file time granularity): when enabled, timestamps are compared with 2-second tolerance to account for FAT filesystem rounding. This is implemented.
