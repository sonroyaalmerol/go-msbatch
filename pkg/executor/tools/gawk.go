package tools

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/benhoyt/goawk/interp"
	goawkparser "github.com/benhoyt/goawk/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

const gawkHelp = `Usage: gawk [POSIX or GNU style options] -f progfile [--] file ...
Usage: gawk [POSIX or GNU style options] [--] 'program' file ...
POSIX options:          GNU long options:
        -f progfile             --file=progfile
        -F fs                   --field-separator=fs
        -v var=val              --assign=var=val
Short options:          GNU long options: (extensions)
        -b                      --characters-as-bytes
        -c                      --traditional
        -C                      --copyright
        -d[file]                --dump-variables[=file]
        -D[file]                --debug[=file]
        -e 'program-text'       --source='program-text'
        -E file                 --exec=file
        -g                      --gen-pot
        -h                      --help
        -i includefile          --include=includefile
        -I                      --trace
        -k                      --csv
        -l library              --load=library
        -L[fatal|invalid|no-ext] --lint[=fatal|invalid|no-ext]
        -M                      --bignum
        -N                      --use-lc-numeric
        -n                      --non-decimal-data
        -o[file]                --pretty-print[=file]
        -O                      --optimize
        -p[file]                --profile[=file]
        -P                      --posix
        -r                      --re-interval
        -s                      --no-optimize
        -S                      --sandbox
        -t                      --lint-old
        -V                      --version

gawk is a pattern scanning and processing language.
By default it reads standard input and writes to standard output.

Examples:
        gawk '{ sum += $1 }; END { print sum }' file
        gawk -F: '{ print $1 }' /etc/passwd
`

type gawkConfig struct {
	fieldSep     string
	programSrc   string
	programFiles []string
	varAssigns   []string
	inputFiles   []string
	csvMode      bool
	tsvMode      bool
	noExec       bool
	noFileWrites bool
	noFileReads  bool
	sandbox      bool
	charsMode    bool
	version      bool
	help         bool
	dumpVars     string
	prettyPrint  string
	optimize     *bool
}

func Gawk(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if p.ShowHelp(cmd, gawkHelp) {
		return nil
	}

	cfg, err := parseGawkArgs(cmd.Args)
	if err != nil {
		fmt.Fprintf(p.Stderr, "gawk: %v\n", err)
		p.Failure()
		return nil
	}

	if cfg.help {
		fmt.Fprint(p.Stdout, gawkHelp)
		p.Success()
		return nil
	}

	if cfg.version {
		fmt.Fprintln(p.Stdout, "GNU Awk 5.4.0, API 4.1, PMA Avon 8-g1, (GNU MPFR 4.2.2, GNU MP 6.3.0)")
		p.Success()
		return nil
	}

	if cfg.programSrc == "" && len(cfg.programFiles) == 0 {
		fmt.Fprintf(p.Stderr, "gawk: no program specified\n")
		p.Failure()
		return nil
	}

	var src string
	if cfg.programSrc != "" {
		src = unescapeQuotes(strings.Trim(cfg.programSrc, "\"'"))
	}

	for _, pf := range cfg.programFiles {
		data, err := os.ReadFile(pathutil.MapPath(pf))
		if err != nil {
			fmt.Fprintf(p.Stderr, "gawk: can't open file %s\n", pf)
			p.Failure()
			return nil
		}
		if src != "" {
			src += "\n"
		}
		src += string(data)
	}

	parserConfig := &goawkparser.ParserConfig{
		Funcs: map[string]any{
			"systime":  gawkSystime,
			"strftime": gawkStrftime,
			"mktime":   gawkMktime,
			"and":      gawkAnd,
			"or":       gawkOr,
			"xor":      gawkXor,
			"compl":    gawkCompl,
			"lshift":   gawkLshift,
			"rshift":   gawkRshift,
			"strtonum": gawkStrtonum,
		},
	}
	prog, err := goawkparser.ParseProgram([]byte(src), parserConfig)
	if err != nil {
		fmt.Fprintf(p.Stderr, "gawk: %v\n", err)
		p.Failure()
		return nil
	}

	var vars []string
	if cfg.fieldSep != "" {
		vars = append(vars, "FS", cfg.fieldSep)
	}
	for _, va := range cfg.varAssigns {
		parts := strings.SplitN(va, "=", 2)
		if len(parts) == 2 {
			vars = append(vars, parts[0], parts[1])
		}
	}

	interpConfig := &interp.Config{
		Stdin:  p.Stdin,
		Output: p.Stdout,
		Error:  p.Stderr,
		Vars:   vars,
		Funcs:  parserConfig.Funcs,
	}

	if cfg.csvMode {
		interpConfig.InputMode = interp.CSVMode
		interpConfig.OutputMode = interp.CSVMode
	} else if cfg.tsvMode {
		interpConfig.InputMode = interp.TSVMode
		interpConfig.OutputMode = interp.TSVMode
	}

	if cfg.sandbox {
		interpConfig.NoExec = true
		interpConfig.NoFileWrites = true
		interpConfig.NoFileReads = true
	}

	if cfg.noExec {
		interpConfig.NoExec = true
	}
	if cfg.noFileWrites {
		interpConfig.NoFileWrites = true
	}
	if cfg.noFileReads {
		interpConfig.NoFileReads = true
	}

	if cfg.charsMode {
		interpConfig.Chars = true
	}

	var stdin io.Reader = p.Stdin
	if stdin == nil {
		stdin = strings.NewReader("")
	}

	if len(cfg.inputFiles) > 0 {
		interpConfig.Argv0 = "gawk"
		mappedFiles := make([]string, len(cfg.inputFiles))
		for i, f := range cfg.inputFiles {
			mappedFiles[i] = pathutil.MapPath(f)
		}
		interpConfig.Args = mappedFiles
	}

	interpConfig.Stdin = stdin

	exitCode, err := interp.ExecProgram(prog, interpConfig)
	if err != nil {
		fmt.Fprintf(p.Stderr, "gawk: %v\n", err)
		p.FailureWithCode(exitCode)
		return nil
	}

	if cfg.dumpVars != "" {
		dumpFile := pathutil.MapPath(cfg.dumpVars)
		_ = os.WriteFile(dumpFile, []byte{}, 0644)
	}

	if cfg.prettyPrint != "" {
		ppFile := pathutil.MapPath(cfg.prettyPrint)
		_ = os.WriteFile(ppFile, []byte(src), 0644)
	}

	p.SetErrorLevel(exitCode)
	return nil
}

func parseGawkArgs(args []string) (*gawkConfig, error) {
	cfg := &gawkConfig{}
	var i int
	programSet := false

	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			if i+1 < len(args) && !programSet {
				cfg.programSrc = args[i+1]
				programSet = true
				cfg.inputFiles = append(cfg.inputFiles, args[i+2:]...)
			} else {
				cfg.inputFiles = append(cfg.inputFiles, args[i+1:]...)
			}
			break
		}

		if !strings.HasPrefix(arg, "-") {
			if !programSet {
				cfg.programSrc = arg
				programSet = true
			} else {
				cfg.inputFiles = append(cfg.inputFiles, arg)
			}
			i++
			continue
		}

		if arg == "-" {
			cfg.inputFiles = append(cfg.inputFiles, arg)
			i++
			continue
		}

		switch {
		case arg == "-F":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("option requires an argument: -F")
			}
			cfg.fieldSep = args[i+1]
			i += 2
		case strings.HasPrefix(arg, "-F"):
			cfg.fieldSep = arg[2:]
			i++

		case arg == "-f":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("option requires an argument: -f")
			}
			cfg.programFiles = append(cfg.programFiles, args[i+1])
			programSet = true
			i += 2
		case strings.HasPrefix(arg, "-f"):
			cfg.programFiles = append(cfg.programFiles, arg[2:])
			programSet = true
			i++

		case arg == "-e":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("option requires an argument: -e")
			}
			cfg.programSrc = args[i+1]
			programSet = true
			i += 2
		case strings.HasPrefix(arg, "-e"):
			cfg.programSrc = arg[2:]
			programSet = true
			i++

		case arg == "-v":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("option requires an argument: -v")
			}
			cfg.varAssigns = append(cfg.varAssigns, args[i+1])
			i += 2
		case strings.HasPrefix(arg, "-v") && len(arg) > 2:
			cfg.varAssigns = append(cfg.varAssigns, arg[2:])
			i++

		case arg == "--field-separator":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("option requires an argument: --field-separator")
			}
			cfg.fieldSep = args[i+1]
			i += 2
		case strings.HasPrefix(arg, "--field-separator="):
			cfg.fieldSep = strings.TrimPrefix(arg, "--field-separator=")
			i++

		case arg == "--file":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("option requires an argument: --file")
			}
			cfg.programFiles = append(cfg.programFiles, args[i+1])
			programSet = true
			i += 2
		case strings.HasPrefix(arg, "--file="):
			cfg.programFiles = append(cfg.programFiles, strings.TrimPrefix(arg, "--file="))
			programSet = true
			i++

		case arg == "--source":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("option requires an argument: --source")
			}
			cfg.programSrc = args[i+1]
			programSet = true
			i += 2
		case strings.HasPrefix(arg, "--source="):
			cfg.programSrc = strings.TrimPrefix(arg, "--source=")
			programSet = true
			i++

		case arg == "--assign":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("option requires an argument: --assign")
			}
			cfg.varAssigns = append(cfg.varAssigns, args[i+1])
			i += 2
		case strings.HasPrefix(arg, "--assign="):
			cfg.varAssigns = append(cfg.varAssigns, strings.TrimPrefix(arg, "--assign="))
			i++

		case arg == "-k", arg == "--csv":
			cfg.csvMode = true
			i++

		case arg == "--tsv":
			cfg.tsvMode = true
			i++

		case arg == "-b", arg == "--characters-as-bytes":
			cfg.charsMode = true
			i++

		case arg == "-S", arg == "--sandbox":
			cfg.sandbox = true
			i++

		case arg == "--no-exec":
			cfg.noExec = true
			i++

		case arg == "--no-file-writes":
			cfg.noFileWrites = true
			i++

		case arg == "--no-file-reads":
			cfg.noFileReads = true
			i++

		case arg == "-O", arg == "--optimize":
			optTrue := true
			cfg.optimize = &optTrue
			i++

		case arg == "-s", arg == "--no-optimize":
			optFalse := false
			cfg.optimize = &optFalse
			i++

		case arg == "-d":
			cfg.dumpVars = "awkvars.out"
			i++
		case strings.HasPrefix(arg, "-d"):
			cfg.dumpVars = arg[2:]
			if cfg.dumpVars == "" {
				cfg.dumpVars = "awkvars.out"
			}
			i++
		case arg == "--dump-variables":
			cfg.dumpVars = "awkvars.out"
			i++
		case arg == "--dump-variables=":
			cfg.dumpVars = "awkvars.out"
			i++
		case strings.HasPrefix(arg, "--dump-variables="):
			cfg.dumpVars = strings.TrimPrefix(arg, "--dump-variables=")
			if cfg.dumpVars == "" {
				cfg.dumpVars = "awkvars.out"
			}
			i++

		case arg == "-o":
			cfg.prettyPrint = "awkprof.out"
			i++
		case strings.HasPrefix(arg, "-o") && len(arg) > 2:
			cfg.prettyPrint = arg[2:]
			if cfg.prettyPrint == "" {
				cfg.prettyPrint = "awkprof.out"
			}
			i++
		case arg == "--pretty-print":
			cfg.prettyPrint = "awkprof.out"
			i++
		case strings.HasPrefix(arg, "--pretty-print="):
			cfg.prettyPrint = strings.TrimPrefix(arg, "--pretty-print=")
			if cfg.prettyPrint == "" {
				cfg.prettyPrint = "awkprof.out"
			}
			i++

		case arg == "-h", arg == "--help":
			cfg.help = true
			i++

		case arg == "-V", arg == "--version":
			cfg.version = true
			i++

		case arg == "-c", arg == "--traditional",
			arg == "-C", arg == "--copyright",
			arg == "-i", arg == "--include",
			arg == "-l", arg == "--load",
			arg == "-L", arg == "--lint",
			arg == "-P", arg == "--posix",
			arg == "-r", arg == "--re-interval",
			arg == "-t", arg == "--lint-old",
			arg == "-M", arg == "--bignum",
			arg == "-N", arg == "--use-lc-numeric",
			arg == "-n", arg == "--non-decimal-data",
			arg == "-g", arg == "--gen-pot",
			arg == "-D", arg == "--debug",
			arg == "-I", arg == "--trace",
			arg == "-E", arg == "--exec",
			arg == "-p", arg == "--profile":
			i++

		case strings.HasPrefix(arg, "-i"),
			strings.HasPrefix(arg, "-l"),
			strings.HasPrefix(arg, "-L"),
			strings.HasPrefix(arg, "--include="),
			strings.HasPrefix(arg, "--load="),
			strings.HasPrefix(arg, "--lint"),
			strings.HasPrefix(arg, "--debug="),
			strings.HasPrefix(arg, "--profile="),
			strings.HasPrefix(arg, "--exec="):
			i++

		default:
			if strings.HasPrefix(arg, "--") {
				i++
			} else if len(arg) == 2 && arg[0] == '-' {
				i++
			} else if len(arg) > 2 && arg[0] == '-' && arg[1] != '-' {
				i++
			} else {
				if !programSet {
					cfg.programSrc = arg
					programSet = true
				} else {
					cfg.inputFiles = append(cfg.inputFiles, arg)
				}
				i++
			}
		}
	}

	return cfg, nil
}

func gawkSystime() int {
	return int(time.Now().Unix())
}

func gawkStrftime(args ...string) string {
	var format string = "%a %b %d %H:%M:%S %Z %Y"
	var timestamp int64 = time.Now().Unix()

	if len(args) >= 1 && args[0] != "" {
		format = args[0]
	}
	if len(args) >= 2 {
		if ts, err := strconv.ParseInt(args[1], 10, 64); err == nil {
			timestamp = ts
		}
	}

	t := time.Unix(timestamp, 0)
	return formatTime(format, t)
}

func formatTime(format string, t time.Time) string {
	result := strings.Builder{}
	i := 0
	for i < len(format) {
		if format[i] == '%' && i+1 < len(format) {
			i++
			switch format[i] {
			case '%':
				result.WriteByte('%')
			case 'a':
				result.WriteString(t.Format("Mon"))
			case 'A':
				result.WriteString(t.Format("Monday"))
			case 'b', 'h':
				result.WriteString(t.Format("Jan"))
			case 'B':
				result.WriteString(t.Format("January"))
			case 'c':
				result.WriteString(t.Format("Mon Jan _2 15:04:05 2006"))
			case 'C':
				result.WriteString(fmt.Sprintf("%02d", t.Year()/100))
			case 'd':
				result.WriteString(fmt.Sprintf("%02d", t.Day()))
			case 'D', 'x':
				result.WriteString(t.Format("01/02/06"))
			case 'e':
				result.WriteString(fmt.Sprintf("%2d", t.Day()))
			case 'F':
				result.WriteString(t.Format("2006-01-02"))
			case 'g':
				_, isoYear := t.ISOWeek()
				result.WriteString(fmt.Sprintf("%02d", isoYear%100))
			case 'G':
				_, isoYear := t.ISOWeek()
				result.WriteString(fmt.Sprintf("%04d", isoYear))
			case 'H':
				result.WriteString(t.Format("15"))
			case 'I':
				hour := t.Hour()
				if hour == 0 {
					hour = 12
				}
				if hour > 12 {
					hour -= 12
				}
				result.WriteString(fmt.Sprintf("%02d", hour))
			case 'j':
				result.WriteString(fmt.Sprintf("%03d", t.YearDay()))
			case 'k':
				result.WriteString(fmt.Sprintf("%2d", t.Hour()))
			case 'l':
				hour := t.Hour()
				if hour == 0 {
					hour = 12
				}
				if hour > 12 {
					hour -= 12
				}
				result.WriteString(fmt.Sprintf("%2d", hour))
			case 'm':
				result.WriteString(t.Format("01"))
			case 'M':
				result.WriteString(t.Format("04"))
			case 'n':
				result.WriteByte('\n')
			case 'N':
				result.WriteString(fmt.Sprintf("%09d", t.Nanosecond()))
			case 'p':
				if t.Hour() < 12 {
					result.WriteString("AM")
				} else {
					result.WriteString("PM")
				}
			case 'P':
				if t.Hour() < 12 {
					result.WriteString("am")
				} else {
					result.WriteString("pm")
				}
			case 'r':
				hour := t.Hour()
				ampm := "AM"
				if hour >= 12 {
					ampm = "PM"
					if hour > 12 {
						hour -= 12
					}
				}
				if hour == 0 {
					hour = 12
				}
				result.WriteString(fmt.Sprintf("%02d:%02d:%02d %s", hour, t.Minute(), t.Second(), ampm))
			case 'R':
				result.WriteString(t.Format("15:04"))
			case 's':
				result.WriteString(fmt.Sprintf("%d", t.Unix()))
			case 'S':
				result.WriteString(t.Format("05"))
			case 't':
				result.WriteByte('\t')
			case 'T', 'X':
				result.WriteString(t.Format("15:04:05"))
			case 'u':
				weekday := int(t.Weekday())
				if weekday == 0 {
					weekday = 7
				}
				result.WriteString(fmt.Sprintf("%d", weekday))
			case 'U':
				_, week := t.ISOWeek()
				result.WriteString(fmt.Sprintf("%02d", week))
			case 'V':
				_, week := t.ISOWeek()
				result.WriteString(fmt.Sprintf("%02d", week))
			case 'w':
				result.WriteString(fmt.Sprintf("%d", int(t.Weekday())))
			case 'W':
				_, week := t.ISOWeek()
				result.WriteString(fmt.Sprintf("%02d", week))
			case 'y':
				result.WriteString(t.Format("06"))
			case 'Y':
				result.WriteString(t.Format("2006"))
			case 'z':
				_, offset := t.Zone()
				sign := "+"
				if offset < 0 {
					sign = "-"
					offset = -offset
				}
				hours := offset / 3600
				mins := (offset % 3600) / 60
				result.WriteString(fmt.Sprintf("%s%02d%02d", sign, hours, mins))
			case 'Z':
				name, _ := t.Zone()
				result.WriteString(name)
			default:
				result.WriteByte('%')
				result.WriteByte(format[i])
			}
		} else {
			result.WriteByte(format[i])
		}
		i++
	}
	return result.String()
}

func gawkMktime(timestr string) int {
	parts := strings.Fields(timestr)
	if len(parts) < 3 {
		return -1
	}

	year, _ := strconv.Atoi(parts[0])
	month, _ := strconv.Atoi(parts[1])
	day, _ := strconv.Atoi(parts[2])

	hour, min, sec := 0, 0, 0
	if len(parts) >= 4 {
		hour, _ = strconv.Atoi(parts[3])
	}
	if len(parts) >= 5 {
		min, _ = strconv.Atoi(parts[4])
	}
	if len(parts) >= 6 {
		sec, _ = strconv.Atoi(parts[5])
	}

	if month < 1 || month > 12 || day < 1 || day > 31 {
		return -1
	}
	if hour < 0 || hour > 23 || min < 0 || min > 59 || sec < 0 || sec > 59 {
		return -1
	}

	t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
	return int(t.Unix())
}

func gawkAnd(args ...int) int {
	if len(args) == 0 {
		return 0
	}
	result := args[0]
	for _, v := range args[1:] {
		result &= v
	}
	return result
}

func gawkOr(args ...int) int {
	if len(args) == 0 {
		return 0
	}
	result := args[0]
	for _, v := range args[1:] {
		result |= v
	}
	return result
}

func gawkXor(args ...int) int {
	if len(args) == 0 {
		return 0
	}
	result := args[0]
	for _, v := range args[1:] {
		result ^= v
	}
	return result
}

func gawkCompl(n int) int {
	return ^n
}

func gawkLshift(n, bits int) int {
	return n << bits
}

func gawkRshift(n, bits int) int {
	return n >> bits
}

func gawkStrtonum(s string) float64 {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "0x") {
		val, err := strconv.ParseInt(s, 0, 64)
		if err != nil {
			return 0
		}
		return float64(val)
	}
	if strings.HasPrefix(s, "0") && len(s) > 1 && s[0] == '0' {
		val, err := strconv.ParseInt(s, 8, 64)
		if err != nil {
			return 0
		}
		return float64(val)
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

func unescapeQuotes(s string) string {
	result := strings.ReplaceAll(s, `\"`, `"`)
	result = strings.ReplaceAll(result, `""`, `"`)
	return strings.TrimSpace(result)
}
