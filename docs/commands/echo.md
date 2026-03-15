# ECHO

Displays text or controls command echoing.

## Syntax

```bat
ECHO [ON | OFF]
ECHO [message]
ECHO.
```

## Behaviour

| Form | Output |
|------|--------|
| `ECHO` (no args) | Displays current echo state: `ECHO is on` or `ECHO is off` |
| `ECHO ON` | Enables echoing of subsequent commands |
| `ECHO OFF` | Disables echoing of subsequent commands |
| `ECHO message` | Prints `message` followed by a newline |
| `ECHO.` | Prints a blank line |

The `@` prefix suppresses the echo of a single line regardless of `ECHO` state:

```bat
@ECHO OFF     :: hides this line even though echo is still on when it runs
ECHO hello    :: printed but not echoed
```

## Blank line variants

`ECHO.` (dot), `ECHO,` (comma), `ECHO;` (semicolon), and `ECHO(` (open-paren) all produce a blank line in real cmd.exe. go-msbatch only supports `ECHO.`.

## Caveats

- `ECHO` with a leading space (`ECHO  hello`) preserves the space in output — the first space after `ECHO` is the separator, but subsequent spaces are part of the message. This is consistent with cmd.exe.
- Trailing spaces on an `ECHO` line are included in the output.
- `ECHO.` must have no space between `ECHO` and `.`. `ECHO .` prints ` .` (space-dot), not a blank line.
- `ECHO,`, `ECHO;`, `ECHO(` etc. for blank lines are **not implemented** — only `ECHO.` is.
