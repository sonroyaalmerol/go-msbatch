package tools

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

const findHelp = `Searches for a text string in a file or files.

FIND [/V] [/C] [/N] [/I] "string" [[path]filename[ ...]]

  /V        Displays all lines NOT containing the specified string.
  /C        Displays only the count of lines containing the string.
  /N        Displays line numbers with the displayed lines.
  /I        Ignores the case of characters when searching for the string.
  "string"  Specifies the text string to find.
  filename  Specifies a file or files to search.

If a filename is not specified, FIND searches text piped from another command.
`

func Find(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// FIND [/V] [/C] [/N] [/I] "string" [file...]
	if len(cmd.Args) == 0 {
		fmt.Fprintf(p.Stderr, "Required parameter missing\n")
		p.FailureWithCode(2)
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
			// Only skip short Windows-style flags like /Y; don't skip Unix
			// absolute paths like /tmp/foo.txt which also start with '/'.
			if strings.HasPrefix(arg, "/") && !strings.ContainsRune(arg[1:], '/') {
				continue
			}
			if searchStr == "" {
				searchStr = strings.Trim(arg, "\"")
			} else {
				files = append(files, pathutil.MapPath(arg))
			}
		}
	}
	if searchStr == "" {
		fmt.Fprintf(p.Stderr, "Required parameter missing\n")
		p.FailureWithCode(2)
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
		if p.Stdin == nil {
			p.Failure()
			return nil
		}
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
		p.Success()
	} else {
		p.Failure()
	}
	return nil
}
