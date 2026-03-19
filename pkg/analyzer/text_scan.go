package analyzer

import (
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

type VarRef struct {
	Name      string
	Line      int
	Col       int
	EndCol    int
	IsDelayed bool
	IsForVar  bool
}

func scanVariableRefs(result *Result, lines []string, uri string) {
	varRefs := []VarRef{}

	for i, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		stripped := strings.TrimLeft(trimmed, "@")

		if strings.HasPrefix(trimmed, "::") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(stripped), "rem ") || strings.ToLower(stripped) == "rem" {
			continue
		}
		if strings.HasPrefix(trimmed, ":") && !strings.HasPrefix(trimmed, "::") {
			continue
		}

		refs := scanLineForVarRefs(line, i)
		varRefs = append(varRefs, refs...)
	}

	for _, ref := range varRefs {
		if ref.IsForVar {
			if sym := result.Symbols.ResolveForVar(ref.Name, ref.Line); sym != nil {
				loc := Location{URI: uri, Line: ref.Line, Col: ref.Col, EndCol: ref.EndCol}
				sym.AddRef(loc, RefRead)
			} else {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Location: Location{
						URI:    uri,
						Line:   ref.Line,
						Col:    ref.Col,
						EndCol: ref.EndCol,
					},
					Message:  "Undefined FOR loop variable: " + ref.Name,
					Severity: SeverityWarning,
				})
			}
		} else {
			if sym := result.Symbols.ResolveVariable(ref.Name, ref.Line); sym != nil {
				loc := Location{URI: uri, Line: ref.Line, Col: ref.Col, EndCol: ref.EndCol}
				sym.AddRef(loc, RefRead)
			}
		}
	}
}

func scanLineForVarRefs(line string, lineIdx int) []VarRef {
	var refs []VarRef

	refs = append(refs, scanPercentVars(line, lineIdx)...)
	refs = append(refs, scanDelayedVars(line, lineIdx)...)

	return refs
}

func scanPercentVars(line string, lineIdx int) []VarRef {
	var refs []VarRef
	rest := line
	offset := 0

	for {
		pct := strings.Index(rest, "%")
		if pct < 0 {
			break
		}

		if pct+2 < len(rest) && rest[pct+1] == '%' {
			char := rest[pct+2]
			if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
				refs = append(refs, VarRef{
					Name:     strings.ToUpper(string(char)),
					Line:     lineIdx,
					Col:      offset + pct,
					EndCol:   offset + pct + 3,
					IsForVar: true,
				})
				offset += pct + 3
				rest = rest[pct+3:]
				continue
			}
		}

		after := rest[pct+1:]
		end := strings.Index(after, "%")
		if end < 0 {
			break
		}

		name := after[:end]
		advance := pct + 1 + end + 1

		if name != "" && (name[0] < '0' || name[0] > '9') {
			if name[0] != '~' {
				baseName, _ := processor.SplitVarModifier(name)
				refs = append(refs, VarRef{
					Name:     strings.ToUpper(baseName),
					Line:     lineIdx,
					Col:      offset + pct + 1,
					EndCol:   offset + pct + 1 + len(name),
					IsForVar: false,
				})
			}
		}

		offset += advance
		rest = rest[advance:]
	}

	return refs
}

func scanDelayedVars(line string, lineIdx int) []VarRef {
	var refs []VarRef
	rest := line
	offset := 0

	for {
		bang := strings.Index(rest, "!")
		if bang < 0 {
			break
		}

		after := rest[bang+1:]
		end := strings.Index(after, "!")
		if end < 0 {
			break
		}

		name := after[:end]
		advance := bang + 1 + end + 1

		if name != "" {
			baseName, _ := processor.SplitVarModifier(name)
			refs = append(refs, VarRef{
				Name:      strings.ToUpper(baseName),
				Line:      lineIdx,
				Col:       offset + bang + 1,
				EndCol:    offset + bang + 1 + len(name),
				IsDelayed: true,
			})
		}

		offset += advance
		rest = rest[advance:]
	}

	return refs
}
