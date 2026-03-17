package processor_test

import (
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func TestPhase5MissingVarBatch(t *testing.T) {
	env := processor.NewEmptyEnvironment(true) // BatchMode = true
	env.SetDelayedExpansion(true)
	got := processor.Phase5DelayedExpand("!MISSING!", env)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestPhase5MissingVarCmd(t *testing.T) {
	env := processor.NewEmptyEnvironment(false) // BatchMode = false
	env.SetDelayedExpansion(true)
	got := processor.Phase5DelayedExpand("!MISSING!", env)
	if got != "!MISSING!" {
		t.Errorf("expected !MISSING!, got %q", got)
	}
}

func TestPhase5Modifiers(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("STR", "hello world")
	env.SetDelayedExpansion(true)

	// Slicing
	got := processor.Phase5DelayedExpand("!STR:~0,5!", env)
	if got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}

	// Substitution
	got = processor.Phase5DelayedExpand("!STR:hello=hi!", env)
	if got != "hi world" {
		t.Errorf("expected 'hi world', got %q", got)
	}
}

func TestRecursiveDelayed(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("DATAFLT", "C:\\Data")
	env.Set("DATAGRV1TA", "%DATAFLT%\\TA")
	env.SetDelayedExpansion(true)

	got := processor.Phase5DelayedExpand("!DATAGRV1TA!", env)
	if got != "C:\\Data\\TA" {
		t.Errorf("expected C:\\Data\\TA, got %q", got)
	}
}
