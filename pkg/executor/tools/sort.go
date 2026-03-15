package tools

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func Sort(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// SORT [/R] [/+n] [file]
	reverse := false
	var reader io.Reader
	if p.Stdin != nil {
		reader = p.Stdin
	} else {
		reader = strings.NewReader("")
	}
	for _, arg := range cmd.Args {
		lower := strings.ToLower(arg)
		if lower == "/r" {
			reverse = true
		} else if strings.HasPrefix(lower, "/") && !strings.ContainsRune(lower[1:], '/') {
			// ignore short Windows-style flags (/+n column sort, etc.)
		} else {
			f, err := os.Open(processor.MapPath(arg))
			if err != nil {
				fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
				p.Env.Set("ERRORLEVEL", "1")
				return nil
			}
			defer f.Close()
			reader = f
		}
	}
	scanner := bufio.NewScanner(reader)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	sort.Strings(lines)
	if reverse {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}
	for _, line := range lines {
		fmt.Fprintln(p.Stdout, line)
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}
