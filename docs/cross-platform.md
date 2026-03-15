# Cross-Platform Behaviour

go-msbatch runs on Linux, macOS, and Windows. This page documents where behaviour diverges from a native cmd.exe session on Windows.

## Path Mapping

Windows drive paths are transparently remapped on Unix hosts:

| Windows path | Unix equivalent |
|--------------|----------------|
| `C:\foo\bar` | `/mnt/c/foo/bar` |
| `D:\data` | `/mnt/d/data` |
| `\foo` (rooted, no drive) | `/foo` |

Backslashes in paths are converted to forward slashes. `filepath.Clean` is applied afterwards.

**Caveats:**

- UNC paths (`\\server\share\path`) are not remapped and will likely fail on Unix.
- Drive-relative paths (`C:foo` — a path relative to the current directory on drive C) are treated as absolute (`/mnt/c/foo`), which is incorrect in general.
- The mapping only applies when `MapPath()` is called. Arguments that look like bare filenames are not mapped.

## ANSI Escape Sequences

The following commands use ANSI escape codes instead of Windows Console API calls:

| Command | ANSI sequence used |
|---------|--------------------|
| `CLS` | `ESC[2J ESC[H` — erase screen and move cursor home |
| `TITLE` | `ESC]0;...\a` — OSC title sequence |
| `COLOR` | `ESC[<n>m` — SGR foreground/background |

On Windows, ANSI output requires a terminal that supports VT sequences (Windows Terminal, VS Code terminal, ConEmu, etc.). The legacy `conhost.exe` may not render them correctly unless virtual terminal processing is enabled (`ENABLE_VIRTUAL_TERMINAL_PROCESSING`).

## File Attributes

Several commands accept attribute-related flags but cannot act on them cross-platform:

| Flag | Command | Status |
|------|---------|--------|
| `/A` (attribute filter) | `DEL` | Not implemented — silently ignored |
| `/A` (archive only) | `XCOPY`, `ROBOCOPY` | Accepted, no effect |
| `/M` (archive + clear) | `XCOPY`, `ROBOCOPY` | Accepted, no effect |
| `/COPY:` ACL options | `ROBOCOPY` | Accepted, no effect |
| `/SEC`, `/SECFIX` | `ROBOCOPY` | Accepted, no effect |

Windows file attributes (archive, hidden, system, read-only) are not natively accessible via Go's `os` package. Hidden files are approximated by the Unix dotfile convention (names starting with `.`).

## Symlinks and Hard Links

`MKLINK` uses `os.Symlink()` for symbolic links and `os.Link()` for hard links.

| Flag | Windows behaviour | Unix behaviour |
|------|-------------------|----------------|
| (none) | File symbolic link | `os.Symlink` |
| `/D` | Directory symbolic link | `os.Symlink` (identical to no flag) |
| `/J` | NTFS junction | `os.Symlink` (closest equivalent) |
| `/H` | Hard link | `os.Link` |

Creating symlinks on Windows may require elevated privileges or Developer Mode.

## ROBOCOPY multithreading

`/MT[:n]` is accepted but the copy always runs single-threaded.

## VER output

`VER` always prints a hard-coded Windows version string regardless of the actual host OS:

```
Microsoft Windows [Version 10.0.19045.5442]
```

## VOL output

`VOL` always prints a placeholder volume label and serial number `0000-0000`.

## ASSOC and FTYPE

Associations and file-type open commands are stored in memory only. They are not read from or written to the Windows registry. They do not persist between interpreter sessions.

## External command environment

When a command falls through to `os/exec` (not a `.bat`/`.cmd` file), the child process inherits the host environment merged with the interpreter's current variable snapshot. However, environment changes made by the child process are **not** reflected back in the interpreter's `Env`. This matches cmd.exe's behaviour for non-batch external commands.

## SET /A division and modulo by zero

Real cmd.exe raises a divide-by-zero error. go-msbatch silently returns `0` for both `/` and `%` when the divisor is zero.

## Case sensitivity

Variable names and command names are compared case-insensitively, matching cmd.exe. However, file paths on case-sensitive Unix filesystems are case-sensitive. A script that relies on Windows case-insensitive file lookup may fail on Linux.
