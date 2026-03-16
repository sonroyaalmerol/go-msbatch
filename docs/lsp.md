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
| Hint     | A `:label` is defined but never referenced by any `GOTO` or `CALL` |
| Hint     | A `SET VAR=...` variable is defined but `%VAR%` is never used |

### Hover

Hover over any built-in command name to see its syntax and flag reference.

### Completion

| Trigger | Completions offered |
|---------|---------------------|
| Start of line | All recognised command names |
| After `GOTO ` or `CALL :` | Label names defined in the file |
| After `%` (open) | Variable names defined by `SET` in the file |

### Go to Definition

- On a `GOTO`/`CALL` target → jumps to the `:label` definition line.
- On a `%VARIABLE%` name → jumps to the `SET VAR=...` line.
- Forward references are supported (label defined after use).

### Find References

- On a `:label` definition or any `GOTO`/`CALL` site → lists all `GOTO` and `CALL` sites for that label.
- On a `%VARIABLE%` → lists all `%VAR%` usage sites in the file.
- Supports the `includeDeclaration` flag.

### Rename

Atomically renames a symbol and all its sites:

- **Label** — updates the `:label` definition and every `GOTO`/`CALL` reference.
- **Variable** — updates the `SET VAR=...` line and every `%VAR%` usage.

### Document Symbols

Provides an outline of the file for the editor's symbol panel:

- `:labels` shown as functions.
- `SET` variables shown as variables with their current value.

### Code Lens

Displays a `N references` annotation above each `:label` definition inline in the editor.

### Semantic Tokens

Full syntax highlighting served by the LSP (no separate TextMate grammar required):

| Token | Highlighted elements |
|-------|----------------------|
| Keyword | `ECHO`, `SET`, `GOTO`, `CALL`, `IF`, `FOR`, and all other built-in commands |
| Function | `:label` definitions (with declaration modifier) and `GOTO`/`CALL` targets |
| Variable | `%VARIABLE%` names |
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
