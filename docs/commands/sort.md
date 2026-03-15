# SORT

Sorts lines from a file or stdin.

## Syntax

```bat
SORT [/R] [/+n] [file]
command | SORT [/R]
```

## Flags

| Flag | Status | Meaning |
|------|--------|---------|
| `/R` | Implemented | Reverse sort (descending) |
| `/+n` | **Not implemented** | Start sort key at column `n` |

## Behaviour

Reads all lines into memory, sorts them alphabetically (case-insensitive by default), and writes them to stdout.

```bat
SORT names.txt
DIR /B | SORT
DIR /B | SORT /R
```

## Caveats

- **`/+n` column sort is not implemented.** In real cmd.exe, `SORT /+5` compares lines starting from the 5th character. go-msbatch always sorts from the beginning of each line.
- **All other flags are silently ignored.** Real cmd.exe also accepts `/L locale` and `/M kilobytes`; these are not supported.
- Sort is **case-insensitive** by default (matches cmd.exe). There is no flag to enable case-sensitive sorting.
- Sort is **not stable** — equal lines may appear in any order relative to each other.
- The entire input is buffered in memory before sorting. Large files may cause high memory usage.
- `SORT` is implemented natively in Go and does not require the host `sort` utility.
