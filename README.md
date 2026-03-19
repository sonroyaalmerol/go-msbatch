# go-msbatch

[![Go Reference](https://pkg.go.dev/badge/github.com/sonroyaalmerol/go-msbatch.svg)](https://pkg.go.dev/github.com/sonroyaalmerol/go-msbatch)

A cross-platform Windows CMD/Batch interpreter written in Go. Mirrors `cmd.exe`'s multi-phase processing model with a recursive-descent parser and executor.

## Installation

Download the latest binary for your platform from the [releases page](https://github.com/sonroyaalmerol/go-msbatch/releases), or install from source:

```bash
go install github.com/sonroyaalmerol/go-msbatch/cmd/msbatch@latest
```

## Usage

```bash
# Run a batch file
msbatch script.bat [arg1 arg2 ...]

# Run with /C (execute command and exit)
msbatch /C "echo hello & set A=1"

# Run with /K (execute command then enter interactive mode)
msbatch /K "set MYVAR=hello"

# Interactive REPL
msbatch
```

### Interactive Mode

When run without arguments, `msbatch` starts an interactive session with:

- **Tab completion** for commands and file paths
- **Command history** (saved to `~/.msbatch_history`)
- **Line continuation** with `^` (More? prompt)
- **Batch file execution** — type `./script.bat` to run a batch file in the current session
- **CMD-style prompt** (customizable via `PROMPT` variable)

```batch
C:\> echo hello
hello
C:\> set MYVAR=world
C:\> echo %MYVAR%
world
C:\> ./myscript.bat arg1 arg2
```

### Wine Integration

On Linux/macOS, `msbatch` can run Windows `.exe` files via Wine:

```bash
# Set the Wine prefix
export MSBATCH_EXE_PREFIX=wine

# Now Windows executables work
msbatch myscript.bat
```

Z: drive paths work natively (`Z:\home\user\file` → `/home/user/file`), enabling full Wine compatibility.

## Library usage

```go
import (
    "github.com/sonroyaalmerol/go-msbatch/pkg/executor"
    "github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// Best effort CMD.EXE compatibility
proc := processor.New(env, args, executor.New())

// Custom command set
reg := executor.NewEmpty()
reg.HandleFunc("print", func(p *processor.Processor, cmd *parser.SimpleCommand) error {
    fmt.Fprintln(p.Stdout, strings.Join(cmd.Args, " "))
    return nil
})
proc := processor.New(env, args, reg)

// Extend built-ins with your own commands
reg := executor.New()
reg.HandleFunc("mycommand", myHandler)
proc := processor.New(env, args, reg)
```

## Testing

```bash
go test ./...            # unit + integration
go test -v ./tests/...   # verbose integration output
```

## Documentation

Full documentation lives in [`docs/`](docs/index.md).

- [Architecture & Processing Phases](docs/architecture.md)
- [Cross-Platform Behaviour](docs/cross-platform.md)
- [Variables & Expansion](docs/language/variables.md)
- [Arithmetic — SET /A](docs/language/arithmetic.md)
- [Control Flow — IF, GOTO, CALL, EXIT, SHIFT](docs/language/control-flow.md)
- [FOR Loops](docs/language/for-loops.md)
- [Redirection & Pipes](docs/language/redirects.md)

### Command reference

| Command(s) | Doc |
|------------|-----|
| `ECHO` | [docs/commands/echo.md](docs/commands/echo.md) |
| `SET` | [docs/commands/set.md](docs/commands/set.md) |
| `CD` / `CHDIR` | [docs/commands/cd.md](docs/commands/cd.md) |
| `TYPE`, `DIR`, `MORE` | [docs/commands/type-dir-more.md](docs/commands/type-dir-more.md) |
| `CLS`, `TITLE`, `COLOR` | [docs/commands/cls-title-color.md](docs/commands/cls-title-color.md) |
| `VER`, `PAUSE`, `BREAK` | [docs/commands/ver-pause-break.md](docs/commands/ver-pause-break.md) |
| `DATE`, `TIME` | [docs/commands/date-time.md](docs/commands/date-time.md) |
| `PATH`, `PROMPT`, `VERIFY`, `VOL` | [docs/commands/path-prompt-verify-vol.md](docs/commands/path-prompt-verify-vol.md) |
| `PUSHD`, `POPD` | [docs/commands/pushd-popd.md](docs/commands/pushd-popd.md) |
| `MKDIR` / `MD`, `RMDIR` / `RD` | [docs/commands/mkdir-rmdir.md](docs/commands/mkdir-rmdir.md) |
| `DEL` / `ERASE` | [docs/commands/del.md](docs/commands/del.md) |
| `COPY` | [docs/commands/copy.md](docs/commands/copy.md) |
| `MOVE`, `REN` / `RENAME` | [docs/commands/move-ren.md](docs/commands/move-ren.md) |
| `MKLINK` | [docs/commands/mklink.md](docs/commands/mklink.md) |
| `START` | [docs/commands/start.md](docs/commands/start.md) |
| `ASSOC`, `FTYPE` | [docs/commands/assoc-ftype.md](docs/commands/assoc-ftype.md) |
| `FIND` | [docs/commands/find.md](docs/commands/find.md) |
| `SORT` | [docs/commands/sort.md](docs/commands/sort.md) |
| `TREE` | [docs/commands/tree.md](docs/commands/tree.md) |
| `XCOPY` | [docs/commands/xcopy.md](docs/commands/xcopy.md) |
| `ROBOCOPY` | [docs/commands/robocopy.md](docs/commands/robocopy.md) |
| `WHERE`, `HOSTNAME`, `WHOAMI`, `TIMEOUT` | [docs/commands/utils.md](docs/commands/utils.md) |
| Passthrough commands | [docs/commands/passthrough.md](docs/commands/passthrough.md) |
