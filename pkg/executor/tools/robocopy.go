package tools

// robocopy.go: complete native cross-platform Robocopy implementation.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/parser"
	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// ── file classification ───────────────────────────────────────────────────────

type rcFileClass int

const (
	rcNewFile rcFileClass = iota // only in source (no matching destination)
	rcNewer                      // source is newer than destination
	rcOlder                      // source is older than destination
	rcSame                       // identical timestamp + size (within granularity)
	rcChanged                    // sizes differ (timestamps may also differ)
)

func (c rcFileClass) label() string {
	switch c {
	case rcNewFile:
		return "New File"
	case rcNewer:
		return "Newer"
	case rcOlder:
		return "Older"
	case rcSame:
		return "same"
	case rcChanged:
		return "Changed"
	default:
		return "Unknown"
	}
}

// ── stats ─────────────────────────────────────────────────────────────────────

// rcStat stores [Total, Copied, Skipped, Mismatch, Failed, Extras] for one
// category of item (dirs, files, or bytes).
type rcStat [6]int64

const (
	rcTotal    = 0
	rcCopied   = 1
	rcSkipped  = 2
	rcMismatch = 3
	rcFailed   = 4
	rcExtras   = 5
)

// rcStats is shared across goroutines when /MT is active; all mutations must
// hold mu.
type rcStats struct {
	mu    sync.Mutex
	dirs  rcStat
	files rcStat
	bytes rcStat
}

func (s *rcStats) addFiles(field int, delta int64) {
	s.mu.Lock()
	s.files[field] += delta
	s.mu.Unlock()
}

// addFilesAndBytes updates a files field and its corresponding bytes field in
// one lock acquisition, keeping the two categories consistent.
func (s *rcStats) addFilesAndBytes(field int, count, byteCount int64) {
	s.mu.Lock()
	s.files[field] += count
	s.bytes[field] += byteCount
	s.mu.Unlock()
}

func (s *rcStats) addDirs(field int, delta int64) {
	s.mu.Lock()
	s.dirs[field] += delta
	s.mu.Unlock()
}

// ── thread-safe writer ────────────────────────────────────────────────────────

// lockedWriter serialises concurrent writes so that log lines from parallel
// goroutines don't interleave.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (lw *lockedWriter) Write(p []byte) (int, error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()
	return lw.w.Write(p)
}

// ── options ───────────────────────────────────────────────────────────────────

type rcOpts struct {
	// Source
	recursive    bool
	includeEmpty bool
	archiveOnly  bool // /A  – stub (no archive bit on Linux/macOS)
	archiveClear bool // /M  – stub
	maxLevel     int  // /LEV:n; 0 = unlimited
	maxAgeDays   int
	maxAgeDate   *time.Time
	minAgeDays   int
	minAgeDate   *time.Time
	fatTimes     bool // /FFT: 2-second time granularity

	// Copy
	listOnly     bool
	moveFiles    bool // /MOV
	moveDirs     bool // /MOVE
	copySymlinks bool // /SL
	createOnly   bool // /CREATE: zero-length files + directory structure
	retries      int
	retryWait    int // seconds
	threads      int // /MT[:n]; 0 = single-threaded

	// Destination
	purge   bool   // /PURGE
	mirror  bool   // /MIR (implies /PURGE + /E)
	attrAdd string // /A+:flags
	attrDel string // /A-:flags

	// Include / Exclude
	excludeOlder   bool
	excludeChanged bool
	excludeNewer   bool
	excludeLonely  bool     // /XL: don't copy files only in source
	excludeExtra   bool     // /XX: suppress listing / deletion of extra dest items
	includeSame    bool     // /IS: overwrite even if same
	excludeJunct   bool     // /XJ
	excludeJunctD  bool     // /XJD
	excludeJunctF  bool     // /XJF
	excludeFiles   []string // /XF
	excludeDirs    []string // /XD
	maxSize        int64    // /MAX:n; 0 = disabled
	minSize        int64    // /MIN:n; 0 = disabled

	// Logging
	logPath     string
	logAppend   bool
	tee         bool
	showTS      bool
	fullPath    bool
	noSize      bool
	noClass     bool
	noFileList  bool
	noDirList   bool
	noHeader    bool
	noSummary   bool
	verbose     bool // /V: also log skipped files
	reportExtra bool // /X: report all extra files

	filePatterns []string
}

// ── entry point ───────────────────────────────────────────────────────────────

const robocopyHelp = `Robust file copy utility.

ROBOCOPY source destination [file [file]...] [options]

  source       Source directory path.
  destination  Destination directory path.
  file         File(s) to copy (default: *.*).

Copy options:
  /S           Copies subdirectories; excludes empty ones.
  /E           Copies subdirectories, including empty ones.
  /LEV:n       Only copies the top n levels of the source tree.
  /MOV         Moves files (deletes from source after copying).
  /MOVE        Moves files and directories.
  /A           Copies only files with the Archive attribute (stub).
  /M           Copies only Archive files; clears the attribute (stub).
  /CREATE      Creates a directory tree and zero-length files only.
  /SL          Copies symbolic links instead of targets.
  /FFT         Assumes FAT file times (2-second granularity).
  /MIR         Mirrors a directory tree (equivalent to /E /PURGE).
  /PURGE       Deletes destination files/directories that no longer exist in source.

Filter options:
  /XO          Excludes older files (source older than destination).
  /XC          Excludes changed files.
  /XN          Excludes newer files.
  /XL          Excludes lonely files (only in source).
  /XX          Excludes extra files (only in destination).
  /IS          Includes same files (overwrite even if identical).
  /XF file...  Excludes files matching given names/wildcards.
  /XD dir...   Excludes directories matching given names.
  /XJ          Excludes junction points.
  /MAX:n       Excludes files larger than n bytes.
  /MIN:n       Excludes files smaller than n bytes.
  /MAXAGE:n    Excludes files older than n days or date (YYYYMMDD).
  /MINAGE:n    Excludes files newer than n days or date (YYYYMMDD).

Performance options:
  /MT[:n]      Multi-threaded copy with n threads (default 8, max 128).
  /R:n         Number of retries on failed copies (default 0).
  /W:n         Wait time in seconds between retries (default 30).

Logging options:
  /L           List only; does not copy, timestamp, or delete any files.
  /V           Produces verbose output, showing skipped files.
  /TS          Includes source file timestamps in the output.
  /FP          Includes the full pathname of files in the output.
  /NS          No size; file sizes are not logged.
  /NC          No class; file classes are not logged.
  /NFL         No file list; file names are not logged.
  /NDL         No directory list; directory names are not logged.
  /NP          No progress; percentage copied is not displayed.
  /NJH         No job header.
  /NJS         No job summary.
  /LOG:file    Writes status output to the log file (overwrites).
  /LOG+:file   Writes status output to the log file (appends).
  /TEE         Writes status output to the console window, and to the log file.

Exit codes (bitwise OR):
  0   No files were copied. No files were mismatched. No failures.
  1   All files were copied successfully.
  2   Extra files or directories were detected. No files were copied.
  4   Some mismatched files or directories were detected.
  8   Some files or directories could not be copied.
  16  Fatal error; invalid parameters or access denied.
`

func Robocopy(p *processor.Processor, cmd *parser.SimpleCommand) error {
	if len(cmd.Args) < 2 {
		fmt.Fprintf(p.Stderr, "ERROR: Invalid number of parameters.\n")
		p.FailureWithCode(16)
		return nil
	}

	src := pathutil.MapPath(cmd.Args[0])
	dst := pathutil.MapPath(cmd.Args[1])

	opts := rcParseOpts(cmd.Args[2:])

	if opts.mirror {
		opts.purge = true
		opts.recursive = true
		opts.includeEmpty = true
	}

	srcInfo, err := os.Stat(src)
	if err != nil || !srcInfo.IsDir() {
		fmt.Fprintf(p.Stderr, "ERROR: Source directory %q not found or is not a directory.\n", src)
		p.FailureWithCode(16)
		return nil
	}
	if filepath.Clean(src) == filepath.Clean(dst) {
		fmt.Fprintf(p.Stderr, "ERROR: Source and Destination must be different.\n")
		p.FailureWithCode(16)
		return nil
	}

	rawOut, closeOut := rcSetupOutput(p, opts)
	if closeOut != nil {
		defer closeOut()
	}

	// Wrap in a lockedWriter when multi-threading so that log lines from
	// concurrent goroutines don't interleave.
	var out io.Writer = rawOut
	if opts.threads > 1 {
		out = &lockedWriter{w: rawOut}
	}

	started := time.Now()
	var stats rcStats

	if !opts.noHeader {
		rcPrintHeader(out, src, dst, opts, started)
	}

	if !opts.listOnly {
		os.MkdirAll(dst, 0755)
	}

	rcProcessDir(out, src, dst, opts, &stats, 0)

	if (opts.purge || opts.mirror) && !opts.listOnly {
		rcPurgeExtras(out, src, dst, opts, &stats)
	}

	if !opts.noSummary {
		rcPrintSummary(out, started, &stats)
	}

	// Exit code is a bitwise combination of:
	//   1 = files copied, 2 = extra files, 4 = mismatches, 8 = failures
	code := 0
	if stats.files[rcFailed] > 0 || stats.dirs[rcFailed] > 0 {
		code |= 8
	}
	if stats.files[rcMismatch] > 0 || stats.dirs[rcMismatch] > 0 {
		code |= 4
	}
	if stats.files[rcExtras] > 0 || stats.dirs[rcExtras] > 0 {
		code |= 2
	}
	if stats.files[rcCopied] > 0 {
		code |= 1
	}

	p.FailureWithCode(code)
	return nil
}

// ── directory walker ──────────────────────────────────────────────────────────

// rcFileWork holds the parameters for a single deferred file-copy task.
type rcFileWork struct {
	src, dst, name string
}

func rcProcessDir(out io.Writer, src, dst string, opts *rcOpts, stats *rcStats, level int) {
	if opts.maxLevel > 0 && level >= opts.maxLevel {
		return
	}

	stats.addDirs(rcTotal, 1)
	stats.addDirs(rcCopied, 1) // every directory we enter counts as "copied"

	if !opts.listOnly {
		os.MkdirAll(dst, 0755)
	}

	if !opts.noDirList {
		dir := src
		if opts.fullPath {
			dir, _ = filepath.Abs(src)
		}
		fmt.Fprintf(out, "\n\t%s%c\n", dir, os.PathSeparator)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		stats.addDirs(rcFailed, 1)
		stats.addDirs(rcCopied, -1) // undo the optimistic count
		return
	}

	// Separate entries into files and subdirectories so that file copies can
	// be parallelised while directory recursion stays serial.
	var files []rcFileWork
	var subdirs []struct{ src, dst string }

	for _, e := range entries {
		name := e.Name()
		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)

		info, err := os.Lstat(srcPath)
		if err != nil {
			continue
		}

		isSymlink := info.Mode()&os.ModeSymlink != 0

		if e.IsDir() || (isSymlink && isSymlinkToDir(srcPath)) {
			if !opts.recursive {
				continue
			}
			if isSymlink && (opts.excludeJunct || opts.excludeJunctD) {
				stats.addDirs(rcTotal, 1)
				stats.addDirs(rcSkipped, 1)
				continue
			}
			if rcDirExcluded(name, srcPath, opts.excludeDirs) {
				stats.addDirs(rcTotal, 1)
				stats.addDirs(rcSkipped, 1)
				continue
			}
			if !opts.includeEmpty {
				if empty, _ := isDirEmpty(srcPath); empty {
					stats.addDirs(rcTotal, 1)
					stats.addDirs(rcSkipped, 1)
					continue
				}
			}
			subdirs = append(subdirs, struct{ src, dst string }{srcPath, dstPath})
		} else {
			if !rcMatchesFilePattern(name, opts.filePatterns) {
				continue
			}
			files = append(files, rcFileWork{srcPath, dstPath, name})
		}
	}

	// ── process files ────────────────────────────────────────────────────────
	if opts.threads > 1 {
		var wg sync.WaitGroup
		sem := make(chan struct{}, opts.threads)
		for _, f := range files {
			wg.Add(1)
			sem <- struct{}{}
			go func(fw rcFileWork) {
				defer wg.Done()
				defer func() { <-sem }()
				rcProcessFile(out, fw.src, fw.dst, fw.name, opts, stats)
			}(f)
		}
		wg.Wait()
	} else {
		for _, f := range files {
			rcProcessFile(out, f.src, f.dst, f.name, opts, stats)
		}
	}

	// ── recurse into subdirectories (always serial) ──────────────────────────
	for _, d := range subdirs {
		rcProcessDir(out, d.src, d.dst, opts, stats, level+1)
		if opts.moveDirs {
			os.Remove(d.src) // succeeds only when now empty
		}
	}
}

// ── file processor ────────────────────────────────────────────────────────────

func rcProcessFile(out io.Writer, src, dst, name string, opts *rcOpts, stats *rcStats) {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		stats.addFiles(rcFailed, 1)
		return
	}

	isSymlink := srcInfo.Mode()&os.ModeSymlink != 0

	// Symlink / junction file exclusion.
	if isSymlink && (opts.excludeJunct || opts.excludeJunctF) {
		stats.addFiles(rcSkipped, 1)
		return
	}

	// Size filters.
	if opts.maxSize > 0 && srcInfo.Size() > opts.maxSize {
		stats.addFiles(rcSkipped, 1)
		return
	}
	if opts.minSize > 0 && srcInfo.Size() < opts.minSize {
		stats.addFiles(rcSkipped, 1)
		return
	}

	// Age filters.
	now := time.Now()
	mtime := srcInfo.ModTime()
	if opts.maxAgeDays > 0 && mtime.Before(now.AddDate(0, 0, -opts.maxAgeDays)) {
		stats.addFiles(rcSkipped, 1)
		return
	}
	if opts.maxAgeDate != nil && mtime.Before(*opts.maxAgeDate) {
		stats.addFiles(rcSkipped, 1)
		return
	}
	if opts.minAgeDays > 0 && mtime.After(now.AddDate(0, 0, -opts.minAgeDays)) {
		stats.addFiles(rcSkipped, 1)
		return
	}
	if opts.minAgeDate != nil && mtime.After(*opts.minAgeDate) {
		stats.addFiles(rcSkipped, 1)
		return
	}

	// File name / pattern exclusion.
	if rcFileExcluded(name, src, opts.excludeFiles) {
		stats.addFiles(rcSkipped, 1)
		return
	}

	// Classify against destination.
	dstInfo, dstErr := os.Lstat(dst)
	var cls rcFileClass
	switch {
	case os.IsNotExist(dstErr):
		cls = rcNewFile
	case dstErr != nil:
		stats.addFiles(rcFailed, 1)
		return
	default:
		cls = rcClassify(srcInfo, dstInfo, opts.fatTimes)
	}

	stats.addFilesAndBytes(rcTotal, 1, srcInfo.Size())

	// Apply copy-decision filters.
	if !rcShouldCopy(cls, opts) {
		stats.addFilesAndBytes(rcSkipped, 1, srcInfo.Size())
		if opts.verbose {
			rcLogFileLine(out, cls, srcInfo, name, src, opts)
		}
		return
	}

	rcLogFileLine(out, cls, srcInfo, name, src, opts)

	if opts.listOnly {
		stats.addFilesAndBytes(rcCopied, 1, srcInfo.Size())
		return
	}

	// Perform the copy, with retries.
	var copyErr error
	for attempt := 0; attempt <= opts.retries; attempt++ {
		if attempt > 0 && opts.retryWait > 0 {
			time.Sleep(time.Duration(opts.retryWait) * time.Second)
		}
		copyErr = rcDoCopy(src, dst, srcInfo, opts)
		if copyErr == nil {
			break
		}
	}

	if copyErr != nil {
		stats.addFilesAndBytes(rcFailed, 1, srcInfo.Size())
		fmt.Fprintf(out, "\tERROR copying %s: %v\n", name, copyErr)
		return
	}

	// Apply /A+: /A-: attribute modifications.
	if opts.attrAdd != "" || opts.attrDel != "" {
		rcApplyAttrs(dst, opts.attrAdd, opts.attrDel)
	}

	stats.addFilesAndBytes(rcCopied, 1, srcInfo.Size())

	if opts.moveFiles {
		os.Remove(src)
	}
}

func rcDoCopy(src, dst string, srcInfo os.FileInfo, opts *rcOpts) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if opts.createOnly {
		f, err := os.Create(dst)
		if err != nil {
			return err
		}
		return f.Close()
	}
	if srcInfo.Mode()&os.ModeSymlink != 0 && opts.copySymlinks {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		os.Remove(dst)
		return os.Symlink(target, dst)
	}
	return copyFile(src, dst)
}

// ── purge extras ──────────────────────────────────────────────────────────────

// rcPurgeExtras deletes files and directories in dst that have no counterpart
// in src (the PURGE / MIR behaviour).
func rcPurgeExtras(out io.Writer, src, dst string, opts *rcOpts, stats *rcStats) {
	entries, err := os.ReadDir(dst)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		srcPath := filepath.Join(src, name)
		dstPath := filepath.Join(dst, name)

		if _, err := os.Lstat(srcPath); os.IsNotExist(err) {
			if opts.excludeExtra {
				continue
			}
			if e.IsDir() {
				stats.addDirs(rcExtras, 1)
				if !opts.noDirList {
					fmt.Fprintf(out, "\t*EXTRA Dir \t\t%s%c\n", dstPath, os.PathSeparator)
				}
				os.RemoveAll(dstPath)
			} else {
				info, _ := os.Lstat(dstPath)
				sz := int64(0)
				if info != nil {
					sz = info.Size()
				}
				stats.addFilesAndBytes(rcExtras, 1, sz)
				if !opts.noFileList {
					fmt.Fprintf(out, "\t*EXTRA File\t%10d  %s\n", sz, name)
				}
				os.Remove(dstPath)
			}
		} else if e.IsDir() {
			rcPurgeExtras(out, srcPath, dstPath, opts, stats)
		}
	}
}

// ── classification & filtering ────────────────────────────────────────────────

func rcClassify(src, dst os.FileInfo, fatTimes bool) rcFileClass {
	threshold := time.Duration(0)
	if fatTimes {
		threshold = 2 * time.Second
	}
	diff := src.ModTime().Sub(dst.ModTime())
	if diff > threshold {
		return rcNewer
	}
	if diff < -threshold {
		return rcOlder
	}
	// Timestamps are within granularity.
	if src.Size() != dst.Size() {
		return rcChanged
	}
	return rcSame
}

func rcShouldCopy(cls rcFileClass, opts *rcOpts) bool {
	switch cls {
	case rcNewFile:
		// /XL: exclude "lonely" (source-only) files.
		return !opts.excludeLonely
	case rcSame:
		// Default: skip identical files; /IS overrides.
		return opts.includeSame
	case rcOlder:
		return !opts.excludeOlder
	case rcNewer:
		return !opts.excludeNewer
	case rcChanged:
		return !opts.excludeChanged
	}
	return true
}

// ── output helpers ────────────────────────────────────────────────────────────

func rcLogFileLine(out io.Writer, cls rcFileClass, info os.FileInfo, name, srcPath string, opts *rcOpts) {
	if opts.noFileList {
		return
	}
	var sb strings.Builder
	sb.WriteString("\t")
	if !opts.noClass {
		sb.WriteString(fmt.Sprintf("%-10s  ", cls.label()))
	}
	if opts.showTS {
		sb.WriteString(fmt.Sprintf("%-20s  ", info.ModTime().Format("2006-01-02 15:04:05")))
	}
	if !opts.noSize {
		sb.WriteString(fmt.Sprintf("%10d  ", info.Size()))
	}
	if opts.fullPath {
		abs, _ := filepath.Abs(srcPath)
		sb.WriteString(abs)
	} else {
		sb.WriteString(name)
	}
	fmt.Fprintln(out, sb.String())
}

func rcSetupOutput(p *processor.Processor, opts *rcOpts) (io.Writer, func()) {
	if opts.logPath == "" {
		return p.Stdout, nil
	}
	flag := os.O_CREATE | os.O_WRONLY
	if opts.logAppend {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}
	lf, err := os.OpenFile(pathutil.MapPath(opts.logPath), flag, 0644)
	if err != nil {
		return p.Stdout, nil
	}
	if opts.tee {
		return io.MultiWriter(lf, p.Stdout), func() { lf.Close() }
	}
	return lf, func() { lf.Close() }
}

func rcPrintHeader(out io.Writer, src, dst string, opts *rcOpts, started time.Time) {
	line := strings.Repeat("-", 79)
	fmt.Fprintln(out, line)
	fmt.Fprintln(out, "   ROBOCOPY     ::     Robust File Copy")
	fmt.Fprintln(out, line)
	fmt.Fprintf(out, "\n  Started : %s\n", started.Format("Mon Jan 02 15:04:05 2006"))
	fmt.Fprintf(out, "   Source : %s%c\n", src, os.PathSeparator)
	fmt.Fprintf(out, "     Dest : %s%c\n", dst, os.PathSeparator)
	fmt.Fprintf(out, "\n    Files : %s\n", strings.Join(opts.filePatterns, " "))
	fmt.Fprintf(out, "  Options : %s\n", rcBuildOptionsString(opts))
	fmt.Fprintln(out, "\n"+line)
}

func rcBuildOptionsString(opts *rcOpts) string {
	var parts []string
	if opts.mirror {
		parts = append(parts, "/MIR")
	} else {
		if opts.recursive && opts.includeEmpty {
			parts = append(parts, "/E")
		} else if opts.recursive {
			parts = append(parts, "/S")
		}
		if opts.purge {
			parts = append(parts, "/PURGE")
		}
	}
	if opts.listOnly {
		parts = append(parts, "/L")
	}
	if opts.moveDirs {
		parts = append(parts, "/MOVE")
	} else if opts.moveFiles {
		parts = append(parts, "/MOV")
	}
	if opts.excludeOlder {
		parts = append(parts, "/XO")
	}
	if opts.maxLevel > 0 {
		parts = append(parts, fmt.Sprintf("/LEV:%d", opts.maxLevel))
	}
	if opts.threads > 1 {
		parts = append(parts, fmt.Sprintf("/MT:%d", opts.threads))
	}
	parts = append(parts, fmt.Sprintf("/R:%d", opts.retries))
	parts = append(parts, fmt.Sprintf("/W:%d", opts.retryWait))
	return strings.Join(parts, " ")
}

func rcPrintSummary(out io.Writer, started time.Time, stats *rcStats) {
	elapsed := time.Since(started)
	line := strings.Repeat("-", 79)

	fmt.Fprintln(out, "\n"+line)
	fmt.Fprintf(out, "%20s %8s %8s %8s %8s %8s %8s\n",
		"", "Total", "Copied", "Skipped", "Mismatch", "FAILED", "Extras")
	fmt.Fprintf(out, "%20s %8d %8d %8d %8d %8d %8d\n",
		"Dirs :",
		stats.dirs[rcTotal], stats.dirs[rcCopied], stats.dirs[rcSkipped],
		stats.dirs[rcMismatch], stats.dirs[rcFailed], stats.dirs[rcExtras])
	fmt.Fprintf(out, "%20s %8d %8d %8d %8d %8d %8d\n",
		"Files :",
		stats.files[rcTotal], stats.files[rcCopied], stats.files[rcSkipped],
		stats.files[rcMismatch], stats.files[rcFailed], stats.files[rcExtras])
	fmt.Fprintf(out, "%20s %8d %8d %8d %8d %8d %8d\n",
		"Bytes :",
		stats.bytes[rcTotal], stats.bytes[rcCopied], stats.bytes[rcSkipped],
		stats.bytes[rcMismatch], stats.bytes[rcFailed], stats.bytes[rcExtras])

	if elapsed > 0 && stats.bytes[rcCopied] > 0 {
		bps := float64(stats.bytes[rcCopied]) / elapsed.Seconds()
		fmt.Fprintf(out, "\n   Speed : %20.0f Bytes/Sec.\n", bps)
		fmt.Fprintf(out, "   Speed : %20.3f MegaBytes/min.\n", bps*60/1024/1024)
	}
	fmt.Fprintf(out, "\n   Ended : %s\n", time.Now().Format("Mon Jan 02 15:04:05 2006"))
	fmt.Fprintln(out, line)
}

// ── argument parser ───────────────────────────────────────────────────────────

func rcParseOpts(args []string) *rcOpts {
	opts := &rcOpts{
		retries:      0,
		retryWait:    30,
		filePatterns: []string{"*.*"},
	}

	var extraPatterns []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)

		// /XF and /XD consume subsequent non-flag arguments.
		if lower == "/xf" {
			for i+1 < len(args) && !strings.HasPrefix(args[i+1], "/") {
				i++
				opts.excludeFiles = append(opts.excludeFiles, args[i])
			}
			continue
		}
		if lower == "/xd" {
			for i+1 < len(args) && !strings.HasPrefix(args[i+1], "/") {
				i++
				opts.excludeDirs = append(opts.excludeDirs, args[i])
			}
			continue
		}

		if !strings.HasPrefix(arg, "/") {
			extraPatterns = append(extraPatterns, arg)
			continue
		}

		switch {
		case lower == "/s":
			opts.recursive = true
		case lower == "/e":
			opts.recursive = true
			opts.includeEmpty = true
		case lower == "/a":
			opts.archiveOnly = true // stub
		case lower == "/m":
			opts.archiveClear = true // stub
		case strings.HasPrefix(lower, "/lev:"):
			if n, err := strconv.Atoi(arg[5:]); err == nil {
				opts.maxLevel = n
			}
		case strings.HasPrefix(lower, "/maxage:"):
			rcParseAge(arg[8:], &opts.maxAgeDays, &opts.maxAgeDate)
		case strings.HasPrefix(lower, "/minage:"):
			rcParseAge(arg[8:], &opts.minAgeDays, &opts.minAgeDate)
		case lower == "/fft":
			opts.fatTimes = true
		case lower == "/l":
			opts.listOnly = true
		case lower == "/mov":
			opts.moveFiles = true
		case lower == "/move":
			opts.moveFiles = true
			opts.moveDirs = true
		case lower == "/sj":
			// copy junctions as junctions — stub, junctions treated as dirs
		case lower == "/sl":
			opts.copySymlinks = true
		case lower == "/create":
			opts.createOnly = true
		case strings.HasPrefix(lower, "/r:"):
			if n, err := strconv.Atoi(arg[3:]); err == nil {
				opts.retries = n
			}
		case strings.HasPrefix(lower, "/w:"):
			if n, err := strconv.Atoi(arg[3:]); err == nil {
				opts.retryWait = n
			}
		case lower == "/mt":
			opts.threads = 8 // default thread count, matching real robocopy
		case strings.HasPrefix(lower, "/mt:"):
			if n, err := strconv.Atoi(arg[4:]); err == nil && n >= 1 && n <= 128 {
				opts.threads = n
			} else {
				opts.threads = 8
			}
		case lower == "/purge":
			opts.purge = true
		case lower == "/mir":
			opts.mirror = true
		case strings.HasPrefix(lower, "/a+:"):
			opts.attrAdd = strings.ToUpper(arg[4:])
		case strings.HasPrefix(lower, "/a-:"):
			opts.attrDel = strings.ToUpper(arg[4:])
		case lower == "/xo":
			opts.excludeOlder = true
		case lower == "/xc":
			opts.excludeChanged = true
		case lower == "/xn":
			opts.excludeNewer = true
		case lower == "/xl":
			opts.excludeLonely = true
		case lower == "/xx":
			opts.excludeExtra = true
		case lower == "/is":
			opts.includeSame = true
		case lower == "/it":
			opts.includeSame = true // treat as /IS
		case lower == "/xj":
			opts.excludeJunct = true
		case lower == "/xjd":
			opts.excludeJunctD = true
		case lower == "/xjf":
			opts.excludeJunctF = true
		case strings.HasPrefix(lower, "/max:"):
			if n, err := strconv.ParseInt(arg[5:], 10, 64); err == nil {
				opts.maxSize = n
			}
		case strings.HasPrefix(lower, "/min:"):
			if n, err := strconv.ParseInt(arg[5:], 10, 64); err == nil {
				opts.minSize = n
			}
		case strings.HasPrefix(lower, "/log+:"):
			opts.logPath = pathutil.MapPath(arg[6:])
			opts.logAppend = true
		case strings.HasPrefix(lower, "/log:"):
			opts.logPath = pathutil.MapPath(arg[5:])
		case lower == "/tee":
			opts.tee = true
		case lower == "/np":
			// no progress — we never print progress anyway
		case lower == "/ts":
			opts.showTS = true
		case lower == "/fp":
			opts.fullPath = true
		case lower == "/ns":
			opts.noSize = true
		case lower == "/nc":
			opts.noClass = true
		case lower == "/nfl":
			opts.noFileList = true
		case lower == "/ndl":
			opts.noDirList = true
		case lower == "/njh":
			opts.noHeader = true
		case lower == "/njs":
			opts.noSummary = true
		case lower == "/v":
			opts.verbose = true
		case lower == "/x":
			opts.reportExtra = true
		// All remaining flags: accepted, not functionally implemented.
		case lower == "/b", lower == "/efsraw", lower == "/compress",
			lower == "/j", lower == "/nooffload", lower == "/reg",
			lower == "/tbd", lower == "/sec", lower == "/secfix",
			lower == "/dst", lower == "/fat", lower == "/copyall",
			lower == "/nocopy", lower == "/nodcopy", lower == "/im",
			lower == "/pf", lower == "/256", lower == "/unicode",
			lower == "/bytes", lower == "/debug", lower == "/eta",
			lower == "/timfix", lower == "/z", lower == "/zb",
			lower == "/nosd", lower == "/nodd", lower == "/quit", lower == "/if":
			// stub
		default:
			if strings.HasPrefix(lower, "/copy:") || strings.HasPrefix(lower, "/dcopy:") ||
				strings.HasPrefix(lower, "/ia:") || strings.HasPrefix(lower, "/xa:") ||
				strings.HasPrefix(lower, "/ipg:") ||
				strings.HasPrefix(lower, "/mon:") || strings.HasPrefix(lower, "/mot:") ||
				strings.HasPrefix(lower, "/rh:") || strings.HasPrefix(lower, "/job:") ||
				strings.HasPrefix(lower, "/save:") || strings.HasPrefix(lower, "/lfsm") ||
				strings.HasPrefix(lower, "/maxlad:") || strings.HasPrefix(lower, "/minlad:") ||
				strings.HasPrefix(lower, "/sd:") || strings.HasPrefix(lower, "/dd:") ||
				strings.HasPrefix(lower, "/unilog") {
				// stub
			}
			// unknown /flag: silently ignored
		}
	}

	if len(extraPatterns) > 0 {
		opts.filePatterns = extraPatterns
	}
	return opts
}

func rcParseAge(s string, days *int, date **time.Time) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return
	}
	if n < 1900 {
		*days = n
	} else {
		t, err := time.Parse("20060102", s)
		if err == nil {
			*date = &t
		}
	}
}

// ── misc helpers ──────────────────────────────────────────────────────────────

// rcMatchesFilePattern reports whether name matches any of the given
// glob patterns (case-insensitive).
func rcMatchesFilePattern(name string, patterns []string) bool {
	for _, pat := range patterns {
		if matched, _ := filepath.Match(strings.ToLower(pat), strings.ToLower(name)); matched {
			return true
		}
	}
	return false
}

// rcFileExcluded reports whether name (or its path) matches any /XF pattern.
func rcFileExcluded(name, path string, patterns []string) bool {
	nameLower := strings.ToLower(name)
	pathLower := strings.ToLower(path)
	for _, pat := range patterns {
		patLower := strings.ToLower(pat)
		if matched, _ := filepath.Match(patLower, nameLower); matched {
			return true
		}
		// Also allow full-path substring matching for path-style patterns.
		if strings.Contains("/", pat) || strings.Contains("\\", pat) {
			if strings.Contains(pathLower, patLower) {
				return true
			}
		}
	}
	return false
}

// rcDirExcluded reports whether name (or its path) matches any /XD pattern.
func rcDirExcluded(name, path string, patterns []string) bool {
	nameLower := strings.ToLower(name)
	pathLower := strings.ToLower(path)
	for _, pat := range patterns {
		patLower := strings.ToLower(pat)
		if matched, _ := filepath.Match(patLower, nameLower); matched {
			return true
		}
		// Only do path-substring matching when the pattern itself contains a
		// separator, to avoid false positives from patterns like "excluded"
		// matching a path component named "included".
		if strings.ContainsAny(pat, "/\\") && strings.Contains(pathLower, patLower) {
			return true
		}
	}
	return false
}

// rcApplyAttrs applies /A+: and /A-: attribute changes to dst.
// On Linux only R (read-only) is supported; other attribute letters are stubs.
func rcApplyAttrs(path, add, del string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	mode := info.Mode()
	if strings.ContainsAny(add, "Rr") {
		mode &^= 0222 // clear write bits → read-only
	}
	if strings.ContainsAny(del, "Rr") {
		mode |= 0200 // set owner-write bit → writable
	}
	os.Chmod(path, mode)
}

// isSymlinkToDir returns true if path is a symlink that resolves to a directory.
func isSymlinkToDir(path string) bool {
	target, err := os.Stat(path) // follows symlink
	return err == nil && target.IsDir()
}
