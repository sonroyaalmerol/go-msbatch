# Go MS-Batch Interpreter

A cross-platform, systematic implementation of the Windows CMD/Batch interpreter in Go. This project aims to faithfully mirror CMD.EXE's parsing and execution logic, including its unique multi-phase processing model.

## Processing Phases

The interpreter follows the documented 6-phase processing model of `cmd.exe`:

### Phase 0: Read Line
*   **Normalisation**: Replaces `0x1A` (Ctrl-Z) with a newline.
*   **Line Continuity**: (Planned) Handling of the caret `^` at the end of a line for multi-line commands.

### Phase 1: Percent Expansion
*   **Variables**: `%VAR%` is replaced with its environment value.
*   **Positional**: `%0` through `%9` and `%*` are resolved.
*   **Manipulation**: Supports slicing (`%VAR:~0,5%`) and substitution (`%VAR:old=new%`).
*   **Escaping**: `%%` becomes a literal `%`.

### Phase 2: Lexing & Parsing
*   The expanded string is tokenized into keywords, text, punctuation, and operators.
*   An Abstract Syntax Tree (AST) is built, identifying commands, blocks `()`, and logical operators (`&&`, `||`, `&`, `|`).

### Phase 3: Echo & Command Suppression
*   Handles the `@` prefix to suppress command echoing.
*   Tracks the global `ECHO ON/OFF` state.

### Phase 4: FOR Variable Expansion
*   Resolves loop variables like `%%i` (in batch) or `%i` (in cmd) just before executing the loop body.
*   Supports tilde modifiers like `%~nxi` (filename and extension).

### Phase 5: Delayed Expansion
*   If enabled, resolves `!VAR!` variables just before execution.
*   Allows variables to be updated and read within the same command block or loop.

---

## Feature Roadmap

### Implemented
- [x] **Core Execution**: Recursive AST execution with instruction pointer (`PC`) for jumps.
- [x] **Environment Scoping**: `SETLOCAL` and `ENDLOCAL` with a persistent environment stack.
- [x] **Control Flow**:
    - `GOTO` (including dynamic labels like `goto %VAR%`).
    - `CALL` (both subroutine `:label` and external commands).
    - `IF` (EXIST, DEFINED, ERRORLEVEL, and string comparison with `/I`).
- [x] **Loops**: `FOR` (files), `FOR /L` (range), and basic `FOR /F` (strings/files).
- [x] **Built-ins**: `ECHO`, `SET`, `SET /A` (basic math), `SET /P` (input), `SHIFT`, `EXIT`, `CD`.
- [x] **I/O Redirection**:
    - Pipes (`|`) using parallel goroutines.
    - File redirection (`>`, `>>`, `<`).
- [x] **Cross-Platform**: Automatic Windows-to-Unix path mapping (e.g., `C:\` -> `/mnt/c/`).

### Unimplemented / Planned
- [ ] **Complex Math**: Full expression evaluation for `SET /A` (currently only supports basic `+`).
- [ ] **Advanced FOR /F**: Full `tokens=`, `delims=`, and `usebackq` option parsing.
- [ ] **Command Output Parsing**: `FOR /F` iteration over command results (e.g., `FOR /F %%i IN ('dir')`).
- [ ] **Delayed Expansion Escaping**: More robust handling of `^!` within delayed expansion blocks.
- [ ] **Line Continuity**: Caret `^` at the very end of a physical line.
- [ ] **Search Path**: Resolution of external commands via the `PATH` variable (currently relies on `os/exec` default).

## Testing

The project includes an extensive integration test suite in the `tests/` directory. Each `.bat` file is matched with a `.out` file containing the expected stdout.

To run the tests:
```bash
go test -v ./tests
```
