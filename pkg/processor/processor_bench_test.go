package processor_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// benchBatch mixes the chain's constructs: echo, set, set /a, if/else, for /l, for-in, rem.
func benchBatch() string {
	var sb strings.Builder
	sb.WriteString("@echo off\r\n")
	for i := range 20 {
		fmt.Fprintf(&sb, "set VAR%d=some-value-%d\r\n", i, i)
		fmt.Fprintf(&sb, "if \"%%VAR1%%\"==\"some-value-1\" ( echo match%d ) else ( echo miss )\r\n", i)
		fmt.Fprintf(&sb, "for /L %%j in (1,1,5) do set /a ACC%d=%%j*2+1\r\n", i)
		sb.WriteString("for %%k in (a b c) do echo item %%k\r\n")
		sb.WriteString("rem a comment line with some text to lex through\r\n")
	}
	sb.WriteString("echo done %VAR1% %VAR2%\r\n")
	return sb.String()
}

func BenchmarkParseExpanded(b *testing.B) {
	src := benchBatch()
	for b.Loop() {
		nodes := processor.ParseExpanded(src)
		if len(nodes) == 0 {
			b.Fatal("no nodes")
		}
	}
}

func BenchmarkPhase1PercentExpand(b *testing.B) {
	env := processor.NewEnvironment(true)
	env.Set("A", "alpha-value")
	env.Set("B", "beta-value")
	env.Set("C", "gamma-value")
	src := "set X=%A%-%B%-%C% & echo %A%/%B%/%C% & if \"%A%\"==\"%B%\" echo %C%"
	for b.Loop() {
		_ = processor.Phase1PercentExpand(src, env, nil, nil)
	}
}

func BenchmarkExecuteScript(b *testing.B) {
	src := "@echo off\r\n" +
		strings.Repeat(
			"for /L %%i in (1,1,50) do (\r\n set /a X=%%i*3+1\r\n set VAR=v%X%\r\n)\r\n"+
				"if \"%VAR%\"==\"v%ERRORLEVEL%\" ( echo hit ) else ( echo miss )\r\n"+
				"for %%f in (one two three) do echo %%f-%VAR%\r\n", 10) +
		"echo done\r\n"
	for b.Loop() {
		env := processor.NewEnvironment(true)
		p := processor.New(env, nil, nil)
		p.Echo = false
		p.Stdout = io.Discard
		p.Stderr = io.Discard
		nodes := processor.ParseExpanded(src)
		if err := p.Execute(nodes); err != nil {
			b.Fatal(err)
		}
	}
}
