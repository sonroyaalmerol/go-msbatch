package processor_test

import (
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func TestEnvironmentSetGet(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("FOO", "bar")
	v, ok := env.Get("FOO")
	if !ok {
		t.Fatal("expected FOO to be set")
	}
	if v != "bar" {
		t.Errorf("expected bar, got %q", v)
	}
}

// TestEnvironmentCaseInsensitiveName verifies CMD normalises names to upper-case.
func TestEnvironmentCaseInsensitiveName(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("MyVar", "hello")
	v, ok := env.Get("myvar")
	if !ok {
		t.Fatal("expected myvar to be found")
	}
	if v != "hello" {
		t.Errorf("expected hello, got %q", v)
	}
}

func TestEnvironmentDelete(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("X", "1")
	env.Delete("X")
	_, ok := env.Get("X")
	if ok {
		t.Error("expected X to be deleted")
	}
}

func TestEnvironmentMissingVar(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	_, ok := env.Get("NOTHERE")
	if ok {
		t.Error("expected missing var to return false")
	}
}

func TestEnvironmentDelayedExpansion(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	if env.DelayedExpansion() {
		t.Error("expected delayed expansion off by default")
	}
	env.SetDelayedExpansion(true)
	if !env.DelayedExpansion() {
		t.Error("expected delayed expansion on after SetDelayedExpansion(true)")
	}
}

func TestEnvironmentBatchMode(t *testing.T) {
	batch := processor.NewEmptyEnvironment(true)
	if !batch.BatchMode() {
		t.Error("expected batch mode true")
	}
	cmdline := processor.NewEmptyEnvironment(false)
	if cmdline.BatchMode() {
		t.Error("expected batch mode false")
	}
}

func TestEnvironmentSnapshot(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set("A", "1")
	env.Set("B", "2")
	snap := env.Snapshot()
	if snap["A"] != "1" {
		t.Errorf("expected A=1 in snapshot, got %q", snap["A"])
	}
	if snap["B"] != "2" {
		t.Errorf("expected B=2 in snapshot, got %q", snap["B"])
	}
	// mutations after snapshot don't affect it
	env.Set("A", "changed")
	if snap["A"] != "1" {
		t.Error("snapshot should not reflect post-snapshot mutations")
	}
}

func TestEnvironmentSpaces(t *testing.T) {
	env := processor.NewEmptyEnvironment(true)
	env.Set(" VAR", "val")
	val, ok := env.Get(" VAR")
	if !ok || val != "val" {
		t.Errorf("expected val, got %q", val)
	}
	_, ok = env.Get("VAR")
	if ok {
		t.Error("expected VAR to not be found if set as ' VAR'")
	}
}
