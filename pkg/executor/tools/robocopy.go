package tools

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// robocopyStats tracks per-run counters for the summary table.
type robocopyStats struct {
	dirs    [6]int // Total, Copied, Skipped, Mismatch, Failed, Extras
	files   [6]int
	failed  bool
}

const (
	statTotal    = 0
	statCopied   = 1
	statSkipped  = 2
	statMismatch = 3
	statFailed   = 4
	statExtras   = 5
)

func Robocopy(p *processor.Processor, cmd *parser.SimpleCommand) error {
	// ROBOCOPY <src> <dst> [file...] [options]
	//
	// Supported flags:
	//   /S        copy subdirectories (non-empty)
	//   /E        copy subdirectories (including empty)
	//   /MIR      mirror src to dst (implies /E, removes extras from dst)
	//   /MOV      move files (delete source files after copy)
	//   /MOVE     move files and dirs (delete source after copy)
	//   /XF <f>   exclude files matching pattern(s)
	//   /XD <d>   exclude directories matching pattern(s)
	//   /NP       no progress (ignored — we never print progress)
	//   /NFL      no file list (suppress per-file log lines)
	//   /NDL      no dir list (suppress per-directory log lines)
	//   /LOG:<f>  write log to file instead of stdout

	if len(cmd.Args) < 2 {
		fmt.Fprintf(p.Stderr, "ERROR: Invalid number of parameters.\n")
		p.Env.Set("ERRORLEVEL", "16")
		return nil
	}

	src := processor.MapPath(cmd.Args[0])
	dst := processor.MapPath(cmd.Args[1])

	recursive := false
	includeEmpty := false
	mirror := false
	moveFiles := false
	moveDirs := false
	noFileList := false
	noDirList := false
	var excludeFiles []string
	var excludeDirs []string
	var filePatterns []string
	logPath := ""

	// parse remaining args (index 2+)
	args := cmd.Args[2:]
	for i := 0; i < len(args); i++ {
		lower := strings.ToLower(args[i])
		switch {
		case lower == "/s":
			recursive = true
		case lower == "/e":
			recursive = true
			includeEmpty = true
		case lower == "/mir":
			mirror = true
			recursive = true
			includeEmpty = true
		case lower == "/mov":
			moveFiles = true
		case lower == "/move":
			moveFiles = true
			moveDirs = true
		case lower == "/np":
			// no progress — always the case in non-interactive mode
		case lower == "/nfl":
			noFileList = true
		case lower == "/ndl":
			noDirList = true
		case lower == "/xf":
			for i+1 < len(args) && !strings.HasPrefix(args[i+1], "/") {
				i++
				excludeFiles = append(excludeFiles, args[i])
			}
		case lower == "/xd":
			for i+1 < len(args) && !strings.HasPrefix(args[i+1], "/") {
				i++
				excludeDirs = append(excludeDirs, args[i])
			}
		case strings.HasPrefix(lower, "/log:"):
			logPath = processor.MapPath(args[i][5:])
		case strings.HasPrefix(lower, "/"):
			// ignore unrecognised flags
		default:
			filePatterns = append(filePatterns, args[i])
		}
	}

	if len(filePatterns) == 0 {
		filePatterns = []string{"*.*"}
	}

	// Resolve output writer — /LOG: redirects all output to a file.
	out := io.Writer(p.Stdout)
	if logPath != "" {
		lf, err := os.Create(logPath)
		if err != nil {
			fmt.Fprintf(p.Stderr, "ERROR: Cannot open log file %s\n", logPath)
			p.Env.Set("ERRORLEVEL", "16")
			return nil
		}
		defer lf.Close()
		out = lf
	}

	started := time.Now()
	printHeader(out, src, dst, filePatterns)

	var stats robocopyStats

	err := robocopyDir(out, src, dst, filePatterns, excludeFiles, excludeDirs,
		recursive, includeEmpty, mirror, moveFiles, moveDirs, noFileList, noDirList, &stats)
	if err != nil {
		stats.failed = true
	}

	// In mirror mode, remove extras (files/dirs in dst not in src).
	if mirror {
		removeExtras(out, src, dst, noDirList, noFileList, &stats)
	}

	printSummary(out, started, &stats)

	// Robocopy exit codes:
	//   0 = no files copied, no mismatch, no failure
	//   1 = files copied successfully
	//   2 = extra files or directories found in dst (MIR)
	//   8 = failures
	code := 0
	if stats.failed || stats.files[statFailed] > 0 || stats.dirs[statFailed] > 0 {
		code = 8
	} else if stats.files[statExtras] > 0 || stats.dirs[statExtras] > 0 {
		code = 2
	} else if stats.files[statCopied] > 0 || stats.dirs[statCopied] > 0 {
		code = 1
	}
	p.Env.Set("ERRORLEVEL", fmt.Sprintf("%d", code))
	return nil
}

func robocopyDir(
	out io.Writer,
	src, dst string,
	filePatterns, excludeFiles, excludeDirs []string,
	recursive, includeEmpty, mirror, moveFiles, moveDirs, noFileList, noDirList bool,
	stats *robocopyStats,
) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source directory not found: %s", src)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	stats.dirs[statTotal]++

	if !noDirList {
		fmt.Fprintf(out, "\n    %s\\\n", src)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, e := range entries {
		name := e.Name()

		if e.IsDir() {
			if !recursive {
				continue
			}
			if isDirExcluded(name, excludeDirs) {
				stats.dirs[statSkipped]++
				continue
			}
			childSrc := filepath.Join(src, name)
			childDst := filepath.Join(dst, name)
			if !includeEmpty {
				if empty, _ := isDirEmpty(childSrc); empty {
					stats.dirs[statSkipped]++
					continue
				}
			}
			stats.dirs[statCopied]++
			if err := robocopyDir(out, childSrc, childDst,
				filePatterns, excludeFiles, excludeDirs,
				recursive, includeEmpty, mirror, moveFiles, moveDirs, noFileList, noDirList, stats); err != nil {
				stats.dirs[statFailed]++
			} else if moveDirs {
				// remove source dir after copy (only when empty)
				os.Remove(childSrc)
			}
		} else {
			if !matchesAnyPattern(name, filePatterns) {
				stats.files[statSkipped]++
				continue
			}
			if isFileExcluded(name, excludeFiles) {
				stats.files[statSkipped]++
				continue
			}

			srcFile := filepath.Join(src, name)
			dstFile := filepath.Join(dst, name)

			stats.files[statTotal]++

			// Skip if destination is identical (same size + mtime).
			if filesAreIdentical(srcFile, dstFile) {
				stats.files[statSkipped]++
				continue
			}

			if err := copyFile(srcFile, dstFile); err != nil {
				stats.files[statFailed]++
				fmt.Fprintf(out, "  ERROR copying %s: %v\n", srcFile, err)
				continue
			}
			stats.files[statCopied]++
			if !noFileList {
				fmt.Fprintf(out, "\t%s\n", name)
			}
			if moveFiles {
				os.Remove(srcFile)
			}
		}
	}
	return nil
}

// removeExtras deletes files and directories in dst that are not present in src.
func removeExtras(out io.Writer, src, dst string, noDirList, noFileList bool, stats *robocopyStats) {
	dstEntries, err := os.ReadDir(dst)
	if err != nil {
		return
	}
	for _, e := range dstEntries {
		name := e.Name()
		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			if e.IsDir() {
				stats.dirs[statExtras]++
				if !noDirList {
					fmt.Fprintf(out, "  *EXTRA Dir\t%s\\\n", dstPath)
				}
				os.RemoveAll(dstPath)
			} else {
				stats.files[statExtras]++
				if !noFileList {
					fmt.Fprintf(out, "  *EXTRA File\t%s\n", dstPath)
				}
				os.Remove(dstPath)
			}
		} else if e.IsDir() {
			removeExtras(out, srcPath, dstPath, noDirList, noFileList, stats)
		}
	}
}

func printHeader(out io.Writer, src, dst string, patterns []string) {
	line := strings.Repeat("-", 79)
	fmt.Fprintln(out, line)
	fmt.Fprintln(out, "   ROBOCOPY     ::     Robust File Copy")
	fmt.Fprintln(out, line)
	fmt.Fprintf(out, "   Source : %s\\\n", src)
	fmt.Fprintf(out, "     Dest : %s\\\n", dst)
	fmt.Fprintf(out, "    Files : %s\n", strings.Join(patterns, " "))
	fmt.Fprintln(out, line)
}

func printSummary(out io.Writer, started time.Time, stats *robocopyStats) {
	elapsed := time.Since(started).Truncate(time.Second)
	line := strings.Repeat("-", 79)
	fmt.Fprintln(out, line)
	fmt.Fprintf(out, "%20s %8s %8s %8s %8s %8s %8s\n",
		"", "Total", "Copied", "Skipped", "Mismatch", "FAILED", "Extras")
	fmt.Fprintf(out, "%20s %8d %8d %8d %8d %8d %8d\n",
		"Dirs :", stats.dirs[0], stats.dirs[1], stats.dirs[2], stats.dirs[3], stats.dirs[4], stats.dirs[5])
	fmt.Fprintf(out, "%20s %8d %8d %8d %8d %8d %8d\n",
		"Files :", stats.files[0], stats.files[1], stats.files[2], stats.files[3], stats.files[4], stats.files[5])
	fmt.Fprintf(out, "%20s %v\n", "Ended :", started.Add(elapsed).Format("Mon Jan 02 15:04:05 2006"))
	fmt.Fprintln(out, line)
}

// filesAreIdentical returns true when dst exists and has the same size and
// modification time as src (same heuristic real robocopy uses by default).
func filesAreIdentical(src, dst string) bool {
	si, err := os.Stat(src)
	if err != nil {
		return false
	}
	di, err := os.Stat(dst)
	if err != nil {
		return false
	}
	return si.Size() == di.Size() && si.ModTime().Equal(di.ModTime())
}

func matchesAnyPattern(name string, patterns []string) bool {
	for _, pat := range patterns {
		if matched, _ := filepath.Match(strings.ToLower(pat), strings.ToLower(name)); matched {
			return true
		}
	}
	return false
}

func isFileExcluded(name string, excludeFiles []string) bool {
	for _, pat := range excludeFiles {
		if matched, _ := filepath.Match(strings.ToLower(pat), strings.ToLower(name)); matched {
			return true
		}
	}
	return false
}

func isDirExcluded(name string, excludeDirs []string) bool {
	for _, pat := range excludeDirs {
		if matched, _ := filepath.Match(strings.ToLower(pat), strings.ToLower(name)); matched {
			return true
		}
	}
	return false
}

func isDirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}
