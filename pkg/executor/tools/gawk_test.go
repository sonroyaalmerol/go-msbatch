package tools

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func newTestProcessor(input string) *processor.Processor {
	env := processor.NewEnvironment(false)
	p := processor.New(env, nil, nil)
	p.Stdin = strings.NewReader(input)
	return p
}

func TestGawkBasic(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple print",
			args:     []string{"{ print $1 }"},
			input:    "hello world\nfoo bar",
			expected: "hello\nfoo\n",
		},
		{
			name:     "field separator",
			args:     []string{"-F", ":", "{ print $1 }"},
			input:    "a:b:c\nd:e:f",
			expected: "a\nd\n",
		},
		{
			name:     "variable assignment",
			args:     []string{"-v", "x=42", "BEGIN { print x }"},
			input:    "",
			expected: "42\n",
		},
		{
			name:     "BEGIN and END blocks",
			args:     []string{"BEGIN { print \"start\" } END { print \"end\" }"},
			input:    "line1\nline2",
			expected: "start\nend\n",
		},
		{
			name:     "NR and NF",
			args:     []string{"{ print NR, NF }"},
			input:    "a b c\nd e",
			expected: "1 3\n2 2\n",
		},
		{
			name:     "sum columns",
			args:     []string{"{ sum += $1 } END { print sum }"},
			input:    "1\n2\n3\n4\n5",
			expected: "15\n",
		},
		{
			name:     "help flag",
			args:     []string{"--help"},
			input:    "",
			expected: "Usage: gawk",
		},
		{
			name:     "version flag",
			args:     []string{"--version"},
			input:    "",
			expected: "GNU Awk 5.4.0",
		},
		{
			name:     "long option field separator",
			args:     []string{"--field-separator=,", "{ print $2 }"},
			input:    "a,b,c\nd,e,f",
			expected: "b\ne\n",
		},
		{
			name:     "multiple variable assignments",
			args:     []string{"-v", "a=1", "-v", "b=2", "BEGIN { print a+b }"},
			input:    "",
			expected: "3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			p := newTestProcessor(tt.input)
			p.Stdout = &stdout
			p.Stderr = &stderr

			cmd := &parser.SimpleCommand{
				Name: "gawk",
				Args: tt.args,
			}

			err := Gawk(p, cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Gawk() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !strings.Contains(stdout.String(), tt.expected) {
				t.Errorf("Gawk() = %q, want containing %q", stdout.String(), tt.expected)
			}
		})
	}
}

func TestGawkBitwiseFunctions(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		input    string
		expected string
	}{
		{
			name:     "and function",
			args:     []string{"BEGIN { print and(5, 3) }"},
			input:    "",
			expected: "1\n",
		},
		{
			name:     "or function",
			args:     []string{"BEGIN { print or(1, 2) }"},
			input:    "",
			expected: "3\n",
		},
		{
			name:     "xor function",
			args:     []string{"BEGIN { print xor(5, 3) }"},
			input:    "",
			expected: "6\n",
		},
		{
			name:     "compl function",
			args:     []string{"BEGIN { print compl(0) }"},
			input:    "",
			expected: "-1\n",
		},
		{
			name:     "lshift function",
			args:     []string{"BEGIN { print lshift(1, 4) }"},
			input:    "",
			expected: "16\n",
		},
		{
			name:     "rshift function",
			args:     []string{"BEGIN { print rshift(16, 2) }"},
			input:    "",
			expected: "4\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			p := newTestProcessor(tt.input)
			p.Stdout = &stdout
			p.Stderr = &stderr

			cmd := &parser.SimpleCommand{
				Name: "gawk",
				Args: tt.args,
			}

			_ = Gawk(p, cmd)

			if !strings.Contains(stdout.String(), tt.expected) {
				t.Errorf("Gawk() = %q, want containing %q", stdout.String(), tt.expected)
			}
		})
	}
}

func TestGawkStrftime(t *testing.T) {
	var stdout, stderr bytes.Buffer

	p := newTestProcessor("")
	p.Stdout = &stdout
	p.Stderr = &stderr

	cmd := &parser.SimpleCommand{
		Name: "gawk",
		Args: []string{`BEGIN { print strftime("%Y", systime()) }`},
	}

	_ = Gawk(p, cmd)

	output := stdout.String()
	if len(output) < 4 {
		t.Errorf("strftime: expected year in output, got %q", output)
	}
}

func TestGawkSystime(t *testing.T) {
	var stdout, stderr bytes.Buffer

	p := newTestProcessor("")
	p.Stdout = &stdout
	p.Stderr = &stderr

	cmd := &parser.SimpleCommand{
		Name: "gawk",
		Args: []string{"BEGIN { print systime() }"},
	}

	_ = Gawk(p, cmd)

	if stderr.Len() > 0 {
		t.Errorf("systime: unexpected error %q", stderr.String())
	}
	if stdout.Len() == 0 {
		t.Errorf("systime: expected output, got empty")
	}
}

func TestGawkMktime(t *testing.T) {
	var stdout, stderr bytes.Buffer

	p := newTestProcessor("")
	p.Stdout = &stdout
	p.Stderr = &stderr

	cmd := &parser.SimpleCommand{
		Name: "gawk",
		Args: []string{`BEGIN { ts = mktime("2024 01 01 00 00 00"); print ts }`},
	}

	_ = Gawk(p, cmd)

	output := stdout.String()
	if !strings.Contains(output, "1704067200") {
		t.Errorf("mktime: expected 1704067200, got %q", output)
	}
}

func TestGawkStrtonum(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "hex number",
			args:     []string{`BEGIN { print strtonum("0x10") }`},
			expected: "16\n",
		},
		{
			name:     "octal number",
			args:     []string{`BEGIN { print strtonum("010") }`},
			expected: "8\n",
		},
		{
			name:     "decimal number",
			args:     []string{`BEGIN { print strtonum("42") }`},
			expected: "42\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			p := newTestProcessor("")
			p.Stdout = &stdout
			p.Stderr = &stderr

			cmd := &parser.SimpleCommand{
				Name: "gawk",
				Args: tt.args,
			}

			_ = Gawk(p, cmd)

			if !strings.Contains(stdout.String(), tt.expected) {
				t.Errorf("strtonum: expected %q, got %q", tt.expected, stdout.String())
			}
		})
	}
}

func TestGawkFileInput(t *testing.T) {
	var stdout, stderr bytes.Buffer

	p := newTestProcessor("")
	p.Stdout = &stdout
	p.Stderr = &stderr

	cmd := &parser.SimpleCommand{
		Name: "gawk",
		Args: []string{"{ print $0 }", "/nonexistent/file.txt"},
	}

	_ = Gawk(p, cmd)

	if stderr.Len() == 0 {
		t.Error("expected error message for nonexistent file")
	}
}

func TestGawkNoProgram(t *testing.T) {
	var stdout, stderr bytes.Buffer

	p := newTestProcessor("")
	p.Stdout = &stdout
	p.Stderr = &stderr

	cmd := &parser.SimpleCommand{
		Name: "gawk",
		Args: []string{},
	}

	_ = Gawk(p, cmd)

	if stderr.Len() == 0 {
		t.Error("expected error message for missing program")
	}
}

func TestGawkCSVMode(t *testing.T) {
	var stdout, stderr bytes.Buffer

	p := newTestProcessor("name,age\nJohn,30\nJane,25")
	p.Stdout = &stdout
	p.Stderr = &stderr

	cmd := &parser.SimpleCommand{
		Name: "gawk",
		Args: []string{"-k", "{ print $1 }"},
	}

	_ = Gawk(p, cmd)

	output := stdout.String()
	if !strings.Contains(output, "name") {
		t.Errorf("CSV mode: expected output containing 'name', got %q", output)
	}
}

func TestGawkSandboxMode(t *testing.T) {
	var stdout, stderr bytes.Buffer

	p := newTestProcessor("test")
	p.Stdout = &stdout
	p.Stderr = &stderr

	cmd := &parser.SimpleCommand{
		Name: "gawk",
		Args: []string{"-S", "{ print }"},
	}

	err := Gawk(p, cmd)
	if err != nil {
		t.Errorf("Sandbox mode caused error: %v", err)
	}
}

func TestGawkCharsMode(t *testing.T) {
	var stdout, stderr bytes.Buffer

	p := newTestProcessor("hello")
	p.Stdout = &stdout
	p.Stderr = &stderr

	cmd := &parser.SimpleCommand{
		Name: "gawk",
		Args: []string{"-b", "{ print length($0) }"},
	}

	_ = Gawk(p, cmd)

	output := stdout.String()
	if !strings.Contains(output, "5") {
		t.Errorf("Chars mode: expected '5', got %q", output)
	}
}

func TestGawkStopParser(t *testing.T) {
	var stdout, stderr bytes.Buffer

	p := newTestProcessor("test line")
	p.Stdout = &stdout
	p.Stderr = &stderr

	cmd := &parser.SimpleCommand{
		Name: "gawk",
		Args: []string{"--", "{ print }"},
	}

	_ = Gawk(p, cmd)

	output := stdout.String()
	if !strings.Contains(output, "test line") {
		t.Errorf("expected output containing 'test line', got %q", output)
	}
}

func TestGawkProgramFromFile(t *testing.T) {
	var stdout, stderr bytes.Buffer

	p := newTestProcessor("hello world")
	p.Stdout = &stdout
	p.Stderr = &stderr

	cmd := &parser.SimpleCommand{
		Name: "gawk",
		Args: []string{"-f", "/nonexistent/program.awk"},
	}

	_ = Gawk(p, cmd)

	if stderr.Len() == 0 {
		t.Error("expected error for nonexistent program file")
	}
}

func TestParseGawkArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantField string
		wantSrc   string
		wantVars  []string
		wantFiles []string
		wantCSV   bool
	}{
		{
			name:      "short field separator",
			args:      []string{"-F", ",", "{ print }"},
			wantField: ",",
			wantSrc:   "{ print }",
		},
		{
			name:      "inline field separator",
			args:      []string{"-F,", "{ print }"},
			wantField: ",",
			wantSrc:   "{ print }",
		},
		{
			name:      "long field separator",
			args:      []string{"--field-separator=|", "{ print }"},
			wantField: "|",
			wantSrc:   "{ print }",
		},
		{
			name:     "variable assignment",
			args:     []string{"-v", "x=10", "{ print x }"},
			wantVars: []string{"x=10"},
			wantSrc:  "{ print x }",
		},
		{
			name:     "inline variable",
			args:     []string{"-vx=10", "{ print x }"},
			wantVars: []string{"x=10"},
			wantSrc:  "{ print x }",
		},
		{
			name:      "source option",
			args:      []string{"--source={ print }", "file.txt"},
			wantSrc:   "{ print }",
			wantFiles: []string{"file.txt"},
		},
		{
			name:    "csv mode",
			args:    []string{"-k", "{ print }"},
			wantCSV: true,
			wantSrc: "{ print }",
		},
		{
			name:      "stop parser",
			args:      []string{"--", "{ print }", "file.txt"},
			wantSrc:   "{ print }",
			wantFiles: []string{"file.txt"},
		},
		{
			name:      "input files",
			args:      []string{"{ print }", "a.txt", "b.txt"},
			wantSrc:   "{ print }",
			wantFiles: []string{"a.txt", "b.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseGawkArgs(tt.args)
			if err != nil {
				t.Fatalf("parseGawkArgs error: %v", err)
			}

			if cfg.fieldSep != tt.wantField {
				t.Errorf("fieldSep = %q, want %q", cfg.fieldSep, tt.wantField)
			}
			if cfg.programSrc != tt.wantSrc {
				t.Errorf("programSrc = %q, want %q", cfg.programSrc, tt.wantSrc)
			}
			if cfg.csvMode != tt.wantCSV {
				t.Errorf("csvMode = %v, want %v", cfg.csvMode, tt.wantCSV)
			}

			if len(tt.wantVars) > 0 {
				for i, v := range tt.wantVars {
					if i >= len(cfg.varAssigns) || cfg.varAssigns[i] != v {
						t.Errorf("varAssigns[%d] = %q, want %q", i, cfg.varAssigns[i], v)
					}
				}
			}

			if len(tt.wantFiles) > 0 {
				for i, f := range tt.wantFiles {
					if i >= len(cfg.inputFiles) || cfg.inputFiles[i] != f {
						t.Errorf("inputFiles[%d] = %q, want %q", i, cfg.inputFiles[i], f)
					}
				}
			}
		})
	}
}
