# go-msbatch

A cross-platform, systematic implementation of the Windows CMD/Batch interpreter in Go. The project attempts to mirror `cmd.exe`'s multi-phase processing model, producing an AST from a recursive-descent parser and executor.

## Usage

```bash
# Run a batch file
go run ./cmd/msbatch script.bat [arg1 arg2 ...]

# Interactive REPL
go run ./cmd/msbatch
```

## Architecture

```
cmd/msbatch/           Binary entry point (file mode + interactive REPL)
internal/lex/          Generic cursor-based state-machine lexer framework
pkg/lexer/             Batch-specific tokenizer (BatchLexer → 19 token types)
pkg/parser/            Recursive-descent AST builder
pkg/processor/         Multi-phase expansion engine + flow-control executor
pkg/executor/          Built-in command registry (Registry) + implementations
tests/                 22 integration test pairs (*.bat + *.out)
```

### Library usage

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

## Processing Phases

The interpreter follows `cmd.exe`'s documented 6-phase model exactly:

| Phase | Name | Description |
|-------|------|-------------|
| 0 | Read Line | Ctrl-Z → `\n`; trailing `^` merges the next line |
| 1 | Percent Expansion | `%VAR%`, `%0`–`%9`, `%*`, `%%`, slicing, substitution, `%~[mods]n` tilde modifiers |
| 2 | Lex & Parse | Tokenise then build AST (commands, blocks, operators) |
| 3 | Echo Suppression | `@` prefix; `ECHO ON`/`OFF` state |
| 4 | FOR Variable Expansion | `%%i` / `%i`, tilde modifiers (`%~nxpdf`, `%~atz`, …) |
| 5 | Delayed Expansion | `!VAR!`; `^!` → literal `!` |

## Implemented Features

### Language

| Feature | Details |
|---------|---------|
| `IF` | `EXIST`, `DEFINED`, `ERRORLEVEL`, `CMDEXTVERSION`, string `==`, word operators (`EQU NEQ LSS LEQ GTR GEQ`), `/I`, `NOT` |
| `FOR` files | `FOR %%i IN (set) DO` with glob expansion |
| `FOR /L` | `FOR /L %%i IN (start,step,end) DO` range loops |
| `FOR /F` | strings, files, command output (`usebackq`), `tokens=`, `delims=`, `eol=`, `skip=` |
| `FOR /D` | `FOR /D %%i IN (pattern) DO` — directory-only glob |
| `FOR /R` | `FOR /R [root] %%i IN (pattern) DO` — recursive directory tree walk |
| `GOTO` | Static and dynamic labels (`goto %VAR%`), `goto :eof` |
| `CALL` | Subroutines (`:label` with args), external commands |
| Blocks | Parenthesised compound statements `( ... )` |
| Binary ops | `&&`, `\|\|`, `&`, `\|` (pipe) |
| Redirects | `>`, `>>`, `<`, `>&N`, `<&N`, `2>` |
| Labels | `:label` definitions |
| Comments | `REM`, `::` |

### Internal Commands

Commands marked † are stubs: the common cases work but edge cases (e.g. setting the system clock, real ASSOC registry writes) are not supported cross-platform.

| Command | Notes |
|---------|-------|
| `ECHO` | Print text; `ECHO ON`/`OFF`/`.` (blank line) |
| `SET` | Variable assignment; `SET /A` (full arithmetic); `SET /P` (user input) |
| `SETLOCAL` / `ENDLOCAL` | Environment snapshot stack |
| `GOTO` | Jump to label (PC-based) |
| `CALL` | `:label` subroutine or external command dispatch |
| `IF` / `FOR` | See Language table |
| `SHIFT` | Rotate positional parameters |
| `CD` / `CHDIR` | Change or print working directory |
| `PUSHD` / `POPD` | Directory stack push/pop |
| `MKDIR` / `MD` | Create directory tree (`os.MkdirAll`) |
| `RMDIR` / `RD` | Remove directory; `/S` recursive, `/Q` quiet |
| `DEL` / `ERASE` | Delete files with glob expansion; `/S` recursive, `/Q /F` flags |
| `COPY` | Copy files; glob source expansion; append (`file1+file2 dest`, `file1 + file2`); `/Y /-Y /B /A /V` flags |
| `MOVE` | Move or rename files; `/Y /-Y` flags |
| `REN` / `RENAME` | Rename files; glob source supported |
| `MKLINK` | Create symlinks (`/D` directory, `/H` hardlink, `/J` junction) |
| `DIR` | List directory contents in Windows-style format |
| `TYPE` | Print file contents |
| `MORE` | Output file or stdin (no interactive paging) † |
| `START` | Launch program; `/WAIT` blocks until exit |
| `CLS` | Clear screen (ANSI escape) |
| `TITLE` | Set terminal title (ANSI escape) |
| `COLOR` | Set foreground/background colour (ANSI escape) |
| `VER` | Print Windows version string |
| `PAUSE` | Wait for a keypress |
| `EXIT` | `EXIT [/B] [code]` |
| `BREAK` | No-op (Ctrl+C checking toggle is obsolete) |
| `DATE` | Display current date; setting system date unsupported † |
| `TIME` | Display current time; setting system time unsupported † |
| `PATH` | Display or set the `PATH` environment variable |
| `PROMPT` | Set the command prompt string |
| `VERIFY` | Toggle/display write-verify state † |
| `VOL` | Display volume label placeholder † |
| `ASSOC` | In-process file extension associations (not persisted to registry) † |
| `FTYPE` | In-process file type open commands (not persisted to registry) † |

### External Commands — Native Implementations

These are implemented natively in Go and work cross-platform without requiring the host tool to be installed.

| Command | Notes |
|---------|-------|
| `FIND` | Search for a string in files or stdin; `/V` `/C` `/N` `/I` flags |
| `HOSTNAME` | Print the machine hostname |
| `ROBOCOPY` | Full implementation: `/S` `/E` `/MIR` `/PURGE` `/MOV` `/MOVE` `/SL` `/CREATE` `/LEV:n` `/R:n` `/W:n` `/XF` `/XD` `/XO` `/XN` `/XC` `/XL` `/XX` `/IS` `/XJ` `/XJD` `/XJF` `/MAX:n` `/MIN:n` `/MAXAGE:n` `/MINAGE:n` `/FFT` `/A+:` `/A-:` `/LOG:` `/LOG+:` `/TEE` `/NFL` `/NDL` `/NJH` `/NJS` `/FP` `/NS` `/NC` `/TS` `/V` — Windows-only flags (`/A` `/M` archive bits, `/COPY:` ACLs) accepted and stubbed |
| `SORT` | Sort lines from a file or stdin; `/R` reverse |
| `TIMEOUT` | Sleep for `/T <seconds>`; `/NOBREAK` accepted and ignored |
| `TREE` | Recursive directory tree with Unicode box-drawing characters |
| `WHERE` | Locate an executable on `PATH`; `/Q` quiet |
| `WHOAMI` | Print the current OS username |
| `XCOPY` | Full implementation: `/S` `/E` `/H` `/D[:date]` `/U` `/EXCLUDE:` `/B` `/C` `/F` `/L` `/Q` `/V` `/W` `/P` `/Y` `/-Y` `/I` `/R` `/T` `/K` `/A` `/M` — Windows-only flags (ACL `/O` `/X`, archive `/A` `/M`) accepted and stubbed |

### External Commands — Passthrough

These are registered explicitly so the `Registry` documents them as known commands. On execution they are forwarded to the host OS executable (same as any unregistered command, but listed here for discoverability).

`ATTRIB` `CHCP` `CHOICE` `CLIP` `COMP` `CURL` `DISKPART` `FC` `FINDSTR` `FORFILES` `GETMAC` `GPUPDATE` `IPCONFIG` `NET` `NETSTAT` `NSLOOKUP` `PING` `REG` `SC` `SCHTASKS` `SETX` `SHUTDOWN` `SSH` `SYSTEMINFO` `TAKEOWN` `TAR` `TASKKILL` `TASKLIST` `TRACERT`

### SET /A Operators

Full operator precedence chain: logical-or (`\|\|`) → logical-and (`&&`) → bitwise-or (`\|`) → bitwise-xor (`^`) → bitwise-and (`&`) → shift (`<<` `>>`) → add/sub → mul/div/mod → unary.

Supported: arithmetic (`+` `-` `*` `/` `%`), unary (`-` `+` `!` `~`), bitwise (`&` `|` `^` `<<` `>>`), compound assignment (`+=` `-=` `*=` `/=` `%=` `&=` `^=` `\|=` `<<=` `>>=`), comma-separated multi-expressions, hex (`0x`), octal (`0`-prefix), parenthesised sub-expressions, variable references.

### Cross-Platform

Windows drive paths are mapped automatically (`C:\foo` → `/mnt/c/foo`). External commands are run via `os/exec`; glob patterns in arguments are expanded before execution. All file system built-ins use Go's `os` package directly and work identically on Linux, macOS, and Windows.

## Testing

```bash
go test ./...            # unit + integration
go test -v ./tests/...   # verbose integration output
```

25 integration tests cover: basic echo/set, control flow, FOR loops, I/O redirection, path handling, arithmetic, logical operators, nesting, strings, SHIFT, subroutines, FOR /F, labels, dynamic GOTO, complex math, advanced FOR /F, line continuation, mkdir/rmdir, del/copy/move, FOR /D and FOR /R, `%~` tilde modifiers on positional parameters, COPY append (`+`), session commands (BREAK/PATH/VERIFY/ASSOC/FTYPE/DATE/TIME/PROMPT), file commands (REN/MKLINK/MORE/XCOPY), external tools (SORT/FIND/TREE/WHERE/HOSTNAME/WHOAMI/TIMEOUT).

## Gaps

| Area | Status |
|------|--------|
| `DATE` / `TIME` set | Setting system clock is unsupported |
| `ASSOC` / `FTYPE` persistence | In-process only; does not read or write the Windows registry |
| `MORE` paging | Outputs all content without interactive paging |
| `DEL /A` attribute filter | Attribute-based file selection skipped |
| `PROMPT` `$N` / `$M` | Drive letter only available on Windows; remote name always empty |
| `XCOPY` / `ROBOCOPY` archive bit | `/A` `/M` flags accepted but have no effect; archive attributes require Windows syscalls |
| `ROBOCOPY` ACL / EFS | `/COPY:S` `/COPY:O` `/COPY:U` `/SEC` `/EFSRAW` accepted but not applied; ACL APIs are Windows-only |
| `ROBOCOPY` multithreading | `/MT[:n]` accepted but runs single-threaded |
| `ROBOCOPY` job files | `/JOB:` `/SAVE:` not implemented |
