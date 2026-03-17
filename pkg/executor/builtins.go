package executor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
	"golang.org/x/term"
)

func cmdEcho(p *processor.Processor, cmd *parser.SimpleCommand) error {
	output, stateChanged := p.HandleEchoBuiltin(cmd.Args)
	if strings.ToLower(cmd.Name) == "echo." && len(cmd.Args) == 0 {
		fmt.Fprintln(p.Stdout)
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	if !stateChanged {
		fmt.Fprintln(p.Stdout, output)
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdSet(p *processor.Processor, cmd *parser.SimpleCommand) error {
	args := cmd.Args
	if len(args) == 0 {
		for k, v := range p.Env.Snapshot() {
			fmt.Fprintf(p.Stdout, "%s=%s\n", k, v)
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}

	arg := strings.Join(args, " ")

	if strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"") {
		arg = arg[1 : len(arg)-1]
	}

	if strings.HasPrefix(strings.ToLower(arg), "/a") {
		_, err := p.EvalArithmetic(arg[2:])
		if err != nil {
			fmt.Fprintf(p.Stderr, "Invalid number.\n")
			p.Env.Set("ERRORLEVEL", "1073741819")
		} else {
			p.Env.Set("ERRORLEVEL", "0")
		}
		return nil
	}

	if strings.HasPrefix(strings.ToLower(args[0]), "/p") {
		promptStr := arg[2:]
		if before, after, ok := strings.Cut(promptStr, "="); ok {
			fmt.Fprint(p.Stdout, after)
			var input string
			fmt.Fscanln(p.Stdin, &input)
			p.HandleSetBuiltin(strings.TrimSpace(before), input)
		}
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}

	if before, after, ok := strings.Cut(arg, "="); ok {
		p.HandleSetBuiltin(before, after)
	} else {
		found := false
		prefix := strings.ToUpper(arg)
		for k, v := range p.Env.Snapshot() {
			if strings.HasPrefix(k, prefix) {
				fmt.Fprintf(p.Stdout, "%s=%s\n", k, v)
				found = true
			}
		}
		if !found {
			fmt.Fprintf(p.Stderr, "Environment variable %s not defined\n", arg)
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdCd(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// Skip /d flag (change drive — irrelevant on Unix but must not be treated as path).
	args := cmd.Args
	if len(args) > 0 && strings.EqualFold(args[0], "/d") {
		args = args[1:]
	}
	if len(args) == 0 {
		pwd, _ := os.Getwd()
		fmt.Fprintln(p.Stdout, pwd)
		p.Env.Set("ERRORLEVEL", "0")
		return nil
	}
	if err := os.Chdir(processor.MapPath(args[0])); err != nil {
		fmt.Fprintf(p.Stderr, "The system cannot find the path specified.\n")
		p.Env.Set("ERRORLEVEL", "1")
	} else {
		p.Env.Set("ERRORLEVEL", "0")
	}
	return nil
}

func cmdType(p *processor.Processor, cmd *parser.SimpleCommand) error {
	failed := false
	for _, arg := range cmd.Args {
		content, err := os.ReadFile(processor.MapPath(arg))
		if err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
			failed = true
			continue
		}
		fmt.Fprint(p.Stdout, string(content))
	}
	if failed {
		p.Env.Set("ERRORLEVEL", "1")
	} else {
		p.Env.Set("ERRORLEVEL", "0")
	}
	return nil
}

func cmdCls(p *processor.Processor, _ *parser.SimpleCommand) error {
	fmt.Fprint(p.Stdout, "\033[2J\033[H")
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdTitle(p *processor.Processor, cmd *parser.SimpleCommand) error {
	fmt.Fprintf(p.Stdout, "\033]0;%s\a", strings.Join(cmd.Args, " "))
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdVer(p *processor.Processor, _ *parser.SimpleCommand) error {
	fmt.Fprintln(p.Stdout, VersionString())
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdPause(p *processor.Processor, _ *parser.SimpleCommand) error {
	fmt.Fprint(p.Stdout, "Press any key to continue . . . ")
	// When stdin is a real terminal, switch to raw mode so any key press
	// (not just Enter) unblocks the read.  Fall back to line-buffered read
	// when stdin is a pipe or redirected file.
	if f, ok := p.Stdin.(*os.File); ok {
		fd := int(f.Fd())
		if term.IsTerminal(fd) {
			if old, err := term.MakeRaw(fd); err == nil {
				io.ReadFull(p.Stdin, make([]byte, 1)) //nolint:errcheck
				term.Restore(fd, old)                //nolint:errcheck
			} else {
				io.ReadFull(p.Stdin, make([]byte, 1)) //nolint:errcheck
			}
		} else {
			io.ReadFull(p.Stdin, make([]byte, 1)) //nolint:errcheck
		}
	} else {
		io.ReadFull(p.Stdin, make([]byte, 1)) //nolint:errcheck
	}
	fmt.Fprintln(p.Stdout)
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdColor(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) > 0 {
		code := cmd.Args[0]
		if len(code) == 2 {
			ansiColors := map[byte]string{
				'0': "30", '1': "34", '2': "32", '3': "36",
				'4': "31", '5': "35", '6': "33", '7': "37",
				'8': "90", '9': "94", 'a': "92", 'b': "96",
				'c': "91", 'd': "95", 'e': "93", 'f': "97",
				'A': "92", 'B': "96", 'C': "91", 'D': "95",
				'E': "93", 'F': "97",
			}
			bgCode := ansiColors[code[0]]
			fgCode := ansiColors[code[1]]
			if bgCode != "" && fgCode != "" {
				bgCode = strings.Replace(bgCode, "3", "4", 1)
				bgCode = strings.Replace(bgCode, "9", "10", 1)
				fmt.Fprintf(p.Stdout, "\033[%s;%sm", bgCode, fgCode)
			}
		}
	} else {
		fmt.Fprint(p.Stdout, "\033[0m")
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdPushd(p *processor.Processor, cmd *parser.SimpleCommand) error {
	pwd, _ := os.Getwd()
	p.DirStack = append(p.DirStack, pwd)
	if len(cmd.Args) > 0 {
		if err := os.Chdir(processor.MapPath(cmd.Args[0])); err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the path specified.\n")
			p.DirStack = p.DirStack[:len(p.DirStack)-1]
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdPopd(p *processor.Processor, _ *parser.SimpleCommand) error {
	if len(p.DirStack) > 0 {
		dir := p.DirStack[len(p.DirStack)-1]
		p.DirStack = p.DirStack[:len(p.DirStack)-1]
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the path specified.\n")
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdMkdir(p *processor.Processor, cmd *parser.SimpleCommand) error {
	for _, arg := range cmd.Args {
		if strings.HasPrefix(arg, "/") {
			continue
		}
		if err := os.MkdirAll(processor.MapPath(arg), 0755); err != nil {
			fmt.Fprintf(p.Stderr, "A subdirectory or file %s already exists.\n", arg)
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdRmdir(p *processor.Processor, cmd *parser.SimpleCommand) error {
	recursive := false
	var paths []string
	for _, arg := range cmd.Args {
		switch strings.ToLower(arg) {
		case "/s":
			recursive = true
		case "/q":
			// quiet — ignore
		default:
			paths = append(paths, arg)
		}
	}
	for _, dirPath := range paths {
		var err error
		if recursive {
			err = os.RemoveAll(processor.MapPath(dirPath))
		} else {
			err = os.Remove(processor.MapPath(dirPath))
		}
		if err != nil {
			fmt.Fprintf(p.Stderr, "The directory is not empty.\n")
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdDel(p *processor.Processor, cmd *parser.SimpleCommand) error {
	recursive := false
	var patterns []string
	for _, arg := range cmd.Args {
		lower := strings.ToLower(arg)
		switch {
		case lower == "/q" || lower == "/f":
			// ignore
		case lower == "/s":
			recursive = true
		case strings.HasPrefix(lower, "/a"):
			// attribute filter — not implemented
		default:
			patterns = append(patterns, arg)
		}
	}
	for _, pat := range patterns {
		mapped := processor.MapPath(pat)
		if recursive {
			base := filepath.Base(mapped)
			filepath.Walk(filepath.Dir(mapped), func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if matched, _ := filepath.Match(base, filepath.Base(path)); matched {
					os.Remove(path)
				}
				return nil
			})
		} else {
			matches, err := filepath.Glob(mapped)
			if err == nil && len(matches) > 0 {
				for _, m := range matches {
					os.Remove(m)
				}
			} else if os.Remove(mapped) != nil {
				fmt.Fprintf(p.Stderr, "Could Not Find %s\n", pat)
				p.Env.Set("ERRORLEVEL", "1")
				return nil
			}
		}
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdCopy(p *processor.Processor, cmd *parser.SimpleCommand) error {
	var rawArgs []string
	for _, arg := range cmd.Args {
		switch strings.ToLower(arg) {
		case "/y", "/-y", "/b", "/a", "/v":
			// ignore flags
		default:
			rawArgs = append(rawArgs, arg)
		}
	}
	if len(rawArgs) < 1 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}

	hasPlus := false
	for _, a := range rawArgs {
		if strings.Contains(a, "+") {
			hasPlus = true
			break
		}
	}

	var srcs []string
	var dst string

	if !hasPlus {
		if len(rawArgs) >= 2 {
			dst = processor.MapPath(rawArgs[len(rawArgs)-1])
			for _, s := range rawArgs[:len(rawArgs)-1] {
				mapped := processor.MapPath(s)
				if matches, err := filepath.Glob(mapped); err == nil && len(matches) > 0 {
					srcs = append(srcs, matches...)
				} else {
					srcs = append(srcs, mapped)
				}
			}
		} else {
			srcs = []string{processor.MapPath(rawArgs[0])}
			dst, _ = os.Getwd()
		}
	} else {
		srcArgs := rawArgs
		if len(rawArgs) >= 2 {
			last := rawArgs[len(rawArgs)-1]
			prev := rawArgs[len(rawArgs)-2]
			if !strings.Contains(last, "+") && prev != "+" && !strings.HasSuffix(prev, "+") {
				dst = processor.MapPath(last)
				srcArgs = rawArgs[:len(rawArgs)-1]
			}
		}
		for _, a := range srcArgs {
			if a == "+" {
				continue
			}
			for part := range strings.SplitSeq(a, "+") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				mapped := processor.MapPath(part)
				if matches, err := filepath.Glob(mapped); err == nil && len(matches) > 0 {
					srcs = append(srcs, matches...)
				} else {
					srcs = append(srcs, mapped)
				}
			}
		}
		if dst == "" && len(srcs) > 0 {
			dst = srcs[0]
		}
	}

	if len(srcs) == 0 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}

	dstTarget := dst
	if info, err := os.Stat(dst); err == nil && info.IsDir() {
		dstTarget = filepath.Join(dst, filepath.Base(srcs[0]))
	}

	switch {
	case !hasPlus && len(srcs) > 1:
		count := 0
		for _, src := range srcs {
			target := dstTarget
			if info, err := os.Stat(dst); err == nil && info.IsDir() {
				target = filepath.Join(dst, filepath.Base(src))
			}
			if err := copyFile(src, target); err != nil {
				fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
				p.Env.Set("ERRORLEVEL", "1")
				return nil
			}
			count++
		}
		fmt.Fprintf(p.Stdout, "        %d file(s) copied.\n", count)
	case !hasPlus:
		if err := copyFile(srcs[0], dstTarget); err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
		fmt.Fprintf(p.Stdout, "        1 file(s) copied.\n")
	default:
		// Append: read all sources into a buffer first so dst == srcs[0] is safe.
		var buf bytes.Buffer
		for _, src := range srcs {
			data, err := os.ReadFile(src)
			if err != nil {
				fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
				p.Env.Set("ERRORLEVEL", "1")
				return nil
			}
			buf.Write(data)
		}
		if err := os.WriteFile(dstTarget, buf.Bytes(), 0666); err != nil {
			fmt.Fprintf(p.Stderr, "Access is denied.\n")
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
		fmt.Fprintf(p.Stdout, "        1 file(s) copied.\n")
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdMove(p *processor.Processor, cmd *parser.SimpleCommand) error {
	var args []string
	for _, arg := range cmd.Args {
		switch strings.ToLower(arg) {
		case "/y", "/-y":
			// ignore
		default:
			args = append(args, arg)
		}
	}
	if len(args) < 2 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	src := processor.MapPath(args[0])
	dst := processor.MapPath(args[1])
	if info, err := os.Stat(dst); err == nil && info.IsDir() {
		dst = filepath.Join(dst, filepath.Base(src))
	}
	if err := os.Rename(src, dst); err != nil {
		fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	fmt.Fprintf(p.Stdout, "        1 file(s) moved.\n")
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

func cmdDir(p *processor.Processor, cmd *parser.SimpleCommand) error {
	dirPath := "."
	for _, arg := range cmd.Args {
		if !strings.HasPrefix(arg, "/") {
			dirPath = processor.MapPath(arg)
			break
		}
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Fprintf(p.Stderr, "File Not Found\n")
		p.Env.Set("ERRORLEVEL", "1")
		return nil
	}
	abs, _ := filepath.Abs(dirPath)
	fmt.Fprintf(p.Stdout, " Directory of %s\n\n", abs)
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		t := info.ModTime()
		if e.IsDir() {
			fmt.Fprintf(p.Stdout, "%s  %s    <DIR>          %s\n",
				t.Format("01/02/2006"), t.Format("03:04 PM"), e.Name())
		} else {
			fmt.Fprintf(p.Stdout, "%s  %s    %14d %s\n",
				t.Format("01/02/2006"), t.Format("03:04 PM"), info.Size(), e.Name())
		}
	}
	p.Env.Set("ERRORLEVEL", "0")
	return nil
}

// copyFile copies src to dst, creating or truncating dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
