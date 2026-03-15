# Passthrough Commands

These commands are registered in the command registry (so they appear in help/listing) but are not implemented natively. On execution they are forwarded to the host OS via `os/exec`.

## List

| Command | Notes |
|---------|-------|
| `ATTRIB` | File attribute management |
| `CHCP` | Change code page |
| `CHOICE` | Prompt user for Y/N choice |
| `CLIP` | Copy stdin to clipboard |
| `COMP` | Compare contents of two files |
| `CURL` | Transfer data with URLs |
| `DISKPART` | Disk partition utility |
| `FC` | File compare |
| `FINDSTR` | Search for strings with regex support |
| `FORFILES` | Batch processing on files |
| `GETMAC` | Display MAC addresses |
| `GPUPDATE` | Refresh Group Policy settings |
| `IPCONFIG` | Display network configuration |
| `NET` | Network services |
| `NETSTAT` | Display network connections |
| `NSLOOKUP` | DNS lookup |
| `PING` | Test network connectivity |
| `REG` | Read/write Windows registry |
| `SC` | Service control |
| `SCHTASKS` | Task scheduler |
| `SETX` | Set persistent environment variables |
| `SHUTDOWN` | Shutdown or restart the computer |
| `SSH` | OpenSSH client |
| `SYSTEMINFO` | Display OS and hardware information |
| `TAKEOWN` | Take ownership of files |
| `TAR` | Archive utility |
| `TASKKILL` | Kill running processes |
| `TASKLIST` | List running processes |
| `TRACERT` | Trace network route |

## Behaviour

When one of these commands is executed, the interpreter calls the host OS binary of the same name using `os/exec`. Arguments are forwarded verbatim.

- Glob patterns in arguments are expanded by the interpreter before forwarding.
- Windows-style paths in arguments are mapped to Unix paths before forwarding.
- The child process inherits a merged environment (host environment + interpreter's current variable snapshot).
- `ERRORLEVEL` is set to the exit code returned by the child process.

## Caveats

- **The command must be installed on the host.** If `ping` is not in `PATH`, the command fails with a "not recognized" error.
- Many of these commands are Windows-only (`REG`, `DISKPART`, `GPUPDATE`, `SCHTASKS`, etc.). On Linux or macOS they will fail unless equivalent tools with the same name happen to be installed.
- **Environment changes made by passthrough commands are not visible to the interpreter.** `SETX` for example writes to the Windows registry but those values will not appear in the interpreter's environment.
- Passthrough commands run in a child process; the interpreter's working directory is set as the child's working directory.

## Unregistered commands

Any command not in the registry (neither built-in, native, nor passthrough) is also forwarded to the host OS the same way. Unregistered commands that resolve to a `.bat` or `.cmd` file are executed in-process (see [architecture.md](../architecture.md)).
