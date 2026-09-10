// Package msbatch embeds the batch interpreter in Go programs.
package msbatch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

var cwdMu sync.Mutex

// Options configures one batch-file execution.
type Options struct {
	Dir         string
	Environment map[string]string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

// Result describes the final interpreter state.
type Result struct {
	ExitCode    int
	Environment map[string]string
}

// Command is the expanded command passed to a custom handler.
type Command struct {
	Name         string
	Arguments    []string
	RawArguments []string
}

// Session exposes mutable state to a custom command handler.
type Session struct {
	processor *processor.Processor
}

// Stdin returns the batch input stream.
func (s *Session) Stdin() io.Reader { return s.processor.Stdin }

// Stdout returns the batch output stream.
func (s *Session) Stdout() io.Writer { return s.processor.Stdout }

// Stderr returns the batch error stream.
func (s *Session) Stderr() io.Writer { return s.processor.Stderr }

// LookupEnv returns one case-insensitive batch environment variable.
func (s *Session) LookupEnv(name string) (string, bool) { return s.processor.Env.Get(name) }

// SetEnv sets one case-insensitive batch environment variable.
func (s *Session) SetEnv(name, value string) { s.processor.Env.Set(name, value) }

// SetExitCode sets both ERRORLEVEL and the final process exit code.
func (s *Session) SetExitCode(code int) { s.processor.SetErrorLevel(code) }

// Handler implements a custom in-process batch command.
type Handler func(context.Context, *Session, Command) error

// Interpreter executes batch files with CMD-compatible built-ins.
// Executions are serialized because CMD directory state is process-global.
type Interpreter struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// New returns an interpreter using the standard command registry.
func New() *Interpreter { return &Interpreter{handlers: make(map[string]Handler)} }

// Handle registers or replaces a case-insensitive custom command.
func (i *Interpreter) Handle(name string, handler Handler) {
	if strings.TrimSpace(name) == "" || handler == nil {
		panic("msbatch: command name and handler are required")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.handlers == nil {
		i.handlers = make(map[string]Handler)
	}
	i.handlers[strings.ToLower(name)] = handler
}

// RunFile executes filename once and returns its exit code and final environment.
func (i *Interpreter) RunFile(ctx context.Context, filename string, args []string, options Options) (result Result, err error) {
	if ctx == nil {
		return result, errors.New("msbatch: nil context")
	}

	cwdMu.Lock()
	defer cwdMu.Unlock()

	oldDir, err := os.Getwd()
	if err != nil {
		return result, fmt.Errorf("get working directory: %w", err)
	}
	oldDriveDirs := pathutil.SnapshotDriveDirs()
	defer func() {
		restoreErr := pathutil.Chdir(oldDir)
		pathutil.RestoreDriveDirs(oldDriveDirs)
		if restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore working directory: %w", restoreErr))
		}
	}()

	if options.Dir != "" {
		if err := pathutil.Chdir(options.Dir); err != nil {
			return result, fmt.Errorf("change working directory: %w", err)
		}
	}

	filename, err = filepath.Abs(filename)
	if err != nil {
		return result, fmt.Errorf("resolve batch file: %w", err)
	}
	content, err := os.ReadFile(filename)
	if err != nil {
		return result, fmt.Errorf("read batch file: %w", err)
	}

	env, baseEnv := newEnvironment(options.Environment)
	registry := executor.New()
	for name, handler := range i.snapshotHandlers() {
		h := handler
		registry.HandleFunc(name, func(p *processor.Processor, cmd *parser.SimpleCommand) error {
			return h(p.Context, &Session{processor: p}, Command{
				Name:         cmd.Name,
				Arguments:    append([]string(nil), cmd.Args...),
				RawArguments: append([]string(nil), cmd.RawArgs...),
			})
		})
	}

	procArgs := append([]string{filename}, args...)
	proc := processor.New(env, procArgs, registry)
	proc.Context = ctx
	proc.BaseEnv = baseEnv
	proc.Stdin = readerOr(options.Stdin, os.Stdin)
	proc.RawStdout = writerOr(options.Stdout, os.Stdout)
	proc.RawStderr = writerOr(options.Stderr, os.Stderr)
	proc.Stdout = processor.NewCRLFWriter(proc.RawStdout)
	proc.Stderr = processor.NewCRLFWriter(proc.RawStderr)
	proc.Console = proc.Stdout
	proc.SetCurrentFile(filename)

	raw := string(content)
	if strings.HasPrefix(raw, "#!") {
		if newline := strings.IndexByte(raw, '\n'); newline >= 0 {
			raw = raw[newline+1:]
		} else {
			raw = ""
		}
	}

	execErr := proc.Execute(processor.ParseExpanded(processor.Phase0ReadLine(raw)))
	result = Result{ExitCode: proc.ExitCode, Environment: proc.Env.Snapshot()}
	if ctxErr := ctx.Err(); ctxErr != nil {
		execErr = errors.Join(execErr, ctxErr)
	}
	return result, execErr
}

func (i *Interpreter) snapshotHandlers() map[string]Handler {
	if i == nil {
		return nil
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	handlers := make(map[string]Handler, len(i.handlers))
	maps.Copy(handlers, i.handlers)
	return handlers
}

func newEnvironment(values map[string]string) (*processor.Environment, []string) {
	if values == nil {
		return processor.NewEnvironment(true), os.Environ()
	}
	env := processor.NewEmptyEnvironment(true)
	base := make([]string, 0, len(values))
	for name, value := range values {
		env.Set(name, value)
		base = append(base, name+"="+value)
	}
	return env, base
}

func readerOr(value, fallback io.Reader) io.Reader {
	if value != nil {
		return value
	}
	return fallback
}

func writerOr(value, fallback io.Writer) io.Writer {
	if value != nil {
		return value
	}
	return fallback
}
