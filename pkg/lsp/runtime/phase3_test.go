package runtime

import (
	"testing"
)

func TestPhase3_SubstringModifier(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		varName    string
		wantValues []string
	}{
		{
			name:       "simple substring :~start",
			src:        "SET STR=Hello World\nSET RESULT=%STR:~6%",
			varName:    "RESULT",
			wantValues: []string{"World"},
		},
		{
			name:       "substring :~start,length",
			src:        "SET STR=Hello World\nSET RESULT=%STR:~0,5%",
			varName:    "RESULT",
			wantValues: []string{"Hello"},
		},
		{
			name:       "negative start offset",
			src:        "SET STR=Hello World\nSET RESULT=%STR:~-5%",
			varName:    "RESULT",
			wantValues: []string{"World"},
		},
		{
			name:       "negative length",
			src:        "SET STR=Hello World\nSET RESULT=%STR:~0,-6%",
			varName:    "RESULT",
			wantValues: []string{"Hello"},
		},
		{
			name:       "negative start and length",
			src:        "SET STR=Hello World\nSET RESULT=%STR:~-5,3%",
			varName:    "RESULT",
			wantValues: []string{"Wor"},
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

			found := false
			for _, pv := range v.Values {
				for _, want := range tt.wantValues {
					if pv.Value == want {
						found = true
						break
					}
				}
				if found {
					break
				}
			}

			if !found {
				t.Errorf("value = %v, want one of %v", v.Values, tt.wantValues)
			}
		})
	}
}

func TestPhase3_SubstitutionModifier(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		varName    string
		wantValues []string
	}{
		{
			name:       "simple substitution",
			src:        "SET STR=Hello World\nSET RESULT=%STR:World=Universe%",
			varName:    "RESULT",
			wantValues: []string{"Hello Universe"},
		},
		{
			name:       "substitution with empty replacement",
			src:        "SET STR=Hello World\nSET RESULT=%STR:World=%",
			varName:    "RESULT",
			wantValues: []string{"Hello "},
		},
		{
			name:       "substitution case insensitive",
			src:        "SET STR=HELLO world\nSET RESULT=%STR:hello=hi%",
			varName:    "RESULT",
			wantValues: []string{"hi world"},
		},
		{
			name:       "substitution with asterisk prefix",
			src:        "SET STR=one two three two four\nSET RESULT=%STR:*two=X%",
			varName:    "RESULT",
			wantValues: []string{"X three two four"},
		},
		{
			name:       "multiple substitutions",
			src:        "SET STR=a b a b a\nSET RESULT=%STR:a=X%",
			varName:    "RESULT",
			wantValues: []string{"X b X b X"},
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

			found := false
			for _, pv := range v.Values {
				for _, want := range tt.wantValues {
					if pv.Value == want {
						found = true
						break
					}
				}
				if found {
					break
				}
			}

			if !found {
				t.Errorf("value = %v, want one of %v", v.Values, tt.wantValues)
			}
		})
	}
}

func TestPhase3_DynamicExpansionAtHover(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		line         int
		col          int
		wantExpanded string
		wantPossible []string
	}{
		{
			name:         "hover on %VAR% shows expanded value",
			src:          "SET MSG=Hello\nECHO %MSG%",
			line:         1,
			col:          5,
			wantExpanded: "Hello",
			wantPossible: []string{"Hello"},
		},
		{
			name:         "hover on %VAR:~0,3% shows expansion result",
			src:          "SET MSG=Hello World\nECHO %MSG:~0,5%",
			line:         1,
			col:          5,
			wantExpanded: "Hello",
			wantPossible: []string{"Hello World"},
		},
		{
			name:         "hover on variable with substitution",
			src:          "SET PATH=test_path\nECHO %PATH:test=replace%",
			line:         1,
			col:          5,
			wantExpanded: "replace_path",
			wantPossible: []string{"test_path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := parseBatch(t, tt.src)
			runtime := NewMiniRuntime(nodes)
			result := runtime.Execute()

			state := result.GetStateAtLine(tt.line)
			if state == nil {
				t.Fatalf("GetStateAtLine(%d) returned nil", tt.line)
			}

			expanded := result.GetExpandedValueAt(tt.line, tt.col)
			if expanded != tt.wantExpanded {
				t.Errorf("expanded = %q, want %q", expanded, tt.wantExpanded)
			}
		})
	}
}

func TestPhase3_DynamicVariables(t *testing.T) {
	src := `ECHO %TIME%
SET CUSTOM=value
ECHO %CUSTOM%`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	expanded := result.GetExpandedValueAt(0, 6)
	if expanded == "" || expanded == "%TIME%" {
		t.Errorf("expected TIME to be expanded, got %q", expanded)
	}

	expanded = result.GetExpandedValueAt(2, 6)
	if expanded != "value" {
		t.Errorf("expanded = %q, want 'value'", expanded)
	}
}

func TestPhase3_NestedExpansion(t *testing.T) {
	tests := []struct {
		name       string
		src        string
		varName    string
		wantValues []string
	}{
		{
			name:       "nested variable expansion",
			src:        "SET A=Hello\nSET B=%A% World\nSET C=%B%!",
			varName:    "C",
			wantValues: []string{"Hello World!"},
		},
		{
			name:       "variable in modifier",
			src:        "SET STR=Hello World\nSET START=0\nSET LEN=5\nSET RESULT=%STR:~0,5%",
			varName:    "RESULT",
			wantValues: []string{"Hello"},
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

			found := false
			for _, pv := range v.Values {
				for _, want := range tt.wantValues {
					if pv.Value == want {
						found = true
						break
					}
				}
				if found {
					break
				}
			}

			if !found {
				t.Errorf("value = %v, want one of %v", v.Values, tt.wantValues)
			}
		})
	}
}
