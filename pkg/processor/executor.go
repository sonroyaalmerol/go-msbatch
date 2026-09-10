package processor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
)

// debugEnabled guards Debug calls whose argument evaluation costs syscalls.
func (p *Processor) debugEnabled() bool {
	return p.Logger.Enabled(context.Background(), slog.LevelDebug)
}

func (p *Processor) Execute(nodes []parser.Node) error {
	if p.debugEnabled() {
		cwd, _ := os.Getwd()
		p.Logger.Debug("executing nodes", "count", len(nodes), "env", p.Env.Snapshot(), "cwd", cwd)
	}
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
	if nodes, ok := p.respliceExpanded(n); ok {
		for _, sub := range nodes {
			if err := p.ExecuteNode(sub); err != nil {
				return err
			}
			if p.Exited {
				break
			}
		}
		return nil
	}
	if p.Debugger.Enabled {
		if action := p.checkDebugBreakpoint(n); action == ActionQuit {
			p.Exited = true
			return nil
		}
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
		p.echoIf(node)
		return p.executeIf(node)
	case *parser.ForNode:
		return p.executeFor(node)
	case *parser.BinaryNode:
		p.echoBinary(node)
		p.echoBlockDepth++
		defer func() { p.echoBlockDepth-- }()
		return p.executeBinary(node)
	case *parser.PipeNode:
		p.echoPipe(node)
		return p.executePipe(node)
	case *parser.LabelNode:
		return nil
	case *parser.CommentNode:
		p.echoComment(node)
		return nil
	case *parser.AbortNode:
		fmt.Fprintf(p.Stderr, "%s\n", node.Message)
		p.SetErrorLevel(255)
		p.Exited = true
		return nil
	default:
		return fmt.Errorf("unknown node type: %T", n)
	}
}

func (p *Processor) checkDebugBreakpoint(n parser.Node) DebugAction {
	pos := n.Pos()
	line := pos.Line + 1

	shouldBreak := false

	if p.Debugger.StepMode {
		switch n.(type) {
		case *parser.LabelNode, *parser.CommentNode:
		default:
			shouldBreak = true
		}
	}

	if IsBreakpointComment(n) {
		shouldBreak = true
	}

	if p.Debugger.IsBreakpoint(line) {
		shouldBreak = true
	}

	if shouldBreak {
		action := p.Debugger.Prompt(p, n)
		if action == ActionQuit {
			return ActionQuit
		}
	}

	return ActionContinue
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

func redirectOpString(k parser.RedirectKind) string {
	switch k {
	case parser.RedirectAppend:
		return ">>"
	case parser.RedirectIn:
		return "<"
	case parser.RedirectOutFD:
		return ">&"
	case parser.RedirectInFD:
		return "<&"
	}
	return ">"
}

// renderRawCommand renders trace text: env vars expanded via phase 1, FOR vars via phase 4 when active.
func (p *Processor) renderRawCommand(n *parser.SimpleCommand) string {
	text := p.ExpandPhase1(n.Name + strings.Join(n.RawArgs, ""))
	text = p.ExpandPhase4(text)
	for _, r := range n.Redirects {
		text += " " + strconv.Itoa(r.FD) + redirectOpString(r.Kind) + p.ExpandPhase1(r.Target)
	}
	return text
}

func (p *Processor) renderTraceLines(n parser.Node) []string {
	switch node := n.(type) {
	case *parser.SimpleCommand:
		return []string{p.renderRawCommand(node)}
	case *parser.BinaryNode:
		return []string{p.renderBinaryText(node)}
	case *parser.PipeNode:
		return []string{p.renderPipeText(node)}
	case *parser.Block:
		return p.renderBodyLines(node.Body)
	case *parser.IfNode:
		lines := []string{p.ifCondText(node) + " ("}
		lines = append(lines, decorateTraceBody(p.renderTraceNodeNoForVars(node.Then))...)
		return append(lines, ") ")
	case *parser.ForNode:
		lines := []string{p.forHeaderText(node)}
		lines = append(lines, decorateTraceBody(p.renderTraceNodeNoForVars(node.Do))...)
		return append(lines, ") ")
	}
	return nil
}

func (p *Processor) renderTraceNodeNoForVars(n parser.Node) []string {
	saved := p.ForVars
	p.ForVars = nil
	var out []string
	switch node := n.(type) {
	case *parser.SimpleCommand:
		out = []string{p.renderRawCommand(node)}
	case *parser.Block:
		out = p.renderBodyLines(node.Body)
	case *parser.IfNode:
		out = []string{p.ifCondText(node) + " ("}
		out = append(out, decorateTraceBody(p.renderTraceNodeNoForVars(node.Then))...)
		out = append(out, ") ")
	case *parser.ForNode:
		out = []string{p.forHeaderText(node)}
		out = append(out, decorateTraceBody(p.renderTraceNodeNoForVars(node.Do))...)
		out = append(out, ") ")
	case *parser.BinaryNode:
		op := binaryOpText(node.Op)
		out = []string{p.renderTraceNodeNoForVars1(node.Left) + "  " + op + " " + p.renderTraceNodeNoForVars1(node.Right)}
	case *parser.PipeNode:
		out = []string{p.renderTraceNodeNoForVars1(node.Left) + "  | " + p.renderTraceNodeNoForVars1(node.Right)}
	}
	p.ForVars = saved
	return out
}

func (p *Processor) renderTraceNodeNoForVars1(n parser.Node) string {
	lines := p.renderTraceNodeNoForVars(n)
	return strings.Join(lines, " ")
}

func (p *Processor) renderBodyLines(nodes []parser.Node) []string {
	var out []string
	for _, c := range nodes {
		out = append(out, p.renderTraceLines(c)...)
	}
	return out
}

// decorateTraceBody mirrors cmd body spacing: first line flush, later lines one leading space, two trailing spaces except one on the last.
func decorateTraceBody(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		lead := ""
		if i > 0 {
			lead = " "
		}
		trail := "  "
		if i == len(lines)-1 {
			trail = " "
		}
		out[i] = lead + l + trail
	}
	return out
}

func (p *Processor) emitTraceLines(lines []string) {
	prompt, ok := p.Env.Get("PROMPT")
	if !ok {
		prompt = "$P$G"
	}
	expanded := p.ExpandPrompt(prompt)
	fmt.Fprintln(p.Console)
	for i, l := range lines {
		if i == 0 {
			l = expanded + l
		}
		fmt.Fprintln(p.Console, l)
	}
}

func (p *Processor) echoTraceable(n parser.Node) bool {
	return p.Echo && p.echoBlockDepth == 0 && !parser.Suppressed(n)
}

func (p *Processor) echoComment(n *parser.CommentNode) {
	if !p.echoTraceable(n) || strings.HasPrefix(n.Text, ":") {
		return
	}
	p.emitTraceLines([]string{"rem" + p.ExpandPhase1(n.Text) + " "})
}

func (p *Processor) ifCondText(n *parser.IfNode) string {
	s := "if"
	if n.CaseInsensitive {
		s += " /I"
	}
	if n.Cond.Not {
		s += " not"
	}
	c := n.Cond
	switch c.Kind {
	case parser.CondCompare:
		s += " " + p.ExpandPhase1(c.Left) + " " + string(c.Op) + " " + p.ExpandPhase1(c.Right)
	case parser.CondExist:
		s += " exist " + p.ExpandPhase1(c.Arg)
	case parser.CondDefined:
		s += " defined " + p.ExpandPhase1(c.Arg)
	case parser.CondErrorLevel:
		s += " errorlevel " + strconv.Itoa(c.Level)
	case parser.CondCmdExtVersion:
		s += " cmdextversion " + strconv.Itoa(c.Level)
	}
	return s
}

type traceForm int

const (
	traceBare traceForm = iota
	traceInline
	traceMulti
)

// traceFormOf mirrors cmd: single-command block echoes inline, multi-command block multiline, bare command bare.
func traceFormOf(n parser.Node) traceForm {
	switch node := n.(type) {
	case *parser.SimpleCommand:
		return traceBare
	case *parser.Block:
		if len(node.Body) == 1 {
			return traceInline
		}
		return traceMulti
	}
	return traceInline
}

func (p *Processor) echoIf(n *parser.IfNode) {
	if !p.echoTraceable(n) {
		return
	}
	thenLines := p.renderTraceNodeNoForVars(n.Then)
	var lines []string
	switch traceFormOf(n.Then) {
	case traceBare:
		lines = []string{p.ifCondText(n) + " " + strings.Join(thenLines, " ") + " "}
	case traceInline:
		lines = []string{p.ifCondText(n) + " (" + strings.Join(thenLines, " ") + " ) "}
	case traceMulti:
		lines = []string{p.ifCondText(n) + " ("}
		lines = append(lines, decorateTraceBody(thenLines)...)
		lines = append(lines, ") ")
	}
	if n.Else != nil {
		elseLines := p.renderTraceNodeNoForVars(n.Else)
		if traceFormOf(n.Else) != traceMulti {
			lines[len(lines)-1] += " else (" + strings.Join(elseLines, " ") + " ) "
		} else {
			lines[len(lines)-1] += " else ("
			lines = append(lines, decorateTraceBody(elseLines)...)
			lines = append(lines, ") ")
		}
	}
	p.emitTraceLines(lines)
}

func binaryOpText(op parser.NodeKind) string {
	switch op {
	case parser.NodeAndThen:
		return "&&"
	case parser.NodeOrElse:
		return "||"
	}
	return "&"
}

func (p *Processor) renderBinaryText(n *parser.BinaryNode) string {
	return strings.Join(p.renderTraceLines(n.Left), " ") + "  " + binaryOpText(n.Op) + " " + strings.Join(p.renderTraceLines(n.Right), " ")
}

func (p *Processor) renderPipeText(n *parser.PipeNode) string {
	return strings.Join(p.renderTraceLines(n.Left), " ") + "  | " + strings.Join(p.renderTraceLines(n.Right), " ")
}

func (p *Processor) echoBinary(n *parser.BinaryNode) {
	if !p.echoTraceable(n) {
		return
	}
	p.emitTraceLines([]string{p.renderBinaryText(n) + " "})
}

func (p *Processor) echoPipe(n *parser.PipeNode) {
	if !p.echoTraceable(n) {
		return
	}
	p.emitTraceLines([]string{p.renderPipeText(n) + " "})
}

func (p *Processor) forHeaderText(n *parser.ForNode) string {
	s := "for"
	switch n.Variant {
	case parser.ForRange:
		s += " /L"
	case parser.ForDir:
		s += " /D"
	case parser.ForRecursive:
		s += " /R"
		if n.Options != "" {
			s += " " + p.ExpandPhase1(n.Options)
		}
	case parser.ForF:
		s += " /F"
		if n.Options != "" {
			opt := p.ExpandPhase1(n.Options)
			if !strings.HasPrefix(opt, `"`) {
				opt = `"` + opt + `"`
			}
			s += " " + opt
		}
	}
	s += " %" + n.Variable + " in ("
	sep := " "
	if n.Variant == parser.ForRange {
		sep = ","
	}
	items := make([]string, len(n.Set))
	for i, it := range n.Set {
		items[i] = p.ExpandPhase1(it)
	}
	return s + strings.Join(items, sep) + ") do "
}

func (p *Processor) echoForHeader(n *parser.ForNode) {
	if !p.echoTraceable(n) {
		return
	}
	header := p.forHeaderText(n)
	body := p.renderTraceNodeNoForVars(n.Do)
	var lines []string
	switch traceFormOf(n.Do) {
	case traceBare:
		lines = []string{header + strings.Join(body, " ") + " "}
	case traceInline:
		lines = []string{header + "(" + strings.Join(body, " ") + " ) "}
	case traceMulti:
		lines = []string{header + "("}
		lines = append(lines, decorateTraceBody(body)...)
		lines = append(lines, ") ")
	}
	p.emitTraceLines(lines)
}

func (p *Processor) execForBody(n *parser.ForNode) error {
	if p.Echo && p.echoBlockDepth == 0 {
		body := p.renderTraceLines(n.Do)
		var lines []string
		switch traceFormOf(n.Do) {
		case traceBare:
			lines = []string{strings.Join(body, " ") + " "}
		case traceInline:
			lines = []string{"(" + strings.Join(body, " ") + " ) "}
		case traceMulti:
			lines = []string{"("}
			lines = append(lines, decorateTraceBody(body)...)
			lines = append(lines, ") ")
		}
		p.emitTraceLines(lines)
	}
	p.echoBlockDepth++
	err := p.ExecuteNode(n.Do)
	p.echoBlockDepth--
	return err
}

func (p *Processor) executeBlock(node *parser.Block) error {
	if p.echoTraceable(node) {
		body := p.renderBodyLines(node.Body)
		var lines []string
		if traceFormOf(node) == traceMulti {
			lines = []string{"("}
			lines = append(lines, decorateTraceBody(body)...)
			lines = append(lines, ") ")
		} else {
			lines = []string{"(" + strings.Join(body, " ") + " ) "}
		}
		p.emitTraceLines(lines)
	}
	p.echoBlockDepth++
	defer func() { p.echoBlockDepth-- }()

	if len(node.Redirects) > 0 {
		var expandedRedirects []parser.Redirect
		for _, r := range node.Redirects {
			expandedRedirects = append(expandedRedirects, parser.Redirect{
				Kind:   r.Kind,
				Target: p.ExpandPhase4(p.ExpandPhase1(r.Target)),
				FD:     r.FD,
			})
		}
		rm := p.newRedirectManager()
		applied := rm.apply(p, expandedRedirects)
		defer rm.close(p)
		if !applied {
			return nil
		}
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
	return fmt.Errorf("%s - %s", "The system cannot find the batch label specified", labelName)
}

func expansionSource(n parser.Node) (string, bool) {
	switch node := n.(type) {
	case *parser.SimpleCommand:
		return node.Raw, node.PreExpanded
	case *parser.Block:
		return node.Raw, node.PreExpanded
	case *parser.IfNode:
		return node.Raw, node.PreExpanded
	case *parser.ForNode:
		return node.Raw, node.PreExpanded
	case *parser.BinaryNode:
		return node.Raw, node.PreExpanded
	case *parser.PipeNode:
		return node.Raw, node.PreExpanded
	default:
		return "", false
	}
}

func (p *Processor) respliceExpanded(n parser.Node) ([]parser.Node, bool) {
	raw, preExpanded := expansionSource(n)
	if preExpanded || raw == "" || !strings.Contains(raw, "%") {
		return nil, false
	}
	expanded := p.ExpandPhase1(raw)
	if expanded == raw || hasUnquotedOperator(raw) || !hasUnquotedOperator(expanded) {
		return nil, false
	}
	nodes := ParseExpanded(expanded)
	for _, sub := range nodes {
		parser.MarkPreExpanded(sub)
		if parser.Suppressed(n) {
			parser.SetSuppressed(sub)
		}
	}
	return nodes, true
}

// hasUnquotedOperator reports whether s contains a bare &, |, < or > outside
// quotes (honoring ^ escapes): the only expansions that change command structure.
func hasUnquotedOperator(s string) bool {
	if !strings.ContainsAny(s, "&|<>") {
		return false
	}
	var inQuote bool
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '^':
			i++
		case '"':
			inQuote = !inQuote
		case '&', '|', '<', '>':
			if !inQuote {
				return true
			}
		}
	}
	return false
}

func (p *Processor) executeSimpleCommand(n *parser.SimpleCommand) error {
	p.ExitCode = 0
	expanded := &parser.SimpleCommand{
		Suppressed:       n.Suppressed,
		RedirectsApplied: n.RedirectsApplied,
		Called:           n.Called,
	}
	expand := func(s string) string {
		if n.PreExpanded {
			return p.ExpandPhase4(s)
		}
		return p.ExpandPhase4(p.ExpandPhase1(s))
	}
	expanded.Name = strings.TrimSpace(expand(n.Name))
	buf := make([]string, 0, len(n.Args)+len(n.RawArgs))
	for _, arg := range n.Args {
		buf = append(buf, expand(arg))
	}
	expanded.Args = buf[:len(n.Args):len(n.Args)]
	for _, arg := range n.RawArgs {
		buf = append(buf, expand(arg))
	}
	expanded.RawArgs = buf[len(n.Args):]
	for _, r := range n.Redirects {
		expanded.Redirects = append(expanded.Redirects, parser.Redirect{
			Kind:   r.Kind,
			Target: strings.TrimSpace(expand(r.Target)),
			FD:     r.FD,
		})
	}

	if strings.ContainsFunc(expanded.Name, unicode.IsSpace) {
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
	}

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

		for _, r := range expanded.Redirects {
			sb.WriteString(" ")
			sb.WriteString(strconv.Itoa(r.FD))
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
		if len(expanded.RawArgs) > 0 || len(expanded.Redirects) > 0 {
			sb.WriteString(" ")
		}

		p.Logger.Debug("echo console output", "line", sb.String())
		fmt.Fprintln(p.Console)
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

	if p.debugEnabled() {
		cwd, _ := os.Getwd()
		p.Logger.Debug("executing command", "name", expanded.Name, "args", expanded.Args, "cwd", cwd)
	}

	filteredArgs := expanded.Args
	for _, arg := range expanded.Args {
		if strings.TrimSpace(arg) == "" && !(len(arg) >= 2 && (arg[0] == '"' || arg[0] == '\'')) {
			filteredArgs = nil
			for _, a := range expanded.Args {
				if strings.TrimSpace(a) != "" || (len(a) >= 2 && (a[0] == '"' || a[0] == '\'')) {
					filteredArgs = append(filteredArgs, a)
				}
			}
			break
		}
	}

	if len(expanded.Redirects) > 0 && !expanded.RedirectsApplied {
		rm := p.newRedirectManager()
		applied := rm.apply(p, expanded.Redirects)
		defer rm.close(p)
		expanded.RedirectsApplied = true
		if !applied {
			return nil
		}
	}

	name := strings.ToLower(expanded.Name)
	switch name {
	case "goto":
		cmdWords := expanded.Words()
		if len(cmdWords) == 0 {
			return nil
		}
		label := strings.Join(cmdWords, "")
		hasColon := strings.HasPrefix(label, ":")
		label = strings.TrimLeft(label, ":")
		label = strings.TrimRight(label, " \t;,=")
		p.Trace.GotoLabel(label)
		if hasColon && strings.ToLower(label) == "eof" {
			p.PC = len(p.Nodes)
			return nil
		}
		return p.jumpToLabel(label)
	case "call":
		cmdWords := expanded.Words()
		if len(cmdWords) == 0 {
			return nil
		}
		target := cmdWords[0]
		restArgs := cmdWords[1:]
		if strings.HasPrefix(target, ":") {
			label := strings.TrimLeft(target, ":")
			p.Trace.CallLabel(label, restArgs)
			p.Trace.Indent()
			p.Logger.Debug("entering subroutine", "label", label, "args", restArgs)
			oldPC := p.PC
			oldArgs := p.Args
			oldOriginalArgs := p.OriginalArgs
			oldEchoDepth := p.echoBlockDepth
			p.echoBlockDepth = 0
			p.Args = append([]string{target}, restArgs...)
			p.OriginalArgs = append([]string(nil), restArgs...)
			if err := p.jumpToLabel(label); err != nil {
				p.Args = oldArgs
				p.OriginalArgs = oldOriginalArgs
				p.echoBlockDepth = oldEchoDepth
				p.Trace.Dedent()
				fmt.Fprintln(p.Stderr, err)
				p.Failure()
				return nil
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
						p.OriginalArgs = oldOriginalArgs
						p.echoBlockDepth = oldEchoDepth
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
			p.OriginalArgs = oldOriginalArgs
			p.echoBlockDepth = oldEchoDepth
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
			Called:           true,
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

func (p *Processor) applyFD(fd int, stream any, rawStream any) {
	switch fd {
	case 0:
		if s, ok := stream.(io.Reader); ok {
			p.Stdin = s
		}
	case 1:
		if s, ok := stream.(io.Writer); ok {
			p.Stdout = s
		}
		if s, ok := rawStream.(io.Writer); ok {
			p.RawStdout = s
		}
	case 2:
		if s, ok := stream.(io.Writer); ok {
			p.Stderr = s
		}
		if s, ok := rawStream.(io.Writer); ok {
			p.RawStderr = s
		}
	default:
		if fd < 3 || fd > 9 {
			return
		}
		h := &FDHandle{}
		if s, ok := stream.(io.Writer); ok {
			h.W = s
		}
		if s, ok := rawStream.(io.Writer); ok {
			h.Raw = s
		}
		if s, ok := stream.(io.Reader); ok {
			h.R = s
		}
		if f, ok := stream.(*os.File); ok {
			h.File = f
		}
		if p.FDs == nil {
			p.FDs = make(map[int]*FDHandle)
		}
		p.FDs[fd] = h
	}
}

// bindRedirectWriter attaches an opened redirection target to fd. Handle 0 is
// treated as 1 for output redirects, matching how cmd.exe resolves "0>file".
func (p *Processor) bindRedirectWriter(fd int, w io.Writer, raw io.Writer, f *os.File) {
	if fd == 0 {
		fd = 1
	}
	p.applyFD(fd, w, raw)
	if f != nil {
		if h, ok := p.FDs[fd]; ok && h != nil {
			h.File = f
		}
	}
}

type redirectManager struct {
	origStdout    io.Writer
	origStdin     io.Reader
	origStderr    io.Writer
	origRawStdout io.Writer
	origRawStderr io.Writer
	origFDs       map[int]*FDHandle
	openedFiles   []*os.File
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
		origStdout:    p.Stdout,
		origStdin:     p.Stdin,
		origStderr:    p.Stderr,
		origRawStdout: p.RawStdout,
		origRawStderr: p.RawStderr,
		origFDs:       maps.Clone(p.FDs),
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
	p.RawStdout = rm.origRawStdout
	p.RawStderr = rm.origRawStderr
	p.FDs = rm.origFDs
}

// apply binds redirects and reports whether they all succeeded. cmd.exe skips
// the command entirely when a redirection fails, so the caller must not run it.
func (rm *redirectManager) apply(p *Processor, redirects []parser.Redirect) bool {
	ok := true
	for _, r := range redirects {
		if r.Kind == parser.RedirectBadDoubleIn {
			op := "<<"
			if r.FD >= 0 {
				op = strconv.Itoa(r.FD) + op
			}
			fmt.Fprintf(p.Stderr, "%s was unexpected at this time.\n", op)
			p.SetErrorLevel(255)
			p.Exited = true
			return false
		}
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
				p.bindRedirectWriter(r.FD, io.Discard, io.Discard, nil)
			} else {
				p.Trace.RedirectWrite(r.Target)
				f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
				if err == nil {
					rm.openedFiles = append(rm.openedFiles, f)
					dw := &debugWriter{underlying: f, logger: p.Logger, fd: r.FD, target: targetPath}
					p.bindRedirectWriter(r.FD, &NewCRLF{w: dw}, dw, f)
				} else {
					p.Logger.Debug("redirect open failed", "path", targetPath, "error", err)
				}
			}
		case parser.RedirectAppend:
			if isNul {
				p.Logger.Debug("redirect to nul (append)", "fd", r.FD)
				p.bindRedirectWriter(r.FD, io.Discard, io.Discard, nil)
			} else {
				p.Trace.RedirectAppend(r.Target)
				f, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
				if err == nil {
					rm.openedFiles = append(rm.openedFiles, f)
					dw := &debugWriter{underlying: f, logger: p.Logger, fd: r.FD, target: targetPath}
					p.bindRedirectWriter(r.FD, &NewCRLF{w: dw}, dw, f)
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
			src, convErr := strconv.Atoi(r.Target)
			if convErr != nil {
				continue
			}
			w, raw, rd, open := p.FDStreams(src)
			if !open {
				fmt.Fprintf(p.Stderr, "The handle could not be duplicated\nduring redirection of handle %d.\n", r.FD)
				p.Failure()
				ok = false
				continue
			}
			if w == nil && rd != nil {
				p.applyFD(r.FD, rd, nil)
				continue
			}
			p.applyFD(r.FD, w, raw)
		}
	}
	return ok
}

func (p *Processor) executeIf(n *parser.IfNode) error {
	conditionMet := false
	cond := n.Cond

	switch cond.Kind {
	case parser.CondExist:
		rawPath := p.ProcessLine(cond.Arg)
		path := pathutil.MapPath(rawPath)
		if strings.ContainsAny(path, "*?[") {
			matches, err := pathutil.GlobCaseInsensitive(path)
			conditionMet = (err == nil && len(matches) > 0)
			if p.debugEnabled() {
				cwd, _ := os.Getwd()
				p.Logger.Debug("IF EXIST check (wildcard)", "raw", rawPath, "mapped", path, "cwd", cwd, "matches", len(matches), "result", conditionMet)
			}
		} else {
			_, err := os.Stat(path)
			conditionMet = (err == nil)
			if p.debugEnabled() {
				cwd, _ := os.Getwd()
				p.Logger.Debug("IF EXIST check", "raw", rawPath, "mapped", path, "cwd", cwd, "error", err, "result", conditionMet)
			}
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
		p.echoBlockDepth++
		err := p.ExecuteNode(n.Then)
		p.echoBlockDepth--
		return err
	} else if n.Else != nil {
		p.echoBlockDepth++
		err := p.ExecuteNode(n.Else)
		p.echoBlockDepth--
		return err
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

func formatForPath(mapped, source string, absolute bool) string {
	if absolute {
		if path, err := filepath.Abs(mapped); err == nil {
			mapped = path
		}
		return pathutil.ToWindowsPath(mapped)
	}
	if pathutil.IsRooted(source) {
		return pathutil.ToWindowsPath(mapped)
	}
	if strings.Contains(source, `\`) {
		return strings.ReplaceAll(mapped, "/", `\`)
	}
	return mapped
}

func (p *Processor) executeFor(n *parser.ForNode) error {
	oldForVars := p.ForVars
	p.ForVars = make(map[string]string)
	maps.Copy(p.ForVars, oldForVars)
	p.echoForHeader(n)
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
					p.ForVars[n.Variable] = formatForPath(m, part, false)
					if err := p.execForBody(n); err != nil {
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
					if err := p.execForBody(n); err != nil {
						return err
					}
					if p.Exited {
						break
					}
				}
			} else if step < 0 {
				for i := start; i >= end; i += step {
					p.ForVars[n.Variable] = strconv.Itoa(i)
					if err := p.execForBody(n); err != nil {
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
						p.ForVars[n.Variable] = formatForPath(filepath.Join(dir, e.Name()), part, false)
						if err := p.execForBody(n); err != nil {
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
			opt = p.ExpandPhase5(p.ExpandPhase1(opt))
			if len(opt) >= 2 && opt[0] == '"' && opt[len(opt)-1] == '"' {
				opt = opt[1 : len(opt)-1]
			}
			rootDir = pathutil.MapPath(opt)
		}
		if path, err := filepath.Abs(rootDir); err == nil {
			rootDir = path
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
							p.ForVars[n.Variable] = formatForPath(m, part, true)
							if err := p.execForBody(n); err != nil {
								walkErr = err
								return errors.New("stop")
							}
							if p.Exited {
								return errors.New("stop")
							}
						}
					} else {
						p.ForVars[n.Variable] = formatForPath(fullPattern, part, true)
						if err := p.execForBody(n); err != nil {
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
				expandedPath := p.ProcessLine(rawItem)
				content, err := os.ReadFile(pathutil.MapPath(expandedPath))
				if err != nil {
					fmt.Fprintf(p.Stderr, "The system cannot find the file %s.\n", expandedPath)
				} else {
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
					if err := p.execForBody(n); err != nil {
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
		varName := string(baseChar + rune(i))
		if spec == "*" {
			startFrom := 0
			if lastIdx >= 0 {
				startFrom = lastIdx + 1
			}
			if startFrom < len(parts) {
				res[varName] = strings.Join(parts[startFrom:], " ")
			} else {
				res[varName] = ""
			}
			continue
		}
		idx, _ := strconv.Atoi(spec)
		if idx > 0 && idx <= len(parts) {
			res[varName] = parts[idx-1]
			if idx-1 > lastIdx {
				lastIdx = idx - 1
			}
			continue
		}
		if i == 0 {
			return nil
		}
		res[varName] = ""
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
	p.ExitCode = rightProcessor.ExitCode
	if leftErr != nil {
		return leftErr
	}
	return rightErr
}
