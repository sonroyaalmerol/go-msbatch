# START

Starts a separate process to run a program or command.

## Syntax

```bat
START ["title"] [/WAIT] [/B] command [args]
```

## Flags

| Flag | Status | Meaning |
|------|--------|---------|
| `/WAIT` | Implemented | Wait for the started process to exit |
| `/B` | Accepted, no effect | Start without creating a new window |
| `/D path` | Accepted, no effect | Working directory for the new process |
| `/MIN` `/MAX` `/NORMAL` | Accepted, no effect | Window state (minimised, maximised, normal) |
| `/HIGH` `/REALTIME` `/ABOVENORMAL` `/BELOWNORMAL` `/LOW` | Accepted, no effect | Process priority |

## Behaviour

Without `/WAIT`:

- The command is launched asynchronously via `os/exec`.
- `ERRORLEVEL` is set to `0` immediately.
- The started process runs concurrently with the rest of the script.

With `/WAIT`:

- The command is launched and the interpreter blocks until it exits.
- `ERRORLEVEL` is set to the process exit code.

```bat
START notepad.exe
START /WAIT setup.exe /silent
START /B background_task.exe
```

## Caveats

- **Title argument** — real cmd.exe treats the first double-quoted argument as the window title. go-msbatch does not parse a leading title argument; the first argument is always treated as the command name.
- **`/D` (working directory)** is parsed but not applied. The started process inherits the current working directory of the interpreter.
- **Priority and window flags** (`/MIN`, `/MAX`, `/HIGH`, etc.) are silently accepted but have no effect.
- **`/B`** is accepted but on Unix there is no concept of a "new window" — all processes run in the same terminal session.
- Without `/WAIT`, there is no way to retrieve the PID or wait for the process later. The process becomes orphaned from the interpreter's perspective.
- `START` does not search for `.bat`/`.cmd` files. If you need to start another batch file asynchronously, use `START cmd /C script.bat` (but `cmd` may not be available on non-Windows hosts).
- Environment changes made by the started process are **never** visible to the interpreter.
