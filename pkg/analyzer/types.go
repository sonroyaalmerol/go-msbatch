package analyzer

type SymbolKind int

const (
	SymbolVariable SymbolKind = iota
	SymbolForVar
	SymbolLabel
	SymbolPositionalArg
)

func (k SymbolKind) String() string {
	switch k {
	case SymbolVariable:
		return "variable"
	case SymbolForVar:
		return "for-variable"
	case SymbolLabel:
		return "label"
	case SymbolPositionalArg:
		return "positional"
	default:
		return "unknown"
	}
}

type Location struct {
	URI    string
	Line   int
	Col    int
	EndCol int
}

type ReferenceKind int

const (
	RefDefinition ReferenceKind = iota
	RefRead
	RefWrite
	RefGoto
	RefCall
)

type Reference struct {
	Location Location
	Kind     ReferenceKind
}

type Symbol struct {
	Name          string
	Kind          SymbolKind
	Definition    Location
	References    []Reference
	Scope         *Scope
	InferredValue string
	IsBuiltin     bool
}

func (s *Symbol) AddRef(loc Location, kind ReferenceKind) {
	s.References = append(s.References, Reference{Location: loc, Kind: kind})
}

func (s *Symbol) RefCount() int {
	count := 0
	for _, r := range s.References {
		if r.Kind != RefDefinition {
			count++
		}
	}
	return count
}

type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
	SeverityHint
)

type Diagnostic struct {
	Location Location
	Message  string
	Severity Severity
}

type CompletionKind int

const (
	CompletionCommand CompletionKind = iota
	CompletionVariable
	CompletionForVar
	CompletionLabel
	CompletionFile
)

type Completion struct {
	Label         string
	Kind          CompletionKind
	Detail        string
	Documentation string
}

type Hover struct {
	Contents string
	Range    Location
}

type TextEdit struct {
	Location Location
	NewText  string
}
