# TYPE, DIR, MORE

---

## TYPE

Displays the contents of one or more text files.

### Syntax

```bat
TYPE filename [filename ...]
```

### Behaviour

Reads each file and writes its contents to stdout. When multiple files are given, they are printed sequentially.

```bat
TYPE readme.txt
TYPE file1.txt file2.txt
TYPE *.txt       :: glob expansion supported
```

### Caveats

- **Text mode only** — no binary detection. All bytes are passed through as-is. Unlike cmd.exe, there is no CR/LF conversion on Unix.
- Printing a binary file will output raw bytes; this may corrupt the terminal.
- `TYPE` on a directory name produces an error.

---

## DIR

Lists the contents of a directory in a simplified Windows-style format.

### Syntax

```bat
DIR [path]
```

### Output format

```
 Directory of /path/to/dir

2024/01/15 12:30          1234 filename.txt
2024/01/15 12:31  <DIR>        subdir
```

### Caveats

- **All `/` flags are silently ignored.** Real cmd.exe supports `/S` (recursive), `/B` (bare names), `/W` (wide), `/L` (lowercase), `/O[n]` (sort), `/P` (pause), `/A[attr]` (filter by attribute), `/X` (show 8.3 names), `/Q` (show owner), and others. None of these are implemented.
- The output format is simplified and does not exactly match cmd.exe's output (different column widths, missing summary lines for free disk space, etc.).
- No attribute-based filtering (`/A:H` for hidden, `/A:D` for directories only, etc.).
- On Linux, hidden files (dotfiles) are shown; real cmd.exe would hide them without `/A:H`.

---

## MORE

Outputs file contents without interactive paging.

### Syntax

```bat
MORE [file]
MORE < file
command | MORE
```

### Behaviour

Reads `file` (or stdin) and writes it to stdout. All content is written immediately without any pause-per-page prompting.

### Caveats

- **Non-interactive** — real cmd.exe's `MORE` pauses after each screenful and waits for a keypress. go-msbatch's `MORE` is effectively equivalent to `TYPE`.
- No `/E`, `/C`, `/P`, `/S`, `/Tn` flags are implemented.
- `MORE +n` (skip first n lines) is not supported.
