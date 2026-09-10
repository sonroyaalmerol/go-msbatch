# FIND

Searches for a text string in files or stdin.

## Syntax

```bat
FIND [/V] [/C] [/N] [/I] "string" [file ...]
```

## Flags

| Flag | Meaning |
|------|---------|
| `/I` | Case-insensitive search |
| `/V` | Print lines that do **not** contain the string |
| `/C` | Print only a count of matching lines |
| `/N` | Prefix each matching line with its line number |

## Behaviour

Without file arguments, reads from stdin.

```bat
FIND "error" app.log
FIND /I /N "warning" *.log
TYPE output.txt | FIND /V "debug"
FIND /C "TODO" src\*.go
```

Every file argument gets a blank line and an uppercase `---------- FILENAME` header, even for a single file, matching cmd.exe:

```
---------- FILE1.TXT
matching line here

---------- FILE2.TXT
another match
```

## Caveats

- **String must be double-quoted.** Unlike grep, the search string is always given as a quoted argument. `FIND error file.txt` (without quotes) will attempt to open a file named `error`.
- **No regex support.** The search string is treated as a literal substring, not a regular expression. Use `FINDSTR` (passthrough to host) for regex.
- `/C` renders the count appended to the header (`---------- FILENAME: 2`) and prints `0` counts for files with zero matches, matching cmd.exe behaviour.
- `FIND` is implemented natively in Go and works cross-platform without needing the host `find` utility.
- File arguments support glob expansion. `FIND "x" *.txt` expands to all matching `.txt` files before the search runs.
