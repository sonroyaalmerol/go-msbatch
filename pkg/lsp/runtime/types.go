package runtime

import (
	"fmt"
	"strings"
)

type ForInfo struct {
	VarName    string
	Line       int
	IterType   string
	IterSource string
}

func (fi ForInfo) IterationDescription() string {
	switch fi.IterType {
	case "files":
		return fmt.Sprintf("files matching %s", fi.IterSource)
	case "range":
		return fmt.Sprintf("range %s", fi.IterSource)
	case "dir":
		return fmt.Sprintf("directories matching %s", fi.IterSource)
	case "f_parse":
		return fmt.Sprintf("lines from %s", fi.IterSource)
	case "recursive":
		return fmt.Sprintf("recursively matching %s", fi.IterSource)
	default:
		return fi.IterSource
	}
}

func (fi ForInfo) HasPattern() bool {
	return fi.IterType == "files" || fi.IterType == "dir" || fi.IterType == "recursive"
}

func (fi ForInfo) String() string {
	return fmt.Sprintf("%%%%%s (%s): %s", fi.VarName, fi.IterType, fi.IterSource)
}

type HoverInfo struct {
	VariableName       string
	PossibleValues     []PossibleValue
	BaseValue          string
	ModifierApplied    string
	ExpandedValue      string
	IsDelayedExpansion bool
	IsForVar           bool
	ForVarName         string
	IterationSource    string
}

func (hi *HoverInfo) Format() string {
	var sb strings.Builder

	if hi.IsForVar {
		sb.WriteString(fmt.Sprintf("**FOR Variable:** `%%%s`\n\n", hi.ForVarName))
		sb.WriteString(fmt.Sprintf("**Iterates over:** `%s`", hi.IterationSource))
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("**Variable:** `%s`", hi.VariableName))

	if hi.IsDelayedExpansion {
		sb.WriteString(" (delayed expansion)")
	}

	if len(hi.PossibleValues) > 0 {
		sb.WriteString("\n\n**Possible values:**")
		for _, pv := range hi.PossibleValues {
			sb.WriteString(fmt.Sprintf("\n- `%s` (line %d)", pv.Value, pv.SourceLine))
		}
	}

	if hi.BaseValue != "" {
		sb.WriteString(fmt.Sprintf("\n\n**Base value:** `%s`", hi.BaseValue))
	}

	if hi.ModifierApplied != "" {
		sb.WriteString(fmt.Sprintf("\n\n**Modifier:** `%s`", hi.ModifierApplied))
	}

	if hi.ExpandedValue != "" {
		sb.WriteString(fmt.Sprintf("\n\n**Expanded:** `%s`", hi.ExpandedValue))
	}

	return sb.String()
}

type RuntimeResult struct {
	Variables  map[string]*VarState
	ForVars    map[string]ForInfo
	LineStates map[int]*State
	FinalState *State
	tokens     []tokenInfo
}

type tokenInfo struct {
	line    int
	col     int
	endCol  int
	varName string
	raw     string
	kind    tokenKind
}

type tokenKind int

const (
	kindNormal tokenKind = iota
	kindDelayed
	kindFor
)

func (r *RuntimeResult) GetVariable(name string) *VarState {
	return r.Variables[strings.ToUpper(name)]
}

func (r *RuntimeResult) GetForVar(name string) *ForInfo {
	upperName := strings.ToUpper(name)
	if info, ok := r.ForVars[upperName]; ok {
		return &info
	}
	if info, ok := r.ForVars[name]; ok {
		return &info
	}
	return nil
}

func (r *RuntimeResult) GetStateAtLine(line int) *State {
	if r.LineStates == nil {
		return nil
	}

	if line < 0 {
		return NewState()
	}

	if state, ok := r.LineStates[line]; ok {
		return state
	}

	var bestLine int = -1
	for l := range r.LineStates {
		if l <= line && l > bestLine {
			bestLine = l
		}
	}

	if bestLine >= 0 {
		return r.LineStates[bestLine]
	}

	return nil
}

func (r *RuntimeResult) GetFinalState() *State {
	return r.FinalState
}

func (r *RuntimeResult) GetExpandedValueAt(line, col int) string {
	for _, ti := range r.tokens {
		if ti.line == line && col >= ti.col && col < ti.endCol {
			state := r.GetStateAtLine(line)
			if state == nil {
				return ""
			}
			v := state.GetVariable(ti.varName)
			if v == nil || len(v.Values) == 0 {
				return ""
			}
			return v.Values[len(v.Values)-1].Value
		}
	}
	return ""
}

func (r *RuntimeResult) GetHoverInfo(line, col int) *HoverInfo {
	for _, ti := range r.tokens {
		if ti.line == line && col >= ti.col && col < ti.endCol {
			info := &HoverInfo{}

			if ti.kind == kindFor {
				forInfo := r.GetForVar(ti.varName)
				if forInfo != nil {
					info.IsForVar = true
					info.ForVarName = forInfo.VarName
					info.IterationSource = forInfo.IterSource
				}
				return info
			}

			info.VariableName = ti.varName
			info.IsDelayedExpansion = ti.kind == kindDelayed

			state := r.GetStateAtLine(line)
			if state != nil {
				v := state.GetVariable(ti.varName)
				if v != nil {
					info.PossibleValues = v.Values
				}
			}

			return info
		}
	}
	return nil
}
