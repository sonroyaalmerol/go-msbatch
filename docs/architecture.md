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

| Syntax | Meaning |
|--------|---------|
| `%%` | Literal `%` |
| `%0`–`%9` | Positional parameters |
| `%*` | All positional parameters joined |
| `%VAR%` | Environment variable |
| `%VAR:~start,len%` | Substring of variable |
| `%VAR:old=new%` | String substitution (case-insensitive, all occurrences) |
| `%~[mods]n` | Tilde modifier on positional parameter |
| `%~$PATHVAR:n` | Search positional parameter in a path variable |

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

`Processor.Execute(nodes)` iterates the node slice using `PC`. Flow-control nodes (`GOTO`, `CALL`, `EXIT`) manipulate `PC` or spawn child processors.

### CALL and subroutine semantics

`CALL :label` creates a **child** `Processor` that shares the **same** `Env` pointer and the same I/O streams, so `SET` changes inside the subroutine are visible to the caller — matching cmd.exe's single-session behaviour.

`EXIT /B` signals the child to stop via an `EXIT_LOCAL` sentinel error; the parent resumes after the `CALL` instruction.

Plain `EXIT` sets `child.Exited = true`, which the parent propagates to itself, terminating the entire session.

### Batch file invocation

When a bare command name resolves to a `.bat` or `.cmd` file (searched in CWD then `PATH`), it is executed **in-process** via the same mechanism as `CALL`, sharing the parent environment. This matches cmd.exe's single-session semantics for batch-calling batch.

### Pipes

Each side of a `|` runs concurrently in a goroutine. An `os.Pipe()` connects the left side's stdout to the right side's stdin. Both sides share a copy of the environment snapshot taken at pipe-setup time; environment changes inside a piped segment do **not** propagate back to the parent.

## Caveats vs Real CMD

- **No interactive command history** — the REPL has no arrow-key history.
- **No `DOSKEY` support** — macro definitions are not implemented.
- **No `CMDEXTVERSION` conditional** always evaluates as version `2`.
- **`ENABLEEXTENSIONS`** is accepted by `SETLOCAL` but has no effect; extensions are always active.
- **No `DISABLEDELAYEDEXPANSION`** — once enabled in a SETLOCAL scope, it cannot be turned off within that scope (ENDLOCAL restores the previous state correctly).
- **FOR variable names** are single ASCII letters only (A–Z, a–z). Multi-character variable names are not supported.
