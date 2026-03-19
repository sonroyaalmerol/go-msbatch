package analyzer

import (
	"strings"
)

type ScopeKind int

const (
	ScopeGlobal ScopeKind = iota
	ScopeSetlocal
	ScopeFor
	ScopeBlock
)

type Scope struct {
	Kind      ScopeKind
	Parent    *Scope
	Children  []*Scope
	Symbols   map[string]*Symbol
	StartLine int
	EndLine   int
	URI       string
}

func NewScope(kind ScopeKind, parent *Scope) *Scope {
	return &Scope{
		Kind:    kind,
		Parent:  parent,
		Symbols: make(map[string]*Symbol),
	}
}

func (s *Scope) Define(sym *Symbol) {
	sym.Scope = s
	key := canonicalName(sym.Name, sym.Kind)
	s.Symbols[key] = sym
	if s.Parent != nil && sym.Kind == SymbolVariable {
		if existing := s.Parent.Resolve(sym.Name, SymbolVariable); existing == nil {
		}
	}
}

func (s *Scope) Resolve(name string, kind SymbolKind) *Symbol {
	key := canonicalName(name, kind)
	if sym, ok := s.Symbols[key]; ok {
		return sym
	}
	if s.Parent != nil {
		return s.Parent.Resolve(name, kind)
	}
	return nil
}

func (s *Scope) ResolveLabel(name string) *Symbol {
	return s.ResolveInGlobal(name, SymbolLabel)
}

func (s *Scope) ResolveInGlobal(name string, kind SymbolKind) *Symbol {
	root := s
	for root.Parent != nil {
		root = root.Parent
	}
	key := canonicalName(name, kind)
	return root.Symbols[key]
}

func (s *Scope) ForEachSymbol(fn func(sym *Symbol)) {
	for _, sym := range s.Symbols {
		fn(sym)
	}
	for _, child := range s.Children {
		child.ForEachSymbol(fn)
	}
}

func (s *Scope) Contains(line int) bool {
	if s.StartLine < 0 || s.EndLine < 0 {
		return true
	}
	return line >= s.StartLine && line <= s.EndLine
}

func (s *Scope) AddChild(child *Scope) {
	child.Parent = s
	s.Children = append(s.Children, child)
}

func canonicalName(name string, kind SymbolKind) string {
	if kind == SymbolForVar {
		return name
	}
	return strings.ToUpper(name)
}
