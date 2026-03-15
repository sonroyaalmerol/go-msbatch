# DATE, TIME

---

## DATE

Displays the current date.

### Syntax

```bat
DATE [/T]
```

### Behaviour

Prints the current date in the format:

```
Mon 01/02/2006
```

The day-of-week abbreviation and `MM/DD/YYYY` layout match a common cmd.exe locale.

### Caveats

- **Display only** — setting the system date is not supported. Real cmd.exe without `/T` prompts for a new date. go-msbatch ignores any input and only displays.
- `/T` (display-only, no prompt) is accepted but the behaviour is the same regardless.
- The date format is fixed and does not respect the system locale or the `DATE` environment variable.
- `%DATE%` as an environment variable is **not automatically populated**. Use `DATE` as a command to display, or use `FOR /F` to capture its output if you need the date in a variable.

---

## TIME

Displays the current time.

### Syntax

```bat
TIME [/T]
```

### Behaviour

Prints the current time in the format:

```
15:04:05.00
```

### Caveats

- **Display only** — setting the system time is not supported. Real cmd.exe without `/T` prompts for a new time. go-msbatch ignores any input and only displays.
- `/T` is accepted but has no effect beyond what the command already does.
- The time format is fixed (`HH:MM:SS.cc` 24-hour) and does not respect locale settings.
- `%TIME%` as an environment variable is **not automatically populated**. Capture `TIME /T` output via `FOR /F` if needed.
