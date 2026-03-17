package processor

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

// Execute runs the AST nodes.
func (p *Processor) Execute(nodes []parser.Node) error {
	p.Nodes = nodes
	p.PC = 0
	p.Exited = false
	for p.PC < len(p.Nodes) && !p.Exited {
		n := p.Nodes[p.PC]
		p.PC++ // Advance PC before execution
		if err := p.ExecuteNode(n); err != nil {
			return err
		}
	}
	return nil
}

// ExecuteNode runs a single AST node.
func (p *Processor) ExecuteNode(n parser.Node) error {
	if p.Exited {
		return nil
	}
	switch node := n.(type) {
	case *parser.SimpleCommand:
		return p.executeSimpleCommand(node)
	case *parser.Block:
		for _, bn := range node.Body {
			if err := p.ExecuteNode(bn); err != nil {
				return err
			}
			if p.Exited {
				break
			}
		}
		return nil
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

func (p *Processor) jumpToLabel(labelName string) error {
	target := strings.ToLower(labelName)
	for i, n := range p.Nodes {
		if lbl, ok := n.(*parser.LabelNode); ok {
			if strings.ToLower(lbl.Name) == target {
				p.PC = i
				return nil
			}
		}
	}
	return fmt.Errorf("the system cannot find the batch label specified - %s", labelName)
}

func (p *Processor) executeSimpleCommand(n *parser.SimpleCommand) error {
	expanded := p.ExpandNode(n)

	words := strings.Fields(expanded.Name)
	if len(words) > 1 {
		expanded.Name = words[0]
		newArgs := append([]string{}, words[1:]...)
		expanded.Args = append(newArgs, expanded.Args...)
	}

	origStdout := p.Stdout
	origStdin := p.Stdin
	origStderr := p.Stderr

	defer func() {
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
				switch r.FD {
				case 0:
					fallthrough
				case 1:
					p.Stdout = f
				case 2:
					p.Stderr = f
				}
			}
		case parser.RedirectAppend:
			f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
			if err == nil {
				switch r.FD {
				case 0:
					fallthrough
				case 1:
					p.Stdout = f
				case 2:
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

	if p.ShouldEcho(n) {
		prompt, ok := p.Env.Get("PROMPT")
		if !ok {
			prompt = "$P$G"
		}
		expandedPrompt := p.ExpandPrompt(prompt)
		fmt.Fprintf(p.Stdout, "%s%s %s\n", expandedPrompt, expanded.Name, strings.Join(expanded.Args, " "))
	}

	// Flow-control commands are handled directly by the Processor because they
	// manipulate Processor state (PC, Args, Exited) that an external executor
	// cannot safely touch.
	name := strings.ToLower(expanded.Name)
	switch name {
	case "goto":
		label := strings.Join(expanded.Args, "")
		label = strings.TrimPrefix(label, ":")
		if strings.ToLower(label) == "eof" {
			p.PC = len(p.Nodes)
			return nil
		}
		return p.jumpToLabel(label)
	case "call":
		if len(expanded.Args) == 0 {
			return nil
		}
		target := expanded.Args[0]
		restArgs := expanded.Args[1:]
		if strings.HasPrefix(target, ":") {
			label := target[1:]
			oldPC := p.PC
			oldArgs := p.Args
			p.Args = append([]string{target}, restArgs...)
			if err := p.jumpToLabel(label); err != nil {
				p.Args = oldArgs
				return err
			}
			p.CallDepth++
			for p.PC < len(p.Nodes) && !p.Exited {
				node := p.Nodes[p.PC]
				p.PC++
				if err := p.ExecuteNode(node); err != nil {
					p.CallDepth--
					if err.Error() == "EXIT_LOCAL" {
						p.PC = oldPC
						p.Args = oldArgs
						return nil
					}
					return err
				}
			}
			p.CallDepth--
			p.PC = oldPC
			p.Args = oldArgs
			return nil
		}
		// Suppress echo on the inner dispatch: the CALL command line was
		// already echoed by the outer executeSimpleCommand call.
		return p.executeSimpleCommand(&parser.SimpleCommand{Name: target, Args: restArgs, Suppressed: true})
	case "exit":
		code := 0
		isLocal := false
		if len(expanded.Args) > 0 {
			if strings.ToLower(expanded.Args[0]) == "/b" {
				isLocal = true
				if len(expanded.Args) > 1 {
					code, _ = strconv.Atoi(expanded.Args[1])
				}
			} else {
				code, _ = strconv.Atoi(expanded.Args[0])
			}
		}
		p.Env.Set("ERRORLEVEL", strconv.Itoa(code))
		if isLocal {
			// EXIT /B — unwind the current CALL :label frame if one exists.
			// The CALL handler catches EXIT_LOCAL and restores its context.
			// If we are NOT inside a CALL (p.CallDepth == 0), fall through to
			// os.Exit so it behaves identically to plain EXIT at the top level.
			if p.CallDepth > 0 {
				return fmt.Errorf("EXIT_LOCAL")
			}
		}
		os.Exit(code)
		return nil
	case "setlocal":
		p.Env.Push()
		return nil
	case "endlocal":
		p.Env.Pop()
		return nil
	case "shift":
		if len(p.Args) > 1 {
			p.Args = append(p.Args[:1], p.Args[2:]...)
		}
		return nil
	}

	// Delegate all other commands to the pluggable executor.
	if p.Executor != nil {
		return p.Executor.ExecCommand(p, expanded)
	}
	return nil
}

func (p *Processor) executeIf(n *parser.IfNode) error {
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

		unquote := func(s string) string {
			if len(s) >= 2 {
				q := s[0]
				if (q == '"' || q == '\'' || q == '`') && s[len(s)-1] == q {
					return s[1 : len(s)-1]
				}
			}
			return s
		}
		left = unquote(left)
		right = unquote(right)

		if n.CaseInsensitive {
			left = strings.ToLower(left)
			right = strings.ToLower(right)
		}

		switch cond.Op {
		case parser.OpEqual, parser.OpEqu:
			conditionMet = (left == right)
		case parser.OpNeq:
			conditionMet = (left != right)
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
		_, conditionMet = p.Env.Get(p.ProcessLine(cond.Arg))
	case parser.CondCmdExtVersion:
		// Command extensions are always version 2 in this implementation.
		conditionMet = (2 >= cond.Level)
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
	oldForVars := p.ForVars
	p.ForVars = make(map[string]string)
	maps.Copy(p.ForVars, oldForVars)
	defer func() { p.ForVars = oldForVars }()

	if n.Variant == parser.ForFiles {
		for _, item := range n.Set {
			expandedItem := p.ProcessLine(item)
			matches, err := filepath.Glob(MapPath(expandedItem))
			if err != nil || len(matches) == 0 {
				matches = []string{expandedItem}
			}
			for _, m := range matches {
				p.ForVars[n.Variable] = m
				if err := p.ExecuteNode(n.Do); err != nil {
					return err
				}
				if p.Exited {
					break
				}
			}
			if p.Exited {
				break
			}
		}
	} else if n.Variant == parser.ForRange {
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
					if p.Exited {
						break
					}
				}
			} else if step < 0 {
				for i := start; i >= end; i += step {
					p.ForVars[n.Variable] = strconv.Itoa(i)
					if err := p.ExecuteNode(n.Do); err != nil {
						return err
					}
					if p.Exited {
						break
					}
				}
			}
		}
	} else if n.Variant == parser.ForDir {
		for _, item := range n.Set {
			expandedItem := p.ProcessLine(item)
			mapped := MapPath(expandedItem)
			dir := filepath.Dir(mapped)
			pattern := filepath.Base(mapped)
			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				if matched, _ := filepath.Match(pattern, e.Name()); matched {
					p.ForVars[n.Variable] = filepath.Join(dir, e.Name())
					if err := p.ExecuteNode(n.Do); err != nil {
						return err
					}
					if p.Exited {
						break
					}
				}
			}
			if p.Exited {
				break
			}
		}
	} else if n.Variant == parser.ForRecursive {
		rootDir := "."
		if n.Options != "" {
			opt := strings.TrimSpace(n.Options)
			opt = p.ProcessLine(opt)
			if len(opt) >= 2 && opt[0] == '"' && opt[len(opt)-1] == '"' {
				opt = opt[1 : len(opt)-1]
			}
			rootDir = MapPath(opt)
		}
		var walkErr error
		err := filepath.Walk(rootDir, func(dirPath string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() {
				return nil
			}
			for _, item := range n.Set {
				expandedItem := p.ProcessLine(item)
				fullPattern := filepath.Join(dirPath, expandedItem)
				if strings.ContainsAny(expandedItem, "*?") {
					matches, err := filepath.Glob(fullPattern)
					if err != nil || len(matches) == 0 {
						continue
					}
					for _, m := range matches {
						p.ForVars[n.Variable] = m
						if err := p.ExecuteNode(n.Do); err != nil {
							walkErr = err
							return errors.New("stop")
						}
						if p.Exited {
							return errors.New("stop")
						}
					}
				} else {
					p.ForVars[n.Variable] = fullPattern
					if err := p.ExecuteNode(n.Do); err != nil {
						walkErr = err
						return errors.New("stop")
					}
					if p.Exited {
						return errors.New("stop")
					}
				}
			}
			return nil
		})
		if walkErr != nil {
			return walkErr
		}
		_ = err
	} else if n.Variant == parser.ForF {
		opts := parseForFOptions(unquoteStr(n.Options))
		for _, item := range n.Set {
			var lines []string
			isCommand := false
			isString := false
			rawItem := item

			if strings.HasPrefix(item, "\"") && strings.HasSuffix(item, "\"") {
				if !opts.usebackq {
					isString = true
					rawItem = item[1 : len(item)-1]
				} else {
					rawItem = item[1 : len(item)-1]
				}
			} else if strings.HasPrefix(item, "'") && strings.HasSuffix(item, "'") {
				if opts.usebackq {
					isString = true
					rawItem = item[1 : len(item)-1]
				} else {
					isCommand = true
					rawItem = item[1 : len(item)-1]
				}
			} else if strings.HasPrefix(item, "`") && strings.HasSuffix(item, "`") {
				if opts.usebackq {
					isCommand = true
					rawItem = item[1 : len(item)-1]
				}
			}

			if isCommand {
				expandedCmd := p.ProcessLine(rawItem)
				out, err := p.captureCommandOutput(expandedCmd)
				if err == nil {
					lines = strings.Split(out, "\n")
				}
			} else if isString {
				expanded := p.ProcessLine(rawItem)
				lines = []string{expanded}
			} else {
				path := MapPath(p.ProcessLine(rawItem))
				content, err := os.ReadFile(path)
				if err == nil {
					lines = strings.Split(string(content), "\n")
				}
			}

			if opts.skip > 0 {
				if opts.skip < len(lines) {
					lines = lines[opts.skip:]
				} else {
					lines = nil
				}
			}
			for _, line := range lines {
				line = strings.TrimRight(line, "\r")
				if line == "" || strings.HasPrefix(line, opts.eol) {
					continue
				}
				f := func(r rune) bool {
					return strings.ContainsRune(opts.delims, r)
				}
				parts := strings.FieldsFunc(line, f)
				tokenMap := applyForTokens(parts, opts.tokens, n.Variable)
				maps.Copy(p.ForVars, tokenMap)
				if len(tokenMap) > 0 {
					if err := p.ExecuteNode(n.Do); err != nil {
						return err
					}
					if p.Exited {
						break
					}
				}
			}
			if p.Exited {
				break
			}
		}
	}
	return nil
}

type forFOptions struct {
	eol      string
	skip     int
	delims   string
	tokens   string
	usebackq bool
}

func unquoteStr(s string) string {
	if len(s) >= 2 {
		f, l := s[0], s[len(s)-1]
		if (f == '"' && l == '"') || (f == '\'' && l == '\'') || (f == '`' && l == '`') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseForFOptions(optStr string) forFOptions {
	opts := forFOptions{delims: " \t", tokens: "1", eol: ";"}
	fields := strings.FieldsSeq(optStr)
	for f := range fields {
		if strings.HasPrefix(f, "delims=") {
			opts.delims = f[7:]
		} else if strings.HasPrefix(f, "tokens=") {
			opts.tokens = f[7:]
		} else if strings.HasPrefix(f, "skip=") {
			opts.skip, _ = strconv.Atoi(f[5:])
		} else if strings.HasPrefix(f, "eol=") {
			opts.eol = f[4:]
		} else if f == "usebackq" {
			opts.usebackq = true
		}
	}
	return opts
}

func applyForTokens(parts []string, tokens string, startVar string) map[string]string {
	res := make(map[string]string)
	if len(parts) == 0 {
		return res
	}
	tokenSpecs := strings.Split(tokens, ",")
	baseChar := rune(startVar[0])
	lastIdx := -1
	for i, spec := range tokenSpecs {
		if spec == "*" {
			startFrom := 0
			if lastIdx >= 0 {
				startFrom = lastIdx + 1
			}
			if startFrom < len(parts) {
				res[string(baseChar+rune(i))] = strings.Join(parts[startFrom:], " ")
			}
			continue
		}
		idx, _ := strconv.Atoi(spec)
		if idx > 0 && idx <= len(parts) {
			varName := string(baseChar + rune(i))
			res[varName] = parts[idx-1]
			if idx-1 > lastIdx {
				lastIdx = idx - 1
			}
		}
	}
	if len(tokenSpecs) == 1 && tokenSpecs[0] == "1" {
		res[startVar] = parts[0]
	}
	return res
}

func (p *Processor) captureCommandOutput(cmdLine string) (string, error) {
	expanded := p.ProcessLine(cmdLine)
	nodes := ParseExpanded(expanded)
	var buf bytes.Buffer
	subProc := New(p.Env, p.Args, p.Executor)
	subProc.Stdout = &buf
	subProc.Stderr = p.Stderr
	subProc.Echo = false
	err := subProc.Execute(nodes)
	return buf.String(), err
}

func (p *Processor) executeBinary(n *parser.BinaryNode) error {
	p.ExecuteNode(n.Left)
	if p.Exited {
		return nil
	}
	switch n.Op {
	case parser.NodeConcat:
		return p.ExecuteNode(n.Right)
	case parser.NodeAndThen:
		levelStr, _ := p.Env.Get("ERRORLEVEL")
		if levelStr == "0" {
			return p.ExecuteNode(n.Right)
		}
	case parser.NodeOrElse:
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
	leftProcessor := *p
	leftProcessor.Stdout = pw
	leftErrChan := make(chan error, 1)
	go func() {
		err := leftProcessor.ExecuteNode(n.Left)
		pw.Close()
		leftErrChan <- err
	}()
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
