package runtime

import (
	"testing"
)

func TestPhase5_GotoForward(t *testing.T) {
	src := `SET A=before
GOTO skip
SET A=skipped
:skip
SET A=after`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	v := result.GetVariable("A")
	if v == nil {
		t.Fatal("variable A not found")
	}

	gotValues := make(map[string]bool)
	for _, pv := range v.Values {
		gotValues[pv.Value] = true
	}

	if gotValues["skipped"] {
		t.Error("value 'skipped' should not be present after GOTO")
	}

	if !gotValues["after"] {
		t.Errorf("missing expected value 'after', got %v", v.Values)
	}
}

func TestPhase5_GotoBackwardLoop(t *testing.T) {
	src := `SET COUNT=0
:loop
SET /A COUNT+=1
IF %COUNT% LSS 3 GOTO loop
SET DONE=yes`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	v := result.GetVariable("COUNT")
	if v == nil {
		t.Fatal("variable COUNT not found")
	}

	if len(v.Values) < 3 {
		t.Errorf("expected at least 3 values for COUNT (loop iterations), got %d: %v", len(v.Values), v.Values)
	}

	vDone := result.GetVariable("DONE")
	if vDone == nil {
		t.Fatal("variable DONE not found")
	}

	gotValues := make(map[string]bool)
	for _, pv := range vDone.Values {
		gotValues[pv.Value] = true
	}
	if !gotValues["yes"] {
		t.Errorf("expected DONE=yes, got %v", vDone.Values)
	}
}

func TestPhase5_GotoEof(t *testing.T) {
	src := `SET A=start
IF "%1"=="" GOTO :EOF
SET A=has_arg
ECHO %A%`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	v := result.GetVariable("A")
	if v == nil {
		t.Fatal("variable A not found")
	}

	gotValues := make(map[string]bool)
	for _, pv := range v.Values {
		gotValues[pv.Value] = true
	}

	if !gotValues["start"] {
		t.Errorf("missing expected value 'start', got %v", v.Values)
	}
}

func TestPhase5_GotoUndefined(t *testing.T) {
	src := `SET A=start
GOTO nonexistent
SET A=unreachable
:done
SET A=finished`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	v := result.GetVariable("A")
	if v == nil {
		t.Fatal("variable A not found")
	}

	if len(v.Values) == 0 {
		t.Fatal("variable A has no values")
	}

	gotValues := make(map[string]bool)
	for _, pv := range v.Values {
		gotValues[pv.Value] = true
	}

	if gotValues["unreachable"] {
		t.Error("value 'unreachable' should not be present due to undefined label")
	}
}

func TestPhase5_CallLabel(t *testing.T) {
	src := `SET A=start
CALL :sub
SET A=after_call
GOTO :EOF

:sub
SET A=in_subroutine
EXIT /B 0`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	v := result.GetVariable("A")
	if v == nil {
		t.Fatal("variable A not found")
	}

	gotValues := make(map[string]bool)
	for _, pv := range v.Values {
		gotValues[pv.Value] = true
	}

	expectedValues := []string{"in_subroutine", "after_call"}
	for _, want := range expectedValues {
		if !gotValues[want] {
			t.Errorf("missing expected value %q, got %v", want, v.Values)
		}
	}
}

func TestPhase5_ComplexControlFlow(t *testing.T) {
	src := `SET MODE=default
IF "%1"=="install" GOTO install
IF "%1"=="uninstall" GOTO uninstall
GOTO usage

:install
SET MODE=installing
GOTO end

:uninstall
SET MODE=uninstalling
GOTO end

:usage
SET MODE=show_usage

:end
SET DONE=yes`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	v := result.GetVariable("MODE")
	if v == nil {
		t.Fatal("variable MODE not found")
	}

	gotValues := make(map[string]bool)
	for _, pv := range v.Values {
		gotValues[pv.Value] = true
	}

	expectedValues := []string{"installing", "uninstalling", "show_usage"}
	for _, want := range expectedValues {
		if !gotValues[want] {
			t.Errorf("missing expected value %q, got %v", want, v.Values)
		}
	}

	vDone := result.GetVariable("DONE")
	if vDone == nil {
		t.Fatal("variable DONE not found")
	}
	if len(vDone.Values) == 0 || vDone.Values[0].Value != "yes" {
		t.Errorf("DONE = %v, want 'yes'", vDone.Values)
	}
}

func TestPhase5_InfiniteLoopDetection(t *testing.T) {
	src := `:forever
SET A=looping
GOTO forever
SET A=unreachable`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	v := result.GetVariable("A")
	if v == nil {
		t.Fatal("variable A not found")
	}

	if len(v.Values) == 0 {
		t.Fatal("variable A has no values")
	}

	gotValues := make(map[string]bool)
	for _, pv := range v.Values {
		gotValues[pv.Value] = true
	}

	if gotValues["unreachable"] {
		t.Error("value 'unreachable' should not be present in infinite loop")
	}

	if !gotValues["looping"] {
		t.Errorf("missing expected value 'looping', got %v", v.Values)
	}
}

func TestPhase5_StateAtLineWithGoto(t *testing.T) {
	src := `SET A=init
GOTO skip
SET B=hidden
:skip
SET C=visible`

	nodes := parseBatch(t, src)
	runtime := NewMiniRuntime(nodes)
	result := runtime.Execute()

	stateAtLine1 := result.GetStateAtLine(1)
	if stateAtLine1 == nil {
		t.Fatal("GetStateAtLine(1) returned nil")
	}

	vB := stateAtLine1.GetVariable("B")
	if vB != nil && len(vB.Values) > 0 && vB.Values[0].Value != "" {
		t.Error("variable B should not be defined at line 1 (before GOTO target)")
	}

	stateAtLine4 := result.GetStateAtLine(4)
	if stateAtLine4 == nil {
		t.Fatal("GetStateAtLine(4) returned nil")
	}

	vC := stateAtLine4.GetVariable("C")
	if vC == nil || len(vC.Values) == 0 {
		t.Fatal("variable C should be defined at line 4")
	}
	if vC.Values[0].Value != "visible" {
		t.Errorf("C = %q, want 'visible'", vC.Values[0].Value)
	}
}
