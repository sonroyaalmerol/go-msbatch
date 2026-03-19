package analyzer

import (
	"strings"
)

func NewVariableSymbol(name string, loc Location) *Symbol {
	return &Symbol{
		Name:       strings.ToUpper(name),
		Kind:       SymbolVariable,
		Definition: loc,
		References: []Reference{},
	}
}

func NewForVarSymbol(name string, loc Location) *Symbol {
	return &Symbol{
		Name:       name,
		Kind:       SymbolForVar,
		Definition: loc,
		References: []Reference{},
	}
}

func NewLabelSymbol(name string, loc Location) *Symbol {
	return &Symbol{
		Name:       strings.ToLower(name),
		Kind:       SymbolLabel,
		Definition: loc,
		References: []Reference{},
	}
}

func NewPositionalSymbol(index int) *Symbol {
	return &Symbol{
		Name:      string(rune('0' + index)),
		Kind:      SymbolPositionalArg,
		IsBuiltin: true,
	}
}

type SymbolTable struct {
	Global  *Scope
	Labels  map[string]*Symbol
	Vars    map[string]*Symbol
	ForVars map[string]*Symbol
}

func NewSymbolTable(uri string) *SymbolTable {
	return &SymbolTable{
		Global: &Scope{
			Kind:      ScopeGlobal,
			Symbols:   make(map[string]*Symbol),
			StartLine: 0,
			EndLine:   -1,
			URI:       uri,
		},
		Labels:  make(map[string]*Symbol),
		Vars:    make(map[string]*Symbol),
		ForVars: make(map[string]*Symbol),
	}
}

func (st *SymbolTable) DefineVariable(name string, loc Location) *Symbol {
	sym := NewVariableSymbol(name, loc)
	sym.Scope = st.Global
	st.Global.Define(sym)
	st.Vars[strings.ToUpper(name)] = sym
	return sym
}

func (st *SymbolTable) DefineForVar(name string, loc Location, scope *Scope) *Symbol {
	sym := NewForVarSymbol(name, loc)
	sym.Scope = scope
	scope.Define(sym)
	key := name
	st.ForVars[key] = sym
	return sym
}

func (st *SymbolTable) DefineLabel(name string, loc Location) *Symbol {
	name = strings.ToLower(name)
	sym := NewLabelSymbol(name, loc)
	sym.Scope = st.Global
	st.Global.Define(sym)
	st.Labels[name] = sym
	return sym
}

func (st *SymbolTable) ResolveVariable(name string, line int) *Symbol {
	name = strings.ToUpper(name)
	scope := st.findScopeAt(line)
	if scope != nil {
		if sym := scope.Resolve(name, SymbolVariable); sym != nil {
			return sym
		}
	}
	return st.Vars[name]
}

func (st *SymbolTable) ResolveForVar(name string, line int) *Symbol {
	scope := st.findScopeAt(line)
	if scope != nil {
		if sym := scope.Resolve(name, SymbolForVar); sym != nil {
			return sym
		}
	}
	return st.ForVars[name]
}

func (st *SymbolTable) ResolveLabel(name string) *Symbol {
	return st.Labels[strings.ToLower(name)]
}

func (st *SymbolTable) findScopeAt(line int) *Scope {
	return findDeepestScope(st.Global, line)
}

func findDeepestScope(root *Scope, line int) *Scope {
	if !root.Contains(line) {
		return nil
	}
	for _, child := range root.Children {
		if child.Contains(line) {
			return findDeepestScope(child, line)
		}
	}
	return root
}

func (st *SymbolTable) AllSymbols(fn func(sym *Symbol)) {
	st.Global.ForEachSymbol(fn)
}
