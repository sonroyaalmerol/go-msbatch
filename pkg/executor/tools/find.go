package tools

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func Find(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// FIND [/V] [/C] [/N] [/I] "string" [file...]
	if len(cmd.Args) == 0 {
		fmt.Fprintf(p.Stderr, "Required parameter missing\n")
		p.Env.Set("ERRORLEVEL", "2")
		return nil
	}
	ignoreCase := false
	printCount := false
	printLineNums := false
	invertMatch := false
	searchStr := ""
	var files []string

	for _, arg := range cmd.Args {
		switch strings.ToLower(arg) {
		case "/i":
			ignoreCase = true
		case "/c":
			printCount = true
		case "/n":
			printLineNums = true
		case "/v":
			invertMatch = true
		default:
			if strings.HasPrefix(arg, "/") {
				continue
			}
			if searchStr == "" {
				searchStr = strings.Trim(arg, "\"")
			} else {
				files = append(files, processor.MapPath(arg))
			}
		}
	}
	if searchStr == "" {
		fmt.Fprintf(p.Stderr, "Required parameter missing\n")
		p.Env.Set("ERRORLEVEL", "2")
		return nil
	}

	needle := searchStr
	if ignoreCase {
		needle = strings.ToLower(needle)
	}

	found := false
	scan := func(r io.Reader, label string) {
		scanner := bufio.NewScanner(r)
		count, lineNum := 0, 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			hay := line
			if ignoreCase {
				hay = strings.ToLower(hay)
			}
			matched := strings.Contains(hay, needle)
			if invertMatch {
				matched = !matched
			}
			if matched {
				found = true
				count++
				if !printCount {
					if printLineNums {
						fmt.Fprintf(p.Stdout, "[%d]%s\n", lineNum, line)
					} else {
						fmt.Fprintln(p.Stdout, line)
					}
				}
			}
		}
		if printCount {
			if label != "" {
				fmt.Fprintf(p.Stdout, "---------- %s: %d\n", label, count)
			} else {
				fmt.Fprintf(p.Stdout, "%d\n", count)
			}
		}
	}

	if len(files) == 0 {
		scan(p.Stdin, "")
	} else {
		for _, pat := range files {
			matches, err := filepath.Glob(pat)
			if err != nil || len(matches) == 0 {
				matches = []string{pat}
			}
			for _, m := range matches {
				f, err := os.Open(m)
				if err != nil {
					fmt.Fprintf(p.Stderr, "File not found - %s\n", m)
					continue
				}
				if len(files) > 1 || len(matches) > 1 {
					fmt.Fprintf(p.Stdout, "\n---------- %s\n", m)
				}
				scan(f, m)
				f.Close()
			}
		}
	}

	if found {
		p.Env.Set("ERRORLEVEL", "0")
	} else {
		p.Env.Set("ERRORLEVEL", "1")
	}
	return nil
}
