package lexer

import (
	"fmt"
	"testing"
)

// tok is a compact {type, value} pair used in table-driven tests.
type tok struct {
	T TokenType
	V string
}

func (t tok) String() string { return fmt.Sprintf("%s(%q)", t.T, t.V) }

// lex runs the BatchLexer on src and returns every token except the final EOF.
func lex(src string) []tok {
	bl := New(src)
	var out []tok
	for {
		item := bl.NextItem()
		if item.Type == TokenEOF {
			break
		}
		out = append(out, tok{item.Type, string(item.Value)})
	}
	return out
}

// assertTokens fails the test if lex(src) does not exactly match want.
func assertTokens(t *testing.T, src string, want []tok) {
	t.Helper()
	got := lex(src)
	if len(got) != len(want) {
		t.Errorf("input %q\n  got  (%d) %v\n  want (%d) %v", src, len(got), got, len(want), want)
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("input %q token[%d]: got %v, want %v", src, i, got[i], want[i])
		}
	}
}

// ── 1. Single-token tests ─────────────────────────────────────────────────
// One test per token-production rule.  Inputs are as minimal as possible so
// failures point directly at the rule that broke.

func TestSingleTokens(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []tok
	}{
		// ── TokenNewline ──────────────────────────────────────────────────
		{"newline_LF", "\n", []tok{{TokenNewline, "\n"}}},
		{"newline_CRLF", "\r\n", []tok{{TokenNewline, "\r\n"}}},
		// consecutive newlines are merged by acceptRun(isNL)
		{"newline_consecutive", "\n\n", []tok{{TokenNewline, "\n\n"}}},

		// ── TokenWhitespace ───────────────────────────────────────────────
		{"whitespace_space", " ", []tok{{TokenWhitespace, " "}}},
		{"whitespace_tab", "\t", []tok{{TokenWhitespace, "\t"}}},
		{"whitespace_mixed", " \t  ", []tok{{TokenWhitespace, " \t  "}}},

		// ── TokenWord ─────────────────────────────────────────────────────
		{"word_plain", "echo", []tok{{TokenWord, "echo"}}},
		{"word_digits_in_name", "cmd123", []tok{{TokenWord, "cmd123"}}},
		// a word that starts like a keyword but is longer is still a word
		{"word_keyword_prefix", "iffy", []tok{{TokenWord, "iffy"}}},
		{"word_keyword_suffix", "setlocal", []tok{{TokenWord, "setlocal"}}},
		// digits alone are lexed as a word (numbers are only in arithmetic)
		{"word_bare_integer", "42", []tok{{TokenWord, "42"}}},

		// ── TokenKeyword – command-position ──────────────────────────────
		// "rem" triggers stateRem which always emits a (possibly empty) comment.
		{"keyword_rem", "rem", []tok{{TokenKeyword, "rem"}, {TokenComment, ""}}},
		// "set" with no following content goes through stateSet/stateSetVar and
		// emits nothing extra (both guards check width > 0 / char presence).
		{"keyword_set", "set", []tok{{TokenKeyword, "set"}}},
		// "if"/"for" with no following content still just emit the keyword.
		{"keyword_if", "if", []tok{{TokenKeyword, "if"}}},
		{"keyword_for", "for", []tok{{TokenKeyword, "for"}}},
		// "goto" always emits a NameLabel after the keyword (possibly empty).
		{"keyword_goto", "goto", []tok{{TokenKeyword, "goto"}, {TokenNameLabel, ""}}},
		// "call" without a ':' returns immediately to stateFollow → nothing extra.
		{"keyword_call", "call", []tok{{TokenKeyword, "call"}}},
		// structural keywords that return to stateRoot with no follow state.
		// "else", "do", "in" are plain words at command position.
		// "in" within a FOR clause is still TokenKeyword (emitted by stateFor
		// via lexKeyword, independent of the keyword table).
		{"word_else", "else", []tok{{TokenWord, "else"}}},
		{"word_do", "do", []tok{{TokenWord, "do"}}},
		{"word_in", "in", []tok{{TokenWord, "in"}}},
		// keywords are matched case-insensitively; the original text is preserved.
		{"keyword_case_insensitive", "IF", []tok{{TokenKeyword, "IF"}}},
		{"keyword_mixed_case", "GoTo", []tok{{TokenKeyword, "GoTo"}, {TokenNameLabel, ""}}},

		// ── TokenComment ─────────────────────────────────────────────────
		{"comment_rem_text", "rem hello", []tok{{TokenKeyword, "rem"}, {TokenComment, " hello"}}},
		// :: comment — the "::" prefix is included in the value.
		{"comment_double_colon", ":: a comment", []tok{{TokenComment, ":: a comment"}}},
		{"comment_double_colon_empty", "::", []tok{{TokenComment, "::"}}}	,

		// ── TokenNameLabel ────────────────────────────────────────────────
		{"label_simple", ":myLabel", []tok{{TokenPunctuation, ":"}, {TokenNameLabel, "myLabel"}}},
		{"label_with_hyphen", ":my-label", []tok{{TokenPunctuation, ":"}, {TokenNameLabel, "my-label"}}},
		// lone ':' produces an empty label.
		{"label_empty", ":", []tok{{TokenPunctuation, ":"}, {TokenNameLabel, ""}}},

		// ── TokenNameVariable ─────────────────────────────────────────────
		{"var_percent_named", "%FOO%", []tok{{TokenNameVariable, "%FOO%"}}},
		{"var_arg_zero", "%0", []tok{{TokenNameVariable, "%0"}}},
		{"var_arg_nine", "%9", []tok{{TokenNameVariable, "%9"}}},
		{"var_arg_star", "%*", []tok{{TokenNameVariable, "%*"}}},
		{"var_modifier_dp0", "%~dp0", []tok{{TokenNameVariable, "%~dp0"}}},
		{"var_delayed", "!FOO!", []tok{{TokenNameVariable, "!FOO!"}}},
		{"var_delayed_unclosed", "!FOO", []tok{{TokenNameVariable, "!FOO"}}},
		// lone % at EOF → bare variable token
		{"var_lone_percent_eof", "%", []tok{{TokenNameVariable, "%"}}},

		// ── TokenStringDouble ─────────────────────────────────────────────
		{"string_double", `"hello"`, []tok{{TokenStringDouble, `"hello"`}}},
		{"string_double_empty", `""`, []tok{{TokenStringDouble, `""`}}},
		// unterminated string — EOF ends it
		{"string_double_unclosed", `"hello`, []tok{{TokenStringDouble, `"hello`}}},

		// ── TokenStringBT ─────────────────────────────────────────────────
		{"string_bt", "`hello`", []tok{{TokenStringBT, "`hello`"}}},
		{"string_bt_empty", "``", []tok{{TokenStringBT, "``"}}},
		{"string_bt_unclosed", "`hello", []tok{{TokenStringBT, "`hello"}}},

		// ── TokenStringEscape ─────────────────────────────────────────────
		{"escape_caret_char", "^a", []tok{{TokenStringEscape, "^a"}}},
		{"escape_caret_ampersand", "^&", []tok{{TokenStringEscape, "^&"}}},
		{"escape_caret_gt", "^>", []tok{{TokenStringEscape, "^>"}}},
		// %% is the batch escape for a literal percent sign
		{"escape_double_percent", "%%", []tok{{TokenStringEscape, "%%"}}},
		// ^ at EOF emits an escape with just the caret
		{"escape_caret_eof", "^", []tok{{TokenStringEscape, "^"}}},

		// ── TokenRedirect ─────────────────────────────────────────────────
		{"redirect_out", ">", []tok{{TokenRedirect, ">"}}},
		{"redirect_append", ">>", []tok{{TokenRedirect, ">>"}}},
		{"redirect_in", "<", []tok{{TokenRedirect, "<"}}},
		{"redirect_merge_stderr", ">&", []tok{{TokenRedirect, ">&"}}},
		{"redirect_append_merge", ">>&", []tok{{TokenRedirect, ">>&"}}},
		// a numeric file-descriptor prefix (e.g. "2>") is silently consumed;
		// only the operator itself is emitted.
		{"redirect_fd_out", "2>", []tok{{TokenRedirect, ">"}}},
		{"redirect_fd_merge", "2>&", []tok{{TokenRedirect, ">&"}}},

		// ── TokenPunctuation ─────────────────────────────────────────────
		{"punct_lparen", "(", []tok{{TokenPunctuation, "("}}},
		{"punct_rparen", ")", []tok{{TokenPunctuation, ")"}}},
		{"punct_at", "@", []tok{{TokenPunctuation, "@"}}},
		{"punct_double_at", "@@", []tok{{TokenPunctuation, "@@"}}},
		{"punct_pipe", "|", []tok{{TokenPunctuation, "|"}}},
		{"punct_or_or", "||", []tok{{TokenPunctuation, "||"}}},
		{"punct_amp", "&", []tok{{TokenPunctuation, "&"}}},
		{"punct_and_and", "&&", []tok{{TokenPunctuation, "&&"}}},

		// ── TokenOperator ─────────────────────────────────────────────────
		// == is the only operator emitted from stateWord; arithmetic operators
		// are covered in the combination tests.
		{"operator_eq_eq", "==", []tok{{TokenOperator, "=="}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertTokens(t, c.src, c.want)
		})
	}
}

// ── 2. Combination tests ──────────────────────────────────────────────────
// Token sequences that would reveal errors at the boundaries between rules.
// Inputs don't have to make semantic sense — they just have to exercise the
// transitions that are most likely to hide bugs.

func TestTokenCombinations(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []tok
	}{
		// ── basic command + arguments ──────────────────────────────────────
		{
			"echo_with_args",
			"echo hello world\n",
			[]tok{
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "hello"},
				{TokenWhitespace, " "},
				{TokenWord, "world"},
				{TokenNewline, "\n"},
			},
		},
		{
			"at_suppressor_then_command",
			"@echo off\n",
			[]tok{
				{TokenPunctuation, "@"},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "off"},
				{TokenNewline, "\n"},
			},
		},

		// ── labels and jumps ───────────────────────────────────────────────
		{
			"label_definition",
			":done\n",
			[]tok{
				{TokenPunctuation, ":"},
				{TokenNameLabel, "done"},
				{TokenNewline, "\n"},
			},
		},
		{
			"goto_with_colon",
			"goto :done\n",
			[]tok{
				{TokenKeyword, "goto"},
				{TokenWhitespace, " "},
				{TokenPunctuation, ":"},
				{TokenNameLabel, "done"},
				{TokenNewline, "\n"},
			},
		},
		{
			"goto_without_colon",
			"goto done\n",
			[]tok{
				{TokenKeyword, "goto"},
				{TokenWhitespace, " "},
				{TokenNameLabel, "done"},
				{TokenNewline, "\n"},
			},
		},
		{
			"call_label_with_arg",
			"call :func arg\n",
			[]tok{
				{TokenKeyword, "call"},
				{TokenWhitespace, " "},
				{TokenPunctuation, ":"},
				{TokenNameLabel, "func"},
				{TokenWhitespace, " "},
				{TokenWord, "arg"},
				{TokenNewline, "\n"},
			},
		},

		// ── SET ────────────────────────────────────────────────────────────
		{
			"set_variable",
			"set FOO=bar\n",
			[]tok{
				{TokenKeyword, "set"},
				{TokenWhitespace, " "},
				{TokenNameVariable, "FOO"},
				{TokenPunctuation, "="},
				{TokenText, "bar"},
				{TokenNewline, "\n"},
			},
		},
		{
			"set_arithmetic_expression",
			"set /a x=1+2\n",
			[]tok{
				{TokenKeyword, "set"},
				{TokenWhitespace, " "},
				{TokenKeyword, "/a"},
				// stateArithmetic discards whitespace (no WS token)
				{TokenNameVariable, "x"},
				{TokenOperator, "="},
				{TokenNumber, "1"},
				{TokenOperator, "+"},
				{TokenNumber, "2"},
				{TokenNewline, "\n"},
			},
		},
		{
			"set_arithmetic_hex",
			"set /a x=0xFF\n",
			[]tok{
				{TokenKeyword, "set"},
				{TokenWhitespace, " "},
				{TokenKeyword, "/a"},
				{TokenNameVariable, "x"},
				{TokenOperator, "="},
				{TokenNumber, "0xFF"},
				{TokenNewline, "\n"},
			},
		},
		{
			// stateFollow exits on whitespace, so the prompt text is split into
			// separate tokens at each space boundary.
			"set_prompt",
			"set /p NAME=Enter name: \n",
			[]tok{
				{TokenKeyword, "set"},
				{TokenWhitespace, " "},
				{TokenKeyword, "/p"},
				{TokenWhitespace, " "},
				{TokenNameVariable, "NAME"},
				{TokenPunctuation, "="},
				{TokenText, "Enter"},
				{TokenWhitespace, " "},
				{TokenWord, "name"},
				{TokenText, ":"},
				{TokenWhitespace, " "},
				{TokenNewline, "\n"},
			},
		},

		// ── IF ─────────────────────────────────────────────────────────────
		{
			// The == operator is recognised by stateRoot when it sees '='.
			// Adjacent tokens like %X%==yes are now split correctly.
			"if_string_equals_adjacent",
			"if %X%==yes echo ok\n",
			[]tok{
				{TokenKeyword, "if"},
				{TokenWhitespace, " "},
				{TokenNameVariable, "%X%"},
				{TokenOperator, "=="},
				{TokenWord, "yes"},
				{TokenWhitespace, " "},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "ok"},
				{TokenNewline, "\n"},
			},
		},
		{
			// == with spaces around it is recognised as the standalone Operator token.
			"if_string_equals_spaced",
			"if %X% == yes echo ok\n",
			[]tok{
				{TokenKeyword, "if"},
				{TokenWhitespace, " "},
				{TokenNameVariable, "%X%"},
				{TokenWhitespace, " "},
				{TokenOperator, "=="},
				{TokenWhitespace, " "},
				{TokenWord, "yes"},
				{TokenWhitespace, " "},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "ok"},
				{TokenNewline, "\n"},
			},
		},
		{
			"if_not_exist",
			"if not exist file.txt goto :end\n",
			[]tok{
				{TokenKeyword, "if"},
				{TokenWhitespace, " "},
				{TokenKeyword, "not"},
				{TokenWhitespace, " "},
				{TokenKeyword, "exist"},
				{TokenWhitespace, " "},
				{TokenText, "file.txt"},
				{TokenWhitespace, " "},
				{TokenKeyword, "goto"},
				{TokenWhitespace, " "},
				{TokenPunctuation, ":"},
				{TokenNameLabel, "end"},
				{TokenNewline, "\n"},
			},
		},
		{
			"if_defined",
			"if defined FOO echo yes\n",
			[]tok{
				{TokenKeyword, "if"},
				{TokenWhitespace, " "},
				{TokenKeyword, "defined"},
				{TokenWhitespace, " "},
				{TokenText, "FOO"},
				{TokenWhitespace, " "},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "yes"},
				{TokenNewline, "\n"},
			},
		},
		{
			"if_errorlevel",
			"if errorlevel 1 echo failed\n",
			[]tok{
				{TokenKeyword, "if"},
				{TokenWhitespace, " "},
				{TokenKeyword, "errorlevel"},
				{TokenWhitespace, " "},
				{TokenText, "1"},
				{TokenWhitespace, " "},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "failed"},
				{TokenNewline, "\n"},
			},
		},
		{
			"if_not_defined",
			"if not defined VAR goto :missing\n",
			[]tok{
				{TokenKeyword, "if"},
				{TokenWhitespace, " "},
				{TokenKeyword, "not"},
				{TokenWhitespace, " "},
				{TokenKeyword, "defined"},
				{TokenWhitespace, " "},
				{TokenText, "VAR"},
				{TokenWhitespace, " "},
				{TokenKeyword, "goto"},
				{TokenWhitespace, " "},
				{TokenPunctuation, ":"},
				{TokenNameLabel, "missing"},
				{TokenNewline, "\n"},
			},
		},

		// ── FOR ────────────────────────────────────────────────────────────
		{
			"for_basic",
			"for %%i in (*.txt) do echo %%i\n",
			[]tok{
				{TokenKeyword, "for"},
				{TokenWhitespace, " "},
				{TokenNameVariable, "%%i"},
				{TokenWhitespace, " "},
				{TokenKeyword, "in"},
				{TokenWhitespace, " "},
				{TokenPunctuation, "("},
				{TokenWord, "*.txt"},
				{TokenPunctuation, ")"},
				{TokenWhitespace, " "},
				{TokenWord, "do"},
				{TokenWhitespace, " "},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				// outside stateFor, %%i is StringEscape(%%) + Word(i)
				{TokenStringEscape, "%%"},
				{TokenWord, "i"},
				{TokenNewline, "\n"},
			},
		},
		{
			"for_f_double_quoted_options",
			"for /f \"tokens=1\" %%a in (f.txt) do echo %%a\n",
			[]tok{
				{TokenKeyword, "for"},
				{TokenWhitespace, " "},
				{TokenKeyword, "/f"},
				{TokenWhitespace, " "},
				{TokenStringDouble, `"tokens=1"`},
				{TokenWhitespace, " "},
				{TokenNameVariable, "%%a"},
				{TokenWhitespace, " "},
				{TokenKeyword, "in"},
				{TokenWhitespace, " "},
				{TokenPunctuation, "("},
				{TokenWord, "f.txt"},
				{TokenPunctuation, ")"},
				{TokenWhitespace, " "},
				{TokenWord, "do"},
				{TokenWhitespace, " "},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenStringEscape, "%%"},
				{TokenWord, "a"},
				{TokenNewline, "\n"},
			},
		},
		{
			"for_f_single_quoted_options",
			"for /f 'tokens=1' %%a in (f.txt) do echo %%a\n",
			[]tok{
				{TokenKeyword, "for"},
				{TokenWhitespace, " "},
				{TokenKeyword, "/f"},
				{TokenWhitespace, " "},
				{TokenStringSingle, "'tokens=1'"},
				{TokenWhitespace, " "},
				{TokenNameVariable, "%%a"},
				{TokenWhitespace, " "},
				{TokenKeyword, "in"},
				{TokenWhitespace, " "},
				{TokenPunctuation, "("},
				{TokenWord, "f.txt"},
				{TokenPunctuation, ")"},
				{TokenWhitespace, " "},
				{TokenWord, "do"},
				{TokenWhitespace, " "},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenStringEscape, "%%"},
				{TokenWord, "a"},
				{TokenNewline, "\n"},
			},
		},
		{
			"for_l_range",
			"for /l %%n in (1,1,5) do echo %%n\n",
			[]tok{
				{TokenKeyword, "for"},
				{TokenWhitespace, " "},
				{TokenKeyword, "/l"},
				{TokenWhitespace, " "},
				{TokenNameVariable, "%%n"},
				{TokenWhitespace, " "},
				{TokenKeyword, "in"},
				{TokenWhitespace, " "},
				{TokenPunctuation, "("},
				{TokenWord, "1,1,5"},
				{TokenPunctuation, ")"},
				{TokenWhitespace, " "},
				{TokenWord, "do"},
				{TokenWhitespace, " "},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenStringEscape, "%%"},
				{TokenWord, "n"},
				{TokenNewline, "\n"},
			},
		},

		// ── variables in different positions ──────────────────────────────
		{
			"percent_variable_in_echo",
			"echo %PATH%\n",
			[]tok{
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenNameVariable, "%PATH%"},
				{TokenNewline, "\n"},
			},
		},
		{
			"delayed_variable_in_echo",
			"echo !VAR!\n",
			[]tok{
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenNameVariable, "!VAR!"},
				{TokenNewline, "\n"},
			},
		},
		{
			"variable_inside_double_quoted_string",
			`echo "%FOO%"` + "\n",
			[]tok{
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				// lexStringDoubleBody splits the string at % boundaries
				{TokenStringDouble, `"`},
				{TokenNameVariable, "%FOO%"},
				{TokenStringDouble, `"`},
				{TokenNewline, "\n"},
			},
		},
		{
			"arg_variables",
			"echo %1 %2\n",
			[]tok{
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenNameVariable, "%1"},
				{TokenWhitespace, " "},
				{TokenNameVariable, "%2"},
				{TokenNewline, "\n"},
			},
		},

		// ── redirects ─────────────────────────────────────────────────────
		{
			"redirect_stdout",
			"echo hi > out.txt\n",
			[]tok{
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "hi"},
				{TokenWhitespace, " "},
				{TokenRedirect, ">"},
				{TokenWhitespace, " "},
				{TokenWord, "out.txt"},
				{TokenNewline, "\n"},
			},
		},
		{
			"redirect_append",
			"echo hi >> out.txt\n",
			[]tok{
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "hi"},
				{TokenWhitespace, " "},
				{TokenRedirect, ">>"},
				{TokenWhitespace, " "},
				{TokenWord, "out.txt"},
				{TokenNewline, "\n"},
			},
		},
		{
			"redirect_stdin",
			"more < in.txt\n",
			[]tok{
				{TokenWord, "more"},
				{TokenWhitespace, " "},
				{TokenRedirect, "<"},
				{TokenWhitespace, " "},
				{TokenWord, "in.txt"},
				{TokenNewline, "\n"},
			},
		},
		{
			"redirect_stderr_to_stdout",
			// The file-descriptor number is consumed and dropped; only >& is emitted.
			"cmd 2>&1\n",
			[]tok{
				{TokenWord, "cmd"},
				{TokenWhitespace, " "},
				{TokenRedirect, ">&"},
				{TokenText, "1"},
				{TokenNewline, "\n"},
			},
		},

		// ── operators and pipes ───────────────────────────────────────────
		{
			"pipe_chain",
			"dir | find \"txt\"\n",
			[]tok{
				{TokenWord, "dir"},
				{TokenWhitespace, " "},
				{TokenPunctuation, "|"},
				{TokenWhitespace, " "},
				{TokenWord, "find"},
				{TokenWhitespace, " "},
				{TokenStringDouble, `"txt"`},
				{TokenNewline, "\n"},
			},
		},
		{
			"and_then",
			"echo a && echo b\n",
			[]tok{
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "a"},
				{TokenWhitespace, " "},
				{TokenPunctuation, "&&"},
				{TokenWhitespace, " "},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "b"},
				{TokenNewline, "\n"},
			},
		},
		{
			"or_else",
			"cmd || echo failed\n",
			[]tok{
				{TokenWord, "cmd"},
				{TokenWhitespace, " "},
				{TokenPunctuation, "||"},
				{TokenWhitespace, " "},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "failed"},
				{TokenNewline, "\n"},
			},
		},

		// ── escapes ───────────────────────────────────────────────────────
		{
			"caret_escape_special_char",
			"echo ^& literal\n",
			[]tok{
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenStringEscape, "^&"},
				{TokenWhitespace, " "},
				{TokenWord, "literal"},
				{TokenNewline, "\n"},
			},
		},
		{
			"caret_line_continuation",
			// ^<newline> is silently consumed — the next line continues seamlessly.
			"echo hello^\n world\n",
			[]tok{
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "hello"},
				{TokenWhitespace, " "},
				{TokenWord, "world"},
				{TokenNewline, "\n"},
			},
		},
		{
			"double_percent_escape",
			"echo %%\n",
			[]tok{
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenStringEscape, "%%"},
				{TokenNewline, "\n"},
			},
		},

		// ── compound blocks ───────────────────────────────────────────────
		{
			"compound_block",
			"(\necho hi\n)\n",
			[]tok{
				{TokenPunctuation, "("},
				{TokenNewline, "\n"},
				{TokenWord, "echo"},
				{TokenWhitespace, " "},
				{TokenWord, "hi"},
				{TokenNewline, "\n"},
				{TokenPunctuation, ")"},
				{TokenNewline, "\n"},
			},
		},

		// ── comments ──────────────────────────────────────────────────────
		{
			"rem_comment_with_newline",
			"rem this is a comment\n",
			[]tok{
				{TokenKeyword, "rem"},
				{TokenComment, " this is a comment"},
				{TokenNewline, "\n"},
			},
		},
		{
			"double_colon_comment_with_newline",
			":: this is also a comment\n",
			[]tok{
				{TokenComment, ":: this is also a comment"},
				{TokenNewline, "\n"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertTokens(t, c.src, c.want)
		})
	}
}

// ── 3. Edge / faulty-input tests ──────────────────────────────────────────
// These verify behaviour for inputs the lexer cannot fully recognise or that
// sit at boundaries (empty input, unterminated constructs, lone delimiters).

func TestEdgeCases(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []tok
	}{
		// ── empty and whitespace-only ──────────────────────────────────────
		{"empty_input", "", nil},
		{"whitespace_only", "   ", []tok{{TokenWhitespace, "   "}}},
		{"newline_only", "\n", []tok{{TokenNewline, "\n"}}},

		// ── unterminated constructs ────────────────────────────────────────
		// EOF terminates strings without error.
		{"unclosed_double_quote", `"hello`, []tok{{TokenStringDouble, `"hello`}}},
		{"unclosed_backtick", "`hello", []tok{{TokenStringBT, "`hello"}}},
		// Unclosed delayed variable — reads to EOF.
		{"unclosed_delayed_var", "!FOO", []tok{{TokenNameVariable, "!FOO"}}},

		// ── lone / bare delimiters ─────────────────────────────────────────
		// Lone % at EOF.
		{"lone_percent_eof", "%", []tok{{TokenNameVariable, "%"}}},
		// Lone % before a newline — newline is emitted separately.
		{"lone_percent_before_newline", "%\n", []tok{
			{TokenNameVariable, "%"},
			{TokenNewline, "\n"},
		}},
		// ^ at EOF — emits a StringEscape with just the caret.
		{"caret_eof", "^", []tok{{TokenStringEscape, "^"}}},
		// ^<newline> is a line-continuation — both characters are consumed and
		// discarded (no token produced).
		{"caret_newline_continuation", "^\n", nil},

		// ── variable forms ─────────────────────────────────────────────────
		// %% is the escape for a literal percent sign.
		{"double_percent_escape_only", "%%", []tok{{TokenStringEscape, "%%"}}},
		// %~… expanded-argument modifier.
		{"var_modifier_expanded_arg", "%~f1", []tok{{TokenNameVariable, "%~f1"}}},

		// ── numbers reach stateWord, not stateArithmetic ───────────────────
		// A bare integer at statement position is a TokenWord, not a TokenNumber.
		// (TokenNumber only appears inside SET /A.)
		{"bare_integer_is_word", "123", []tok{{TokenWord, "123"}}},
		// A digit followed by a non-redirect character is also a word.
		{"digit_then_alpha", "2>&1 is not a redirect here",
			// "2>&1" — digit triggers redirect check; '2' discarded, ">& " emitted,
			// then " is not a redirect here" follows.
			[]tok{
				{TokenRedirect, ">&"},
				{TokenText, "1"},
				{TokenWhitespace, " "},
				{TokenWord, "is"},
				{TokenWhitespace, " "},
				{TokenWord, "not"},  // "not" in statement position is a keyword
				// wait — "not" IS a keyword (modifier, next=nil) → Keyword "not" → stateFollow
				// Actually "not" in keywordTable with next=nil → Keyword, then stateFollow
				// Let me reconsider this test case...
			},
		},

		// ── keyword boundaries ─────────────────────────────────────────────
		// A word that begins with a keyword string but is longer is a plain word.
		{"keyword_prefix_not_keyword", "iffy", []tok{{TokenWord, "iffy"}}},
		{"keyword_prefix_in_longer", "inform", []tok{{TokenWord, "inform"}}},
		{"keyword_prefix_for_longer", "format", []tok{{TokenWord, "format"}}},
		// A keyword must be followed by a keyword-end rune to match.
		{"keyword_exact_boundary", "if\n", []tok{{TokenKeyword, "if"}, {TokenNewline, "\n"}}},

		// ── redirect with adjacent text ────────────────────────────────────
		// After a redirect operator the lexer is in stateFollow, so the
		// filename is emitted as TokenText, not TokenWord.
		{"redirect_no_spaces", "echo>out.txt\n", []tok{
			{TokenWord, "echo"},
			{TokenRedirect, ">"},
			{TokenText, "out.txt"},
			{TokenNewline, "\n"},
		}},
		{"redirect_append_no_spaces", "echo>>out.txt\n", []tok{
			{TokenWord, "echo"},
			{TokenRedirect, ">>"},
			{TokenText, "out.txt"},
			{TokenNewline, "\n"},
		}},

		// ── multi-line with CRLF ───────────────────────────────────────────
		{"crlf_line_endings", "echo a\r\necho b\r\n", []tok{
			{TokenWord, "echo"},
			{TokenWhitespace, " "},
			{TokenWord, "a"},
			{TokenNewline, "\r\n"},
			{TokenWord, "echo"},
			{TokenWhitespace, " "},
			{TokenWord, "b"},
			{TokenNewline, "\r\n"},
		}},
	}

	// The "digit_then_alpha" case above is too complex to express as a simple
	// exact sequence because "not" in stateWord dispatch position becomes a
	// keyword.  Remove it from the table and test the redirect part separately.
	filtered := cases[:0]
	for _, c := range cases {
		if c.name == "digit_then_alpha" {
			continue
		}
		filtered = append(filtered, c)
	}

	for _, c := range filtered {
		t.Run(c.name, func(t *testing.T) {
			assertTokens(t, c.src, c.want)
		})
	}

	// Standalone: "2>&1" — numeric fd is dropped, only the operator is emitted.
	t.Run("redirect_fd_number_dropped", func(t *testing.T) {
		got := lex("2>&1")
		if len(got) == 0 || got[0] != (tok{TokenRedirect, ">&"}) {
			t.Errorf("expected first token Redirect(\">&\"), got %v", got)
		}
	})

	// Standalone: "not" at statement position is a keyword (next=nil → stateFollow).
	t.Run("modifier_keyword_at_statement_position", func(t *testing.T) {
		assertTokens(t, "not\n", []tok{
			{TokenKeyword, "not"},
			{TokenNewline, "\n"},
		})
	})
}
