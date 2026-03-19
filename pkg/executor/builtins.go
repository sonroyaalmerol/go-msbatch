package executor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor/tools"
	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
	"golang.org/x/term"
)

func cmdEcho(p *processor.Processor, cmd *parser.SimpleCommand) error {
	output, stateChanged := p.HandleEchoBuiltin(cmd.RawArgs)
	if !stateChanged {
		p.Logger.Debug("echo command execution", "output", output)
	}
	if strings.ToLower(cmd.Name) == "echo." && len(cmd.RawArgs) == 0 {
		fmt.Fprintln(p.Stdout)
		p.Success()
		return nil
	}
	if !stateChanged {
		fmt.Fprintln(p.Stdout, output)
	}
	return p.Success()
}

func cmdSet(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		for k, v := range p.Env.Snapshot() {
			fmt.Fprintf(p.Stdout, "%s=%s\n", k, v)
		}
		p.Success()
		return nil
	}

	// Join raw args to preserve spacing, then trim exactly one leading delimiter run
	arg := processor.ExtractRawArgString(cmd.RawArgs)

	arg = pathutil.StripQuotes(arg)

	if strings.HasPrefix(strings.ToLower(arg), "/a") {
		_, err := p.EvalArithmetic(arg[2:])
		if err != nil {
			fmt.Fprintf(p.Stderr, "Invalid number.\n")
			p.FailureWithCode(1073741819)
		} else {
			p.Success()
		}
		return nil
	}

	if strings.HasPrefix(strings.ToLower(arg), "/p") {
		promptStr := arg[2:]
		if before, after, ok := strings.Cut(promptStr, "="); ok {
			fmt.Fprint(p.Stdout, after)
			var input string
			fmt.Fscanln(p.Stdin, &input)
			p.HandleSetBuiltin(strings.TrimSpace(before), input)
		}
		p.Success()
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
			p.Failure()
			return nil
		}
	}
	return p.Success()
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
		p.Success()
		return nil
	}
	mappedPath := pathutil.MapPath(args[0])
	oldDir, _ := os.Getwd()
	if err := os.Chdir(mappedPath); err != nil {
		p.Logger.Debug("directory change failed", "from", oldDir, "to", mappedPath, "error", err.Error())
		fmt.Fprintf(p.Stderr, "The system cannot find the path specified.\n")
		p.Failure()
	} else {
		p.Logger.Debug("directory changed", "from", oldDir, "to", mappedPath)
		p.Success()
	}
	return nil
}

func cmdType(p *processor.Processor, cmd *parser.SimpleCommand) error {
	failed := false
	for _, arg := range cmd.Args {
		content, err := os.ReadFile(pathutil.MapPath(arg))
		if err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
			failed = true
			continue
		}
		fmt.Fprint(p.Stdout, string(content))
	}
	if failed {
		p.Failure()
	} else {
		p.Success()
	}
	return nil
}

func cmdCls(p *processor.Processor, _ *parser.SimpleCommand) error {
	fmt.Fprint(p.Stdout, "\033[2J\033[H")
	return p.Success()
}

func cmdTitle(p *processor.Processor, cmd *parser.SimpleCommand) error {
	arg := processor.ExtractRawArgString(cmd.RawArgs)
	fmt.Fprintf(p.Stdout, "\033]0;%s\a", arg)
	return p.Success()
}

func cmdVer(p *processor.Processor, _ *parser.SimpleCommand) error {
	fmt.Fprintln(p.Stdout, VersionString())
	return p.Success()
}

func cmdPause(p *processor.Processor, _ *parser.SimpleCommand) error {
	fmt.Fprint(p.Stdout, "Press any key to continue . . . ")

	// CMD's PAUSE always interacts with the terminal, even if Stdin is redirected.
	// On Unix, try /dev/tty first.
	var input io.Reader = p.Stdin
	var fd int = -1

	if runtime.GOOS != "windows" {
		if f, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0); err == nil {
			defer f.Close()
			input = f
			fd = int(f.Fd())
		}
	}

	if fd == -1 {
		if f, ok := input.(*os.File); ok {
			fd = int(f.Fd())
		}
	}

	if fd != -1 && term.IsTerminal(fd) {
		if old, err := term.MakeRaw(fd); err == nil {
			defer term.Restore(fd, old)         //nolint:errcheck
			io.ReadFull(input, make([]byte, 1)) //nolint:errcheck
		} else {
			io.ReadFull(input, make([]byte, 1)) //nolint:errcheck
		}
	} else {
		io.ReadFull(input, make([]byte, 1)) //nolint:errcheck
	}

	fmt.Fprintln(p.Stdout)
	return p.Success()
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
	return p.Success()
}

func cmdPushd(p *processor.Processor, cmd *parser.SimpleCommand) error {
	pwd, _ := os.Getwd()
	p.DirStack = append(p.DirStack, pwd)
	if len(cmd.Args) > 0 {
		mappedPath := pathutil.MapPath(cmd.Args[0])
		oldDir, _ := os.Getwd()
		if err := os.Chdir(mappedPath); err != nil {
			p.Logger.Debug("directory change failed (pushd)", "from", oldDir, "to", mappedPath, "error", err.Error())
			fmt.Fprintf(p.Stderr, "The system cannot find the path specified.\n")
			p.DirStack = p.DirStack[:len(p.DirStack)-1]
			p.Failure()
			return nil
		}
		p.Logger.Debug("directory changed (pushd)", "from", oldDir, "to", mappedPath)
	}
	return p.Success()
}

func cmdPopd(p *processor.Processor, _ *parser.SimpleCommand) error {
	if len(p.DirStack) > 0 {
		dir := p.DirStack[len(p.DirStack)-1]
		p.DirStack = p.DirStack[:len(p.DirStack)-1]
		oldDir, _ := os.Getwd()
		if err := os.Chdir(dir); err != nil {
			p.Logger.Debug("directory change failed (popd)", "from", oldDir, "to", dir, "error", err.Error())
			fmt.Fprintf(p.Stderr, "The system cannot find the path specified.\n")
			p.Failure()
			return nil
		}
		p.Logger.Debug("directory changed (popd)", "from", oldDir, "to", dir)
	}
	return p.Success()
}

func cmdMkdir(p *processor.Processor, cmd *parser.SimpleCommand) error {
	for _, arg := range cmd.Args {
		if strings.HasPrefix(arg, "/") {
			continue
		}
		if err := os.MkdirAll(pathutil.MapPath(arg), 0755); err != nil {
			fmt.Fprintf(p.Stderr, "A subdirectory or file %s already exists.\n", arg)
			p.Failure()
			return nil
		}
	}
	return p.Success()
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
			err = os.RemoveAll(pathutil.MapPath(dirPath))
		} else {
			err = os.Remove(pathutil.MapPath(dirPath))
		}
		if err != nil {
			fmt.Fprintf(p.Stderr, "The directory is not empty.\n")
			p.Failure()
			return nil
		}
	}
	return p.Success()
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
		mapped := pathutil.MapPath(pat)
		if recursive {
			base := filepath.Base(mapped)
			filepath.Walk(filepath.Dir(mapped), func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if matched := pathutil.MatchCaseInsensitive(base, filepath.Base(path)); matched {
					os.Remove(path)
				}
				return nil
			})
		} else {
			matches, err := pathutil.GlobCaseInsensitive(mapped)
			if err == nil && len(matches) > 0 {
				for _, m := range matches {
					os.Remove(m)
				}
			} else if os.Remove(mapped) != nil {
				fmt.Fprintf(p.Stderr, "Could Not Find %s\n", pat)
				p.Failure()
				return nil
			}
		}
	}
	return p.Success()
}

func cmdCopy(p *processor.Processor, cmd *parser.SimpleCommand) error {
	var rawArgs []string
	suppressPrompt := true

	for _, arg := range cmd.Args {
		lower := strings.ToLower(arg)
		switch lower {
		case "/y":
			suppressPrompt = true
		case "/-y":
			suppressPrompt = false
		case "/b", "/a", "/v":
		default:
			rawArgs = append(rawArgs, arg)
		}
	}

	if len(rawArgs) < 1 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Failure()
		return nil
	}

	hasPlus := false
	for _, a := range rawArgs {
		if strings.Contains(a, "+") {
			hasPlus = true
			break
		}
	}

	type srcEntry struct {
		path    string
		pattern string
	}
	var srcs []srcEntry
	var dst string
	var dstPattern string

	if !hasPlus {
		if len(rawArgs) >= 2 {
			dstPattern = rawArgs[len(rawArgs)-1]
			dst = pathutil.MapPath(dstPattern)
			for _, s := range rawArgs[:len(rawArgs)-1] {
				mapped := pathutil.MapPath(s)
				if matches, err := pathutil.GlobCaseInsensitive(mapped); err == nil && len(matches) > 0 {
					for _, m := range matches {
						srcs = append(srcs, srcEntry{path: m, pattern: s})
					}
				} else {
					srcs = append(srcs, srcEntry{path: mapped, pattern: s})
				}
			}
		} else {
			srcs = []srcEntry{{path: pathutil.MapPath(rawArgs[0]), pattern: rawArgs[0]}}
			dst, _ = os.Getwd()
		}
	} else {
		srcArgs := rawArgs
		if len(rawArgs) >= 2 {
			last := rawArgs[len(rawArgs)-1]
			prev := rawArgs[len(rawArgs)-2]
			if !strings.Contains(last, "+") && prev != "+" && !strings.HasSuffix(prev, "+") {
				dstPattern = last
				dst = pathutil.MapPath(last)
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
				mapped := pathutil.MapPath(part)
				if matches, err := pathutil.GlobCaseInsensitive(mapped); err == nil && len(matches) > 0 {
					for _, m := range matches {
						srcs = append(srcs, srcEntry{path: m, pattern: part})
					}
				} else {
					srcs = append(srcs, srcEntry{path: mapped, pattern: part})
				}
			}
		}
		if dst == "" && len(srcs) > 0 {
			dst = srcs[0].path
		}
	}

	if len(srcs) == 0 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Failure()
		return nil
	}

	confirmOverwrite := func(target string) bool {
		if suppressPrompt {
			return true
		}
		if _, err := os.Stat(target); os.IsNotExist(err) {
			return true
		}
		if p.Stdin == nil {
			return true
		}
		fmt.Fprintf(p.Stdout, "Overwrite %s? (Yes/No/All): ", target)
		var response string
		fmt.Fscanln(p.Stdin, &response)
		switch strings.ToUpper(strings.TrimSpace(response)) {
		case "Y", "YES":
			return true
		case "A", "ALL":
			suppressPrompt = true
			return true
		default:
			return false
		}
	}

	resolveDst := func(srcPath, srcPattern string) string {
		if tools.HasWildcards(dstPattern) {
			return tools.ResolveWildcardDst(srcPath, srcPattern, dstPattern, dst)
		}
		return dst
	}

	dstTarget := resolveDst(srcs[0].path, srcs[0].pattern)
	if info, err := os.Stat(dst); err == nil && info.IsDir() {
		dstTarget = filepath.Join(dst, filepath.Base(srcs[0].path))
	}

	switch {
	case !hasPlus && len(srcs) > 1:
		count := 0
		for _, src := range srcs {
			target := resolveDst(src.path, src.pattern)
			if info, err := os.Stat(dst); err == nil && info.IsDir() {
				target = filepath.Join(dst, filepath.Base(src.path))
			}
			if !confirmOverwrite(target) {
				continue
			}
			if err := tools.CopyFile(src.path, target); err != nil {
				fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
				p.Failure()
				return nil
			}
			count++
		}
		fmt.Fprintf(p.Stdout, "        %d file(s) copied.\n", count)
	case !hasPlus:
		if !confirmOverwrite(dstTarget) {
			return p.Success()
		}
		if err := tools.CopyFile(srcs[0].path, dstTarget); err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
			p.Failure()
			return nil
		}
		fmt.Fprintf(p.Stdout, "        1 file(s) copied.\n")
	default:
		if !confirmOverwrite(dstTarget) {
			return p.Success()
		}
		var buf bytes.Buffer
		for _, src := range srcs {
			data, err := os.ReadFile(src.path)
			if err != nil {
				fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
				p.Failure()
				return nil
			}
			buf.Write(data)
		}
		if err := os.WriteFile(dstTarget, buf.Bytes(), 0666); err != nil {
			fmt.Fprintf(p.Stderr, "Access is denied.\n")
			p.Failure()
			return nil
		}
		fmt.Fprintf(p.Stdout, "        1 file(s) copied.\n")
	}
	return p.Success()
}

func cmdMove(p *processor.Processor, cmd *parser.SimpleCommand) error {
	var args []string
	for _, arg := range cmd.Args {
		switch strings.ToLower(arg) {
		case "/y", "/-y":
		default:
			args = append(args, arg)
		}
	}
	if len(args) < 2 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Failure()
		return nil
	}

	srcPattern := args[0]
	dstPattern := args[1]
	src := pathutil.MapPath(srcPattern)
	dst := pathutil.MapPath(dstPattern)

	srcs := tools.GlobOrLiteral(src)

	dstHasWildcard := tools.HasWildcards(dstPattern)

	count := 0
	for _, srcPath := range srcs {
		target := dst
		if info, err := os.Stat(dst); err == nil && info.IsDir() {
			target = filepath.Join(dst, filepath.Base(srcPath))
		} else if dstHasWildcard {
			target = tools.ResolveWildcardDst(srcPath, srcPattern, dstPattern, dst)
		}
		if err := os.Rename(srcPath, target); err != nil {
			fmt.Fprintf(p.Stderr, "The system cannot find the file specified.\n")
			p.Failure()
			return nil
		}
		count++
	}
	fmt.Fprintf(p.Stdout, "        %d file(s) moved.\n", count)
	return p.Success()
}

func cmdDir(p *processor.Processor, cmd *parser.SimpleCommand) error {
	bare := false
	recursive := false
	dirPath := "."

	for _, arg := range cmd.Args {
		lower := strings.ToLower(arg)
		switch lower {
		case "/b":
			bare = true
		case "/s":
			recursive = true
		default:
			if !strings.HasPrefix(arg, "/") || strings.ContainsRune(arg[1:], '/') {
				dirPath = pathutil.MapPath(arg)
			}
		}
	}

	var fileCount, dirCount int
	abs, _ := filepath.Abs(dirPath)

	if recursive {
		err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			relPath, _ := filepath.Rel(dirPath, path)
			if relPath == "." {
				return nil
			}
			if info.IsDir() {
				dirCount++
				if bare {
					fmt.Fprintf(p.Stdout, "%s\n", path)
				} else {
					t := info.ModTime()
					fmt.Fprintf(p.Stdout, "%s  %s    <DIR>          %s\n",
						t.Format("01/02/2006"), t.Format("03:04 PM"), path)
				}
			} else {
				fileCount++
				if bare {
					fmt.Fprintf(p.Stdout, "%s\n", path)
				} else {
					t := info.ModTime()
					fmt.Fprintf(p.Stdout, "%s  %s    %14d %s\n",
						t.Format("01/02/2006"), t.Format("03:04 PM"), info.Size(), path)
				}
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(p.Stderr, "File Not Found\n")
			p.Failure()
			return nil
		}
		if !bare {
			fmt.Fprintf(p.Stdout, "\n Total Files Listed:\n")
			fmt.Fprintf(p.Stdout, "%15d File(s)%14d bytes\n", fileCount, int64(0))
			fmt.Fprintf(p.Stdout, "%15d Dir(s)\n", dirCount)
		}
		return p.Success()
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		fmt.Fprintf(p.Stderr, "File Not Found\n")
		p.Failure()
		return nil
	}

	if !bare {
		fmt.Fprintf(p.Stdout, " Directory of %s\n\n", abs)
	}

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if e.IsDir() {
			dirCount++
			if bare {
				fmt.Fprintf(p.Stdout, "%s\n", e.Name())
			} else {
				t := info.ModTime()
				fmt.Fprintf(p.Stdout, "%s  %s    <DIR>          %s\n",
					t.Format("01/02/2006"), t.Format("03:04 PM"), e.Name())
			}
		} else {
			fileCount++
			if bare {
				fmt.Fprintf(p.Stdout, "%s\n", e.Name())
			} else {
				t := info.ModTime()
				fmt.Fprintf(p.Stdout, "%s  %s    %14d %s\n",
					t.Format("01/02/2006"), t.Format("03:04 PM"), info.Size(), e.Name())
			}
		}
	}

	if !bare {
		fmt.Fprintf(p.Stdout, "\n%15d File(s)%14d bytes\n", fileCount, int64(0))
		fmt.Fprintf(p.Stdout, "%15d Dir(s)\n", dirCount)
	}

	return p.Success()
}
