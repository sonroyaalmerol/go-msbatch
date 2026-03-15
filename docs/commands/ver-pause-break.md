# VER, PAUSE, BREAK

---

## VER

Displays the Windows version string.

### Syntax

```bat
VER
```

### Behaviour

Always prints:

```
Microsoft Windows [Version 10.0.19045.5442]
```

### Caveats

- **Hard-coded output.** The string does not reflect the actual OS or version. On Linux or macOS the same Windows version string is printed.
- Scripts that parse the `VER` output to detect the OS version will always see the same fixed string.
- Real cmd.exe reads the version from the Windows kernel via `GetVersionEx()`. There is no equivalent cross-platform API.

---

## PAUSE

Waits for the user to press any key.

### Syntax

```bat
PAUSE
```

### Behaviour

Prints `Press any key to continue . . . ` (without a newline) then reads one byte from stdin. Execution continues after the keypress.

### Caveats

- Reads exactly one byte using `io.ReadFull`. In non-interactive scripts where stdin is piped or redirected, `PAUSE` will read the first byte of stdin and continue immediately.
- Real cmd.exe suppresses the echo of the keypress. go-msbatch does not — the byte is read raw but the terminal may still echo it depending on the terminal mode.
- If stdin is at EOF, `PAUSE` continues without waiting.

---

## BREAK

No-op.

### Syntax

```bat
BREAK [ON | OFF]
```

### Behaviour

Does nothing. `ERRORLEVEL` is set to `0`.

### Background

In early versions of DOS, `BREAK ON` extended Ctrl-C checking to more operations. In modern cmd.exe this is accepted but has no meaningful effect. go-msbatch preserves this no-op behaviour.
