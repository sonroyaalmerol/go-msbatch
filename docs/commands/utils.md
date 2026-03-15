# WHERE, HOSTNAME, WHOAMI, TIMEOUT

---

## WHERE

Locates an executable on the search path.

### Syntax

```bat
WHERE [/Q] name [name ...]
```

### Flags

| Flag | Meaning |
|------|---------|
| `/Q` | Quiet — suppress output, only set ERRORLEVEL |

### Behaviour

Uses `exec.LookPath()` to find each `name` in `PATH`. Prints the full path on success.

```bat
WHERE python
WHERE /Q git && ECHO git found
```

### Caveats

- Searches `PATH` only. Does not support `WHERE /R dir pattern` (recursive search in a specific directory) — this form is silently treated as a regular lookup.
- On Unix, `WHERE` finds executables by searching `PATH` directories for an exact filename match. It does not auto-append `.exe`, `.bat`, `.cmd` extensions as it would on Windows.
- `ERRORLEVEL` is `0` if found, `1` if not found.
- `WHERE` is implemented natively in Go and does not require the host `where` or `which` utility.

---

## HOSTNAME

Displays the machine hostname.

### Syntax

```bat
HOSTNAME
```

### Behaviour

Calls `os.Hostname()` and prints the result. No flags.

### Caveats

- Returns the OS-level hostname, which on Linux/macOS will be a Unix hostname (not a Windows FQDN or NetBIOS name).
- Implemented natively in Go.

---

## WHOAMI

Displays the current user name.

### Syntax

```bat
WHOAMI
```

### Behaviour

Calls `os/user.Current()` and prints `user.Username`.

### Caveats

- On Unix, returns the Unix username (e.g. `alice`). Real cmd.exe returns `DOMAIN\username` format. Scripts that parse `WHOAMI` output expecting a backslash separator will not work on Unix.
- No flags (`/USER`, `/GROUPS`, `/PRIV`, `/LOGONID`, `/ALL`) are implemented.
- Implemented natively in Go.

---

## TIMEOUT

Pauses execution for a specified number of seconds.

### Syntax

```bat
TIMEOUT /T seconds [/NOBREAK]
```

### Flags

| Flag | Status | Meaning |
|------|--------|---------|
| `/T seconds` | Implemented | Sleep for `seconds` seconds |
| `/NOBREAK` | Accepted, no effect | Prevent Ctrl-C from interrupting |
| `/T -1` | **Not implemented** | Wait indefinitely for a keypress |

### Behaviour

```bat
TIMEOUT /T 5
TIMEOUT /T 10 /NOBREAK
```

Sleeps for the given number of seconds using `time.Sleep`.

### Caveats

- **`/T -1` (wait indefinitely for keypress) is not implemented.** go-msbatch will treat `-1` seconds as immediately returning (zero sleep duration is clamped).
- **`/NOBREAK` is accepted but ignored.** The timeout can always be interrupted by signals (Ctrl-C) since go-msbatch does not intercept them during sleep.
- Real cmd.exe counts down and displays `Waiting for X seconds, press a key to continue ...`. go-msbatch sleeps silently without any countdown display.
- Implemented natively in Go.
