# go-msbatch Documentation

A cross-platform Windows CMD/Batch interpreter written in Go.

## Overview

- [Architecture & Processing Phases](architecture.md)
- [Cross-Platform Behaviour](cross-platform.md)
- [Trace Debugging](trace-debugging.md)

## Language Features

- [Variables & Expansion](language/variables.md) — `%VAR%`, `%~modifiers`, slicing, substitution, `!delayed!`
- [Arithmetic](language/arithmetic.md) — `SET /A` operators and precedence
- [Control Flow](language/control-flow.md) — `IF`, `GOTO`, `CALL`, `EXIT`, `SHIFT`
- [FOR Loops](language/for-loops.md) — files, `/L`, `/D`, `/R`, `/F`
- [Redirection & Pipes](language/redirects.md) — `>`, `>>`, `<`, `2>`, `|`

## Built-in Commands

| Command | Doc |
|---------|-----|
| `ECHO` | [echo.md](commands/echo.md) |
| `SET` | [set.md](commands/set.md) |
| `CD` / `CHDIR` | [cd.md](commands/cd.md) |
| `TYPE`, `DIR`, `MORE` | [type-dir-more.md](commands/type-dir-more.md) |
| `CLS`, `TITLE`, `COLOR` | [cls-title-color.md](commands/cls-title-color.md) |
| `VER`, `PAUSE`, `BREAK` | [ver-pause-break.md](commands/ver-pause-break.md) |
| `DATE`, `TIME` | [date-time.md](commands/date-time.md) |
| `PATH`, `PROMPT`, `VERIFY`, `VOL` | [path-prompt-verify-vol.md](commands/path-prompt-verify-vol.md) |
| `PUSHD`, `POPD` | [pushd-popd.md](commands/pushd-popd.md) |
| `MKDIR` / `MD`, `RMDIR` / `RD` | [mkdir-rmdir.md](commands/mkdir-rmdir.md) |
| `DEL` / `ERASE` | [del.md](commands/del.md) |
| `COPY` | [copy.md](commands/copy.md) |
| `MOVE`, `REN` / `RENAME` | [move-ren.md](commands/move-ren.md) |
| `MKLINK` | [mklink.md](commands/mklink.md) |
| `START` | [start.md](commands/start.md) |
| `ASSOC`, `FTYPE` | [assoc-ftype.md](commands/assoc-ftype.md) |

## Native External Commands

| Command | Doc |
|---------|-----|
| `FIND` | [find.md](commands/find.md) |
| `SORT` | [sort.md](commands/sort.md) |
| `TREE` | [tree.md](commands/tree.md) |
| `XCOPY` | [xcopy.md](commands/xcopy.md) |
| `ROBOCOPY` | [robocopy.md](commands/robocopy.md) |
| `WHERE`, `HOSTNAME`, `WHOAMI`, `TIMEOUT` | [utils.md](commands/utils.md) |

## External (Passthrough) Commands

Any unrecognised command is forwarded to the host OS automatically — no static list required: [passthrough.md](commands/passthrough.md)
