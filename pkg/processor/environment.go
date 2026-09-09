// Package processor implements the CMD.EXE batch-processing phases on top of
// the parsed AST.
package processor

import (
	"maps"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
)

// Environment stores CMD variables and controls expansion behaviour.
type Environment struct {
	mu               sync.RWMutex
	vars             map[string]string
	delayedExpansion bool // phase 5 enabled flag
	batchMode        bool // true = batch file, false = command-line mode
	stack            []envFrame
}

// NewEnvironment creates an Environment pre-populated from the OS environment.
func NewEnvironment(batchMode bool) *Environment {
	e := &Environment{
		vars:      make(map[string]string),
		batchMode: batchMode,
	}
	for _, kv := range os.Environ() {
		if before, after, ok := strings.Cut(kv, "="); ok {
			e.vars[strings.ToUpper(before)] = after
		}
	}
	e.SetErrorLevel(0)
	return e
}

// SetErrorLevel updates the ERRORLEVEL variable.
func (e *Environment) SetErrorLevel(code int) {
	e.Set("ERRORLEVEL", strconv.Itoa(code))
}

// BuiltinVarNames returns the set of variable names that exist in a freshly
// initialised CMD environment: every OS environment variable (from os.Environ)
// plus CMD-specific builtins such as ERRORLEVEL. Static-analysis tools (e.g.
// the LSP) use this to suppress false-positive "not defined" diagnostics for
// variables that are always present at runtime.
func BuiltinVarNames() map[string]bool {
	snap := NewEnvironment(false).Snapshot()
	m := make(map[string]bool, len(snap))
	for k := range snap {
		m[k] = true
	}
	return m
}

// NewEmptyEnvironment creates an Environment with no OS variables.
// Useful for deterministic tests.
func NewEmptyEnvironment(batchMode bool) *Environment {
	e := &Environment{
		vars:      make(map[string]string),
		batchMode: batchMode,
	}
	e.SetErrorLevel(0)
	return e
}

// Set stores a variable.  Names are normalised to upper-case per CMD semantics.
func (e *Environment) Set(name, value string) {
	e.mu.Lock()
	e.vars[strings.ToUpper(name)] = value
	e.mu.Unlock()
}

// Get retrieves a variable.  Returns ("", false) when the variable is absent.
func (e *Environment) Get(name string) (string, bool) {
	e.mu.RLock()
	v, ok := e.vars[strings.ToUpper(name)]
	e.mu.RUnlock()
	return v, ok
}

// Delete removes a variable.
func (e *Environment) Delete(name string) {
	e.mu.Lock()
	delete(e.vars, strings.ToUpper(name))
	e.mu.Unlock()
}

// SetDelayedExpansion enables or disables phase-5 delayed expansion.
func (e *Environment) SetDelayedExpansion(enabled bool) {
	e.mu.Lock()
	e.delayedExpansion = enabled
	e.mu.Unlock()
}

// DelayedExpansion reports whether delayed expansion is enabled.
func (e *Environment) DelayedExpansion() bool {
	e.mu.RLock()
	v := e.delayedExpansion
	e.mu.RUnlock()
	return v
}

// SetBatchMode changes whether percent-number expressions expand as batch arguments.
func (e *Environment) SetBatchMode(enabled bool) {
	e.mu.Lock()
	e.batchMode = enabled
	e.mu.Unlock()
}

// BatchMode reports whether the environment is in batch-file mode.
func (e *Environment) BatchMode() bool {
	e.mu.RLock()
	v := e.batchMode
	e.mu.RUnlock()
	return v
}

// Snapshot returns a shallow copy of all current variables.
func (e *Environment) Snapshot() map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m := make(map[string]string, len(e.vars))
	maps.Copy(m, e.vars)
	return m
}

// StackDepth returns the number of saved environment frames (i.e. how many
// unmatched SETLOCAL calls are currently open).
func (e *Environment) StackDepth() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.stack)
}

type envFrame struct {
	vars             map[string]string
	delayedExpansion bool
	cwd              string
	driveDirs        map[byte]string
}

// Push saves the environment and drive-local directories for SETLOCAL. pi:keep
func (e *Environment) Push() {
	cwd, _ := os.Getwd()
	frame := envFrame{
		delayedExpansion: e.DelayedExpansion(),
		cwd:              cwd,
		driveDirs:        pathutil.SnapshotDriveDirs(),
	}
	e.mu.Lock()
	frame.vars = maps.Clone(e.vars)
	e.stack = append(e.stack, frame)
	e.mu.Unlock()
}

// Pop restores the state saved by SETLOCAL, including the current directory. pi:keep
func (e *Environment) Pop() {
	e.mu.Lock()
	if len(e.stack) == 0 {
		e.mu.Unlock()
		return
	}
	frame := e.stack[len(e.stack)-1]
	e.stack = e.stack[:len(e.stack)-1]
	e.vars = frame.vars
	e.delayedExpansion = frame.delayedExpansion
	e.mu.Unlock()

	pathutil.RestoreDriveDirs(frame.driveDirs)
	_ = pathutil.Chdir(frame.cwd)
}
