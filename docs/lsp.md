# msbatch-lsp — Language Server Protocol

`msbatch-lsp` is a Language Server Protocol (LSP) server for CMD/batch scripts (`.bat`, `.cmd`). It runs as a separate binary and communicates over stdin/stdout using the standard JSON-RPC 2.0 LSP protocol, so it works with any LSP-capable editor.

## Installation

Download a pre-built binary from the [releases page](https://github.com/sonroyaalmerol/go-msbatch/releases) (Linux and macOS only — see [Windows note](#windows) below), or install from source:

```bash
go install github.com/sonroyaalmerol/go-msbatch/cmd/msbatch-lsp@latest
```

## Editor setup

### Neovim (nvim-lspconfig)

```lua
local lspconfig = require('lspconfig')
local configs = require('lspconfig.configs')

if not configs.msbatch then
  configs.msbatch = {
    default_config = {
      cmd = { 'msbatch-lsp' },
      filetypes = { 'dosbatch' },
      root_dir = lspconfig.util.find_git_ancestor,
      single_file_support = true,
    },
  }
end

lspconfig.msbatch.setup {}
```

### VS Code

Install a generic LSP client extension (e.g. [generic-lsp](https://marketplace.visualstudio.com/items?itemName=llvm-vs-code-extensions.vscode-clangd) pattern), then add to `settings.json`:

```json
{
  "languageServerExample.serverPath": "/usr/local/bin/msbatch-lsp",
  "languageServerExample.filetypes": ["bat"]
}
```

Or use the [bat-lsp](https://marketplace.visualstudio.com/search?term=batch&target=VSCode) extension category and point it at `msbatch-lsp`.

### Helix

Add to `~/.config/helix/languages.toml`:

```toml
[[language]]
name = "batch"
scope = "source.batch"
file-types = ["bat", "cmd"]
language-servers = ["msbatch-lsp"]

[language-server.msbatch-lsp]
command = "msbatch-lsp"
```

### Emacs (eglot)

```emacs-lisp
(add-to-list 'eglot-server-programs
             '((bat-mode) . ("msbatch-lsp")))
(add-hook 'bat-mode-hook #'eglot-ensure)
```

## Features

### Diagnostics

Errors and hints are published on every file open and change.

| Severity | Condition |
|----------|-----------|
| Warning  | `GOTO` or `CALL :label` target is not defined anywhere in the file |
| Warning  | A `%%V` FOR loop variable is used outside its loop's scope |
| Warning  | `!VAR!` delayed-expansion syntax is used but `SETLOCAL ENABLEDELAYEDEXPANSION` is not present in the file |
| Hint     | A `:label` is defined but never referenced by any `GOTO` or `CALL` |
| Hint     | A variable is defined but never used (`%VAR%` or `!VAR!` never appears) |
| Hint     | A `%VAR%` or `!VAR!` is used but not defined anywhere in the file (suppressed for built-in CMD variables and for files that make external `CALL <file>.bat` calls, since called scripts can set any variable) |

Modifier expressions (`%VAR:~start,len%`, `%VAR:old=new%`) are fully understood — the base variable name is extracted and checked against definitions, so no false-positive "not defined" hint is raised.

Built-in CMD variables (`ERRORLEVEL`, `PATH`, `USERNAME`, `WINDIR`, etc.) are derived at startup from the host OS environment via `processor.BuiltinVarNames()` so the set always matches what the processor itself recognises.

### Hover

Hover over any built-in command name to see its syntax and flag reference.

### Completion

| Trigger | Completions offered |
|---------|---------------------|
| Start of line | All recognised command names |
| After `GOTO ` or `CALL :` | Label names defined in the file |
| After `%` (open) | `SET` variable names from the current file and files explicitly `CALL`-ed from it (closes with `%`) |
| After `%%` | In-scope `FOR` loop variables only (closes with nothing — the letter is inserted) |
| After `!` (open) | Delayed-expansion variable names — only offered when `SETLOCAL ENABLEDELAYEDEXPANSION` is present (closes with `!`) |

All variable completions use a `textEdit` with an explicit replacement range so the full token (including opening/closing sigils) is inserted correctly regardless of editor settings.

### Variable scoping

- **`SET` variables** are file-wide. Completions, Go-to-Definition, and Find References search the current file first, then fall back to files explicitly `CALL`-ed from the current file. Variables in unrelated files that are not reachable via a `CALL` chain are never surfaced.
- **`FOR` loop variables** (`%%V`) are scoped strictly to the loop body (single-line or block `do (...)`). They are never resolved across file boundaries.
- **`FOR /F` implicit token variables** — when `tokens=N,M` or `tokens=N-M` is specified, the additional captured tokens are automatically assigned to successive loop letters (e.g. `tokens=2,3 %%a` also defines `%%b`). All such variables are treated as in-scope within the loop body.
- **Delayed-expansion variables** (`!VAR!`) reference the same underlying `SET` variable store and respect the same file-wide scope. Cross-file lookup follows the same `CALL` chain as `%VAR%`.
- **Modifier expressions** — `%VAR:~start,len%` (substring), `%VAR:~-n%` (from end), and `%VAR:old=new%` (replacement) are all recognised. The LSP extracts the base variable name for definition lookup, reference search, and diagnostics, and highlights the full modifier expression as a single variable token.

### Go to Definition

- On a `GOTO`/`CALL :label` target → jumps to the `:label` definition line.
- On a `CALL other.bat` → jumps to `other.bat` if it exists in the workspace.
- On a `%VARIABLE%`, `%VAR:modifier%`, or `!VAR!` → jumps to the `SET VAR=...` line (searches the current file first, then falls back to files explicitly `CALL`-ed from the current file).
- `%%V` FOR loop variables resolve only within their own loop scope and never cross file boundaries.
- Forward references are supported (label defined after use).

### Find References

- On a `:label` definition or any `GOTO`/`CALL` site → lists all `GOTO` and `CALL` sites for that label locally.
- On a `%VARIABLE%`, `%VAR:modifier%`, or `!VAR!` → lists all usage sites (both `%VAR%` and `!VAR!` forms) across the current file and files explicitly `CALL`-ed from it.
- `%%V` FOR loop variable references are restricted to the loop's scope and never cross file boundaries.
- Supports the `includeDeclaration` flag.

### Rename

Atomically renames a symbol and all its sites:

- **Label** — updates the `:label` definition and every `GOTO`/`CALL` reference in the current file.
- **SET variable** (`%VAR%` or `!VAR!`) — updates the `SET VAR=...` line and every `%VAR%`/`!VAR!` usage across the current file and files explicitly `CALL`-ed from it.
- **FOR loop variable** (`%%V`) — renames the loop variable letter in the `FOR` statement and all in-scope references within that loop body. Changes are local to the current file.

### Document Symbols

Provides an outline of the file for the editor's symbol panel:

- `:labels` shown as functions.
- `SET` variables shown as variables with their current value.

### Workspace Symbols

Provides project-wide symbol search (e.g. via `Ctrl+T` or `Cmd+T`):
- Search for any `:label` or `SET` variable across all `.bat` and `.cmd` files in the loaded workspace.

### Workspace File Watching

The language server actively watches the workspace for file changes (`workspace/didChangeWatchedFiles`). Creating, deleting, or modifying `.bat` or `.cmd` files outside of the editor will automatically update the language server's index and diagnostics.

### Code Lens

Displays a `N references` annotation above each `:label` definition inline in the editor.

### Semantic Tokens

Full syntax highlighting served by the LSP (no separate TextMate grammar required):

| Token | Highlighted elements |
|-------|----------------------|
| Keyword | `ECHO`, `SET`, `GOTO`, `CALL`, `IF`, `FOR`, and all other built-in commands |
| Function | `:label` definitions (with declaration modifier) and `GOTO`/`CALL` targets |
| Variable | `%VARIABLE%` names, `%VAR:modifier%` expressions (highlighted as a whole), `%%V` FOR loop variables, and `!VAR!` delayed-expansion variables |
| Comment | `::` comment lines and `REM` lines |
| String | Quoted strings `"..."` |

### Folding Ranges

Each label section (from `:label` to just before the next `:label` or end of file) is exposed as a foldable region.

### Code Actions

| Action | Trigger |
|--------|---------|
| Create missing label | `GOTO` or `CALL :` target that has no matching `:label` definition |

The quick-fix inserts the missing `:labelname` at the end of the file.

## Windows

Pre-built Windows binaries are not available due to a build failure in a transitive dependency (`tliron/kutil` v0.3.11 references `termenv` API that changed in v0.15). Windows users can:

- **WSL** — install the Linux binary inside WSL and configure your editor to use it.
- **Build from source** — once the upstream dependency is fixed, `go install` will work natively on Windows.
