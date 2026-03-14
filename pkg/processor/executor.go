package processor

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

// Execute runs the AST nodes.
func (p *Processor) Execute(nodes []parser.Node) error {
	for _, n := range nodes {
		if err := p.ExecuteNode(n); err != nil {
			return err
		}
	}
	return nil
}

// ExecuteNode runs a single AST node.
func (p *Processor) ExecuteNode(n parser.Node) error {
	switch node := n.(type) {
	case *parser.SimpleCommand:
		return p.executeSimpleCommand(node)
	case *parser.Block:
		return p.Execute(node.Body)
	case *parser.IfNode:
		return p.executeIf(node)
	case *parser.ForNode:
		return p.executeFor(node)
	case *parser.BinaryNode:
		return p.executeBinary(node)
	case *parser.PipeNode:
		return p.executePipe(node)
	case *parser.LabelNode, *parser.CommentNode:
		return nil
	default:
		return fmt.Errorf("unknown node type: %T", n)
	}
}

func (p *Processor) executeSimpleCommand(n *parser.SimpleCommand) error {
	// 1. Expand name and args
	expanded := p.ExpandNode(n)

	// Setup IO overrides for redirections (basic)
	origStdout := p.Stdout
	origStdin := p.Stdin
	origStderr := p.Stderr

	defer func() {
		// Basic cleanup of files we opened
		if f, ok := p.Stdout.(*os.File); ok && f != os.Stdout && f != origStdout {
			f.Close()
		}
		if f, ok := p.Stdin.(*os.File); ok && f != os.Stdin && f != origStdin {
			f.Close()
		}
		if f, ok := p.Stderr.(*os.File); ok && f != os.Stderr && f != origStderr {
			f.Close()
		}
		p.Stdout = origStdout
		p.Stdin = origStdin
		p.Stderr = origStderr
	}()

	for _, r := range expanded.Redirects {
		targetPath := MapPath(r.Target)
		switch r.Kind {
		case parser.RedirectOut:
			f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
			if err == nil {
				if r.FD == 1 || r.FD == 0 { // 0 is default from extractFD for >, but 1 is usually standard out
					p.Stdout = f
				} else if r.FD == 2 {
					p.Stderr = f
				}
			}
		case parser.RedirectAppend:
			f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
			if err == nil {
				if r.FD == 1 || r.FD == 0 {
					p.Stdout = f
				} else if r.FD == 2 {
					p.Stderr = f
				}
			}
		case parser.RedirectIn:
			f, err := os.Open(targetPath)
			if err == nil {
				p.Stdin = f
			}
		}
	}

	// 2. Echo if needed
	if p.ShouldEcho(n) {
		prompt, ok := p.Env.Get("PROMPT")
		if !ok {
			prompt = "$P$G" // Default CMD prompt
		}

		// Basic expansion of prompt variables
		expandedPrompt := prompt
		if strings.Contains(expandedPrompt, "$P") {
			pwd, _ := os.Getwd()
			expandedPrompt = strings.ReplaceAll(expandedPrompt, "$P", pwd)
		}
		if strings.Contains(expandedPrompt, "$p") {
			pwd, _ := os.Getwd()
			expandedPrompt = strings.ReplaceAll(expandedPrompt, "$p", pwd)
		}
		expandedPrompt = strings.ReplaceAll(expandedPrompt, "$G", ">")
		expandedPrompt = strings.ReplaceAll(expandedPrompt, "$g", ">")
		expandedPrompt = strings.ReplaceAll(expandedPrompt, "$S", " ")
		expandedPrompt = strings.ReplaceAll(expandedPrompt, "$s", " ")

		fmt.Fprintf(p.Stdout, "%s%s %s\n", expandedPrompt, expanded.Name, strings.Join(expanded.Args, " "))
	}
	name := strings.ToLower(expanded.Name)
	switch name {
	case "echo", "echo.":
		output, stateChanged := p.HandleEchoBuiltin(expanded.Args)
		// If the command was "echo.", print an empty line even if there were no args.
		if name == "echo." && len(expanded.Args) == 0 {
			fmt.Fprintln(p.Stdout)
			p.Env.Set("ERRORLEVEL", "0")
			return nil
		}
		if !stateChanged {
			fmt.Fprintln(p.Stdout, output)
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "set":
		if len(expanded.Args) == 0 {
			for k, v := range p.Env.Snapshot() {
				fmt.Fprintf(p.Stdout, "%s=%s\n", k, v)
			}
			p.Env.Set("ERRORLEVEL", "0")
			return nil
		}

		arg := strings.Join(expanded.Args, " ")

		// Handle /A (Arithmetic)
		if len(expanded.Args) > 0 && strings.HasPrefix(strings.ToLower(expanded.Args[0]), "/a") {
			expr := arg[2:] // remove /A
			if before, after, ok := strings.Cut(expr, "="); ok {
				// Basic math for "+ 1"
				after = strings.TrimSpace(after)
				val := 0
				if strings.Contains(after, "+") {
					parts := strings.SplitSeq(after, "+")
					for part := range parts {
						n, _ := strconv.Atoi(strings.TrimSpace(part))
						val += n
					}
				} else {
					val, _ = strconv.Atoi(after)
				}
				p.HandleSetBuiltin(strings.TrimSpace(before), strconv.Itoa(val))
			}
			p.Env.Set("ERRORLEVEL", "0")
			return nil
		}

		// Handle /P (Prompt)
		if len(expanded.Args) > 0 && strings.HasPrefix(strings.ToLower(expanded.Args[0]), "/p") {
			promptStr := arg[2:] // remove /P
			if before, after, ok := strings.Cut(promptStr, "="); ok {
				fmt.Fprint(p.Stdout, after)

				var input string
				fmt.Fscanln(p.Stdin, &input)
				p.HandleSetBuiltin(strings.TrimSpace(before), input)
			}
			p.Env.Set("ERRORLEVEL", "0")
			return nil
		}

		if before, after, ok := strings.Cut(arg, "="); ok {
			p.HandleSetBuiltin(before, after)
		} else {
			// e.g. "set P" to list all variables starting with P
			found := false
			prefix := strings.ToUpper(arg)
			for k, v := range p.Env.Snapshot() {
				if strings.HasPrefix(k, prefix) {
					fmt.Fprintf(p.Stdout, "%s=%s\n", k, v)
					found = true
				}
			}
			if !found {
				fmt.Fprintf(p.Stderr, "Environment variable %s not defined\n", arg)
				p.Env.Set("ERRORLEVEL", "1")
				return nil
			}
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "cd", "chdir":
		if len(expanded.Args) == 0 {
			pwd, _ := os.Getwd()
			fmt.Fprintln(p.Stdout, pwd)
			p.Env.Set("ERRORLEVEL", "0")
			return nil
		}
		path := MapPath(expanded.Args[0])
		if err := os.Chdir(path); err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the path specified.\n")
			p.Env.Set("ERRORLEVEL", "1")
		} else {
			p.Env.Set("ERRORLEVEL", "0")
		}
		return nil
	case "exit":
		code := 0
		if len(expanded.Args) > 0 {
			if expanded.Args[0] != "/b" && expanded.Args[0] != "/B" {
				code, _ = strconv.Atoi(expanded.Args[0])
			} else if len(expanded.Args) > 1 {
				code, _ = strconv.Atoi(expanded.Args[1])
			}
		}
		os.Exit(code)
	}

	// 4. Handle external commands
	return p.runExternalCommand(expanded)
}

func (p *Processor) runExternalCommand(n *parser.SimpleCommand) error {
	// Map executable path
	cmdName := MapPath(n.Name)

	// Map arguments if they look like paths
	mappedArgs := make([]string, len(n.Args))
	for i, arg := range n.Args {
		if strings.Contains(arg, "\\") || (len(arg) >= 2 && arg[1] == ':') {
			mappedArgs[i] = MapPath(arg)
		} else {
			mappedArgs[i] = arg
		}
	}

	cmd := exec.Command(cmdName, mappedArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range p.Env.Snapshot() {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			p.Env.Set("ERRORLEVEL", strconv.Itoa(exitErr.ExitCode()))
		} else {
			fmt.Fprintf(os.Stderr, "'%s' is not recognized as an internal or external command, operable program or batch file.\n", n.Name)
			p.Env.Set("ERRORLEVEL", "9009")
		}
	} else {
		p.Env.Set("ERRORLEVEL", "0")
	}

	return nil
}

func (p *Processor) executeIf(n *parser.IfNode) error {
	// For simplicity, let's implement basic IF EXIST and comparison
	conditionMet := false
	cond := n.Cond

	switch cond.Kind {
	case parser.CondExist:
		path := MapPath(p.ProcessLine(cond.Arg))
		_, err := os.Stat(path)
		conditionMet = (err == nil)
	case parser.CondCompare:
		left := p.ProcessLine(cond.Left)
		right := p.ProcessLine(cond.Right)
		if n.CaseInsensitive {
			left = strings.ToLower(left)
			right = strings.ToLower(right)
		}

		switch cond.Op {
		case parser.OpEqual, parser.OpEqu:
			conditionMet = (left == right)
		case parser.OpNeq:
			conditionMet = (left != right)
		// For LSS, LEQ, GTR, GEQ we might need numeric comparison if both are numbers
		case parser.OpLss:
			conditionMet = (left < right)
		case parser.OpLeq:
			conditionMet = (left <= right)
		case parser.OpGtr:
			conditionMet = (left > right)
		case parser.OpGeq:
			conditionMet = (left >= right)
		}
	case parser.CondDefined:
		_, conditionMet = p.Env.Get(cond.Arg)
	case parser.CondErrorLevel:
		currLevelStr, _ := p.Env.Get("ERRORLEVEL")
		currLevel, _ := strconv.Atoi(currLevelStr)
		conditionMet = (currLevel >= cond.Level)
	}

	if cond.Not {
		conditionMet = !conditionMet
	}

	if conditionMet {
		return p.ExecuteNode(n.Then)
	} else if n.Else != nil {
		return p.ExecuteNode(n.Else)
	}

	return nil
}

func (p *Processor) executeFor(n *parser.ForNode) error {
	// Save current ForVars
	oldForVars := p.ForVars
	p.ForVars = make(map[string]string)
	maps.Copy(p.ForVars, oldForVars)
	defer func() { p.ForVars = oldForVars }()

	if n.Variant == parser.ForFiles {
		for _, item := range n.Set {
			expandedItem := p.ProcessLine(item)
			// Glob the item
			matches, err := filepath.Glob(MapPath(expandedItem))
			if err != nil || len(matches) == 0 {
				// If no matches, CMD uses the literal item
				matches = []string{expandedItem}
			}

			for _, m := range matches {
				p.ForVars[n.Variable] = m
				if err := p.ExecuteNode(n.Do); err != nil {
					return err
				}
			}
		}
	} else if n.Variant == parser.ForRange {
		// FOR /L %var IN (start,step,end) DO command
		if len(n.Set) >= 3 {
			startStr := strings.TrimRight(p.ProcessLine(n.Set[0]), ",")
			stepStr := strings.TrimRight(p.ProcessLine(n.Set[1]), ",")
			endStr := strings.TrimRight(p.ProcessLine(n.Set[2]), ",")

			start, _ := strconv.Atoi(startStr)
			step, _ := strconv.Atoi(stepStr)
			end, _ := strconv.Atoi(endStr)

			if step > 0 {
				for i := start; i <= end; i += step {
					p.ForVars[n.Variable] = strconv.Itoa(i)
					if err := p.ExecuteNode(n.Do); err != nil {
						return err
					}
				}
			} else if step < 0 {
				for i := start; i >= end; i += step {
					p.ForVars[n.Variable] = strconv.Itoa(i)
					if err := p.ExecuteNode(n.Do); err != nil {
						return err
					}
				}
			}
		}
	} else if n.Variant == parser.ForF {
		// Basic FOR /F implementation for literal strings and basic files
		// Proper FOR /F is complex, parsing "tokens= delims=" etc.
		// For now, let's assume it iterates over words in the set if they are strings.
		for _, item := range n.Set {
			expandedItem := p.ProcessLine(item)
			p.ForVars[n.Variable] = expandedItem
			if err := p.ExecuteNode(n.Do); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Processor) executeBinary(n *parser.BinaryNode) error {
	p.ExecuteNode(n.Left)

	switch n.Op {
	case parser.NodeConcat: // &
		return p.ExecuteNode(n.Right)
	case parser.NodeAndThen: // &&
		levelStr, _ := p.Env.Get("ERRORLEVEL")
		if levelStr == "0" {
			return p.ExecuteNode(n.Right)
		}
	case parser.NodeOrElse: // ||
		levelStr, _ := p.Env.Get("ERRORLEVEL")
		if levelStr != "0" {
			return p.ExecuteNode(n.Right)
		}
	}
	return nil
}

func (p *Processor) executePipe(n *parser.PipeNode) error {
	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}

	// Run left side with output to pipe
	leftProcessor := *p // shallow copy
	leftProcessor.Stdout = pw

	leftErrChan := make(chan error, 1)
	go func() {
		err := leftProcessor.ExecuteNode(n.Left)
		pw.Close() // Close writer so reader gets EOF
		leftErrChan <- err
	}()

	// Run right side with input from pipe
	rightProcessor := *p
	rightProcessor.Stdin = pr

	rightErrChan := make(chan error, 1)
	go func() {
		err := rightProcessor.ExecuteNode(n.Right)
		pr.Close()
		rightErrChan <- err
	}()

	leftErr := <-leftErrChan
	rightErr := <-rightErrChan

	if leftErr != nil {
		return leftErr
	}
	return rightErr
}
