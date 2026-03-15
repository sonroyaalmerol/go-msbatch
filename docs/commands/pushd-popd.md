# PUSHD / POPD

Manages a stack of saved working directories.

## PUSHD

```bat
PUSHD [path]
```

Saves the current directory onto the directory stack, then optionally changes to `path`.

```bat
PUSHD C:\temp      :: save cwd, cd to C:\temp (→ /mnt/c/temp on Linux)
PUSHD              :: save cwd only, do not change directory
```

## POPD

```bat
POPD
```

Pops the most recently pushed directory from the stack and changes back to it.

## Example

```bat
PUSHD work
ECHO In: %CD%
PUSHD sub
ECHO In: %CD%
POPD
ECHO Back: %CD%
POPD
ECHO Back: %CD%
```

## Caveats

- The stack is stored in memory as a simple slice on the `Processor`. It does not persist across separate interpreter invocations.
- If `POPD` is called with an empty stack, it silently does nothing (no error).
- Real cmd.exe's `PUSHD` can accept a UNC path and temporarily maps it as a network drive. go-msbatch does not support UNC path remapping.
- `PUSHD` with a path uses the same `os.Chdir()` call as `CD`; all caveats from [cd.md](cd.md) apply.
- `%CD%` is not a real environment variable in go-msbatch — there is no automatic `CD` variable. Use `CD` command output or capture via `FOR /F` if needed.
