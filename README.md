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
tests/                 17 integration test pairs (*.bat + *.out)
```

## Processing Phases

The interpreter follows `cmd.exe`'s documented 6-phase model exactly:

| Phase | Name | Description |
|-------|------|-------------|
| 0 | Read Line | Ctrl-Z → `\n`; trailing `^` merges the next line |
| 1 | Percent Expansion | `%VAR%`, `%0`–`%9`, `%*`, `%%`, slicing, substitution |
| 2 | Lex & Parse | Tokenise then build AST (commands, blocks, operators) |
| 3 | Echo Suppression | `@` prefix; `ECHO ON`/`OFF` state |
| 4 | FOR Variable Expansion | `%%i` / `%i`, tilde modifiers (`%~nxi`, `%~dp`, …) |
| 5 | Delayed Expansion | `!VAR!`; `^!` → literal `!` |

## Implemented Features

### Language

| Feature | Details |
|---------|---------|
| `IF` | `EXIST`, `DEFINED`, `ERRORLEVEL`, string `==`, word operators (`EQU NEQ LSS LEQ GTR GEQ`), `/I`, `NOT` |
| `FOR` files | `FOR %%i IN (set) DO` with glob expansion |
| `FOR /L` | `FOR /L %%i IN (start,step,end) DO` range loops |
| `FOR /F` | strings, files, command output (`usebackq`), `tokens=`, `delims=`, `eol=`, `skip=` |
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
| `EXIT` | `EXIT [/B] [code]` |

### SET /A Operators

Arithmetic (`+` `-` `*` `/` `%`), unary (`-` `+` `!` `~`), compound assignment (`+=` `-=` `*=` `/=` `%=` `&=` `^=` `\|=` `<<=` `>>=`), comma-separated multi-expressions, hex (`0x`), octal (`0`-prefix), parenthesised sub-expressions, variable references.

### Cross-Platform

Windows drive paths are mapped automatically (`C:\foo` → `/mnt/c/foo`). External commands that don't exist on the host are attempted via `os/exec`; glob patterns in arguments are expanded before execution.

## Testing

```bash
go test ./...            # unit + integration
go test -v ./tests/...   # verbose integration output
```

17 integration tests cover: basic echo/set, control flow, FOR loops, I/O redirection, path handling, arithmetic, logical operators, nesting, strings, SHIFT, subroutines, FOR /F, labels, dynamic GOTO, complex math, advanced FOR /F, line continuation.

## Gaps & Planned Work

| Area | Status |
|------|--------|
| Binary bitwise ops in `SET /A` (`&` `\|` `^` `<<` `>>`) | Compound-assign only; standalone binary form not yet parsed |
| `IF CMDEXTVERSION` | Type defined; executor branch not wired |
| FOR var modifiers `%~f` `%~s` `%~a` `%~t` `%~z` | Only `~n` `~x` `~p` `~d` implemented |
| Built-in `TYPE` | Falls through to host `type`/`cat` |
| Built-in `DIR` | Falls through to host `dir`/`ls` |
| Built-in `COPY` / `MOVE` / `DEL` / `MKDIR` / `RMDIR` | Falls through to host equivalents |
| Built-in `PUSHD` / `POPD` | Falls through to host |
| Built-in `TITLE` / `CLS` / `COLOR` / `VER` / `PAUSE` | Falls through to host |
| `PROMPT` variable codes | Only `$P` and `$G` expanded |
| `%~` in Phase 1 | Modifier syntax partially skipped |
