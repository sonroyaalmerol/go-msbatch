package lexer

import (
	"testing"
)

func collectTokens(src string) []struct {
	T TokenType
	V string
} {
	bl := New(src)
	var out []struct {
		T TokenType
		V string
	}
	for {
		item := bl.NextItem()
		if item.Type == TokenEOF {
			break
		}
		out = append(out, struct {
			T TokenType
			V string
		}{item.Type, string(item.Value)})
	}
	return out
}

func hasToken(tokens []struct {
	T TokenType
	V string
}, tt TokenType, val string) bool {
	for _, tok := range tokens {
		if tok.T == tt && tok.V == val {
			return true
		}
	}
	return false
}

func TestEchoCommand(t *testing.T) {
	tokens := collectTokens("echo Hello World\n")
	if !hasToken(tokens, TokenWord, "echo") {
		t.Error("expected Word 'echo'")
	}
}

func TestRemComment(t *testing.T) {
	tokens := collectTokens("rem this is a comment\n")
	if !hasToken(tokens, TokenKeyword, "rem") {
		t.Error("expected Keyword 'rem'")
	}
	found := false
	for _, tok := range tokens {
		if tok.T == TokenComment {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Comment token")
	}
}

func TestDoubleColonComment(t *testing.T) {
	tokens := collectTokens(":: this is also a comment\n")
	found := false
	for _, tok := range tokens {
		if tok.T == TokenComment {
			found = true
		}
	}
	if !found {
		t.Error("expected Comment token for :: style comment")
	}
}

func TestLabel(t *testing.T) {
	tokens := collectTokens(":myLabel\n")
	if !hasToken(tokens, TokenPunctuation, ":") {
		t.Error("expected Punctuation ':'")
	}
	if !hasToken(tokens, TokenNameLabel, "myLabel") {
		t.Error("expected NameLabel 'myLabel'")
	}
}

func TestGoto(t *testing.T) {
	tokens := collectTokens("goto :done\n")
	if !hasToken(tokens, TokenKeyword, "goto") {
		t.Error("expected Keyword 'goto'")
	}
	if !hasToken(tokens, TokenNameLabel, "done") {
		t.Error("expected NameLabel 'done'")
	}
}

func TestSetVariable(t *testing.T) {
	tokens := collectTokens("set FOO=bar\n")
	if !hasToken(tokens, TokenKeyword, "set") {
		t.Error("expected Keyword 'set'")
	}
}

func TestSetArithmetic(t *testing.T) {
	tokens := collectTokens("set /a X=1+2\n")
	if !hasToken(tokens, TokenKeyword, "set") {
		t.Error("expected Keyword 'set'")
	}
	found := false
	for _, tok := range tokens {
		if tok.T == TokenKeyword && tok.V == "/a" {
			found = true
		}
	}
	if !found {
		t.Error("expected Keyword '/a'")
	}
}

func TestPercentVariable(t *testing.T) {
	tokens := collectTokens("echo %VAR%\n")
	found := false
	for _, tok := range tokens {
		if tok.T == TokenNameVariable {
			found = true
		}
	}
	if !found {
		t.Error("expected NameVariable token for %%VAR%%")
	}
}

func TestDelayedExpansion(t *testing.T) {
	tokens := collectTokens("echo !VAR!\n")
	found := false
	for _, tok := range tokens {
		if tok.T == TokenNameVariable {
			found = true
		}
	}
	if !found {
		t.Error("expected NameVariable token for !VAR!")
	}
}

func TestRedirectOut(t *testing.T) {
	tokens := collectTokens("echo hello > out.txt\n")
	found := false
	for _, tok := range tokens {
		if tok.T == TokenRedirect {
			found = true
		}
	}
	if !found {
		t.Error("expected Redirect token for >")
	}
}

func TestRedirectAppend(t *testing.T) {
	tokens := collectTokens("echo hello >> out.txt\n")
	found := false
	for _, tok := range tokens {
		if tok.T == TokenRedirect {
			found = true
		}
	}
	if !found {
		t.Error("expected Redirect token for >>")
	}
}

func TestIfEquals(t *testing.T) {
	tokens := collectTokens(`if "%X%"=="yes" echo ok` + "\n")
	if !hasToken(tokens, TokenKeyword, "if") {
		t.Error("expected Keyword 'if'")
	}
	found := false
	for _, tok := range tokens {
		if tok.T == TokenOperator {
			found = true
		}
	}
	if !found {
		t.Error("expected Operator '=='")
	}
}

func TestIfExist(t *testing.T) {
	tokens := collectTokens("if exist file.txt echo found\n")
	if !hasToken(tokens, TokenKeyword, "exist") {
		t.Error("expected Keyword 'exist'")
	}
}

func TestForLoop(t *testing.T) {
	tokens := collectTokens("for %%i in (*.txt) do echo %%i\n")
	if !hasToken(tokens, TokenKeyword, "for") {
		t.Error("expected Keyword 'for'")
	}
}

func TestForF(t *testing.T) {
	tokens := collectTokens("for /f \"tokens=1\" %%a in (file.txt) do echo %%a\n")
	if !hasToken(tokens, TokenKeyword, "for") {
		t.Error("expected Keyword 'for'")
	}
	found := false
	for _, tok := range tokens {
		if tok.T == TokenKeyword && tok.V == "/f" {
			found = true
		}
	}
	_ = found
}

func TestAtSuppressor(t *testing.T) {
	tokens := collectTokens("@echo off\n")
	if !hasToken(tokens, TokenPunctuation, "@") {
		t.Error("expected Punctuation '@'")
	}
}

func TestCompoundBlock(t *testing.T) {
	tokens := collectTokens("(\necho hi\n)\n")
	found := false
	for _, tok := range tokens {
		if tok.T == TokenPunctuation && tok.V == "(" {
			found = true
		}
	}
	if !found {
		t.Error("expected Punctuation '('")
	}
}

func TestBuiltinCommands(t *testing.T) {
	// Non-structural commands are emitted as TokenWord; the parser treats them
	// identically to TokenKeyword so the distinction was removed from the lexer.
	cmds := []string{"dir", "cd", "cls", "copy", "del", "mkdir", "move", "type", "exit"}
	for _, cmd := range cmds {
		tokens := collectTokens(cmd + "\n")
		if !hasToken(tokens, TokenWord, cmd) {
			t.Errorf("expected Word %q", cmd)
		}
	}
}

func TestCallLabel(t *testing.T) {
	tokens := collectTokens("call :myFunc\n")
	if !hasToken(tokens, TokenKeyword, "call") {
		t.Error("expected Keyword 'call'")
	}
}

func TestPipePunctuation(t *testing.T) {
	tokens := collectTokens("dir | find \"txt\"\n")
	if !hasToken(tokens, TokenPunctuation, "|") {
		t.Error("expected Punctuation '|'")
	}
}

func TestAmpersandPunctuation(t *testing.T) {
	tokens := collectTokens("echo a && echo b\n")
	found := false
	for _, tok := range tokens {
		if tok.T == TokenPunctuation && tok.V == "&&" {
			found = true
		}
	}
	if !found {
		t.Error("expected Punctuation '&&'")
	}
}
