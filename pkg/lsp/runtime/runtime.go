package runtime

import (
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/lexer"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
)

type MiniRuntime struct {
	nodes      []parser.Node
	tokens     []lexer.Item
	osEnv      map[string]string
	lineStates map[int]*State
	forVars    map[string]ForInfo
	tokenInfos []tokenInfo
}

func NewMiniRuntime(nodes []parser.Node) *MiniRuntime {
	return &MiniRuntime{
		nodes:   nodes,
		osEnv:   make(map[string]string),
		forVars: make(map[string]ForInfo),
	}
}

func NewMiniRuntimeWithTokens(nodes []parser.Node, tokens []lexer.Item) *MiniRuntime {
	return &MiniRuntime{
		nodes:   nodes,
		tokens:  tokens,
		osEnv:   make(map[string]string),
		forVars: make(map[string]ForInfo),
	}
}

func (r *MiniRuntime) Execute() *RuntimeResult {
	r.lineStates = make(map[int]*State)
	r.tokenInfos = make([]tokenInfo, 0)

	state := NewState()
	state = r.executeNodes(r.nodes, state) // Capture the returned state!

	return &RuntimeResult{
		Variables:  state.variables,
		ForVars:    r.forVars,
		LineStates: r.lineStates,
		FinalState: state,
		tokens:     r.tokenInfos,
	}
}

func (r *MiniRuntime) executeNodes(nodes []parser.Node, state *State) *State {
	for _, node := range nodes {
		state = r.executeNode(node, state)
		r.recordLineStateAfter(node, state)
	}
	return state
}

func (r *MiniRuntime) recordLineStateAfter(node parser.Node, state *State) {
	line := r.getStartLine(node)
	if line >= 0 {
		r.recordLineState(line, state)
	}
}

func (r *MiniRuntime) getStartLine(node parser.Node) int {
	switch n := node.(type) {
	case *parser.SimpleCommand:
		return n.Line
	case *parser.LabelNode:
		return n.Line
	case *parser.CommentNode:
		return n.Line
	default:
		return -1
	}
}

func (r *MiniRuntime) executeNode(node parser.Node, state *State) *State {
	switch n := node.(type) {
	case *parser.SimpleCommand:
		return r.executeCommand(n, state)
	case *parser.LabelNode:
		r.recordLineState(n.Line, state)
		return state
	case *parser.CommentNode:
		r.recordLineState(n.Line, state)
		return state
	case *parser.IfNode:
		return r.executeIf(n, state)
	case *parser.ForNode:
		return r.executeFor(n, state)
	case *parser.Block:
		return r.executeBlock(n, state)
	case *parser.BinaryNode:
		return r.executeBinary(n, state)
	case *parser.PipeNode:
		return r.executePipe(n, state)
	default:
		return state
	}
}

func (r *MiniRuntime) executeIf(node *parser.IfNode, state *State) *State {
	r.recordLineState(node.Line, state)

	thenState := state.Fork()
	thenState = r.executeNode(node.Then, thenState)

	var finalState *State
	if node.Else != nil {
		elseState := state.Fork()
		elseState = r.executeNode(node.Else, elseState)
		finalState = thenState.Merge(elseState)
	} else {
		finalState = thenState.Merge(state)
	}

	return finalState
}

func (r *MiniRuntime) executeFor(node *parser.ForNode, state *State) *State {
	r.recordLineState(node.Line, state)

	var iterType string
	var iterSource string

	switch node.Variant {
	case parser.ForRange:
		iterType = "range"
		if len(node.Set) > 0 {
			iterSource = "(" + strings.Join(node.Set, ",") + ")"
		}
	case parser.ForDir:
		iterType = "dir"
		iterSource = strings.Join(node.Set, " ")
	case parser.ForRecursive:
		iterType = "recursive"
		iterSource = strings.Join(node.Set, " ")
	case parser.ForF:
		iterType = "f_parse"
		iterSource = strings.Join(node.Set, " ")
	default:
		iterType = "files"
		iterSource = strings.Join(node.Set, " ")
	}

	forInfo := ForInfo{
		VarName:    strings.ToUpper(node.Variable),
		Line:       node.Line,
		IterType:   iterType,
		IterSource: iterSource,
	}
	r.forVars[forInfo.VarName] = forInfo

	if node.Do == nil {
		return state
	}

	var values []string
	if node.Variant == parser.ForRange && len(node.Set) >= 3 {
		start, _ := strconv.Atoi(node.Set[0])
		step, _ := strconv.Atoi(node.Set[1])
		end, _ := strconv.Atoi(node.Set[2])
		if step == 0 {
			step = 1
		}

		if step > 0 {
			for i := start; i <= end; i += step {
				values = append(values, strconv.Itoa(i))
			}
		} else if step < 0 {
			for i := start; i >= end; i += step {
				values = append(values, strconv.Itoa(i))
			}
		}
	} else {
		values = node.Set
	}

	var accumulatedState *State
	for i, val := range values {
		iterState := state.Fork()
		iterState.SetVar(forInfo.VarName, PossibleValue{
			Value:      val,
			SourceLine: node.Line,
			SourceType: "FOR_ITER",
		})
		iterState.SetForVar(node.Variable, val)
		resultState := r.executeNode(node.Do, iterState)
		if i == 0 {
			accumulatedState = resultState
		} else {
			accumulatedState = accumulatedState.Merge(resultState)
		}
	}

	if accumulatedState != nil {
		return accumulatedState.Merge(state)
	}
	return state
}

func (r *MiniRuntime) executeBlock(node *parser.Block, state *State) *State {
	r.recordLineState(node.Line, state)
	return r.executeNodes(node.Body, state)
}

func (r *MiniRuntime) executeBinary(node *parser.BinaryNode, state *State) *State {
	leftState := r.executeNode(node.Left, state)
	return r.executeNode(node.Right, leftState)
}

func (r *MiniRuntime) executePipe(node *parser.PipeNode, state *State) *State {
	leftState := r.executeNode(node.Left, state)
	return r.executeNode(node.Right, leftState)
}

func (r *MiniRuntime) executeCommand(cmd *parser.SimpleCommand, state *State) *State {
	r.recordLineState(cmd.Line, state)

	name := strings.ToUpper(cmd.Name)
	args := cmd.Args

	switch name {
	case "SET":
		r.executeSet(cmd, args, state)
	case "SETLOCAL":
		r.executeSetlocal(cmd, args, state)
	case "ENDLOCAL":
		r.executeEndlocal(cmd, state)
	}

	return state
}

func (r *MiniRuntime) executeSet(cmd *parser.SimpleCommand, args []string, state *State) {
	fullArg := strings.Join(cmd.RawArgs, "")
	if strings.HasPrefix(strings.ToUpper(fullArg), "/A ") {
		fullArg = fullArg[3:]
	}

	before, after, hasEq := strings.Cut(fullArg, "=")
	if !hasEq {
		return
	}
	varName := strings.ToUpper(strings.TrimSpace(before))
	value := after

	value = r.expandVariables(value, state)
	value = r.expandForVariables(value, state)

	pv := PossibleValue{
		Value:      value,
		SourceLine: cmd.Line,
		SourceType: "SET",
	}
	state.SetVar(varName, pv)
}

func (r *MiniRuntime) expandForVariables(src string, state *State) string {
	runes := []rune(src)
	var sb strings.Builder

	for i := 0; i < len(runes); {
		if runes[i] == '%' && i+2 < len(runes) && runes[i+1] == '%' {
			varName := string(runes[i+2])
			if val, ok := state.GetForVar(varName); ok {
				sb.WriteString(val)
				i += 3
				continue
			}
		}
		sb.WriteRune(runes[i])
		i++
	}

	return sb.String()
}

func (r *MiniRuntime) expandVariables(src string, state *State) string {
	var sb strings.Builder
	runes := []rune(src)

	for i := 0; i < len(runes); {
		if runes[i] == '%' && i+1 < len(runes) {
			end := r.findClosingPercent(runes, i+1)
			if end > i+1 {
				varName := string(runes[i+1 : end])
				expanded := r.resolveVar(varName, state)
				sb.WriteString(expanded)
				i = end + 1
				continue
			}
		}
		sb.WriteRune(runes[i])
		i++
	}

	return sb.String()
}

func (r *MiniRuntime) findClosingPercent(runes []rune, start int) int {
	for i := start; i < len(runes); i++ {
		if runes[i] == '%' {
			return i
		}
	}
	return -1
}

func (r *MiniRuntime) resolveVar(rawName string, state *State) string {
	varName := strings.ToUpper(rawName)

	v := state.GetVariable(varName)
	if v != nil && len(v.Values) > 0 {
		return v.Values[len(v.Values)-1].Value
	}

	if val, ok := r.osEnv[varName]; ok {
		return val
	}

	return "%" + rawName + "%"
}

func (r *MiniRuntime) executeSetlocal(cmd *parser.SimpleCommand, args []string, state *State) {
	state.SetScopeDepth(state.ScopeDepth() + 1)

	for _, arg := range args {
		argUpper := strings.ToUpper(arg)
		if argUpper == "ENABLEDELAYEDEXPANSION" {
			state.SetDelayedExpansion(true)
		} else if argUpper == "DISABLEDELAYEDEXPANSION" {
			state.SetDelayedExpansion(false)
		}
	}
}

func (r *MiniRuntime) executeEndlocal(cmd *parser.SimpleCommand, state *State) {
	if state.ScopeDepth() > 0 {
		state.SetScopeDepth(state.ScopeDepth() - 1)
	}
}

func (r *MiniRuntime) recordLineState(line int, state *State) {
	if r.lineStates == nil {
		r.lineStates = make(map[int]*State)
	}
	r.lineStates[line] = state.Fork()
}
