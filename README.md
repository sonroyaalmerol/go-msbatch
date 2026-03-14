# go-msbatch

A cross-platform, systematic implementation of the Windows CMD/Batch interpreter in Go. The project faithfully mirrors `cmd.exe`'s multi-phase processing model, producing an AST from a recursive-descent parser and executing it with a PC-based engine.

## Usage

```bash
# Run a batch file
go run . script.bat [arg1 arg2 ...]

# Interactive REPL
go run .
```

## Architecture

```
internal/lex/          Generic cursor-based state-machine lexer framework
pkg/lexer/             Batch-specific tokenizer (BatchLexer → 19 token types)
pkg/parser/            Recursive-descent AST builder
pkg/processor/         Multi-phase expansion engine + executor
main.go                CLI entry point (file mode or interactive REPL)
tests/                 22 integration test pairs (*.bat + *.out)
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

### Built-in Commands

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
| `DIR` | List directory contents in Windows-style format |
| `TYPE` | Print file contents |
| `CLS` | Clear screen (ANSI escape) |
| `TITLE` | Set terminal title (ANSI escape) |
| `COLOR` | Set foreground/background colour (ANSI escape) |
| `VER` | Print Windows version string |
| `PAUSE` | Wait for a keypress |
| `EXIT` | `EXIT [/B] [code]` |

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

22 integration tests cover: basic echo/set, control flow, FOR loops, I/O redirection, path handling, arithmetic, logical operators, nesting, strings, SHIFT, subroutines, FOR /F, labels, dynamic GOTO, complex math, advanced FOR /F, line continuation, mkdir/rmdir, del/copy/move, FOR /D and FOR /R, `%~` tilde modifiers on positional parameters, COPY append (`+`).

## Gaps & Planned Work

| Area | Status |
|------|--------|
| `PROMPT` variable codes | `$N` drive letter only available on Windows; `$M` (remote name) always empty |
| `DEL /A` attribute filter | Attribute-based file selection skipped |
| `ASSOC` / `FTYPE` | File association queries not implemented |
| `FIND` / `FINDSTR` | Fall through to host; no native implementation |
