package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/logging"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func newProcessor(env *processor.Environment, args []string, exec processor.CommandExecutor, debugMode processor.DebugMode) *processor.Processor {
	proc := processor.New(env, args, exec)
	proc.Logger = logging.NewLoggerFromEnv()
	if debugMode != processor.DebugOff {
		proc.Debugger.SetMode(debugMode)
	}
	return proc
}

func main() {
	exeName := strings.ToLower(filepath.Base(os.Args[0]))
	exeName = strings.TrimSuffix(exeName, ".exe")
	if exeName != "msbatch" {
		reg := executor.New()
		if _, ok := reg.NamesMap()[exeName]; ok {
			runAsTool(exeName, os.Args[1:])
			return
		}
	}

	args := os.Args[1:]
	traceMode := logging.TraceOff
	traceOutput := io.Writer(nil)
	debugMode := processor.DebugOff

	if envTrace := os.Getenv("MSBATCH_TRACE"); envTrace != "" {
		switch strings.ToLower(envTrace) {
		case "1", "on", "true":
			traceMode = logging.TraceOn
		case "2", "verbose":
			traceMode = logging.TraceVerbose
		case "0", "off", "false":
			traceMode = logging.TraceOff
		}
	}

	if envTraceFile := os.Getenv("MSBATCH_TRACE_FILE"); envTraceFile != "" {
		f, err := os.Create(envTraceFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating trace file: %v\n", err)
			os.Exit(1)
		}
		traceOutput = f
		if traceMode == logging.TraceOff {
			traceMode = logging.TraceOn
		}
	}

	if envDebug := os.Getenv("MSBATCH_BREAKPOINTS"); envDebug != "" {
		switch strings.ToLower(envDebug) {
		case "1", "on", "true":
			debugMode = processor.DebugBreakpoints
		case "0", "off", "false":
			debugMode = processor.DebugOff
		}
	}

	if envStep := os.Getenv("MSBATCH_STEP"); envStep != "" {
		switch strings.ToLower(envStep) {
		case "1", "on", "true":
			debugMode = processor.DebugStepMode
		case "0", "off", "false":
		}
	}

	var filteredArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--trace" || arg == "-trace" {
			traceMode = logging.TraceOn
		} else if arg == "--trace-verbose" || arg == "-trace-verbose" {
			traceMode = logging.TraceVerbose
		} else if strings.HasPrefix(arg, "--trace=") {
			switch strings.ToLower(arg[8:]) {
			case "1", "on", "true":
				traceMode = logging.TraceOn
			case "2", "verbose":
				traceMode = logging.TraceVerbose
			}
		} else if strings.HasPrefix(arg, "--trace-file=") {
			f, err := os.Create(arg[13:])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating trace file: %v\n", err)
				os.Exit(1)
			}
			traceOutput = f
		} else if arg == "--trace-file" && i+1 < len(args) {
			i++
			f, err := os.Create(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating trace file: %v\n", err)
				os.Exit(1)
			}
			traceOutput = f
		} else if arg == "--debug" || arg == "-debug" {
			debugMode = processor.DebugBreakpoints
		} else if arg == "--step" || arg == "-step" {
			debugMode = processor.DebugStepMode
		} else if strings.HasPrefix(arg, "/") && len(arg) > 1 && arg[1] != '?' {
			switch strings.ToUpper(arg) {
			case "/TRACE":
				traceMode = logging.TraceOn
			case "/TRACE:V", "/TRACE:VERBOSE":
				traceMode = logging.TraceVerbose
			case "/DEBUG":
				debugMode = processor.DebugBreakpoints
			case "/STEP":
				debugMode = processor.DebugStepMode
			default:
				filteredArgs = append(filteredArgs, arg)
			}
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}
	args = filteredArgs

	logging.InitTrace(traceMode, traceOutput)

	if len(args) == 0 {
		runInteractive(debugMode)
		return
	}
	switch strings.ToUpper(args[0]) {
	case "/C":
		if len(args) > 1 {
			runCommand(strings.Join(args[1:], " "), debugMode)
		}
	case "/K":
		if len(args) > 1 {
			runCommand(strings.Join(args[1:], " "), debugMode)
		}
		runInteractive(debugMode)
	default:
		runFile(args[0], args, debugMode)
	}
}

// runAsTool executes a single registered tool and exits with its ERRORLEVEL.
func runAsTool(name string, args []string) {
	env := processor.NewEnvironment(false)
	reg := executor.New()
	proc := newProcessor(env, nil, reg, processor.DebugOff)

	cmd := &parser.SimpleCommand{
		Name:    name,
		RawArgs: args,
		Args:    args, // Rough approximation, tools usually use Words() or RawArgs
	}

	if err := reg.ExecCommand(proc, cmd); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Exit with the ERRORLEVEL set by the tool.
	if lv, ok := proc.Env.Get("ERRORLEVEL"); ok {
		if code, err := strconv.Atoi(lv); err == nil {
			os.Exit(code)
		}
	}
	os.Exit(0)
}

func runFile(filename string, args []string, debugMode processor.DebugMode) {
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	env := processor.NewEnvironment(true)
	proc := newProcessor(env, args, executor.New(), debugMode)
	proc.SetCurrentFile(filename)

	raw := string(content)
	if strings.HasPrefix(raw, "#!") {
		if nl := strings.IndexByte(raw, '\n'); nl >= 0 {
			raw = raw[nl+1:]
		} else {
			raw = ""
		}
	}

	src := processor.Phase0ReadLine(raw)
	nodes := processor.ParseExpanded(src)

	if err := proc.Execute(nodes); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

// runCommand executes a single command string and exits.
func runCommand(cmdStr string, debugMode processor.DebugMode) {
	env := processor.NewEnvironment(false)
	proc := newProcessor(env, nil, executor.New(), debugMode)
	nodes := processor.ParseExpanded(cmdStr)
	if err := proc.Execute(nodes); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

// shellCompleter implements readline.AutoCompleter.
// First word → complete registered command names.
// Subsequent words → complete file/directory paths.
type shellCompleter struct {
	commands []string
}

func newShellCompleter(reg *executor.Registry) *shellCompleter {
	return &shellCompleter{commands: reg.Names()}
}

func (c *shellCompleter) Do(line []rune, pos int) ([][]rune, int) {
	lineStr := string(line[:pos])
	endsSpace := strings.HasSuffix(lineStr, " ") || strings.HasSuffix(lineStr, "\t")
	words := strings.Fields(lineStr)
	isCmd := len(words) == 0 || (len(words) == 1 && !endsSpace)

	prefix := ""
	if !endsSpace && len(words) > 0 {
		prefix = words[len(words)-1]
	}

	if isCmd {
		lower := strings.ToLower(prefix)
		var matches [][]rune
		for _, name := range c.commands {
			if strings.HasPrefix(name, lower) {
				matches = append(matches, []rune(name[len(lower):]))
			}
		}
		return matches, len([]rune(prefix))
	}

	return pathComplete(prefix)
}

func pathComplete(prefix string) ([][]rune, int) {
	var dir, base string
	if prefix == "" || strings.HasSuffix(prefix, "/") || strings.HasSuffix(prefix, string(os.PathSeparator)) {
		dir = prefix
		if dir == "" {
			dir = "."
		}
		base = ""
	} else {
		dir = filepath.Dir(prefix)
		base = filepath.Base(prefix)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0
	}

	baseLower := strings.ToLower(base)
	var matches [][]rune
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(strings.ToLower(name), baseLower) {
			suffix := name[len(base):]
			if e.IsDir() {
				suffix += "/"
			}
			matches = append(matches, []rune(suffix))
		}
	}
	return matches, len([]rune(base))
}

func runInteractive(debugMode processor.DebugMode) {
	env := processor.NewEnvironment(false)
	reg := executor.New()
	proc := newProcessor(env, nil, reg, debugMode)
	proc.Echo = false // readline already shows typed input; suppress batch-style echo

	// Default CMD-style prompt: "path> "
	if _, ok := proc.Env.Get("PROMPT"); !ok {
		proc.Env.Set("PROMPT", "$P$G ")
	}

	histFile := ""
	if home, err := os.UserHomeDir(); err == nil {
		histFile = filepath.Join(home, ".msbatch_history")
	}

	rl, err := readline.NewEx(&readline.Config{
		HistoryFile:       histFile,
		AutoComplete:      newShellCompleter(reg),
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
	})
	if err != nil {
		// Not a TTY or readline unavailable — fall back to plain scanner.
		runInteractiveFallback(proc)
		return
	}
	defer rl.Close()

	// Wire the processor I/O through readline so that output doesn't
	// corrupt the prompt line when subprocesses write to the terminal.
	proc.Stdout = rl.Stdout()
	proc.Stderr = rl.Stdout() // keep stderr on the same managed writer
	proc.Stdin = os.Stdin

	fmt.Fprintln(rl.Stdout(), executor.VersionString())
	fmt.Fprintln(rl.Stdout())

	for {
		promptStr, _ := proc.Env.Get("PROMPT")
		rl.SetPrompt(proc.ExpandPrompt(promptStr))

		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			// Ctrl+C — cancel current input, show a new prompt.
			continue
		}
		if err != nil {
			// Ctrl+D / EOF.
			fmt.Fprintln(rl.Stdout())
			break
		}

		// Handle CMD-style ^ line continuation.
		// A trailing ^ (optionally followed by spaces) causes the shell to
		// show "More? " and append the next line before parsing.
		for {
			trimmed := strings.TrimRight(line, " \t")
			if !strings.HasSuffix(trimmed, "^") {
				break
			}
			rl.SetPrompt("More? ")
			cont, err := rl.Readline()
			if err != nil {
				break
			}
			line = trimmed[:len(trimmed)-1] + cont
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		if handled, err := handleInteractiveLine(proc, line, rl.Stdout()); handled {
			if err != nil {
				fmt.Fprintf(rl.Stdout(), "Error: %v\n", err)
			}
			continue
		}

		nodes := processor.ParseExpanded(line)
		if err := proc.Execute(nodes); err != nil {
			fmt.Fprintf(rl.Stdout(), "Error: %v\n", err)
		}
	}
}

// runInteractiveFallback is the non-TTY / no-readline fallback.
func runInteractiveFallback(proc *processor.Processor) {
	proc.Logger = logging.NewLoggerFromEnv()
	proc.Echo = false
	fmt.Println(executor.VersionString())
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		promptStr, _ := proc.Env.Get("PROMPT")
		fmt.Print(proc.ExpandPrompt(promptStr))

		if !scanner.Scan() {
			break
		}
		line := scanner.Text()

		// CMD-style ^ line continuation: trailing ^ causes "More? " prompt.
		for {
			trimmed := strings.TrimRight(line, " \t")
			if !strings.HasSuffix(trimmed, "^") {
				break
			}
			fmt.Print("More? ")
			if !scanner.Scan() {
				break
			}
			line = trimmed[:len(trimmed)-1] + scanner.Text()
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		if handled, err := handleInteractiveLine(proc, line, os.Stdout); handled {
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			continue
		}

		nodes := processor.ParseExpanded(line)
		if err := proc.Execute(nodes); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}

// isBatchFile checks if the given path looks like a batch file invocation.
// It returns the cleaned file path and true if it is, or empty string and false otherwise.
func isBatchFile(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}

	// Split to get the first token (potential batch file path)
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "", false
	}

	firstToken := parts[0]

	// Check if it has a .bat or .cmd extension
	lower := strings.ToLower(firstToken)
	if !strings.HasSuffix(lower, ".bat") && !strings.HasSuffix(lower, ".cmd") {
		return "", false
	}

	// Check if the file exists
	if _, err := os.Stat(firstToken); err != nil {
		return "", false
	}

	return firstToken, true
}

// handleInteractiveLine checks if the line is a batch file invocation and executes it.
// Returns (true, nil) if handled, (true, error) if handled with error, (false, nil) if not a batch file.
func handleInteractiveLine(proc *processor.Processor, line string, stdout io.Writer) (bool, error) {
	batchFile, isBatch := isBatchFile(line)
	if !isBatch {
		return false, nil
	}

	// Read the batch file
	content, err := os.ReadFile(batchFile)
	if err != nil {
		return true, fmt.Errorf("error reading file: %v", err)
	}

	// Strip Unix shebang so scripts can start with #!/usr/bin/env msbatch
	raw := string(content)
	if strings.HasPrefix(raw, "#!") {
		if nl := strings.IndexByte(raw, '\n'); nl >= 0 {
			raw = raw[nl+1:]
		} else {
			raw = ""
		}
	}

	// Parse and execute in the current processor context
	src := processor.Phase0ReadLine(raw)
	nodes := processor.ParseExpanded(src)

	if err := proc.Execute(nodes); err != nil {
		return true, err
	}

	return true, nil
}
