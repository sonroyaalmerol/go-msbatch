# Arithmetic — SET /A

`SET /A expression` evaluates an integer arithmetic expression and stores the result in a variable.

```bat
SET /A X=2+3         :: X=5
SET /A Y=X*4         :: Y=20
SET /A Z=Y/3         :: Z=6  (integer division, truncates toward zero)
SET /A R=Y%3         :: R=2  (modulo)
```

Variable names in expressions do not require `%…%` quoting:

```bat
SET NUM=10
SET /A RESULT=NUM*2+1    :: RESULT=21
```

## Operator Precedence (high to low)

| Level | Operators |
|-------|-----------|
| Unary | `- + ! ~` |
| Multiplicative | `* / %` |
| Additive | `+ -` |
| Shift | `<< >>` |
| Bitwise AND | `&` |
| Bitwise XOR | `^` |
| Bitwise OR | `\|` |
| Logical AND | `&&` |
| Logical OR | `\|\|` |
| Assignment | `= += -= *= /= %= &= ^= \|= <<= >>=` |

Parentheses can override precedence: `SET /A R=(2+3)*4`.

## Number Literals

| Prefix | Base |
|--------|------|
| `0x` or `0X` | Hexadecimal |
| Leading `0` | Octal |
| No prefix | Decimal |

```bat
SET /A HEX=0xFF    :: 255
SET /A OCT=010     :: 8
```

## Multi-expression (comma separator)

Multiple assignments can be chained with commas. Each is evaluated left-to-right and the value of the last expression is stored in `ERRORLEVEL`:

```bat
SET /A A=1, B=2, C=A+B    :: A=1, B=2, C=3
```

## Logical and Comparison Operators

These return `1` (true) or `0` (false):

```bat
SET /A R=5>3       :: 1
SET /A R=5==5      :: 1
SET /A R=!0        :: 1  (logical NOT of 0)
```

## Caveats

- **Division and modulo by zero** silently return `0` rather than raising an error. Real cmd.exe prints `Divide by zero error.` and sets `ERRORLEVEL` to a non-zero value.
- **Integer size** is 64-bit signed. Overflow wraps silently.
- **`%` in batch scripts** must be doubled (`%%`) to produce a literal `%` for the modulo operator when used inside a `.bat` file. In interactive (non-batch) mode, a single `%` is sufficient.
  ```bat
  :: In a .bat file:
  SET /A R=10%%3    :: R=1
  :: Interactive:
  SET /A R=10%3     :: R=1
  ```
- **No floating-point** — all values are integers. `SET /A 5/2` is `2`, not `2.5`.
- **Undefined variable** in an expression evaluates as `0`.
