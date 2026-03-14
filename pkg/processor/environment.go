// Package processor implements the CMD.EXE batch-processing phases on top of
// the parsed AST.
package processor

import (
	"maps"
	"os"
	"strings"
	"sync"
)

// Environment stores CMD variables and controls expansion behaviour.
type Environment struct {
	mu               sync.RWMutex
	vars             map[string]string
	delayedExpansion bool // phase 5 enabled flag
	batchMode        bool // true = batch file, false = command-line mode
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
	e.vars["ERRORLEVEL"] = "0"
	return e
}

// NewEmptyEnvironment creates an Environment with no OS variables.
// Useful for deterministic tests.
func NewEmptyEnvironment(batchMode bool) *Environment {
	e := &Environment{
		vars:      make(map[string]string),
		batchMode: batchMode,
	}
	e.vars["ERRORLEVEL"] = "0"
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

// BatchMode reports whether the environment is in batch-file mode.
func (e *Environment) BatchMode() bool {
	return e.batchMode
}

// Snapshot returns a shallow copy of all current variables.
func (e *Environment) Snapshot() map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m := make(map[string]string, len(e.vars))
	maps.Copy(m, e.vars)
	return m
}
