package processor

import (
	"path/filepath"
	"strconv"
	"strings"
)

// Phase0ReadLine applies phase-0 line-reading rules:
//   - Replace 0x1A (Ctrl-Z) with '\n'.
//   - Line continuity: Merge lines ending with ^ (if not escaped by another ^).
func Phase0ReadLine(src string) string {
	src = strings.ReplaceAll(src, "\x1a", "\n")

	lines := strings.Split(src, "\n")
	var result []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		for strings.HasSuffix(line, "^") {
			// Check if the caret itself is escaped (e.g. ^^ at the end)
			if strings.HasSuffix(line, "^^") {
				break
			}
			line = line[:len(line)-1]
			if i+1 < len(lines) {
				i++
				line += strings.TrimLeft(lines[i], " \t")
			} else {
				break
			}
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// Phase1PercentExpand performs phase-1 percent expansion on src.
func Phase1PercentExpand(src string, env *Environment, args []string) string {
	runes := []rune(src)
	var sb strings.Builder
	for i := 0; i < len(runes); {
		r := runes[i]
		if r != '%' {
			sb.WriteRune(r)
			i++
			continue
		}

		// Peek ahead
		if i+1 >= len(runes) {
			sb.WriteRune('%')
			i++
			continue
		}

		next := runes[i+1]
		switch {
		case next == '%':
			// %% -> % (escaped literal percent)
			sb.WriteRune('%')
			i += 2
		case next >= '0' && next <= '9':
			// %0..%9 positional argument (batch mode only)
			if env.BatchMode() {
				idx := int(next - '0')
				if idx < len(args) {
					sb.WriteString(args[idx])
				}
			} else {
				sb.WriteRune('%')
				sb.WriteRune(next)
			}
			i += 2
		case next == '*':
			// %* all positional arguments (batch mode only)
			if env.BatchMode() {
				if len(args) > 0 {
					sb.WriteString(strings.Join(args, " "))
				}
			} else {
				sb.WriteRune('%')
				sb.WriteRune('*')
			}
			i += 2
		case next == '~':
			// %~ modifier - not fully implemented, just skip the modifier part
			// and treat as variable if name follows.
			i++               // skip %
			sb.WriteRune('%') // leave for later? No, this is tricky.
			// For now, let's just treat it as start of a variable name.
		default:
			// %NAME% or %VAR:~start,len% or %VAR:old=new% or bare %
			end := indexRuneFrom(runes, '%', i+1)
			if end < 0 {
				sb.WriteRune('%')
				i++
				continue
			}
			rawName := string(runes[i+1 : end])
			i = end + 1

			varName := rawName
			manipulation := ""
			if before, after, ok := strings.Cut(rawName, ":"); ok {
				varName = before
				manipulation = after
			}

			val, ok := env.Get(varName)
			if !ok {
				if !env.BatchMode() {
					sb.WriteRune('%')
					sb.WriteString(rawName)
					sb.WriteRune('%')
				}
				continue
			}

			if manipulation != "" {
				if strings.HasPrefix(manipulation, "~") {
					val = applySlicing(val, manipulation[1:])
				} else if strings.Contains(manipulation, "=") {
					if before, after, found := strings.Cut(manipulation, "="); found {
						val = applySubstitution(val, before, after)
					}
				}
			}
			sb.WriteString(val)
		}
	}
	return sb.String()
}

func indexRuneFrom(runes []rune, r rune, start int) int {
	for i := start; i < len(runes); i++ {
		if runes[i] == r {
			return i
		}
	}
	return -1
}

func applySlicing(val, sliceExpr string) string {
	startStr := sliceExpr
	lenStr := ""
	if before, after, ok := strings.Cut(sliceExpr, ","); ok {
		startStr = before
		lenStr = after
	}

	start, _ := strconv.Atoi(startStr)
	hasLen := lenStr != ""
	length, _ := strconv.Atoi(lenStr)

	runes := []rune(val)
	vLen := len(runes)

	if start < 0 {
		start = vLen + start
	}
	if start < 0 {
		start = 0
	}
	if start > vLen {
		return ""
	}

	if !hasLen {
		return string(runes[start:])
	}

	if length < 0 {
		length = vLen + length - start
	}
	if length < 0 {
		length = 0
	}
	end := min(start+length, vLen)
	if end < start {
		return ""
	}

	return string(runes[start:end])
}

func applySubstitution(val, old, new string) string {
	if strings.HasPrefix(old, "*") {
		search := old[1:]
		if search == "" {
			return new
		}
		if idx := strings.Index(strings.ToLower(val), strings.ToLower(search)); idx >= 0 {
			return new + val[idx+len(search):]
		}
		return val
	}
	if old == "" {
		return val
	}

	var res strings.Builder
	lowerVal := strings.ToLower(val)
	lowerOld := strings.ToLower(old)

	curr := 0
	for {
		idx := strings.Index(lowerVal[curr:], lowerOld)
		if idx < 0 {
			res.WriteString(val[curr:])
			break
		}
		res.WriteString(val[curr : curr+idx])
		res.WriteString(new)
		curr += idx + len(old)
	}
	return res.String()
}

// Phase4ForVarExpand performs phase-4 FOR-variable expansion on src.
func Phase4ForVarExpand(src string, forVars map[string]string) string {
	if len(forVars) == 0 {
		return src
	}
	runes := []rune(src)
	var sb strings.Builder
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '%' && i+1 < len(runes) {
			next := runes[i+1]
			if next == '%' && i+2 < len(runes) {
				// %%X
				varName := string(runes[i+2])
				if val, ok := forVars[varName]; ok {
					sb.WriteString(val)
					i += 2
					continue
				}
			} else if next == '~' {
				// %~modifiersX — scan for the first char that is in forVars
				j := i + 2
				varIdx := -1
				for k := j; k < len(runes); k++ {
					if _, ok := forVars[string(runes[k])]; ok {
						varIdx = k
						break
					}
				}
				if varIdx >= j {
					mods := string(runes[j:varIdx])
					varName := string(runes[varIdx])
					if val, ok := forVars[varName]; ok {
						sb.WriteString(applyForVarModifiers(val, mods))
						i = varIdx
						continue
					}
				}
			} else {
				// %X
				varName := string(next)
				if val, ok := forVars[varName]; ok {
					sb.WriteString(val)
					i++
					continue
				}
			}
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// applyForVarModifiers applies FOR variable modifiers (n=name, x=ext, p=path, d=drive).
func applyForVarModifiers(val, mods string) string {
	result := val
	for _, mod := range strings.ToLower(mods) {
		switch mod {
		case 'n':
			base := filepath.Base(result)
			ext := filepath.Ext(base)
			result = base[:len(base)-len(ext)]
		case 'x':
			result = filepath.Ext(filepath.Base(result))
		case 'p':
			result = filepath.Dir(result)
		case 'd':
			if len(result) >= 2 && result[1] == ':' {
				result = result[:2]
			} else {
				result = ""
			}
		}
	}
	return result
}

// Phase5DelayedExpand performs phase-5 delayed variable expansion (!VAR!).
func Phase5DelayedExpand(src string, env *Environment) string {
	if !env.DelayedExpansion() {
		return src
	}
	runes := []rune(src)
	var sb strings.Builder
	for i := 0; i < len(runes); {
		r := runes[i]

		// ^! is an escaped bang — emits a literal !
		if r == '^' && i+1 < len(runes) && runes[i+1] == '!' {
			sb.WriteRune('!')
			i += 2
			continue
		}

		if r != '!' {
			sb.WriteRune(r)
			i++
			continue
		}

		end := indexRuneFrom(runes, '!', i+1)
		if end < 0 {
			sb.WriteRune('!')
			i++
			continue
		}

		name := string(runes[i+1 : end])
		i = end + 1

		if val, ok := env.Get(name); ok {
			sb.WriteString(val)
		} else if !env.BatchMode() {
			// In command-line mode, undefined !VAR! is left unchanged.
			sb.WriteRune('!')
			sb.WriteString(name)
			sb.WriteRune('!')
		}
	}
	return sb.String()
}
