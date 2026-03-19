package tools

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

const findstrHelp = `Searches for strings in files.

FINDSTR [/B] [/E] [/L] [/R] [/S] [/I] [/X] [/V] [/N] [/M] [/O] [/P]
        [/A:color] [/G:file] [/F:file] [/C:string] [/D:dirlist]
        strings [[path]filename[ ...]]

  /B        Matches pattern if at the beginning of a line.
  /E        Matches pattern if at the end of a line.
  /L        Uses search strings literally.
  /R        Uses search strings as regular expressions.
  /S        Searches for matching files in the current directory and all
            subdirectories.
  /I        Specifies that the search is not to be case-sensitive.
  /X        Prints lines that match exactly.
  /V        Prints only lines that do not contain a match.
  /N        Prints the line number before each line that matches.
  /M        Prints only the filename if a file contains a match.
  /O        Prints character offset before each matching line.
  /P        Skip files with non-printable characters.
  /A:color  Specifies color attribute with two hex digits.
  /G:file   Gets search strings from the specified file.
  /F:file   Reads file list from the specified file.
  /C:string Uses the specified string as a literal search string.
  /D:dir    Searches a semicolon-delimited list of directories.
  strings   Text to be searched for.
  filename  Filename(s) to search.
`

type findstrOptions struct {
	matchBegin    bool
	matchEnd      bool
	literal       bool
	regex         bool
	recursive     bool
	ignoreCase    bool
	exactLine     bool
	invertMatch   bool
	printLineNums bool
	printFileOnly bool
	printOffset   bool
	skipNonPrint  bool
	stringsFile   string
	fileList      string
	dirList       []string
	patterns      []string
	files         []string
}

func parseFindstrArgs(args []string) (*findstrOptions, error) {
	opts := &findstrOptions{}

	for i := range args {
		arg := args[i]

		upper := strings.ToUpper(arg)

		switch {
		case upper == "/B" || upper == "-B":
			opts.matchBegin = true
		case upper == "/E" || upper == "-E":
			opts.matchEnd = true
		case upper == "/L" || upper == "-L":
			opts.literal = true
		case upper == "/R" || upper == "-R":
			opts.regex = true
		case upper == "/S" || upper == "-S":
			opts.recursive = true
		case upper == "/I" || upper == "-I":
			opts.ignoreCase = true
		case upper == "/X" || upper == "-X":
			opts.exactLine = true
		case upper == "/V" || upper == "-V":
			opts.invertMatch = true
		case upper == "/N" || upper == "-N":
			opts.printLineNums = true
		case upper == "/M" || upper == "-M":
			opts.printFileOnly = true
		case upper == "/O" || upper == "-O":
			opts.printOffset = true
		case upper == "/P" || upper == "-P":
			opts.skipNonPrint = true
		case strings.HasPrefix(upper, "/A:") || strings.HasPrefix(upper, "-A:"):
			// color attribute - accepted but ignored
		case strings.HasPrefix(upper, "/G:") || strings.HasPrefix(upper, "-G:"):
			opts.stringsFile = arg[3:]
		case strings.HasPrefix(upper, "/F:") || strings.HasPrefix(upper, "-F:"):
			opts.fileList = arg[3:]
		case strings.HasPrefix(upper, "/C:") || strings.HasPrefix(upper, "-C:"):
			opts.patterns = append(opts.patterns, pathutil.StripQuotes(arg[3:]))
		case strings.HasPrefix(upper, "/D:") || strings.HasPrefix(upper, "-D:"):
			for d := range strings.SplitSeq(arg[3:], ",") {
				if d != "" {
					opts.dirList = append(opts.dirList, d)
				}
			}
		default:
			if (strings.HasPrefix(arg, "/") || strings.HasPrefix(arg, "-")) &&
				len(arg) == 2 {
				continue
			}
			if len(opts.patterns) == 0 && len(opts.files) == 0 && opts.stringsFile == "" {
				// If the pattern is quoted, treat it as a single pattern.
				// Otherwise, split it into multiple space-separated patterns
				// to match cmd.exe findstr behavior.
				if len(arg) >= 2 && (arg[0] == '"' || arg[0] == '\'') && arg[len(arg)-1] == arg[0] {
					opts.patterns = append(opts.patterns, arg[1:len(arg)-1])
				} else {
					for word := range strings.FieldsSeq(arg) {
						opts.patterns = append(opts.patterns, word)
					}
				}
			} else {
				opts.files = append(opts.files, pathutil.MapPath(pathutil.StripQuotes(arg)))
			}
		}
	}

	return opts, nil
}

func buildFindstrRegexes(opts *findstrOptions) ([]*regexp.Regexp, error) {
	useRegex := opts.regex

	if !opts.literal && !opts.regex && len(opts.patterns) > 0 {
		if slices.ContainsFunc(opts.patterns, containsRegexMeta) {
			useRegex = true
		}
	}

	var regexes []*regexp.Regexp
	for _, pat := range opts.patterns {
		var expr string

		if useRegex {
			expr = convertFindstrRegex(pat)
		} else {
			expr = regexp.QuoteMeta(pat)
		}

		if opts.exactLine || (opts.matchBegin && opts.matchEnd) {
			expr = "^" + expr + "$"
		} else if opts.matchBegin {
			expr = "^" + expr
		} else if opts.matchEnd {
			expr = expr + "$"
		}

		flags := "(?m)"
		if opts.ignoreCase {
			flags = "(?mi)"
		}

		re, err := regexp.Compile(flags + expr)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", pat, err)
		}
		regexes = append(regexes, re)
	}
	return regexes, nil
}

func containsRegexMeta(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		switch s[i] {
		case '.', '*', '^', '$', '[':
			return true
		}
	}
	return false
}

func convertFindstrRegex(pat string) string {
	var b strings.Builder
	i := 0
	for i < len(pat) {
		ch := pat[i]
		if ch == '\\' && i+1 < len(pat) {
			next := pat[i+1]
			switch next {
			case '<':
				b.WriteString(`\b`)
				i += 2
				continue
			case '>':
				b.WriteString(`\b`)
				i += 2
				continue
			default:
				b.WriteByte('\\')
				b.WriteByte(next)
				i += 2
				continue
			}
		}
		b.WriteByte(ch)
		i++
	}
	return b.String()
}

func hasNonPrintable(line string) bool {
	for _, r := range line {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return true
		}
	}
	return false
}

func lineMatchesFindstr(line string, regexes []*regexp.Regexp) bool {
	for _, re := range regexes {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

func Findstr(p *processor.Processor, cmd *parser.SimpleCommand) error {

	if len(cmd.Args) == 0 {
		fmt.Fprintf(p.Stderr, "FINDSTR: required parameter missing\n")
		p.FailureWithCode(2)
		return nil
	}

	opts, err := parseFindstrArgs(cmd.Args)
	if err != nil {
		fmt.Fprintf(p.Stderr, "FINDSTR: %v\n", err)
		p.FailureWithCode(2)
		return nil
	}

	if opts.stringsFile != "" {
		path := pathutil.MapPath(opts.stringsFile)
		lines, err := readLines(path)
		if err != nil {
			fmt.Fprintf(p.Stderr, "FINDSTR: cannot open %s\n", opts.stringsFile)
			p.FailureWithCode(2)
			return nil
		}
		opts.patterns = append(opts.patterns, lines...)
	}

	if len(opts.patterns) == 0 {
		fmt.Fprintf(p.Stderr, "FINDSTR: required parameter missing\n")
		p.FailureWithCode(2)
		return nil
	}

	regexes, err := buildFindstrRegexes(opts)
	if err != nil {
		fmt.Fprintf(p.Stderr, "FINDSTR: %v\n", err)
		p.FailureWithCode(2)
		return nil
	}

	if opts.fileList != "" {
		path := pathutil.MapPath(opts.fileList)
		lines, err := readLines(path)
		if err != nil {
			fmt.Fprintf(p.Stderr, "FINDSTR: cannot open %s\n", opts.fileList)
			p.FailureWithCode(2)
			return nil
		}
		for _, l := range lines {
			opts.files = append(opts.files, pathutil.MapPath(l))
		}
	}

	found := false

	scan := func(r io.Reader, label string, multiFile bool) {
		scanner := bufio.NewScanner(r)
		lineNum := 0
		offset := 0
		for scanner.Scan() {
			line := scanner.Text()
			lineNum++
			lineLen := len(line) + 1

			if opts.skipNonPrint && hasNonPrintable(line) {
				offset += lineLen
				continue
			}

			matched := lineMatchesFindstr(line, regexes)
			if opts.invertMatch {
				matched = !matched
			}

			if matched {
				found = true
				if opts.printFileOnly {
					if label != "" {
						fmt.Fprintln(p.Stdout, label)
					}
					return
				}

				var sb strings.Builder
				if multiFile && label != "" {
					sb.WriteString(label)
					sb.WriteByte(':')
				}
				if opts.printLineNums {
					sb.WriteString(strconv.Itoa(lineNum))
					sb.WriteByte(':')
				}
				if opts.printOffset {
					sb.WriteString(strconv.Itoa(offset))
					sb.WriteByte(':')
				}
				sb.WriteString(line)
				fmt.Fprintln(p.Stdout, sb.String())
			}

			offset += lineLen
		}
	}

	if len(opts.files) == 0 && len(opts.dirList) == 0 {
		if p.Stdin == nil {
			p.Failure()
			return nil
		}
		scan(p.Stdin, "", false)
	} else {
		var targets []string

		if len(opts.dirList) > 0 {
			for _, dir := range opts.dirList {
				for _, pat := range opts.files {
					base := filepath.Base(pat)
					full := filepath.Join(pathutil.MapPath(dir), base)
					targets = append(targets, full)
				}
			}
		} else {
			targets = opts.files
		}

		var allMatches []string
		for _, pat := range targets {
			if opts.recursive {
				base := filepath.Base(pat)
				dir := filepath.Dir(pat)
				err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return nil
					}
					if !d.IsDir() {
						matched, _ := filepath.Match(base, filepath.Base(path))
						if matched {
							allMatches = append(allMatches, path)
						}
					}
					return nil
				})
				if err != nil {
					fmt.Fprintf(p.Stderr, "FINDSTR: error walking %s\n", dir)
				}
			} else {
				matches, err := filepath.Glob(pat)
				if err != nil || len(matches) == 0 {
					matches = []string{pat}
				}
				allMatches = append(allMatches, matches...)
			}
		}

		multiFile := len(allMatches) > 1
		for _, m := range allMatches {
			f, err := os.Open(m)
			if err != nil {
				fmt.Fprintf(p.Stderr, "File not found - %s\n", m)
				continue
			}
			scan(f, m, multiFile)
			f.Close()
		}
	}

	if found {
		p.Success()
	} else {
		p.Failure()
	}
	return nil
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}
