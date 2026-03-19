package runtime

import (
	"testing"
)

func TestForInfo(t *testing.T) {
	tests := []struct {
		name           string
		info           ForInfo
		wantIterDesc   string
		wantHasPattern bool
	}{
		{
			name: "files iterator",
			info: ForInfo{
				VarName:    "f",
				Line:       1,
				IterType:   "files",
				IterSource: "*.txt",
			},
			wantIterDesc:   "files matching *.txt",
			wantHasPattern: true,
		},
		{
			name: "range iterator",
			info: ForInfo{
				VarName:    "i",
				Line:       5,
				IterType:   "range",
				IterSource: "(1,1,10)",
			},
			wantIterDesc:   "range (1,1,10)",
			wantHasPattern: false,
		},
		{
			name: "directory iterator",
			info: ForInfo{
				VarName:    "d",
				Line:       3,
				IterType:   "dir",
				IterSource: "*",
			},
			wantIterDesc:   "directories matching *",
			wantHasPattern: true,
		},
		{
			name: "file parse iterator",
			info: ForInfo{
				VarName:    "a",
				Line:       2,
				IterType:   "f_parse",
				IterSource: "output.txt",
			},
			wantIterDesc:   "lines from output.txt",
			wantHasPattern: false,
		},
		{
			name: "recursive iterator",
			info: ForInfo{
				VarName:    "r",
				Line:       7,
				IterType:   "recursive",
				IterSource: "*.go",
			},
			wantIterDesc:   "recursively matching *.go",
			wantHasPattern: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := tt.info.IterationDescription()
			if desc != tt.wantIterDesc {
				t.Errorf("IterationDescription() = %q, want %q", desc, tt.wantIterDesc)
			}

			hasPattern := tt.info.HasPattern()
			if hasPattern != tt.wantHasPattern {
				t.Errorf("HasPattern() = %v, want %v", hasPattern, tt.wantHasPattern)
			}
		})
	}
}

func TestForInfoString(t *testing.T) {
	info := ForInfo{
		VarName:    "f",
		Line:       1,
		IterType:   "files",
		IterSource: "*.txt",
	}

	want := "%%f (files): *.txt"
	if got := info.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestRuntimeResult(t *testing.T) {
	t.Run("get variable", func(t *testing.T) {
		result := &RuntimeResult{
			Variables: map[string]*VarState{
				"TEST": {
					Name: "TEST",
					Values: []PossibleValue{
						{Value: "value1", SourceLine: 1},
					},
				},
			},
		}

		v := result.GetVariable("TEST")
		if v == nil {
			t.Fatal("GetVariable returned nil")
		}
		if v.Name != "TEST" {
			t.Errorf("variable name = %q, want 'TEST'", v.Name)
		}
	})

	t.Run("get variable case insensitive", func(t *testing.T) {
		result := &RuntimeResult{
			Variables: map[string]*VarState{
				"TEST": {Name: "TEST"},
			},
		}

		v := result.GetVariable("test")
		if v == nil {
			t.Error("GetVariable should be case insensitive")
		}

		v = result.GetVariable("TeSt")
		if v == nil {
			t.Error("GetVariable should be case insensitive")
		}
	})

	t.Run("get for var", func(t *testing.T) {
		result := &RuntimeResult{
			ForVars: map[string]ForInfo{
				"i": {
					VarName:    "i",
					IterType:   "range",
					IterSource: "(1,1,10)",
				},
			},
		}

		info := result.GetForVar("i")
		if info == nil {
			t.Fatal("GetForVar returned nil")
		}
		if info.IterType != "range" {
			t.Errorf("IterType = %q, want 'range'", info.IterType)
		}
	})

	t.Run("get final state", func(t *testing.T) {
		result := &RuntimeResult{
			FinalState: NewState(),
		}
		result.FinalState.SetVar("FINAL", PossibleValue{Value: "end", SourceLine: 10})

		state := result.GetFinalState()
		if state == nil {
			t.Fatal("GetFinalState returned nil")
		}

		v := state.GetVariable("FINAL")
		if v == nil {
			t.Fatal("variable FINAL not found in final state")
		}
		if v.Values[0].Value != "end" {
			t.Errorf("FINAL = %q, want 'end'", v.Values[0].Value)
		}
	})

	t.Run("get state at line", func(t *testing.T) {
		s0 := NewState()
		s0.SetVar("A", PossibleValue{Value: "line0", SourceLine: 0})

		s5 := NewState()
		s5.SetVar("A", PossibleValue{Value: "line5", SourceLine: 5})

		result := &RuntimeResult{
			LineStates: map[int]*State{
				0: s0,
				5: s5,
			},
		}

		state := result.GetStateAtLine(0)
		if state == nil {
			t.Fatal("GetStateAtLine(0) returned nil")
		}
		v := state.GetVariable("A")
		if v == nil || v.Values[0].Value != "line0" {
			t.Errorf("at line 0: A = %v, want 'line0'", v)
		}

		state = result.GetStateAtLine(5)
		if state == nil {
			t.Fatal("GetStateAtLine(5) returned nil")
		}
		v = state.GetVariable("A")
		if v == nil || v.Values[0].Value != "line5" {
			t.Errorf("at line 5: A = %v, want 'line5'", v)
		}
	})

	t.Run("get state at line - nearest before", func(t *testing.T) {
		s0 := NewState()
		s0.SetVar("A", PossibleValue{Value: "line0", SourceLine: 0})

		s10 := NewState()
		s10.SetVar("A", PossibleValue{Value: "line10", SourceLine: 10})

		result := &RuntimeResult{
			LineStates: map[int]*State{
				0:  s0,
				10: s10,
			},
		}

		state := result.GetStateAtLine(5)
		if state == nil {
			t.Fatal("GetStateAtLine(5) returned nil")
		}
		v := state.GetVariable("A")
		if v == nil || v.Values[0].Value != "line0" {
			t.Errorf("at line 5 (should use line 0 state): A = %v, want 'line0'", v)
		}
	})
}

func TestHoverInfo(t *testing.T) {
	t.Run("basic format", func(t *testing.T) {
		info := &HoverInfo{
			VariableName: "TEST",
			PossibleValues: []PossibleValue{
				{Value: "value1", SourceLine: 1, SourceType: "SET"},
				{Value: "value2", SourceLine: 5, SourceType: "SET"},
			},
		}

		formatted := info.Format()
		if formatted == "" {
			t.Error("Format() returned empty string")
		}
	})

	t.Run("format with modifier", func(t *testing.T) {
		info := &HoverInfo{
			VariableName:    "PATH",
			BaseValue:       "/usr/local/bin",
			ModifierApplied: "~0,5",
			ExpandedValue:   "/usr/",
		}

		formatted := info.Format()
		if formatted == "" {
			t.Error("Format() returned empty string")
		}
	})

	t.Run("format for var", func(t *testing.T) {
		info := &HoverInfo{
			IsForVar:        true,
			ForVarName:      "f",
			IterationSource: "*.txt",
		}

		formatted := info.Format()
		if formatted == "" {
			t.Error("Format() returned empty string")
		}
	})

	t.Run("format delayed expansion", func(t *testing.T) {
		info := &HoverInfo{
			VariableName:       "COUNT",
			IsDelayedExpansion: true,
			PossibleValues: []PossibleValue{
				{Value: "1", SourceLine: 5},
				{Value: "2", SourceLine: 5},
				{Value: "3", SourceLine: 5},
			},
		}

		formatted := info.Format()
		if formatted == "" {
			t.Error("Format() returned empty string")
		}
	})
}
