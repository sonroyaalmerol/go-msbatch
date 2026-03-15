# MKLINK

Creates symbolic links, hard links, and directory junctions.

## Syntax

```bat
MKLINK [[/D] | [/H] | [/J]] link target
```

## Flags

| Flag | Link type |
|------|-----------|
| (none) | Symbolic link to a file |
| `/D` | Symbolic link to a directory |
| `/J` | Directory junction (NTFS) |
| `/H` | Hard link to a file |

## Behaviour

```bat
MKLINK mylink.txt original.txt         :: file symlink
MKLINK /D linkdir targetdir            :: directory symlink
MKLINK /J junction C:\target           :: directory junction
MKLINK /H hardlink.txt original.txt    :: hard link
```

On success prints:

```
symbolic link created for mylink.txt <<===>> original.txt
```

(or "Hard link" / "Junction" for the other types.)

## Caveats

- **`/D` and `/J` are treated identically on Unix** — both result in a call to `os.Symlink()`. NTFS junctions are a Windows-specific concept (a reparse point pointing to a local absolute path) with no direct Unix equivalent.
- **On Windows**, creating symbolic links requires either elevated privileges (Administrator) or that Developer Mode is enabled. Hard links (`/H`) do not require elevation.
- **`/H` cannot cross filesystems** — `os.Link()` will fail if `link` and `target` are on different mount points. The error is reported to stderr.
- The target path is used exactly as provided. Relative targets are relative to the directory containing the link, which matches the behaviour of `ln -s` on Unix and `MKLINK` on Windows.
- `MKLINK` does not verify that the target exists before creating the link (except for hard links, where the target must exist).
