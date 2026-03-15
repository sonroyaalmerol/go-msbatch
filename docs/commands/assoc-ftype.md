# ASSOC and FTYPE

---

## ASSOC

Displays or modifies file-extension associations.

### Syntax

```bat
ASSOC                   :: list all associations
ASSOC .ext              :: show association for .ext
ASSOC .ext=typename     :: set association
ASSOC .ext=             :: delete association
```

### Behaviour

Associates a file extension with a file type name. The association is used by `FTYPE` to determine which program opens a file.

```bat
ASSOC .txt=txtfile
ASSOC .txt              :: prints: .txt=txtfile
ASSOC .txt=             :: removes association
```

### Caveats

- **In-memory only** — associations are stored in a map on the `Registry` object. They are **not read from or written to** the Windows registry. Changes do not persist between interpreter sessions.
- On a fresh start, no associations are pre-populated from the system registry.
- Real cmd.exe reads and writes `HKEY_CLASSES_ROOT`. Scripts that use `ASSOC` to permanently configure the system will not work as intended.

---

## FTYPE

Displays or modifies file-type open commands.

### Syntax

```bat
FTYPE                       :: list all file types
FTYPE typename              :: show open command for typename
FTYPE typename=command      :: set open command
FTYPE typename=             :: delete file type
```

### Behaviour

Associates a file type name (from `ASSOC`) with the command used to open files of that type. `%1` in the command is replaced by the filename.

```bat
FTYPE txtfile=notepad.exe %1
FTYPE txtfile               :: prints: txtfile=notepad.exe %1
FTYPE txtfile=              :: removes entry
```

### Caveats

- **In-memory only** — same as `ASSOC`. Not persisted and not pre-loaded from the Windows registry.
- `FTYPE` does not actually execute files. It only stores the association. There is no built-in mechanism to launch a file using its registered type (unlike double-clicking in Explorer).
