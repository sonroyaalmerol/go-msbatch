package processor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
)

func (p *Processor) Execute(nodes []parser.Node) error {
	p.Logger.Debug("executing nodes", "count", len(nodes), "env", p.Env.Snapshot(), "cwd", func() string { cwd, _ := os.Getwd(); return cwd }())
	p.Nodes = nodes
	p.PC = 0
	p.Exited = false
	for p.PC < len(p.Nodes) && !p.Exited {
		n := p.Nodes[p.PC]
		if err := p.ExecuteNode(n); err != nil {
			return err
		}
		if !p.Exited && p.PC < len(p.Nodes) && p.Nodes[p.PC] == n {
			p.PC++
		}
	}
	return nil
}

func (p *Processor) ExecuteNode(n parser.Node) error {
	if p.Exited {
		return nil
	}
	if p.Trace.Enabled() {
		p.traceNode(n)
	}
	switch node := n.(type) {
	case *parser.SimpleCommand:
		return p.executeSimpleCommand(node)
	case *parser.Block:
		return p.executeBlock(node)
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

func (p *Processor) traceNode(n parser.Node) {
	if !p.Trace.Enabled() {
		return
	}
	line := n.Pos().Line + 1
	switch node := n.(type) {
	case *parser.SimpleCommand:
		if node.Name != "" {
			args := node.Words()
			p.Trace.Line(line, node.Name+" "+strings.Join(args, " "))
		}
	case *parser.LabelNode:
		p.Trace.Line(line, ":"+node.Name)
	case *parser.CommentNode:
		p.Trace.Line(line, "REM "+node.Text)
	}
}

func (p *Processor) executeBlock(node *parser.Block) error {
	rm := p.newRedirectManager()

	if len(node.Redirects) > 0 {
		var expandedRedirects []parser.Redirect
		for _, r := range node.Redirects {
			expandedRedirects = append(expandedRedirects, parser.Redirect{
				Kind:   r.Kind,
				Target: p.ExpandPhase4(p.ExpandPhase1(r.Target)),
				FD:     r.FD,
			})
		}
		rm.apply(p, expandedRedirects)
		defer rm.close(p)
	}

	for _, bn := range node.Body {
		if err := p.ExecuteNode(bn); err != nil {
			return err
		}
		if p.Exited {
			break
		}
	}
	return nil
}

func (p *Processor) jumpToLabel(labelName string) error {
	p.Logger.Debug("jumping to label", "label", labelName)
	target := strings.ToLower(labelName)

	// In CMD, labels are global to the file. Jumping to a label inside
	// a block (IF/FOR) effectively breaks out of that block and continues
	// from the label's position in the flat sequence of nodes.
	// We search p.Nodes which is the flat list of all nodes at the current level.
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
	// 1. Initial expansion: Phase 1 (percents) and Phase 4 (FOR variables)
	// These are expanded before command name resolution and echoing.
	expanded := &parser.SimpleCommand{
		Suppressed:       n.Suppressed,
		RedirectsApplied: n.RedirectsApplied,
	}
	expanded.Name = strings.TrimSpace(p.ExpandPhase4(p.ExpandPhase1(n.Name)))
	for _, arg := range n.Args {
		expanded.Args = append(expanded.Args, p.ExpandPhase4(p.ExpandPhase1(arg)))
	}
	for _, arg := range n.RawArgs {
		expanded.RawArgs = append(expanded.RawArgs, p.ExpandPhase4(p.ExpandPhase1(arg)))
	}
	for _, r := range n.Redirects {
		expanded.Redirects = append(expanded.Redirects, parser.Redirect{
			Kind:   r.Kind,
			Target: strings.TrimSpace(p.ExpandPhase4(p.ExpandPhase1(r.Target))),
			FD:     r.FD,
		})
	}

	// 2. Command name splitting (e.g. %VAR% containing "cmd args")
	words := strings.Fields(expanded.Name)
	if len(words) > 1 {
		expanded.Name = words[0]
		newArgs := words[1:]
		expanded.Args = append(newArgs, expanded.Args...)

		// Update RawArgs to keep mirroring the name and arguments correctly
		var newRaw []string
		for i, w := range newArgs {
			if i > 0 {
				newRaw = append(newRaw, " ")
			}
			newRaw = append(newRaw, w)
		}
		if len(expanded.RawArgs) > 0 {
			newRaw = append(newRaw, " ")
		}
		expanded.RawArgs = append(newRaw, expanded.RawArgs...)
	}

	// 3. Command echoing (happens after Phase 1/4 but BEFORE Phase 5)
	if p.ShouldEcho(n) {
		prompt, ok := p.Env.Get("PROMPT")
		if !ok {
			prompt = "$P$G"
		}
		expandedPrompt := p.ExpandPrompt(prompt)

		var sb strings.Builder
		sb.WriteString(expandedPrompt)
		sb.WriteString(expanded.Name)
		sb.WriteString(strings.Join(expanded.RawArgs, ""))

		// Include redirections in the echoed output
		for _, r := range expanded.Redirects {
			sb.WriteString(" ")
			if r.FD != 1 && r.FD != 0 {
				sb.WriteString(strconv.Itoa(r.FD))
			}
			switch r.Kind {
			case parser.RedirectOut:
				sb.WriteString(">")
			case parser.RedirectAppend:
				sb.WriteString(">>")
			case parser.RedirectIn:
				sb.WriteString("<")
			case parser.RedirectOutFD:
				sb.WriteString(">&")
			case parser.RedirectInFD:
				sb.WriteString("<&")
			}
			sb.WriteString(r.Target)
		}

		p.Logger.Debug("echo console output", "line", sb.String())
		fmt.Fprintln(p.Console, sb.String())
	}

	// 4. Final expansion: Phase 5 (delayed expansion)
	// This happens just before execution.
	expanded.Name = strings.TrimSpace(p.ExpandPhase5(expanded.Name))
	for i := range expanded.Args {
		expanded.Args[i] = p.ExpandPhase5(expanded.Args[i])
	}
	for i := range expanded.RawArgs {
		expanded.RawArgs[i] = p.ExpandPhase5(expanded.RawArgs[i])
	}
	for i := range expanded.Redirects {
		expanded.Redirects[i].Target = strings.TrimSpace(p.ExpandPhase5(expanded.Redirects[i].Target))
	}

	p.Logger.Debug("executing command", "name", expanded.Name, "args", expanded.Args, "cwd", func() string { cwd, _ := os.Getwd(); return cwd }())

	// 5. Clean up args for commands that expect words (removing empty expanded arguments)
	var filteredArgs []string
	for _, arg := range expanded.Args {
		if strings.TrimSpace(arg) != "" || (len(arg) >= 2 && (arg[0] == '"' || arg[0] == '\'')) {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	rm := p.newRedirectManager()
	defer rm.close(p)

	if !expanded.RedirectsApplied {
		rm.apply(p, expanded.Redirects)
		expanded.RedirectsApplied = true
	}

	name := strings.ToLower(expanded.Name)
	cmdWords := expanded.Words()
	switch name {
	case "goto":
		if len(cmdWords) == 0 {
			return nil
		}
		label := strings.Join(cmdWords, "")
		hasColon := strings.HasPrefix(label, ":")
		label = strings.TrimLeft(label, ":")
		p.Trace.GotoLabel(label)
		if hasColon && strings.ToLower(label) == "eof" {
			p.PC = len(p.Nodes)
			return nil
		}
		return p.jumpToLabel(label)
	case "call":
		if len(cmdWords) == 0 {
			return nil
		}
		target := cmdWords[0]
		restArgs := cmdWords[1:]
		if strings.HasPrefix(target, ":") {
			label := strings.TrimLeft(target, ":")
			p.Trace.CallLabel(label, restArgs)
			p.Trace.Indent()
			if strings.ToLower(label) == "eof" {
				p.PC = len(p.Nodes)
				p.Trace.Dedent()
				return nil
			}
			p.Logger.Debug("entering subroutine", "label", label, "args", restArgs)
			oldPC := p.PC
			oldArgs := p.Args
			p.Args = append([]string{target}, restArgs...)
			if err := p.jumpToLabel(label); err != nil {
				p.Args = oldArgs
				p.Trace.Dedent()
				return err
			}
			p.CallDepth++
			for p.PC < len(p.Nodes) && !p.Exited {
				node := p.Nodes[p.PC]
				p.PC++
				if err := p.ExecuteNode(node); err != nil {
					p.CallDepth--
					p.Trace.Dedent()
					if err.Error() == "EXIT_LOCAL" {
						p.Trace.ReturnFromLabel()
						p.PC = oldPC
						p.Args = oldArgs
						return nil
					}
					return err
				}
			}
			p.CallDepth--
			p.Trace.Dedent()
			p.Trace.ReturnFromLabel()
			p.PC = oldPC
			p.Args = oldArgs
			return nil
		}
		var reconstructedRaw []string
		for i, arg := range restArgs {
			if i > 0 {
				reconstructedRaw = append(reconstructedRaw, " ")
			}
			reconstructedRaw = append(reconstructedRaw, arg)
		}
		err := p.executeSimpleCommand(&parser.SimpleCommand{
			Name:             target,
			Args:             restArgs,
			RawArgs:          reconstructedRaw,
			Suppressed:       true,
			RedirectsApplied: true,
		})
		p.Trace.Dedent()
		return err
	case "exit":
		code := 0
		isLocal := false
		if len(filteredArgs) > 0 {
			if strings.ToLower(filteredArgs[0]) == "/b" {
				isLocal = true
				if len(filteredArgs) > 1 {
					code, _ = strconv.Atoi(filteredArgs[1])
				}
			} else {
				code, _ = strconv.Atoi(filteredArgs[0])
			}
		}
		p.FailureWithCode(code)
		p.Trace.Exit(code, isLocal)
		if isLocal {
			if p.CallDepth > 0 {
				return fmt.Errorf("EXIT_LOCAL")
			}
		}
		p.Exited = true
		return nil
	case "setlocal":
		p.Env.Push()
		for _, arg := range filteredArgs {
			switch strings.ToLower(arg) {
			case "enabledelayedexpansion":
				p.Env.SetDelayedExpansion(true)
			case "disabledelayedexpansion":
				p.Env.SetDelayedExpansion(false)
			}
		}
		return nil
	case "endlocal":
		p.Env.Pop()
		return nil
	case "shift":
		start := 0
		for _, arg := range filteredArgs {
			if strings.HasPrefix(arg, "/") {
				if n, err := strconv.Atoi(arg[1:]); err == nil {
					start = n
				}
			}
		}

		if start >= 0 && start < len(p.Args) {
			p.Logger.Debug("shifting arguments", "start", start, "before", p.Args)
			p.Args = append(p.Args[:start], p.Args[start+1:]...)
			p.Logger.Debug("arguments shifted", "after", p.Args)
		}
		return nil
	}

	// Delegate all other commands to the pluggable executor.
	if p.Executor != nil {
		return p.Executor.ExecCommand(p, expanded)
	}
	return nil
}

func (p *Processor) applyFD(fd int, stream any) {
	switch fd {
	case 0:
		if s, ok := stream.(io.Reader); ok {
			p.Stdin = s
		}
	case 1:
		if s, ok := stream.(io.Writer); ok {
			p.Stdout = s
		}
	case 2:
		if s, ok := stream.(io.Writer); ok {
			p.Stderr = s
		}
	}
}

type redirectManager struct {
	origStdout  io.Writer
	origStdin   io.Reader
	origStderr  io.Writer
	openedFiles []*os.File
}

type debugWriter struct {
	underlying io.Writer
	logger     *slog.Logger
	fd         int
	target     string
}

func (dw *debugWriter) Write(p []byte) (n int, err error) {
	n, err = dw.underlying.Write(p)
	if n > 0 {
		content := string(p[:n])
		if len(content) > 200 {
			content = content[:200] + "... (truncated)"
		}
		dw.logger.Debug("redirect write", "fd", dw.fd, "target", dw.target, "content", content)
	}
	return n, err
}

func (p *Processor) newRedirectManager() *redirectManager {
	return &redirectManager{
		origStdout: p.Stdout,
		origStdin:  p.Stdin,
		origStderr: p.Stderr,
	}
}

func (rm *redirectManager) close(p *Processor) {
	for _, f := range rm.openedFiles {
		f.Sync()
		f.Close()
	}
	p.Stdout = rm.origStdout
	p.Stdin = rm.origStdin
	p.Stderr = rm.origStderr
}

func (rm *redirectManager) apply(p *Processor, redirects []parser.Redirect) {
	for _, r := range redirects {
		targetPath := pathutil.MapPath(r.Target)
		isNul := strings.EqualFold(r.Target, "nul")
		kindStr := ">"
		switch r.Kind {
		case parser.RedirectAppend:
			kindStr = ">>"
		case parser.RedirectIn:
			kindStr = "<"
		case parser.RedirectOutFD:
			kindStr = ">&"
		case parser.RedirectInFD:
			kindStr = "<&"
		}
		p.Logger.Debug("applying redirect", "kind", kindStr, "fd", r.FD, "target", r.Target, "path", targetPath)

		switch r.Kind {
		case parser.RedirectOut:
			if isNul {
				p.Logger.Debug("redirect to nul", "fd", r.FD)
				switch r.FD {
				case 0, 1:
					p.Stdout = io.Discard
				case 2:
					p.Stderr = io.Discard
				}
			} else {
				p.Trace.RedirectWrite(r.Target)
				f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
				if err == nil {
					rm.openedFiles = append(rm.openedFiles, f)
					switch r.FD {
					case 0, 1:
						p.Stdout = &debugWriter{underlying: f, logger: p.Logger, fd: r.FD, target: targetPath}
					case 2:
						p.Stderr = &debugWriter{underlying: f, logger: p.Logger, fd: r.FD, target: targetPath}
					}
				} else {
					p.Logger.Debug("redirect open failed", "path", targetPath, "error", err)
				}
			}
		case parser.RedirectAppend:
			if isNul {
				p.Logger.Debug("redirect to nul (append)", "fd", r.FD)
				switch r.FD {
				case 0, 1:
					p.Stdout = io.Discard
				case 2:
					p.Stderr = io.Discard
				}
			} else {
				p.Trace.RedirectAppend(r.Target)
				f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
				if err == nil {
					rm.openedFiles = append(rm.openedFiles, f)
					switch r.FD {
					case 0, 1:
						p.Stdout = &debugWriter{underlying: f, logger: p.Logger, fd: r.FD, target: targetPath}
					case 2:
						p.Stderr = &debugWriter{underlying: f, logger: p.Logger, fd: r.FD, target: targetPath}
					}
				} else {
					p.Logger.Debug("redirect open failed", "path", targetPath, "error", err)
				}
			}
		case parser.RedirectIn:
			if isNul {
				p.Logger.Debug("redirect stdin from nul")
				p.Stdin = bytes.NewReader(nil)
			} else {
				p.Trace.RedirectRead(r.Target)
				f, err := os.Open(targetPath)
				if err == nil {
					rm.openedFiles = append(rm.openedFiles, f)
					p.Stdin = f
				} else {
					p.Logger.Debug("redirect open failed", "path", targetPath, "error", err)
				}
			}
		case parser.RedirectOutFD, parser.RedirectInFD:
			p.Logger.Debug("redirect fd to fd", "from", r.FD, "to", r.Target)
			switch r.Target {
			case "0":
				p.applyFD(r.FD, p.Stdin)
			case "1":
				p.applyFD(r.FD, p.Stdout)
			case "2":
				p.applyFD(r.FD, p.Stderr)
			}
		}
	}
}

func (p *Processor) executeIf(n *parser.IfNode) error {
	conditionMet := false
	cond := n.Cond

	switch cond.Kind {
	case parser.CondExist:
		rawPath := p.ProcessLine(cond.Arg)
		path := pathutil.MapPath(rawPath)
		cwd, _ := os.Getwd()
		if strings.ContainsAny(path, "*?[") {
			matches, err := pathutil.GlobCaseInsensitive(path)
			conditionMet = (err == nil && len(matches) > 0)
			p.Logger.Debug("IF EXIST check (wildcard)", "raw", rawPath, "mapped", path, "cwd", cwd, "matches", len(matches), "result", conditionMet)
		} else {
			_, err := os.Stat(path)
			conditionMet = (err == nil)
			p.Logger.Debug("IF EXIST check", "raw", rawPath, "mapped", path, "cwd", cwd, "error", err, "result", conditionMet)
		}
	case parser.CondCompare:
		left := p.ProcessLine(cond.Left)
		right := p.ProcessLine(cond.Right)

		left = pathutil.StripQuotes(left)
		right = pathutil.StripQuotes(right)

		isNumeric := false
		var lVal, rVal int
		if l, err := strconv.Atoi(left); err == nil {
			if r, err := strconv.Atoi(right); err == nil {
				isNumeric = true
				lVal = l
				rVal = r
			}
		}

		if n.CaseInsensitive && !isNumeric {
			left = strings.ToLower(left)
			right = strings.ToLower(right)
		}

		switch cond.Op {
		case parser.OpEqual, parser.OpEqu:
			if isNumeric {
				conditionMet = (lVal == rVal)
			} else {
				conditionMet = (left == right)
			}
		case parser.OpNeq:
			if isNumeric {
				conditionMet = (lVal != rVal)
			} else {
				conditionMet = (left != right)
			}
		case parser.OpLss:
			if isNumeric {
				conditionMet = (lVal < rVal)
			} else {
				conditionMet = (left < right)
			}
		case parser.OpLeq:
			if isNumeric {
				conditionMet = (lVal <= rVal)
			} else {
				conditionMet = (left <= right)
			}
		case parser.OpGtr:
			if isNumeric {
				conditionMet = (lVal > rVal)
			} else {
				conditionMet = (left > right)
			}
		case parser.OpGeq:
			if isNumeric {
				conditionMet = (lVal >= rVal)
			} else {
				conditionMet = (left >= right)
			}
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

func splitForSetItems(s string) []string {
	var result []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(s); i++ {
		c := s[i]

		if inQuote {
			current.WriteByte(c)
			if c == quoteChar {
				inQuote = false
				quoteChar = 0
			}
			continue
		}

		if c == '"' || c == '\'' {
			inQuote = true
			quoteChar = c
			current.WriteByte(c)
			continue
		}

		if c == ' ' || c == '\t' || c == ',' || c == ';' {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(c)
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

func (p *Processor) executeFor(n *parser.ForNode) error {
	oldForVars := p.ForVars
	p.ForVars = make(map[string]string)
	maps.Copy(p.ForVars, oldForVars)
	defer func() { p.ForVars = oldForVars }()

	if n.Variant == parser.ForFiles {
		for _, item := range n.Set {
			expandedItem := p.ProcessLine(item)
			for _, part := range splitForSetItems(expandedItem) {
				matches, err := pathutil.GlobCaseInsensitive(pathutil.MapPath(part))
				if err != nil || len(matches) == 0 {
					matches = []string{part}
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
			for _, part := range splitForSetItems(expandedItem) {
				mapped := pathutil.MapPath(part)
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
					if pathutil.MatchCaseInsensitive(pattern, e.Name()) {
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
			rootDir = pathutil.MapPath(opt)
		}
		var walkErr error
		err := filepath.Walk(rootDir, func(dirPath string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() {
				return nil
			}
			for _, item := range n.Set {
				expandedItem := p.ProcessLine(item)
				for _, part := range splitForSetItems(expandedItem) {
					fullPattern := filepath.Join(dirPath, part)
					if strings.ContainsAny(part, "*?") {
						matches, err := pathutil.GlobCaseInsensitive(fullPattern)
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

			if opts.usebackq {
				if strings.HasPrefix(item, "`") && strings.HasSuffix(item, "`") {
					isCommand = true
					rawItem = item[1 : len(item)-1]
				} else if strings.HasPrefix(item, "'") && strings.HasSuffix(item, "'") {
					isString = true
					rawItem = item[1 : len(item)-1]
				} else if strings.HasPrefix(item, "\"") && strings.HasSuffix(item, "\"") {
					rawItem = item[1 : len(item)-1]
				}
			} else {
				if strings.HasPrefix(item, "'") && strings.HasSuffix(item, "'") {
					isCommand = true
					rawItem = item[1 : len(item)-1]
				} else if strings.HasPrefix(item, "\"") && strings.HasSuffix(item, "\"") {
					isString = true
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
				path := pathutil.MapPath(p.ProcessLine(rawItem))
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
