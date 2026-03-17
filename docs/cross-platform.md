# Cross-Platform Behaviour

go-msbatch runs on Linux, macOS, and Windows. This page documents where behaviour diverges from a native cmd.exe session on Windows.

## Path Mapping

Windows drive letters are transparently remapped to Unix mount points. Backslashes are converted to forward slashes and `filepath.Clean` is applied afterwards.

### Lookup order (first non-empty value wins)

For each drive letter the interpreter consults three sources in order:

1. **`MSBATCH_DRIVE_<LETTER>`** — per-drive override (letter must be uppercase in the var name).
2. **`MSBATCH_DRIVE_ROOT`** — common prefix applied to all unmapped drives.
3. Built-in default **`/mnt/<letter>`** (WSL2 convention).

### Examples

```sh
# All defaults (WSL2-style)
# C:\foo\bar  →  /mnt/c/foo/bar
# D:\data     →  /mnt/d/data

# Shift all drives under /drives/
export MSBATCH_DRIVE_ROOT=/drives/
# C:\foo\bar  →  /drives/c/foo/bar
# D:\data     →  /drives/d/data

# Pin individual drives, let others fall through to MSBATCH_DRIVE_ROOT
export MSBATCH_DRIVE_C=/windows
export MSBATCH_DRIVE_D=/media/data
export MSBATCH_DRIVE_ROOT=/mnt/
# C:\foo\bar  →  /windows/foo/bar
# D:\data     →  /media/data/data
# E:\tmp      →  /mnt/e/tmp   (fallback to MSBATCH_DRIVE_ROOT)
```

All variables are read from the **host** environment (not from inside the batch script).

### UNC paths

UNC paths (`\\server\share\path`) are mapped through a separate set of variables with the same three-level lookup as drive letters:

| Variable | Scope | Example value |
|----------|-------|---------------|
| `MSBATCH_UNC_<SERVER>_<SHARE>` | Exact server + share | `MSBATCH_UNC_MYSERVER_DOCS=/home/user/docs` |
| `MSBATCH_UNC_<SERVER>` | All shares on a server | `MSBATCH_UNC_MYSERVER=/mnt/myserver` |
| `MSBATCH_UNC_ROOT` | All UNC paths | `MSBATCH_UNC_ROOT=/mnt/unc` |

Server and share names are normalised to uppercase and non-alphanumeric characters are collapsed to `_` when forming the variable name:

```sh
# \\build-srv\releases\v1.0  →  MSBATCH_UNC_BUILD_SRV_RELEASES
export MSBATCH_UNC_BUILD_SRV_RELEASES=/opt/releases

# \\nas\media  →  MSBATCH_UNC_NAS_MEDIA
export MSBATCH_UNC_NAS_MEDIA=/mnt/nas/media

# \\nas\*  (all shares on nas fall back to per-server var)
export MSBATCH_UNC_NAS=/mnt/nas
# \\nas\photos  →  /mnt/nas/photos
# \\nas\media   →  /mnt/nas/media (overridden above if both are set)

# Everything else falls back to MSBATCH_UNC_ROOT
export MSBATCH_UNC_ROOT=/mnt/unc
# \\other-server\share\file.txt  →  /mnt/unc/other-server/share/file.txt
```

When **no UNC variable matches**, the path is passed through unchanged (with backslashes converted to forward slashes) so the OS can handle or reject it.

**Caveats:**

- Drive-relative paths (`C:foo` — relative to the current directory on drive C) are treated as absolute, which is incorrect in general.
- The mapping only applies when `MapPath()` is called. Arguments that look like bare filenames are not mapped.

## Running .exe Files

On non-Windows hosts, invoking a `.exe` binary (e.g. `program.exe` or `C:\Tools\app.exe`) follows a two-step resolution process.

### 1. Native Binary Lookup

Before applying an execution prefix, `go-msbatch` checks if a "native" version of the requested command exists on the host system. It searches for a binary with the same name but **without** the `.exe` extension in:

1. The directory containing the `.exe` (if a path was provided).
2. The current working directory (if no path was provided).
3. Every directory in the system `PATH`.

For example, if you run `ls.exe` on Linux, `go-msbatch` will find `/usr/bin/ls` and execute it natively. This allows many scripts to run unchanged if the corresponding tools are installed on the host OS.

### 2. Execution Prefix (Compatibility Layer)

If no native counterpart is found, the command requires a compatibility layer. `go-msbatch` does **not** manage any such layer itself; it simply prepends whatever command you configure to every `.exe` invocation.

[Wine](https://www.winehq.org/) is the most common choice, but anything that acts as a transparent prefix works (e.g. `box64 wine`, a custom wrapper script, a container entry-point, etc.).

#### Configuration

Set the `MSBATCH_EXE_PREFIX` host environment variable to the executable (and any extra flags) you want prepended to every `.exe` invocation:

```sh
# Wine — use whatever "wine" is on PATH
export MSBATCH_EXE_PREFIX=wine

# Explicit 64-bit Wine binary
export MSBATCH_EXE_PREFIX=wine64

# Wine with a custom bottle/prefix
export MSBATCH_EXE_PREFIX="wine --bottle /home/user/.wine-bottles/myapp"

# box64 + Wine (ARM/RISC-V hosts)
export MSBATCH_EXE_PREFIX="box64 wine"

# Any other wrapper that accepts a Windows exe path as its first argument
export MSBATCH_EXE_PREFIX="/usr/local/bin/my-compat-layer"
```

When `MSBATCH_EXE_PREFIX` is **not** set (the default), any `.exe` invocation fails immediately with:

```
cannot execute 'program.exe': no exe prefix configured (set MSBATCH_EXE_PREFIX, e.g. MSBATCH_EXE_PREFIX=wine)
```

`ERRORLEVEL` is set to `9009` in that case, matching the standard "not recognised" exit code.

### How it works

The variable is split on whitespace.  The first token becomes the executable; any remaining tokens are prepended before the `.exe` path.  The `.exe` path is run through `MapPath` (drive-letter remapping) so the prefix tool receives a valid Unix path to the binary:

```
MSBATCH_EXE_PREFIX="wine64 --some-flag"
C:\Tools\app.exe C:\data\file.txt
→  wine64 --some-flag /mnt/c/Tools/app.exe C:\data\file.txt
                      ^^^^^^^^^^^^^^^^^^^^^ ^^^^^^^^^^^^^^^
                      mapped (prefix tool   NOT mapped — the
                      needs a Unix path     Windows binary handles
                      to load the binary)   this via Windows APIs
```

Arguments are intentionally **not** converted to Unix paths.  The Windows binary resolves paths through Windows API calls, which the compatibility layer (e.g. Wine) intercepts and translates internally.  Passing Unix paths as arguments would break any program that treats those strings as Windows paths.

### Caveats

- The exe prefix is never used for `.bat` / `.cmd` files — those are always run in-process.
- Glob patterns in arguments (e.g. `*.txt`) are **not** expanded — expansion is left to the Windows binary or the compatibility layer's own shell layer.
- Exit codes from the prefix command are forwarded to `ERRORLEVEL` as-is.
- Tool-specific environment variables (e.g. `WINEPREFIX`) must be configured separately in the host environment; go-msbatch does not set them.

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

## VER output

`VER` prints the string controlled by the `MSBATCH_VERSION` host environment variable.  When unset, the default is derived from the host OS at runtime:

| Host OS | Source | Example output |
|---------|--------|----------------|
| Linux / macOS | `uname -s` + `uname -r` | `Linux [Version 6.19.5-3-cachyos]` |
| Windows | `cmd /c ver` | `Microsoft Windows [Version 10.0.19045]` |
| *(exec fails)* | `runtime.GOOS` | `linux` |

Override it before launching the interpreter:

```sh
export MSBATCH_VERSION="MyApp Runner [Version 2.1.0]"
msbatch myscript.bat
```

The same string is also printed as the interactive session banner when no script file is provided.

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
