# CLS, TITLE, COLOR

---

## CLS

Clears the terminal screen.

### Syntax

```bat
CLS
```

### Behaviour

Sends ANSI escape codes `ESC[2J` (erase display) followed by `ESC[H` (move cursor to home position) to stdout.

### Caveats

- Uses ANSI escape sequences, **not** the Windows Console API (`FillConsoleOutputCharacter`). On Windows, this requires a terminal with VT processing enabled (Windows Terminal, VS Code, ConEmu). The legacy `conhost.exe` without ANSI support will display the escape codes as literal characters.
- If stdout is redirected to a file, the ANSI codes are written to the file.

---

## TITLE

Sets the terminal window title.

### Syntax

```bat
TITLE string
```

### Behaviour

Sends the OSC escape sequence `ESC]0;string\a` to stdout.

### Caveats

- Uses ANSI/VT sequences. Not all terminals support the OSC title sequence.
- If stdout is redirected to a file, the escape sequence is written to the file.
- Real cmd.exe uses `SetConsoleTitle()` (Win32 API), which always works. go-msbatch's approach is terminal-dependent.

---

## COLOR

Sets the foreground and background colours of the terminal.

### Syntax

```bat
COLOR [attr]
COLOR            :: reset to defaults
```

`attr` is a two-digit hex string: first digit is background, second is foreground.

### Windows colour codes

| Code | Colour |
|------|--------|
| `0` | Black |
| `1` | Dark Blue |
| `2` | Dark Green |
| `3` | Dark Cyan |
| `4` | Dark Red |
| `5` | Dark Magenta |
| `6` | Dark Yellow (Brown) |
| `7` | Light Gray |
| `8` | Dark Gray |
| `9` | Blue |
| `A` | Green |
| `B` | Cyan |
| `C` | Red |
| `D` | Magenta |
| `E` | Yellow |
| `F` | White |

```bat
COLOR 0A     :: black background, green foreground
COLOR F4     :: white background, red foreground
COLOR        :: reset
```

### Caveats

- Uses ANSI SGR escape codes. The mapping from Windows 16-colour codes to ANSI 8/16-colour codes is approximate.
- Background colours higher than `7` (bright backgrounds) are emulated via ANSI codes that many terminals do not support as true background colours; the result is terminal-dependent.
- If `attr` has both digits the same (e.g. `COLOR 00`), real cmd.exe returns an error (`ERRORLEVEL 1`) since that would make text invisible. go-msbatch does not enforce this check.
- Colour changes affect the entire terminal session via escape codes, not just subsequent output.
