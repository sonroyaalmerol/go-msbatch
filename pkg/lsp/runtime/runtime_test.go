package runtime

import (
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

func parseBatch(t *testing.T, src string) []parser.Node {
	t.Helper()
	l := lexer.New(src)
	var tokens []lexer.Item
	for {
		tok := l.NextItem()
		if tok.Type == lexer.TokenEOF || (tok.Type == 0 && len(tok.Value) == 0) {
			break
		}
		tokens = append(tokens, tok)
	}
	p := parser.NewFromTokens(tokens)
	return p.Parse()
}

func TestPhase1_BasicSetTracking(t *testing.T) {
	tests := []struct {
		name           string
		src            string
		varName        string
		wantValue      string
		wantLine       int
		wantSourceType string
	}{
		{
			name:           "simple SET",
			src:            "SET VAR=hello",
			varName:        "VAR",
			wantValue:      "hello",
			wantLine:       0,
			wantSourceType: "SET",
		},
		{
			name:           "SET with spaces in value",
			src:            "SET MSG=Hello World",
			varName:        "MSG",
			wantValue:      "Hello World",
			wantLine:       0,
			wantSourceType: "SET",
		},
		{
			name:           "SET with quoted value",
			src:            `SET PATH="C:\Program Files"`,
			varName:        "PATH",
			wantValue:      `"C:\Program Files"`,
			wantLine:       0,
			wantSourceType: "SET",
		},
		{
			name:           "multiple SETs - last wins for single value",
			src:            "SET VAR=first\nSET VAR=second",
			varName:        "VAR",
			wantValue:      "second",
			wantLine:       1,
			wantSourceType: "SET",
		},
		{
			name:           "SET empty value",
			src:            "SET EMPTY=",
			varName:        "EMPTY",
			wantValue:      "",
			wantLine:       0,
			wantSourceType: "SET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := parseBatch(t, tt.src)
			runtime := NewMiniRuntime(nodes)
			result := runtime.Execute()

			v := result.GetVariable(tt.varName)
			if v == nil {
				t.Fatalf("GetVariable(%q) returned nil", tt.varName)
			}

			if len(v.Values) == 0 {
				t.Fatalf("variable %q has no values", tt.varName)
			}

			got := v.Values[0]
			if got.Value != tt.wantValue {
				t.Errorf("value = %q, want %q", got.Value, tt.wantValue)
			}
			if got.SourceLine != tt.wantLine {
				t.Errorf("source line = %d, want %d", got.SourceLine, tt.wantLine)
			}
			if got.SourceType != tt.wantSourceType {
				t.Errorf("source type = %q, want %q", got.SourceType, tt.wantSourceType)
			}
		})
	}
}

func TestPhase1_SetWithVariableExpansion(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		varName   string
		wantValue string
	}{
		{
			name:      "SET with variable reference",
			src:       "SET A=hello\nSET B=%A%",
			varName:   "B",
			wantValue: "hello",
		},
		{
			name:      "SET with chained variable reference",
			src:       "SET A=world\nSET B=%A%\nSET C=%B%",
			varName:   "C",
			wantValue: "world",
		},
		{
			name:      "SET with undefined variable - kept as literal",
			src:       "SET RESULT=%UNDEFINED%",
			varName:   "RESULT",
			wantValue: "%UNDEFINED%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := parseBatch(t, tt.src)
			runtime := NewMiniRuntime(nodes)
			result := runtime.Execute()

			v := result.GetVariable(tt.varName)
			if v == nil {
				t.Fatalf("GetVariable(%q) returned nil", tt.varName)
			}

			if len(v.Values) == 0 {
				t.Fatalf("variable %q has no values", tt.varName)
			}

			if v.Values[0].Value != tt.wantValue {
				t.Errorf("value = %q, want %q", v.Values[0].Value, tt.wantValue)
			}
		})
	}
}

func TestPhase1_StateAtLine(t *testing.T) {
	src := `SET A=first
SET B=second
SET C=third`

	tests := []struct {
		name     string
		line     int
		wantVars map[string]string
		dontWant []string
	}{
		{
			name:     "before any SET",
			line:     -1,
			wantVars: map[string]string{},
		},
		{
			name:     "after first SET",
			line:     0,
			wantVars: map[string]string{"A": "first"},
			dontWant: []string{"B", "C"},
		},
		{
			name:     "after second SET",
			line:     1,
			wantVars: map[string]string{"A": "first", "B": "second"},
			dontWant: []string{"C"},
		},
		{
			name:     "after all SETs",
			line:     2,
			wantVars: map[string]string{"A": "first", "B": "second", "C": "third"},
		},
	}

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := result.GetStateAtLine(tt.line)
			if state == nil {
				t.Fatalf("GetStateAtLine(%d) returned nil", tt.line)
			}

			for varName, wantVal := range tt.wantVars {
				v := state.GetVariable(varName)
				if v == nil {
					t.Errorf("expected variable %q to exist", varName)
					continue
				}
				if len(v.Values) == 0 {
					t.Errorf("variable %q has no values", varName)
					continue
				}
				if v.Values[0].Value != wantVal {
					t.Errorf("variable %q = %q, want %q", varName, v.Values[0].Value, wantVal)
				}
			}

			for _, varName := range tt.dontWant {
				if v := state.GetVariable(varName); v != nil {
					t.Errorf("expected variable %q to not exist yet", varName)
				}
			}
		})
	}
}

func TestPhase1_CaseInsensitiveVariables(t *testing.T) {
	src := `set myvar=hello
SET MYVAR=world
Set MyVar=test`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	v := result.GetVariable("MYVAR")
	if v == nil {
		t.Fatal("GetVariable(MYVAR) returned nil")
	}

	if len(v.Values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(v.Values))
	}

	if v.Values[0].Value != "test" {
		t.Errorf("value = %q, want %q", v.Values[0].Value, "test")
	}
}
