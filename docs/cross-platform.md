# Cross-Platform Behaviour

go-msbatch runs on Linux, macOS, and Windows. This page documents where behaviour diverges from a native cmd.exe session on Windows.

## Path Mapping

Windows drive letters are transparently remapped to Unix paths. Backslashes are converted to forward slashes and `filepath.Clean` is applied afterwards.

### Default Drive Mappings (Wine-style)

By default, go-msbatch uses Wine-style drive mappings:

| Drive | Default Mapping | Description |
|-------|-----------------|-------------|
| **Z:** | `/` | Linux root — provides access to entire Unix filesystem |
| **C:** | `drive_c` | Relative path (typical Wine convention) |
| **D:-Y:** | `drive_d`-`drive_y` | Relative paths |

This matches how Wine maps drives by default, making it easy to use scripts that access the Unix filesystem via `Z:\`.

### Lookup Order (first match wins)

For each drive letter, the interpreter consults these sources in order:

1. **`MSBATCH_DRIVE_<LETTER>`** — per-drive override (letter must be uppercase in the var name).
2. **`MSBATCH_PREFIX`** — common prefix for all drives except Z.
3. Built-in defaults (see table above).

### Examples

```sh
# Default Wine-style mappings
# Z:\home\user\file.txt  →  /home/user/file.txt
# Z:\data\results        →  /data/results
# C:\Windows\System32    →  drive_c/Windows/System32
# D:\programs            →  drive_d/programs

# Set a Wine prefix for all drives (except Z:)
export MSBATCH_PREFIX=/home/user/.wine
# C:\Windows\System32  →  /home/user/.wine/drive_c/Windows/System32
# D:\programs          →  /home/user/.wine/drive_d/programs
# Z:\home\user         →  /home/user  (Z: always maps to /)

# Override specific drives
export MSBATCH_DRIVE_Z=/data/speedtestfdb
export MSBATCH_DRIVE_C=/mnt/windows
# Z:\Data\Flight\1026   →  /data/speedtestfdb/Data/Flight/1026
# C:\Users\test         →  /mnt/windows/Users/test

# Common pattern: map Z: to your data directory
export MSBATCH_DRIVE_Z=/data
# Z:\Data\Flight\1026\GRV1\TA\ProcFiles  →  /data/Data/Flight/1026/GRV1/TA/ProcFiles
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

On non-Windows hosts, invoking a `.exe` binary (e.g. `program.exe` or `C:\Tools\app.exe`) follows a resolution process with automatic Wine fallback.

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

### 3. Automatic Wine Fallback for Unknown Commands

When a command is not found as an internal command, batch file, or native binary, `go-msbatch` will automatically try running it via Wine (if `MSBATCH_EXE_PREFIX` is configured). This allows scripts to call Windows executables without the `.exe` extension:

```
# If "mytool" is not found as a native command or batch file,
# go-msbatch will try: wine mytool.exe
mytool arg1 arg2
```

This fallback respects Wine's internal PATH, so commands in `C:\Windows\System32` and other Wine-mapped directories will be found even if they're not visible to the host OS.

### How it works

The variable is split on whitespace. The first token becomes the executable; any remaining tokens are prepended before the `.exe` path. The `.exe` path is passed directly to Wine (not mapped) so Wine can resolve it through its own Windows APIs:

```
MSBATCH_EXE_PREFIX="wine64 --some-flag"
C:\Tools\app.exe C:\data\file.txt
→  wine64 --some-flag C:\Tools\app.exe C:\data\file.txt
                       ^^^^^^^^^^^^^^^^^ NOT mapped — Wine resolves
                       this via Windows APIs

Z:\home\user\file.txt
→  wine64 ... Z:\home\user\file.txt
                       ^^^^^^^^^^^^^^^^^^^ Z: paths work in Wine!
```

**Z: drive paths are preserved** when passed to Wine. In Wine, `Z:` maps to the Unix root `/`, so `Z:\home\user\file.txt` correctly accesses `/home/user/file.txt`.

Arguments are intentionally **not** converted to Unix paths. The Windows binary resolves paths through Windows API calls, which the compatibility layer (e.g. Wine) intercepts and translates internally. Passing Unix paths as arguments would break any program that treats those strings as Windows paths.

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

## Debug Logging

`go-msbatch` includes a built-in debug logging system powered by Go's `log/slog` library. It provides deep visibility into the interpreter's internal operations, including command execution, variable expansion, and control flow transitions.

> **Note:** This is for internal interpreter debugging. For interactive batch script debugging with breakpoints, see [Trace Debugging & Interactive Debugger](trace-debugging.md).

### Enabling Logs

Set the `MSBATCH_DEBUG` environment variable to `true`, `1`, `on`, or any non-empty value (except `0` or `false`) to enable debug output.

```sh
export MSBATCH_DEBUG=true
```

### Configuring Output

By default, logs are written to **stderr**. You can redirect them to **stdout** or a specific file using the `MSBATCH_DEBUG_FILE` variable:

```sh
# Log to stdout
export MSBATCH_DEBUG_FILE=stdout

# Log to a specific file (appends if file exists)
export MSBATCH_DEBUG_FILE=msbatch_debug.log
```

### What is logged?

- **Execution Start**: Total node count and a snapshot of the initial environment variables.
- **Command Dispatch**: The name and fully expanded arguments of every command before it runs.
- **Control Flow**: Label jumps (`GOTO`), subroutine entries (`CALL :label`), and batch file invocations.
- **External Commands**: Native OS command execution, including mapped Unix paths and arguments.

## Case sensitivity

Variable names and command names are compared case-insensitively, matching cmd.exe. However, file paths on case-sensitive Unix filesystems are case-sensitive. A script that relies on Windows case-insensitive file lookup may fail on Linux.
