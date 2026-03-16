# External (Passthrough) Commands

Any command that is not a built-in or a natively-implemented tool is forwarded to the host OS via `os/exec`.  There is no static list — the fallback is unconditional.

## Behaviour

- Arguments are forwarded verbatim after Windows-style path mapping and glob expansion.
- The child process inherits a merged environment (host environment + interpreter's current variable snapshot).
- `ERRORLEVEL` is set to the exit code returned by the child process.
- `.bat` / `.cmd` files found on the path are executed in-process instead (see [architecture.md](../architecture.md)).

## Common examples

The following commands work out of the box on a typical system because the host binary is present in `PATH`.  They receive no special treatment — they are simply forwarded like any other unknown command.

| Command | Notes |
|---------|-------|
| `CURL` | Transfer data with URLs |
| `FINDSTR` | Search for strings with regex support |
| `IPCONFIG` | Display network configuration |
| `NET` | Network services |
| `PING` | Test network connectivity |
| `SSH` | OpenSSH client |
| `TAR` | Archive utility |
| `TASKKILL` / `TASKLIST` | Process management |

This list is illustrative, not exhaustive.  Any executable on `PATH` can be invoked the same way.

## Caveats

- **The command must be installed on the host.** If the binary is not in `PATH`, the command fails with a "not recognized" error.
- Many Windows-specific commands (`REG`, `DISKPART`, `GPUPDATE`, `SCHTASKS`, etc.) will fail on Linux or macOS unless equivalent tools with the same name happen to be installed.
- **Environment changes made by external commands are not visible to the interpreter.** A command like `SETX` writes to the Windows registry but those values will not appear in the interpreter's environment.
- External commands run in a child process; the interpreter's working directory is passed as the child's working directory.
