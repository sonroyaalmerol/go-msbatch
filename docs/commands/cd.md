# CD / CHDIR

Displays the current working directory or changes to a new one.

## Syntax

```bat
CD [path]
CHDIR [path]
```

## Behaviour

| Form           | Action                                                |
| -------------- | ----------------------------------------------------- |
| `CD` (no args) | Prints the current working directory                  |
| `CD path`      | Changes to `path`                                     |
| `CD ..`        | Moves to the parent directory                         |
| `CD \`         | Moves to the root of the current drive (on Unix: `/`) |

## Path Handling

Windows-style paths are mapped to Unix equivalents automatically:

```bat
CD C:\Users\Alice    :: equivalent to cd /mnt/c/Users/Alice on Linux
CD ..\sibling
```

See [cross-platform.md](../cross-platform.md) for full path mapping rules.

## Caveats

- `CD` calls `os.Chdir()`, which changes the **process-level** working directory. This affects all subsequent relative path operations across the entire interpreter.
- Per-drive current directories are tracked like cmd.exe: `CD D:\other` without `/D` validates the directory and remembers it for drive `D:` without leaving the current drive; `CD /D D:\other` also switches to it; a bare `D:` command switches to drive `D:`'s remembered directory.
- The `ERRORLEVEL` is set to `1` if the directory does not exist or cannot be accessed.
