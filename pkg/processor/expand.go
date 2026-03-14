package processor

import (
	"strings"
	"unicode"
)

// Phase0ReadLine applies phase-0 line-reading rules:
//   - Replace 0x1A (Ctrl-Z) with '\n'.
func Phase0ReadLine(src string) string {
	return strings.ReplaceAll(src, "\x1a", "\n")
}

// Phase1PercentExpand performs phase-1 percent expansion on src.
//
// Rules (batch mode):
//   - %% → %
//   - %0..%9 → positional argument (looked up in args)
//   - %* → all positional arguments joined by space
//   - %VAR% → value of VAR from env; missing variable → "" (empty)
//
// In command-line mode (env.BatchMode() == false):
//   - %0..%9 and %* are left unchanged
//   - Missing %VAR% is left unchanged (not emptied)
//   - %%VAR%% → %value%
func Phase1PercentExpand(src string, env *Environment, args []string) string {
	runes := []rune(src)
	var sb strings.Builder
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r != '%' {
			sb.WriteRune(r)
			i++
			continue
		}
		// found %
		if i+1 >= len(runes) {
			// trailing lone %
			sb.WriteRune('%')
			i++
			continue
		}
		next := runes[i+1]
		switch {
		case next == '%':
			// %% → %
			sb.WriteRune('%')
			i += 2
		case next >= '0' && next <= '9':
			idx := int(next - '0')
			if env.BatchMode() && idx < len(args) {
				sb.WriteString(args[idx])
			} else if env.BatchMode() {
				// out-of-range positional → empty in batch
			} else {
				// command-line mode: leave unchanged
				sb.WriteRune('%')
				sb.WriteRune(next)
			}
			i += 2
		case next == '*':
			if env.BatchMode() {
				sb.WriteString(strings.Join(args, " "))
			} else {
				sb.WriteString("%*")
			}
			i += 2
		default:
			// %NAME% or bare %
			end := indexRuneFrom(runes, '%', i+1)
			if end < 0 {
				// No closing % — leave the % literal in batch; leave unchanged in cmdline
				sb.WriteRune('%')
				i++
				continue
			}
			name := string(runes[i+1 : end])
			val, ok := env.Get(name)
			if ok {
				sb.WriteString(val)
			} else if !env.BatchMode() {
				// Command-line mode: leave undefined %VAR% unchanged
				sb.WriteRune('%')
				sb.WriteString(name)
				sb.WriteRune('%')
			}
			// batch mode missing var → empty (write nothing)
			i = end + 1
		}
	}
	return sb.String()
}

// Phase4ForVarExpand performs phase-4 FOR-variable expansion on src.
//
// In batch mode %%X is the loop-variable token; after phase-1 it becomes %X.
// This phase resolves %X against the supplied forVars map (case-sensitive).
//
// forVars maps single-character names (e.g. "i") to their current loop value.
func Phase4ForVarExpand(src string, forVars map[string]string) string {
	if len(forVars) == 0 {
		return src
	}
	runes := []rune(src)
	var sb strings.Builder
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r != '%' {
			sb.WriteRune(r)
			i++
			continue
		}
		if i+1 >= len(runes) {
			sb.WriteRune('%')
			i++
			continue
		}
		varName := string(runes[i+1])
		if val, ok := forVars[varName]; ok {
			sb.WriteString(val)
			i += 2
			continue
		}
		// Check for ~modifier form: %~fzdi  (modifier chars followed by var name)
		if runes[i+1] == '~' {
			j := i + 2
			for j < len(runes) && isModifierChar(runes[j]) {
				j++
			}
			if j < len(runes) {
				possibleVar := string(runes[j])
				if val, ok := forVars[possibleVar]; ok {
					// modifiers apply to value — for now return value as-is
					sb.WriteString(applyForModifiers(string(runes[i+2:j]), val))
					i = j + 1
					continue
				}
			}
		}
		sb.WriteRune('%')
		i++
	}
	return sb.String()
}

// Phase5DelayedExpand performs phase-5 delayed variable expansion on src.
//
// Only active when env.DelayedExpansion() is true.
// !VAR! → value of VAR; undefined → "" in batch, left unchanged in cmdline.
// ^! inside a !-containing token is an escaped ! and becomes !.
func Phase5DelayedExpand(src string, env *Environment) string {
	if !env.DelayedExpansion() {
		return src
	}
	runes := []rune(src)
	var sb strings.Builder
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r == '^' && i+1 < len(runes) && runes[i+1] == '!' {
			// ^! → literal !
			sb.WriteRune('!')
			i += 2
			continue
		}
		if r != '!' {
			sb.WriteRune(r)
			i++
			continue
		}
		// found opening !
		end := indexRuneFrom(runes, '!', i+1)
		if end < 0 {
			// no closing ! — leave unchanged
			if !env.BatchMode() {
				sb.WriteRune('!')
			}
			i++
			continue
		}
		name := string(runes[i+1 : end])
		val, ok := env.Get(name)
		if ok {
			sb.WriteString(val)
		} else if !env.BatchMode() {
			// command-line mode: leave undefined !VAR! unchanged
			sb.WriteRune('!')
			sb.WriteString(name)
			sb.WriteRune('!')
		}
		// batch mode missing var → empty
		i = end + 1
	}
	return sb.String()
}

// ---- helpers ---------------------------------------------------------------

// indexRuneFrom returns the index of the first occurrence of r in runes at or
// after position start, or -1 if not found.
func indexRuneFrom(runes []rune, r rune, start int) int {
	for i := start; i < len(runes); i++ {
		if runes[i] == r {
			return i
		}
	}
	return -1
}

// isModifierChar reports whether r is a valid FOR variable modifier character.
// Modifier chars: f z d p n x s a t (and digits for field widths).
func isModifierChar(r rune) bool {
	return strings.ContainsRune("fzdpnxsatFZDPNXSAT", r) || unicode.IsDigit(r)
}

// applyForModifiers applies FOR-variable tilde modifiers to value.
// Only a subset is implemented; the rest return value unchanged.
func applyForModifiers(mods, value string) string {
	for _, mod := range strings.ToLower(mods) {
		switch mod {
		case 'n':
			// file name without extension
			if idx := strings.LastIndex(value, "."); idx >= 0 {
				value = value[:idx]
			}
		case 'x':
			// extension only
			if idx := strings.LastIndex(value, "."); idx >= 0 {
				value = value[idx:]
			} else {
				value = ""
			}
		}
	}
	return value
}
