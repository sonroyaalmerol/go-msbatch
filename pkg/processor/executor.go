package processor

import (
	"bytes"
	"fmt"
	"io"
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
		// Blocks execute their own nodes
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
				if r.FD == 1 || r.FD == 0 {
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

	if p.ShouldEcho(n) {
		prompt, ok := p.Env.Get("PROMPT")
		if !ok {
			prompt = "$P$G"
		}

		expandedPrompt := prompt
		if strings.Contains(expandedPrompt, "$P") || strings.Contains(expandedPrompt, "$p") {
			pwd, _ := os.Getwd()
			expandedPrompt = strings.ReplaceAll(expandedPrompt, "$P", pwd)
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
	case "goto":
		label := strings.Join(expanded.Args, "")
		if strings.HasPrefix(label, ":") {
			label = label[1:]
		}
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
			for p.PC < len(p.Nodes) && !p.Exited {
				node := p.Nodes[p.PC]
				p.PC++
				if err := p.ExecuteNode(node); err != nil {
					if err.Error() == "EXIT_LOCAL" {
						p.PC = oldPC
						p.Args = oldArgs
						return nil
					}
					return err
				}
			}
			p.PC = oldPC
			p.Args = oldArgs
			return nil
		}
		return p.executeSimpleCommand(&parser.SimpleCommand{Name: target, Args: restArgs})
	case "echo", "echo.":
		output, stateChanged := p.HandleEchoBuiltin(expanded.Args)
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

		if len(expanded.Args) > 0 && strings.HasPrefix(strings.ToLower(expanded.Args[0]), "/a") {
			expr := arg[2:]
			_, err := p.EvalArithmetic(expr)
			if err != nil {
				fmt.Fprintf(p.Stderr, "Invalid number.\n")
				p.Env.Set("ERRORLEVEL", "1073741819")
			} else {
				p.Env.Set("ERRORLEVEL", "0")
			}
			return nil
		}

		if len(expanded.Args) > 0 && strings.HasPrefix(strings.ToLower(expanded.Args[0]), "/p") {
			promptStr := arg[2:]
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
			return fmt.Errorf("EXIT_LOCAL")
		}
		p.Exited = true
		return nil
	case "type":
		for _, arg := range expanded.Args {
			content, err := os.ReadFile(MapPath(arg))
			if err != nil {
				fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
				p.Env.Set("ERRORLEVEL", "1")
				continue
			}
			fmt.Fprint(p.Stdout, string(content))
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "cls":
		fmt.Fprint(p.Stdout, "\033[2J\033[H")
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "title":
		fmt.Fprintf(p.Stdout, "\033]0;%s\a", strings.Join(expanded.Args, " "))
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "ver":
		fmt.Fprintln(p.Stdout, "Microsoft Windows [Version 10.0.19045.5442]")
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "pause":
		fmt.Fprint(p.Stdout, "Press any key to continue . . . ")
		io.ReadFull(p.Stdin, make([]byte, 1)) //nolint:errcheck
		fmt.Fprintln(p.Stdout)
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "color":
		if len(expanded.Args) > 0 {
			code := expanded.Args[0]
			if len(code) == 2 {
				bg := code[0]
				fg := code[1]
				ansiColors := map[byte]string{
					'0': "30", '1': "34", '2': "32", '3': "36",
					'4': "31", '5': "35", '6': "33", '7': "37",
					'8': "90", '9': "94", 'a': "92", 'b': "96",
					'c': "91", 'd': "95", 'e': "93", 'f': "97",
					'A': "92", 'B': "96", 'C': "91", 'D': "95",
					'E': "93", 'F': "97",
				}
				bgCode := ansiColors[bg]
				fgCode := ansiColors[fg]
				if bgCode != "" && fgCode != "" {
					bgCode = strings.Replace(bgCode, "3", "4", 1)
					bgCode = strings.Replace(bgCode, "9", "10", 1)
					fmt.Fprintf(p.Stdout, "\033[%s;%sm", bgCode, fgCode)
				}
			}
		} else {
			fmt.Fprint(p.Stdout, "\033[0m")
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "pushd":
		pwd, _ := os.Getwd()
		p.DirStack = append(p.DirStack, pwd)
		if len(expanded.Args) > 0 {
			path := MapPath(expanded.Args[0])
			if err := os.Chdir(path); err != nil {
				fmt.Fprintf(p.Stderr, "The system cannot find the path specified.\n")
				p.DirStack = p.DirStack[:len(p.DirStack)-1]
				p.Env.Set("ERRORLEVEL", "1")
				return nil
			}
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "popd":
		if len(p.DirStack) > 0 {
			dir := p.DirStack[len(p.DirStack)-1]
			p.DirStack = p.DirStack[:len(p.DirStack)-1]
			if err := os.Chdir(dir); err != nil {
				fmt.Fprintf(p.Stderr, "The system cannot find the path specified.\n")
				p.Env.Set("ERRORLEVEL", "1")
				return nil
			}
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "mkdir", "md":
		for _, arg := range expanded.Args {
			if strings.HasPrefix(arg, "/") {
				continue
			}
			path := MapPath(arg)
			if err := os.MkdirAll(path, 0755); err != nil {
				fmt.Fprintf(p.Stderr, "A subdirectory or file %s already exists.\n", arg)
				p.Env.Set("ERRORLEVEL", "1")
				return nil
			}
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "rmdir", "rd":
		recursive := false
		var paths []string
		for _, arg := range expanded.Args {
			lower := strings.ToLower(arg)
			if lower == "/s" {
				recursive = true
				continue
			}
			if lower == "/q" {
				continue
			}
			paths = append(paths, arg)
		}
		for _, dirPath := range paths {
			path := MapPath(dirPath)
			var err error
			if recursive {
				err = os.RemoveAll(path)
			} else {
				err = os.Remove(path)
			}
			if err != nil {
				fmt.Fprintf(p.Stderr, "The directory is not empty.\n")
				p.Env.Set("ERRORLEVEL", "1")
				return nil
			}
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "del", "erase":
		recursive := false
		var patterns []string
		for _, arg := range expanded.Args {
			lower := strings.ToLower(arg)
			if lower == "/q" || lower == "/f" {
				continue
			}
			if lower == "/s" {
				recursive = true
				continue
			}
			if strings.HasPrefix(lower, "/a") {
				continue
			}
			patterns = append(patterns, arg)
		}
		for _, pat := range patterns {
			mapped := MapPath(pat)
			if recursive {
				dir := filepath.Dir(mapped)
				base := filepath.Base(mapped)
				filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return nil
					}
					if matched, _ := filepath.Match(base, filepath.Base(path)); matched {
						os.Remove(path)
					}
					return nil
				})
			} else {
				matches, err := filepath.Glob(mapped)
				if err == nil && len(matches) > 0 {
					for _, m := range matches {
						os.Remove(m)
					}
				} else {
					if os.Remove(mapped) != nil {
						fmt.Fprintf(p.Stderr, "Could Not Find %s\n", pat)
						p.Env.Set("ERRORLEVEL", "1")
						return nil
					}
				}
			}
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "copy":
		var args []string
		for _, arg := range expanded.Args {
			lower := strings.ToLower(arg)
			if lower == "/y" || lower == "/-y" || lower == "/b" || lower == "/a" {
				continue
			}
			args = append(args, arg)
		}
		if len(args) < 1 {
			fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
		dst := "."
		var srcs []string
		if len(args) >= 2 {
			dst = MapPath(args[len(args)-1])
			for _, s := range args[:len(args)-1] {
				mapped := MapPath(s)
				if matches, err := filepath.Glob(mapped); err == nil && len(matches) > 0 {
					srcs = append(srcs, matches...)
				} else {
					srcs = append(srcs, mapped)
				}
			}
		} else {
			srcs = []string{MapPath(args[0])}
			dst, _ = os.Getwd()
		}
		count := 0
		for _, src := range srcs {
			target := dst
			if info, err := os.Stat(dst); err == nil && info.IsDir() {
				target = filepath.Join(dst, filepath.Base(src))
			}
			if err := copyFile(src, target); err != nil {
				fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
				p.Env.Set("ERRORLEVEL", "1")
				return nil
			}
			count++
		}
		fmt.Fprintf(p.Stdout, "        %d file(s) copied.\n", count)
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "move":
		var args []string
		for _, arg := range expanded.Args {
			lower := strings.ToLower(arg)
			if lower == "/y" || lower == "/-y" {
				continue
			}
			args = append(args, arg)
		}
		if len(args) < 2 {
			fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
		src := MapPath(args[0])
		dst := MapPath(args[1])
		if info, err := os.Stat(dst); err == nil && info.IsDir() {
			dst = filepath.Join(dst, filepath.Base(src))
		}
		if err := os.Rename(src, dst); err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
		fmt.Fprintf(p.Stdout, "        1 file(s) moved.\n")
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	case "dir":
		dirPath := "."
		for _, arg := range expanded.Args {
			if !strings.HasPrefix(arg, "/") {
				dirPath = MapPath(arg)
				break
			}
		}
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			fmt.Fprintf(p.Stderr, "File Not Found\n")
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
		abs, _ := filepath.Abs(dirPath)
		fmt.Fprintf(p.Stdout, " Directory of %s\n\n", abs)
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			t := info.ModTime()
			if e.IsDir() {
				fmt.Fprintf(p.Stdout, "%s  %s    <DIR>          %s\n",
					t.Format("01/02/2006"), t.Format("03:04 PM"), e.Name())
			} else {
				fmt.Fprintf(p.Stdout, "%s  %s    %14d %s\n",
					t.Format("01/02/2006"), t.Format("03:04 PM"), info.Size(), e.Name())
			}
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}

	return p.runExternalCommand(expanded)
}

func (p *Processor) runExternalCommand(n *parser.SimpleCommand) error {
	cmdName := MapPath(n.Name)
	var mappedArgs []string
	for _, arg := range n.Args {
		mapped := arg
		if strings.Contains(arg, "\\") || (len(arg) >= 2 && arg[1] == ':') {
			mapped = MapPath(arg)
		}
		// Expand glob patterns (CMD.EXE passes globs to the OS; on Unix the shell
		// would normally do this, but exec.Command bypasses the shell).
		if strings.ContainsAny(mapped, "*?[") {
			if matches, err := filepath.Glob(mapped); err == nil && len(matches) > 0 {
				mappedArgs = append(mappedArgs, matches...)
				continue
			}
		}
		mappedArgs = append(mappedArgs, mapped)
	}

	cmd := exec.Command(cmdName, mappedArgs...)
	cmd.Stdout = p.Stdout
	cmd.Stderr = p.Stderr
	cmd.Stdin = p.Stdin
	cmd.Env = os.Environ()
	for k, v := range p.Env.Snapshot() {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			p.Env.Set("ERRORLEVEL", strconv.Itoa(exitErr.ExitCode()))
		} else {
			fmt.Fprintf(p.Stderr, "'%s' is not recognized as an internal or external command, operable program or batch file.\n", n.Name)
			p.Env.Set("ERRORLEVEL", "9009")
		}
	} else {
		p.Env.Set("ERRORLEVEL", "0")
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
				tokenMap := applyForTokens(line, parts, opts.delims, opts.tokens, n.Variable)
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

func applyForTokens(fullLine string, parts []string, delims string, tokens string, startVar string) map[string]string {
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

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func (p *Processor) captureCommandOutput(cmdLine string) (string, error) {
	expanded := p.ProcessLine(cmdLine)
	nodes := ParseExpanded(expanded)
	var buf bytes.Buffer
	subProc := New(p.Env, p.Args)
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
