package tools

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// newProc returns a Processor with buffered stdout/stderr and an empty stdin.
// The returned buffers let tests inspect what was written.
func newProc(stdin io.Reader) (*processor.Processor, *bytes.Buffer, *bytes.Buffer) {
	env := processor.NewEnvironment(false)
	noop := processor.CommandExecutorFunc(func(*processor.Processor, *parser.SimpleCommand) error { return nil })
	proc := processor.New(env, nil, noop)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	proc.Stdout = out
	proc.Stderr = errOut
	proc.Stdin = stdin // nil means non-interactive; individual tools guard against it
	return proc, out, errOut
}

// cmd builds a SimpleCommand.
func cmd(name string, args ...string) *parser.SimpleCommand {
	return &parser.SimpleCommand{Name: name, Args: args}
}

// errorLevel reads the ERRORLEVEL variable from the processor.
func errorLevel(p *processor.Processor) string {
	v, _ := p.Env.Get("ERRORLEVEL")
	return v
}

// writeFile creates a file with the given content inside dir.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
