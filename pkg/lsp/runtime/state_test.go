package runtime

import (
	"testing"
)

func TestPossibleValue(t *testing.T) {
	tests := []struct {
		name   string
		pv     PossibleValue
		String string
	}{
		{
			name: "basic value",
			pv: PossibleValue{
				Value:      "hello",
				SourceLine: 5,
				SourceType: "SET",
			},
			String: "SET@5: hello",
		},
		{
			name: "FOR iteration value",
			pv: PossibleValue{
				Value:      "file.txt",
				SourceLine: 3,
				SourceType: "FOR_ITER",
			},
			String: "FOR_ITER@3: file.txt",
		},
		{
			name: "empty value",
			pv: PossibleValue{
				Value:      "",
				SourceLine: 1,
				SourceType: "SET",
			},
			String: "SET@1: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pv.String(); got != tt.String {
				t.Errorf("String() = %q, want %q", got, tt.String)
			}
		})
	}
}

func TestVarState(t *testing.T) {
	t.Run("add value", func(t *testing.T) {
		v := &VarState{Name: "TEST"}
		v.AddValue(PossibleValue{Value: "first", SourceLine: 1, SourceType: "SET"})
		v.AddValue(PossibleValue{Value: "second", SourceLine: 2, SourceType: "SET"})

		if len(v.Values) != 2 {
			t.Errorf("expected 2 values, got %d", len(v.Values))
		}
	})

	t.Run("dedup same value different line", func(t *testing.T) {
		v := &VarState{Name: "TEST"}
		v.AddValue(PossibleValue{Value: "same", SourceLine: 1, SourceType: "SET"})
		v.AddValue(PossibleValue{Value: "same", SourceLine: 2, SourceType: "SET"})

		if len(v.Values) != 2 {
			t.Errorf("expected 2 values (same value, different sources), got %d", len(v.Values))
		}
	})

	t.Run("last value", func(t *testing.T) {
		v := &VarState{
			Name: "TEST",
			Values: []PossibleValue{
				{Value: "first", SourceLine: 1},
				{Value: "second", SourceLine: 2},
			},
		}

		last := v.LastValue()
		if last == nil || last.Value != "second" {
			t.Errorf("LastValue() = %v, want 'second'", last)
		}
	})

	t.Run("last value empty", func(t *testing.T) {
		v := &VarState{Name: "TEST"}
		if v.LastValue() != nil {
			t.Error("LastValue() on empty VarState should return nil")
		}
	})

	t.Run("unique values", func(t *testing.T) {
		v := &VarState{
			Name: "TEST",
			Values: []PossibleValue{
				{Value: "a", SourceLine: 1},
				{Value: "b", SourceLine: 2},
				{Value: "a", SourceLine: 3},
			},
		}

		unique := v.UniqueValues()
		if len(unique) != 2 {
			t.Errorf("expected 2 unique values, got %d: %v", len(unique), unique)
		}
	})
}

func TestState(t *testing.T) {
	t.Run("fork preserves variables", func(t *testing.T) {
		original := NewState()
		original.SetVar("A", PossibleValue{Value: "original", SourceLine: 0})

		forked := original.Fork()
		forked.SetVar("B", PossibleValue{Value: "forked_only", SourceLine: 1})

		if v := forked.GetVariable("A"); v == nil || v.Values[0].Value != "original" {
			t.Error("forked state should inherit variable A")
		}
		if v := original.GetVariable("B"); v != nil {
			t.Error("original state should not have variable B from fork")
		}
	})

	t.Run("merge combines values", func(t *testing.T) {
		s1 := NewState()
		s1.SetVar("A", PossibleValue{Value: "branch1", SourceLine: 1})

		s2 := NewState()
		s2.SetVar("A", PossibleValue{Value: "branch2", SourceLine: 2})

		merged := s1.Merge(s2)
		v := merged.GetVariable("A")
		if v == nil {
			t.Fatal("merged state missing variable A")
		}

		if len(v.Values) != 2 {
			t.Errorf("expected 2 values after merge, got %d", len(v.Values))
		}

		gotValues := make(map[string]bool)
		for _, pv := range v.Values {
			gotValues[pv.Value] = true
		}
		if !gotValues["branch1"] || !gotValues["branch2"] {
			t.Errorf("merge missing values, got: %v", v.Values)
		}
	})

	t.Run("merge with nil", func(t *testing.T) {
		s := NewState()
		s.SetVar("A", PossibleValue{Value: "test", SourceLine: 0})

		merged := s.Merge(nil)
		v := merged.GetVariable("A")
		if v == nil || len(v.Values) != 1 || v.Values[0].Value != "test" {
			t.Error("merging with nil should return original state")
		}
	})

	t.Run("scope depth", func(t *testing.T) {
		s := NewState()
		if s.ScopeDepth() != 0 {
			t.Errorf("initial scope depth = %d, want 0", s.ScopeDepth())
		}

		s.SetScopeDepth(2)
		if s.ScopeDepth() != 2 {
			t.Errorf("scope depth = %d, want 2", s.ScopeDepth())
		}
	})

	t.Run("delayed expansion toggle", func(t *testing.T) {
		s := NewState()
		if s.DelayedExpansion() {
			t.Error("delayed expansion should be false initially")
		}

		s.SetDelayedExpansion(true)
		if !s.DelayedExpansion() {
			t.Error("delayed expansion should be true after setting")
		}

		forked := s.Fork()
		if !forked.DelayedExpansion() {
			t.Error("forked state should inherit delayed expansion setting")
		}
	})
}

func TestStateMergeComplex(t *testing.T) {
	s1 := NewState()
	s1.SetVar("A", PossibleValue{Value: "a1", SourceLine: 1})
	s1.SetVar("B", PossibleValue{Value: "b1", SourceLine: 1})

	s2 := NewState()
	s2.SetVar("A", PossibleValue{Value: "a2", SourceLine: 2})

	s3 := NewState()
	s3.SetVar("C", PossibleValue{Value: "c3", SourceLine: 3})

	merged := s1.Merge(s2).Merge(s3)

	tests := []struct {
		varName string
		values  []string
	}{
		{"A", []string{"a1", "a2"}},
		{"B", []string{"b1"}},
		{"C", []string{"c3"}},
	}

	for _, tt := range tests {
		t.Run(tt.varName, func(t *testing.T) {
			v := merged.GetVariable(tt.varName)
			if v == nil {
				t.Fatalf("variable %q not found", tt.varName)
			}

			gotValues := make(map[string]bool)
			for _, pv := range v.Values {
				gotValues[pv.Value] = true
			}

			for _, want := range tt.values {
				if !gotValues[want] {
					t.Errorf("variable %q missing value %q, got %v", tt.varName, want, v.Values)
				}
			}
		})
	}
}
