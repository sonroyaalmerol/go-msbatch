# TREE

Displays a visual directory tree.

## Syntax

```bat
TREE [path]
```

## Behaviour

Recursively walks the directory starting at `path` (default: current directory) and prints a Unicode box-drawing tree:

```
myproject
├───src
│   ├───main
│   └───utils
├───tests
└───docs
```

## Caveats

- **All flags are silently ignored.** Real cmd.exe supports:
  - `/F` — show filenames in each directory (go-msbatch shows directories only)
  - `/A` — use ASCII characters instead of box-drawing characters
  No flag has any effect in go-msbatch.
- Only **directories** are shown in the tree. Files within directories are never listed (i.e. `/F` is effectively always off).
- Uses Unicode box-drawing characters (`├`, `└`, `─`, `│`). If the terminal or output file does not support Unicode, these may appear as garbled characters.
- `TREE` is implemented natively in Go and does not require the host `tree` utility.
