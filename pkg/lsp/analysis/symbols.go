package analysis

import (
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

type Position struct {
	Line int
	Col  int
}

type Range struct {
	Start Position
	End   Position
}

type Label struct {
	Name       string
	Definition Range
	References []Range
}

type VariableKind int

const (
	VarKindNormal VariableKind = iota
	VarKindDelayed
	VarKindFor
)

type VariableReference struct {
	Range Range
	Kind  VariableKind
	IsSet bool
}

type Variable struct {
	Name       string
	Definition Range
	Value      string
	References []VariableReference
	ScopeDepth int
}

type Scope struct {
	Depth       int
	StartLine   int
	Vars        map[string]*Variable
	HasSetlocal bool
}

type Diagnostic struct {
	Line     int
	Col      int
	EndLine  int
	EndCol   int
	Severity string
	Message  string
	Source   string
	Code     string
}

type Result struct {
	Labels      map[string]*Label
	Variables   map[string]*Variable
	Scopes      []Scope
	Diagnostics []Diagnostic
}

func Analyze(nodes []parser.Node, tokens []lexer.Item) *Result {
	r := &Result{
		Labels:    make(map[string]*Label),
		Variables: make(map[string]*Variable),
		Scopes:    []Scope{{Depth: 0, Vars: make(map[string]*Variable)}},
	}
	for _, node := range nodes {
		r.analyzeNode(node, 0)
	}
	r.collectVarRefsFromTokens(tokens)
	r.generateDiagnostics()
	return r
}

func (r *Result) currentScope() *Scope {
	return &r.Scopes[len(r.Scopes)-1]
}

func (r *Result) pushScope() {
	newDepth := len(r.Scopes)
	r.Scopes = append(r.Scopes, Scope{
		Depth: newDepth,
		Vars:  make(map[string]*Variable),
	})
}

func (r *Result) pushScopeWithLine(line int) {
	newDepth := len(r.Scopes)
	r.Scopes = append(r.Scopes, Scope{
		Depth:     newDepth,
		StartLine: line,
		Vars:      make(map[string]*Variable),
	})
}

func (r *Result) popScope() {
	if len(r.Scopes) > 1 {
		r.Scopes = r.Scopes[:len(r.Scopes)-1]
	}
}

func (r *Result) analyzeNode(node parser.Node, scopeDepth int) {
	switch n := node.(type) {
	case *parser.LabelNode:
		name := strings.ToUpper(n.Name)
		label := &Label{
			Name: name,
			Definition: Range{
				Start: Position{Line: n.Line, Col: n.Col},
				End:   Position{Line: n.EndLine, Col: n.EndCol},
			},
		}
		r.Labels[name] = label

	case *parser.SimpleCommand:
		r.analyzeCommand(n, scopeDepth)

	case *parser.IfNode:
		r.analyzeNode(n.Then, scopeDepth)
		if n.Else != nil {
			r.analyzeNode(n.Else, scopeDepth)
		}

	case *parser.ForNode:
		r.pushScope()
		varName := strings.ToUpper(n.Variable)
		v := &Variable{
			Name: varName,
			Definition: Range{
				Start: Position{Line: n.VarLine, Col: n.VarCol},
				End:   Position{Line: n.VarLine, Col: n.VarCol + 1},
			},
			ScopeDepth: len(r.Scopes) - 1,
		}
		r.Variables[varName] = v
		r.currentScope().Vars[varName] = v
		if n.Do != nil {
			r.analyzeNode(n.Do, len(r.Scopes)-1)
		}
		r.popScope()

	case *parser.Block:
		for _, child := range n.Body {
			r.analyzeNode(child, scopeDepth)
		}

	case *parser.BinaryNode:
		r.analyzeNode(n.Left, scopeDepth)
		r.analyzeNode(n.Right, scopeDepth)

	case *parser.PipeNode:
		r.analyzeNode(n.Left, scopeDepth)
		r.analyzeNode(n.Right, scopeDepth)
	}
}

func (r *Result) analyzeCommand(cmd *parser.SimpleCommand, scopeDepth int) {
	name := strings.ToUpper(cmd.Name)
	args := cmd.Args

	switch name {
	case "SET", "SETLOCAL", "ENDLOCAL", "GOTO", "CALL":
	default:
		return
	}

	if name == "SET" && len(args) > 0 {
		r.analyzeSet(cmd, args, scopeDepth)
		return
	}

	if name == "SETLOCAL" {
		r.currentScope().HasSetlocal = true
		r.pushScopeWithLine(cmd.Line)
		return
	}

	if name == "ENDLOCAL" {
		r.popScope()
		return
	}

	if name == "GOTO" && len(args) > 0 {
		labelName := strings.ToUpper(strings.TrimSpace(args[0]))
		if labelName == ":EOF" {
			return
		}
		labelName = strings.TrimPrefix(labelName, ":")
		if label, ok := r.Labels[labelName]; ok {
			label.References = append(label.References, Range{
				Start: Position{Line: cmd.Line, Col: cmd.Col},
				End:   Position{Line: cmd.EndLine, Col: cmd.EndCol},
			})
		} else {
			r.Labels[labelName] = &Label{
				Name: labelName,
				Definition: Range{
					Start: Position{Line: -1, Col: -1},
					End:   Position{Line: -1, Col: -1},
				},
				References: []Range{{
					Start: Position{Line: cmd.Line, Col: cmd.Col},
					End:   Position{Line: cmd.EndLine, Col: cmd.EndCol},
				}},
			}
		}
		return
	}

	if name == "CALL" && len(args) > 0 {
		arg0 := strings.TrimSpace(args[0])
		if after, ok := strings.CutPrefix(arg0, ":"); ok {
			labelName := strings.ToUpper(after)
			if label, ok := r.Labels[labelName]; ok {
				label.References = append(label.References, Range{
					Start: Position{Line: cmd.Line, Col: cmd.Col},
					End:   Position{Line: cmd.EndLine, Col: cmd.EndCol},
				})
			} else {
				r.Labels[labelName] = &Label{
					Name: labelName,
					Definition: Range{
						Start: Position{Line: -1, Col: -1},
						End:   Position{Line: -1, Col: -1},
					},
					References: []Range{{
						Start: Position{Line: cmd.Line, Col: cmd.Col},
						End:   Position{Line: cmd.EndLine, Col: cmd.EndCol},
					}},
				}
			}
		}
	}
}

func (r *Result) analyzeSet(cmd *parser.SimpleCommand, args []string, scopeDepth int) {
	fullArg := strings.Join(args, "")
	if strings.HasPrefix(strings.ToUpper(fullArg), "/A ") {
		fullArg = fullArg[3:]
	}

	before, after, hasEq := strings.Cut(fullArg, "=")
	if !hasEq {
		return
	}
	varName := strings.ToUpper(strings.TrimSpace(before))
	value := after

	v := &Variable{
		Name: varName,
		Definition: Range{
			Start: Position{Line: cmd.Line, Col: cmd.Col},
			End:   Position{Line: cmd.EndLine, Col: cmd.EndCol},
		},
		Value:      value,
		ScopeDepth: scopeDepth,
	}
	if existing, ok := r.Variables[varName]; ok {
		existing.Definition = v.Definition
		existing.Value = value
	} else {
		r.Variables[varName] = v
		r.currentScope().Vars[varName] = v
	}
}

func (r *Result) collectVarRefsFromTokens(tokens []lexer.Item) {
	for _, t := range tokens {
		switch t.Type {
		case lexer.TokenVariable, lexer.TokenDelayedExpansion:
			varName := string(t.Value)
			varName = strings.TrimPrefix(varName, "%")
			varName = strings.TrimSuffix(varName, "%")
			varName = strings.TrimPrefix(varName, "!")
			varName = strings.TrimSuffix(varName, "!")
			varName = strings.ToUpper(varName)

			kind := VarKindNormal
			if t.Type == lexer.TokenDelayedExpansion {
				kind = VarKindDelayed
			}

			ref := VariableReference{
				Range: Range{
					Start: Position{Line: t.Line, Col: t.Col},
					End:   Position{Line: t.EndLine, Col: t.EndCol},
				},
				Kind: kind,
			}

			if v, ok := r.Variables[varName]; ok {
				v.References = append(v.References, ref)
			} else {
				v = &Variable{
					Name:       varName,
					ScopeDepth: 0,
					References: []VariableReference{ref},
				}
				r.Variables[varName] = v
			}

		case lexer.TokenForVar:
			varName := strings.ToUpper(string(t.Value))
			varName = strings.TrimPrefix(varName, "%%")
			if len(varName) > 1 {
				varName = varName[:1]
			}
			ref := VariableReference{
				Range: Range{
					Start: Position{Line: t.Line, Col: t.Col},
					End:   Position{Line: t.EndLine, Col: t.EndCol},
				},
				Kind: VarKindFor,
			}
			if v, ok := r.Variables[varName]; ok {
				v.References = append(v.References, ref)
			}
		}
	}
}

func (r *Result) GetLabelAt(line, col int) *Label {
	for _, l := range r.Labels {
		if l.Definition.Start.Line == line && col >= l.Definition.Start.Col && col < l.Definition.End.Col {
			return l
		}
		for _, ref := range l.References {
			if ref.Start.Line == line && col >= ref.Start.Col && col < ref.End.Col {
				return l
			}
		}
	}
	return nil
}

func (r *Result) GetVariableAt(line, col int) *Variable {
	for _, v := range r.Variables {
		for _, ref := range v.References {
			if ref.Range.Start.Line == line && col >= ref.Range.Start.Col && col < ref.Range.End.Col {
				return v
			}
		}
	}
	return nil
}

func (r *Result) generateDiagnostics() {
	for name, label := range r.Labels {
		if label.Definition.Start.Line < 0 {
			for _, ref := range label.References {
				r.Diagnostics = append(r.Diagnostics, Diagnostic{
					Line:     ref.Start.Line,
					Col:      ref.Start.Col,
					EndLine:  ref.End.Line,
					EndCol:   ref.End.Col,
					Severity: "error",
					Message:  "undefined label: " + name,
					Source:   "msbatch",
					Code:     "undefined-label",
				})
			}
		}
	}

	for i := 1; i < len(r.Scopes); i++ {
		scope := r.Scopes[i]
		r.Diagnostics = append(r.Diagnostics, Diagnostic{
			Line:     scope.StartLine,
			Col:      0,
			EndLine:  scope.StartLine,
			EndCol:   1,
			Severity: "warning",
			Message:  "missing ENDLOCAL for SETLOCAL",
			Source:   "msbatch",
			Code:     "missing-endlocal",
		})
	}
}
