# Architecture & Processing Phases

go-msbatch mirrors cmd.exe's documented multi-phase processing model.

## Package Layout

```
cmd/msbatch/           Binary entry point (file mode + interactive REPL)
internal/lex/          Generic cursor-based state-machine lexer framework
pkg/lexer/             Batch-specific tokenizer → 18 token types
pkg/parser/            Recursive-descent AST builder
pkg/processor/         Multi-phase expansion engine + flow-control executor
pkg/executor/          Built-in command registry + implementations
pkg/executor/tools/    Native cross-platform tool implementations
```

## The Six Phases

Each line (after line-continuation joining) is processed through these phases before execution.

### Phase 0 — Read Line

- `\x1A` (Ctrl-Z) is replaced by `\n`.
- A trailing `^` merges the current line with the next (line continuation).
- `^^` at end-of-line is an escaped caret, **not** a continuation.

### Phase 1 — Percent Expansion

Performed at parse time before the lexer sees the text.

| Syntax             | Meaning                                                 |
| ------------------ | ------------------------------------------------------- |
| `%%`               | Literal `%`                                             |
| `%0`–`%9`          | Positional parameters                                   |
| `%*`               | All positional parameters joined                        |
| `%VAR%`            | Environment variable                                    |
| `%VAR:~start,len%` | Substring of variable                                   |
| `%VAR:old=new%`    | String substitution (case-insensitive, all occurrences) |
| `%~[mods]n`        | Tilde modifier on positional parameter                  |
| `%~$PATHVAR:n`     | Search positional parameter in a path variable          |

See [variables.md](language/variables.md) for full modifier table.

### Phase 2 — Lex & Parse

The expanded text is tokenised by `pkg/lexer` (BatchLexer) and fed to the recursive-descent parser in `pkg/parser`. The result is a slice of AST nodes:

- `SimpleCommand` — a command name plus arguments and redirects
- `IfNode` — conditional
- `ForNode` — loop
- `PipeNode` / `BinaryNode` — operator chains (`|`, `&&`, `||`, `&`)
- `Block` — parenthesised compound statement
- `LabelNode` — `:label` definition

### Phase 3 — Echo Suppression

- A leading `@` suppresses echoing of that line regardless of `ECHO` state.
- `ECHO OFF` suppresses subsequent lines; `ECHO ON` re-enables.
- The `Suppressed` flag is set on `SimpleCommand` nodes by the parser; the executor honours it.

### Phase 4 — FOR Variable Expansion

Inside a `FOR` body, `%%V` (script mode) or `%V` (interactive) is expanded to the loop variable's current value. Tilde modifiers (`%~nxV`, `%~atzV`, …) are also resolved here.

### Phase 5 — Delayed Expansion

Active only when `SETLOCAL ENABLEDELAYEDEXPANSION` has been issued.

- `!VAR!` expands to the variable's value at **execution time** (not parse time).
- `^!` produces a literal `!`.

This allows loop bodies to observe variable changes made by earlier iterations.

## Execution Model

The `Processor` struct holds:

- `Env` — a shared `Environment` (variable map + SETLOCAL snapshot stack)
- `Args` — the `%0`–`%9` / `%*` positional parameter list
- `PC` — program counter (index into the node slice) for `GOTO`
- `Executor` — the command registry (`pkg/executor.Registry`)
- `Stdout`, `Stderr`, `Stdin` — I/O streams (default to `os.Stdout` etc.)
- `Echo` — current echo state
- `Exited` — set when `EXIT` (without `/B`) is executed
- `DirStack` — `PUSHD`/`POPD` directory stack
- `ForVars` — active FOR loop variable bindings

`Processor.Execute(nodes)` iterates the node slice using `PC`. Flow-control nodes (`GOTO`, `CALL`, `EXIT`) manipulate `PC`; pipe execution runs each side on a copied `Processor` value.

### CALL and subroutine semantics

`CALL :label` runs on the **same** `Processor` — the positional parameters (`%1`–`%9`, `%*`) are rebound to the call's arguments and restored when the subroutine returns, so `SET` changes inside the subroutine are visible to the caller — matching cmd.exe's single-session behaviour.

`EXIT /B` stops the subroutine via an `EXIT_LOCAL` sentinel; execution resumes after the `CALL` instruction.

Plain `EXIT` sets `Exited`, terminating the entire session.

### Batch file invocation

When a bare command name resolves to a `.bat` or `.cmd` file (searched in CWD then `PATH`), it is executed **in-process**, sharing the parent environment. A direct invocation transfers control (the caller does not resume); `CALL` returns when the child ends — matching cmd.exe.

### Pipes

Each side of a `|` runs concurrently in a goroutine. An `os.Pipe()` connects the left side's stdout to the right side's stdin. Both sides share a copy of the environment snapshot taken at pipe-setup time; environment changes inside a piped segment do **not** propagate back to the parent.

## Caveats vs Real CMD

- **No interactive command history** — the REPL has no arrow-key history.
- **No `DOSKEY` support** — macro definitions are not implemented.
- **No `CMDEXTVERSION` conditional** always evaluates as version `2`.
- **`ENABLEEXTENSIONS`** is accepted by `SETLOCAL` but has no effect; extensions are always active.
- **`DISABLEDELAYEDEXPANSION`** turns delayed expansion off within the scope, as in cmd.exe.
- **FOR variable names** are single characters (any character except `%`); multi-character names are not supported, matching cmd.exe.
- **Invalid `%~` modifiers** are left literal instead of cmd.exe's two-line "path operator ... is invalid" error plus script abort.
- **Unquoted `<<` in a batch line** (e.g. `set /a s=1<<4`) is evaluated instead of cmd.exe's `1<< was unexpected at this time.` error and script abort; quote the expression (`set /a "s=1<<4"`) as with cmd.exe.
- **`FIND`, `SORT`, `WHERE`, `TIMEOUT`, `XCOPY`, `ROBOCOPY`, `TREE`, `MORE` are internal implementations** (external `.exe` programs on real Windows); edge-case output and flags may differ from the real executables.
- **`GAWK`/`AWK` is an internal Go implementation** (goawk); programs relying on GNU awk extensions may behave differently, and files written from awk scripts get LF endings rather than CRLF.
- **`PKZIP`, `PKUNZIP`, `PKZIPC` are mapped to 7-Zip** when available, since PKZIP does not exist on Unix hosts.
- **`%DATE%` and `%TIME%` are fixed to the en-US layout** (`Mon MM/DD/YYYY`, `H:MM:SS.cc`) and do not follow the host locale.
