package tools

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

type xcopyOpts struct {
	// Source filters
	archiveOnly     bool       // /A  – copy only files with archive bit (stub)
	archiveClear    bool       // /M  – copy archive files and clear bit (stub)
	includeHidden   bool       // /H  – include hidden/system files
	afterDate       *time.Time // /D:mm-dd-yyyy
	newerOnly       bool       // /D  – only if src is newer than dst
	updateOnly      bool       // /U  – only if dst already exists
	recursive       bool       // /S
	includeEmpty    bool       // /E  – include empty subdirectories
	excludePatterns []string   // /EXCLUDE:file[+file2]...

	// Copy behaviour
	copySymlink   bool // /B  – copy symlink itself
	continueOnErr bool // /C  – continue on error
	showFull      bool // /F  – display full src→dst paths
	listOnly      bool // /L  – list without copying
	quiet         bool // /Q  – suppress per-file output
	verify        bool // /V  – verify destination is readable
	promptStart   bool // /W  – press key before starting
	promptPerFile bool // /P  – prompt before each file
	suppressOver  bool // /Y  – overwrite without prompting
	promptOver    bool // /-Y – always prompt before overwriting

	// Destination
	assumeDir         bool // /I – if in doubt, dst is a directory
	overwriteReadOnly bool // /R – overwrite read-only files
	dirsOnly          bool // /T – create directory tree only, no files
	keepAttribs       bool // /K – preserve attributes (default: strip read-only)

	// Runtime state
	overwriteAll bool
}

// Xcopy implements the XCOPY command.
func Xcopy(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) == 0 {
		fmt.Fprintf(p.Stderr, "The syntax of the command is incorrect.\n")
		p.Env.Set("ERRORLEVEL", "4")
		return nil
	}

	opts, srcArg, dstArg, err := parseXcopyArgs(cmd.Args)
	if err != nil {
		fmt.Fprintf(p.Stderr, "%v\n", err)
		p.Env.Set("ERRORLEVEL", "4")
		return nil
	}

	// COPYCMD=/Y suppresses the overwrite prompt globally.
	copycmd, _ := p.Env.Get("COPYCMD")
	if strings.Contains(strings.ToUpper(copycmd), "/Y") {
		opts.suppressOver = true
	}

	// /W: wait for keypress before starting.
	if opts.promptStart {
		fmt.Fprintf(p.Stdout, "Press any key to begin copying file(s)\n")
		if p.Stdin != nil {
			bufio.NewReader(p.Stdin).ReadByte()
		}
	}

	// Resolve source.
	mappedSrc := processor.MapPath(srcArg)
	isGlob := strings.ContainsAny(filepath.Base(srcArg), "*?")

	var srcPaths []string
	if isGlob {
		matches, err := filepath.Glob(mappedSrc)
		if err != nil || len(matches) == 0 {
			fmt.Fprintf(p.Stderr, "File not found - %s\n", srcArg)
			p.Env.Set("ERRORLEVEL", "1")
			return nil
		}
		srcPaths = matches
	} else {
		srcPaths = []string{mappedSrc}
	}

	// Determine whether destination is a file path or a directory.
	mappedDst := ""
	if dstArg != "" {
		mappedDst = processor.MapPath(dstArg)
	}

	srcInfo, _ := os.Lstat(mappedSrc)
	isSingleFileSrc := !isGlob && srcInfo != nil && !srcInfo.IsDir()

	dstIsDir := false
	switch {
	case mappedDst == "":
		cwd, _ := os.Getwd()
		mappedDst = cwd
		dstIsDir = true
	case strings.HasSuffix(dstArg, "\\") || strings.HasSuffix(dstArg, "/"):
		dstIsDir = true
	case strings.HasSuffix(dstArg, "*"):
		// Trailing * forces file destination (suppress F/D prompt).
		mappedDst = mappedDst[:len(mappedDst)-1]
		dstIsDir = false
	default:
		if info, err := os.Stat(mappedDst); err == nil {
			dstIsDir = info.IsDir()
		} else if !isSingleFileSrc || opts.assumeDir || isGlob {
			dstIsDir = true
		} else {
			// Single file, non-existent destination, no /I → prompt.
			dstIsDir = xcopyPromptFileOrDir(p, dstArg) == "D"
		}
	}

	count, failed := 0, 0
	for _, src := range srcPaths {
		info, err := os.Lstat(src)
		if err != nil {
			fmt.Fprintf(p.Stderr, "File not found - %s\n", src)
			failed++
			if !opts.continueOnErr {
				break
			}
			continue
		}

		if info.IsDir() {
			dst := mappedDst
			if dstIsDir && len(srcPaths) > 1 {
				dst = filepath.Join(mappedDst, filepath.Base(src))
			}
			n, f := xcopyWalkDir(p, src, dst, opts)
			count += n
			failed += f
		} else {
			dst := mappedDst
			if dstIsDir {
				dst = filepath.Join(mappedDst, filepath.Base(src))
			}
			ok, err := xcopyCopyOne(p, src, dst, opts)
			if err != nil {
				failed++
				if !opts.continueOnErr {
					break
				}
			} else if ok {
				count++
			}
		}
	}

	if !opts.listOnly {
		fmt.Fprintf(p.Stdout, "%d File(s) copied\n", count)
	}
	if failed > 0 {
		p.Env.Set("ERRORLEVEL", "5")
	} else if count == 0 {
		p.Env.Set("ERRORLEVEL", "1")
	} else {
		p.Env.Set("ERRORLEVEL", "0")
	}
	return nil
}

// xcopyWalkDir copies a directory tree from src to dst.
func xcopyWalkDir(p *processor.Processor, src, dst string, opts *xcopyOpts) (copied, failed int) {
	if !opts.listOnly && (opts.includeEmpty || !opts.dirsOnly) {
		os.MkdirAll(dst, 0755)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return 0, 1
	}

	for _, e := range entries {
		name := e.Name()
		if !opts.includeHidden && isHidden(name) {
			continue
		}

		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)

		if absSrc, err := filepath.Abs(srcPath); err == nil {
			if isExcluded(absSrc, opts.excludePatterns) {
				continue
			}
		}

		if e.IsDir() {
			if !opts.recursive {
				continue
			}
			if !opts.includeEmpty {
				if empty, _ := isDirEmpty(srcPath); empty {
					continue
				}
			}
			if opts.dirsOnly && !opts.listOnly {
				os.MkdirAll(dstPath, 0755)
			}
			n, f := xcopyWalkDir(p, srcPath, dstPath, opts)
			copied += n
			failed += f
		} else {
			ok, err := xcopyCopyOne(p, srcPath, dstPath, opts)
			if err != nil {
				failed++
				if !opts.continueOnErr {
					return copied, failed
				}
			} else if ok {
				copied++
			}
		}
	}
	return copied, failed
}

// xcopyCopyOne copies a single file, applying all filters and prompts.
func xcopyCopyOne(p *processor.Processor, src, dst string, opts *xcopyOpts) (bool, error) {
	info, err := os.Lstat(src)
	if err != nil {
		return false, err
	}

	// Hidden file filter.
	if !opts.includeHidden && isHidden(filepath.Base(src)) {
		return false, nil
	}

	// /EXCLUDE: pattern filter.
	if absSrc, err := filepath.Abs(src); err == nil {
		if isExcluded(absSrc, opts.excludePatterns) {
			return false, nil
		}
	}

	// /U: only update existing destination files.
	if opts.updateOnly {
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			return false, nil
		}
	}

	// /D:date — skip files older than the given date.
	if opts.afterDate != nil {
		if info.ModTime().Before(*opts.afterDate) {
			return false, nil
		}
	}

	// /D (no date) — skip if src is not newer than dst.
	if opts.newerOnly && opts.afterDate == nil {
		if dstInfo, err := os.Stat(dst); err == nil {
			if !info.ModTime().After(dstInfo.ModTime()) {
				return false, nil
			}
		}
	}

	// /T: directory structure only — skip files.
	if opts.dirsOnly {
		return false, nil
	}

	// /L: list without copying.
	if opts.listOnly {
		if !opts.quiet {
			absSrc, _ := filepath.Abs(src)
			if opts.showFull {
				absDst, _ := filepath.Abs(dst)
				fmt.Fprintf(p.Stdout, "%s -> %s\n", toBackslash(absSrc), toBackslash(absDst))
			} else {
				fmt.Fprintf(p.Stdout, "%s\n", toBackslash(absSrc))
			}
		}
		return true, nil
	}

	// /P: prompt before creating each file.
	if opts.promptPerFile {
		if !xcopyPromptYN(p, fmt.Sprintf("Create %s", toBackslash(dst))) {
			return false, nil
		}
	}

	// Overwrite checks.
	if dstInfo, err := os.Stat(dst); err == nil && !dstInfo.IsDir() {
		if opts.promptOver && !opts.overwriteAll {
			switch xcopyPromptOverwrite(p, toBackslash(dst)) {
			case "N":
				return false, nil
			case "A":
				opts.overwriteAll = true
			}
		}
		// Read-only destination.
		if dstInfo.Mode()&0200 == 0 {
			if opts.overwriteReadOnly {
				os.Chmod(dst, dstInfo.Mode()|0200)
			} else {
				fmt.Fprintf(p.Stderr, "Access denied - %s\n", toBackslash(dst))
				return false, fmt.Errorf("access denied: %s", dst)
			}
		}
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return false, err
	}

	// Perform the copy.
	if opts.copySymlink && info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return false, err
		}
		os.Remove(dst)
		if err := os.Symlink(target, dst); err != nil {
			return false, err
		}
	} else {
		if err := copyFile(src, dst); err != nil {
			fmt.Fprintf(p.Stderr, "Error copying %s\n", toBackslash(src))
			return false, err
		}
	}

	// /K: preserve attributes; default behaviour strips read-only.
	if opts.keepAttribs {
		os.Chmod(dst, info.Mode())
	} else {
		if dstInfo, err := os.Stat(dst); err == nil {
			os.Chmod(dst, dstInfo.Mode()|0200)
		}
	}

	// /V: verify destination is readable.
	if opts.verify {
		f, err := os.Open(dst)
		if err != nil {
			return false, fmt.Errorf("verification failed: %s", toBackslash(dst))
		}
		f.Close()
	}

	// Display source path (/Q suppresses; /F shows full src→dst).
	if !opts.quiet {
		if opts.showFull {
			absSrc, _ := filepath.Abs(src)
			absDst, _ := filepath.Abs(dst)
			fmt.Fprintf(p.Stdout, "%s -> %s\n", toBackslash(absSrc), toBackslash(absDst))
		} else {
			fmt.Fprintf(p.Stdout, "%s\n", toBackslash(src))
		}
	}

	return true, nil
}

// ---- argument parser ----

func parseXcopyArgs(args []string) (*xcopyOpts, string, string, error) {
	opts := &xcopyOpts{}
	var positional []string

	for i := range args {
		arg := args[i]
		lower := strings.ToLower(arg)
		switch {
		case lower == "/a":
			opts.archiveOnly = true
		case lower == "/m":
			opts.archiveClear = true
		case lower == "/h":
			opts.includeHidden = true
		case lower == "/d":
			opts.newerOnly = true
		case strings.HasPrefix(lower, "/d:"):
			t, err := time.Parse("01-02-2006", arg[3:])
			if err != nil {
				return nil, "", "", fmt.Errorf("invalid date format: %s", arg[3:])
			}
			opts.afterDate = &t
		case lower == "/u":
			opts.updateOnly = true
		case lower == "/s":
			opts.recursive = true
		case lower == "/e":
			opts.recursive = true
			opts.includeEmpty = true
		case strings.HasPrefix(lower, "/exclude:"):
			parts := strings.Split(arg[9:], "+")
			for _, f := range parts {
				pats, err := loadExcludeFile(processor.MapPath(f))
				if err != nil {
					return nil, "", "", fmt.Errorf("cannot open exclude file: %s", f)
				}
				opts.excludePatterns = append(opts.excludePatterns, pats...)
			}
		case lower == "/b":
			opts.copySymlink = true
		case lower == "/c":
			opts.continueOnErr = true
		case lower == "/f":
			opts.showFull = true
		case lower == "/l":
			opts.listOnly = true
		case lower == "/q":
			opts.quiet = true
		case lower == "/v":
			opts.verify = true
		case lower == "/w":
			opts.promptStart = true
		case lower == "/p":
			opts.promptPerFile = true
		case lower == "/y":
			opts.suppressOver = true
		case lower == "/-y":
			opts.promptOver = true
		case lower == "/i":
			opts.assumeDir = true
		case lower == "/r":
			opts.overwriteReadOnly = true
		case lower == "/t":
			opts.dirsOnly = true
			opts.recursive = true
		case lower == "/k":
			opts.keepAttribs = true
		case lower == "/compress", lower == "/noclone", lower == "/j", lower == "/z",
			lower == "/g", lower == "/n", lower == "/o", lower == "/x",
			lower == "/sparse", lower == "/-sparse":
			// accepted but not functionally implemented
		default:
			if !strings.HasPrefix(arg, "/") {
				positional = append(positional, arg)
			}
		}
	}

	if len(positional) == 0 {
		return nil, "", "", fmt.Errorf("the syntax of the command is incorrect")
	}
	src := positional[0]
	dst := ""
	if len(positional) > 1 {
		dst = positional[1]
	}
	return opts, src, dst, nil
}

// ---- prompts ----

func xcopyPromptFileOrDir(p *processor.Processor, dst string) string {
	fmt.Fprintf(p.Stdout,
		"Does %s specify a file name\nor directory name on the target\n(F = file, D = directory)? ", dst)
	if p.Stdin == nil {
		return "D"
	}
	line, _ := bufio.NewReader(p.Stdin).ReadString('\n')
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "F") {
		return "F"
	}
	return "D"
}

func xcopyPromptYN(p *processor.Processor, msg string) bool {
	fmt.Fprintf(p.Stdout, "%s (Yes/No)? ", msg)
	if p.Stdin == nil {
		return true
	}
	line, _ := bufio.NewReader(p.Stdin).ReadString('\n')
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "Y")
}

func xcopyPromptOverwrite(p *processor.Processor, dst string) string {
	fmt.Fprintf(p.Stdout, "Overwrite %s (Yes/No/All)? ", dst)
	if p.Stdin == nil {
		return "Y"
	}
	line, _ := bufio.NewReader(p.Stdin).ReadString('\n')
	upper := strings.ToUpper(strings.TrimSpace(line))
	switch {
	case strings.HasPrefix(upper, "A"):
		return "A"
	case strings.HasPrefix(upper, "N"):
		return "N"
	default:
		return "Y"
	}
}

// ---- helpers ----

func loadExcludeFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var pats []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			pats = append(pats, line)
		}
	}
	return pats, nil
}

// isExcluded reports whether absPath contains any of the exclude patterns
// (case-insensitive substring match, as real XCOPY does).
func isExcluded(absPath string, patterns []string) bool {
	lower := strings.ToLower(absPath)
	for _, pat := range patterns {
		if strings.Contains(lower, strings.ToLower(pat)) {
			return true
		}
	}
	return false
}

// isHidden uses the dotfile convention as a cross-platform proxy for the
// Windows hidden attribute (which requires a syscall to query).
func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

// toBackslash converts forward slashes to backslashes for Windows-style output.
func toBackslash(p string) string {
	return strings.ReplaceAll(p, "/", "\\")
}
