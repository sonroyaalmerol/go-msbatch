package processor

import (
	"os"
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

		// ^% is an escaped percent — emits a literal %
		if r == '^' && i+1 < len(runes) && runes[i+1] == '%' {
			sb.WriteRune('%')
			i += 2
			continue
		}

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
			// %~[modifiers][n] or %~$VARNAME:n — tilde modifier on positional parameter.
			i += 2 // skip '%' and '~'

			// Collect modifier letters.
			const modChars = "fFdDpPnNxXsSeEaAtTzZ"
			var mods strings.Builder
			for i < len(runes) && strings.ContainsRune(modChars, runes[i]) {
				mods.WriteRune(runes[i])
				i++
			}

			// Optional $VARNAME: path-search modifier.
			pathVar := ""
			if i < len(runes) && runes[i] == '$' {
				i++ // skip '$'
				var varBuf strings.Builder
				for i < len(runes) && runes[i] != ':' && runes[i] != 0 && runes[i] != '\n' && runes[i] != '\r' {
					varBuf.WriteRune(runes[i])
					i++
				}
				if i < len(runes) && runes[i] == ':' {
					i++ // skip ':'
					pathVar = varBuf.String()
				}
			}

			// Expect a digit 0-9.
			if i < len(runes) && runes[i] >= '0' && runes[i] <= '9' && env.BatchMode() {
				idx := int(runes[i] - '0')
				i++
				argVal := ""
				if idx < len(args) {
					argVal = args[idx]
				}
				// Strip surrounding quotes — the base %~ behaviour.
				if len(argVal) >= 2 && argVal[0] == '"' && argVal[len(argVal)-1] == '"' {
					argVal = argVal[1 : len(argVal)-1]
				}
				if pathVar != "" {
					// Search for the file in the directories listed in the named variable.
					if searchPath, ok := env.Get(pathVar); ok {
						found := false
						for _, dir := range filepath.SplitList(searchPath) {
							candidate := filepath.Join(dir, argVal)
							if _, err := os.Stat(candidate); err == nil {
								argVal = candidate
								found = true
								break
							}
						}
						if !found {
							argVal = ""
						}
					} else {
						argVal = ""
					}
				}
				if mods.Len() > 0 {
					argVal = applyForVarModifiers(argVal, mods.String())
				}
				sb.WriteString(argVal)
			} else {
				// No digit or not batch mode — emit literally.
				sb.WriteString("%~")
				sb.WriteString(mods.String())
				if pathVar != "" {
					sb.WriteByte('$')
					sb.WriteString(pathVar)
					sb.WriteByte(':')
				}
			}
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

			val, ok := env.Get(rawName)
			if !ok {
				varName, manipulation := SplitVarModifier(rawName)
				val, ok = env.Get(varName)
				if ok && manipulation != "" {
					if strings.HasPrefix(manipulation, "~") {
						val = applySlicing(val, manipulation[1:])
					} else if strings.Contains(manipulation, "=") {
						if before, after, ok := strings.Cut(manipulation, "="); ok {
							val = applySubstitution(val, before, after)
						}
					}
				}
			}

			if !ok {
				if !env.BatchMode() {
					sb.WriteRune('%')
					sb.WriteString(rawName)
					sb.WriteRune('%')
				}
				continue
			}

			sb.WriteString(val)
		}
	}
	return sb.String()
}

// SplitVarModifier splits a raw percent-expansion expression into the base
// variable name and optional modifier string (the part after the first ':').
//
//	"STR:~0,5"   → ("STR", "~0,5")
//	"VAR:old=new" → ("VAR", "old=new")
//	"PLAIN"       → ("PLAIN", "")
func SplitVarModifier(expr string) (name, modifier string) {
	if before, after, ok := strings.Cut(expr, ":"); ok {
		return before, after
	}
	return expr, ""
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

// applyForVarModifiers applies FOR/positional-parameter tilde modifiers to val.
//
// Path-component modifiers (d, p, n, x) are all extracted from the same
// resolved value and their results concatenated, matching cmd.exe behaviour
// where e.g. %~dp0 = drive + directory (not p applied to the output of d).
//
// Supported modifiers:
//
//	f / s / e  — resolve to absolute path (applied first)
//	d          — drive letter ("C:" on Windows, "" on Unix)
//	p          — directory path with trailing separator
//	n          — filename without extension
//	x          — extension (including dot)
//	a          — file attributes string
//	t          — last-modified timestamp
//	z          — file size in bytes
func applyForVarModifiers(val, mods string) string {
	lower := strings.ToLower(mods)

	// f / s / e: resolve to absolute path first.
	if strings.ContainsAny(lower, "fse") {
		if abs, err := filepath.Abs(val); err == nil {
			val = abs
		}
	}

	hasD := strings.ContainsRune(lower, 'd')
	hasP := strings.ContainsRune(lower, 'p')
	hasN := strings.ContainsRune(lower, 'n')
	hasX := strings.ContainsRune(lower, 'x')

	if hasD || hasP || hasN || hasX {
		// Separate the drive prefix so p/n/x operate on the path portion only.
		drive := ""
		pathPart := val
		if len(val) >= 2 && val[1] == ':' {
			drive = val[:2]
			pathPart = val[2:]
		}

		var result strings.Builder
		if hasD {
			result.WriteString(drive)
		}
		if hasP {
			dir := filepath.Dir(pathPart)
			switch {
			case dir == ".":
				// Relative filename with no directory component → empty.
			case dir == "/" || dir == string(filepath.Separator):
				result.WriteString(dir)
			default:
				result.WriteString(dir)
				if !strings.HasSuffix(dir, string(filepath.Separator)) {
					result.WriteRune(filepath.Separator)
				}
			}
		}
		if hasN {
			base := filepath.Base(val)
			ext := filepath.Ext(base)
			result.WriteString(base[:len(base)-len(ext)])
		}
		if hasX {
			result.WriteString(filepath.Ext(filepath.Base(val)))
		}
		val = result.String()
	}

	// Informational modifiers applied to the current value as a path.
	for _, mod := range lower {
		switch mod {
		case 'a':
			if fi, err := os.Stat(val); err == nil {
				if fi.IsDir() {
					val = "d---------"
				} else {
					val = "--a------"
				}
			}
		case 't':
			if fi, err := os.Stat(val); err == nil {
				val = fi.ModTime().Format("01/02/2006 03:04 PM")
			}
		case 'z':
			if fi, err := os.Stat(val); err == nil {
				val = strconv.FormatInt(fi.Size(), 10)
			}
		}
	}
	return val
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

		rawName := string(runes[i+1 : end])
		i = end + 1

		val, ok := env.Get(rawName)
		if !ok {
			varName, manipulation := SplitVarModifier(rawName)
			val, ok = env.Get(varName)
			if ok && manipulation != "" {
				if strings.HasPrefix(manipulation, "~") {
					val = applySlicing(val, manipulation[1:])
				} else if strings.Contains(manipulation, "=") {
					if before, after, ok := strings.Cut(manipulation, "="); ok {
						val = applySubstitution(val, before, after)
					}
				}
			}
		}

		if !ok {
			if !env.BatchMode() {
				// In command-line mode, undefined !VAR! is left unchanged.
				sb.WriteRune('!')
				sb.WriteString(rawName)
				sb.WriteRune('!')
			}
			continue
		}

		// If the expanded value contains percent signs, perform another pass of
		// percent expansion on it. This handles the "expanded further" requirement
		// for nested variables produced by delayed expansion.
		if strings.ContainsRune(val, '%') {
			val = Phase1PercentExpand(val, env, nil)
		}

		sb.WriteString(val)
	}
	return sb.String()
}
