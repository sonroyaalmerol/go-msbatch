package tools

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sonroyaalmerol/go-msbatch/pkg/pathutil"
)

func isDirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

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

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	out.Close()

	return os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
}

func CopyFile(src, dst string) error {
	return copyFile(src, dst)
}

func HasWildcards(pattern string) bool {
	return strings.ContainsAny(pattern, "*?")
}

func GlobOrLiteral(pattern string) []string {
	matches, err := pathutil.GlobCaseInsensitive(pattern)
	if err != nil || len(matches) == 0 {
		return []string{pattern}
	}
	return matches
}

func ResolveWildcardDst(srcPath, srcPattern, dstPattern, dst string) string {
	srcBase := filepath.Base(srcPath)
	srcPatBase := patternBase(srcPattern)
	dstPatBase := patternBase(dstPattern)
	newDstBase := SubstituteWildcard(srcBase, srcPatBase, dstPatBase)
	if HasWildcards(dst) {
		return filepath.Join(filepath.Dir(dst), newDstBase)
	}
	return filepath.Join(dst, newDstBase)
}

// patternBase extracts the base (filename) part of a pattern that may contain
// either Unix-style (/) or Windows-style (\) path separators. It normalizes
// the separators first to ensure filepath.Base works correctly on all platforms.
func patternBase(pattern string) string {
	normalized := strings.ReplaceAll(pattern, "\\", "/")
	return filepath.Base(normalized)
}

type WildcardPos struct {
	Index      int
	IsStar     bool
	IsQuestion bool
}

func FindWildcards(pattern string) []WildcardPos {
	var positions []WildcardPos
	for i, c := range pattern {
		if c == '*' {
			positions = append(positions, WildcardPos{Index: i, IsStar: true})
		} else if c == '?' {
			positions = append(positions, WildcardPos{Index: i, IsQuestion: true})
		}
	}
	return positions
}

func ExtractMatches(name, pattern string, wildcards []WildcardPos) []string {
	var matches []string
	nameIdx := 0
	patIdx := 0

	for _, wc := range wildcards {
		for patIdx < wc.Index && nameIdx < len(name) {
			if pattern[patIdx] == name[nameIdx] || pattern[patIdx] == '?' {
				patIdx++
				nameIdx++
			} else {
				patIdx++
			}
		}

		patIdx = wc.Index + 1

		if wc.IsStar {
			nextFixed := ""
			if patIdx < len(pattern) {
				end := strings.IndexAny(pattern[patIdx:], "*?")
				if end >= 0 {
					nextFixed = pattern[patIdx : patIdx+end]
				} else {
					nextFixed = pattern[patIdx:]
				}
			}

			if nextFixed == "" {
				matches = append(matches, name[nameIdx:])
				nameIdx = len(name)
			} else {
				endIdx := strings.Index(name[nameIdx:], nextFixed)
				if endIdx >= 0 {
					matches = append(matches, name[nameIdx:nameIdx+endIdx])
					nameIdx += endIdx
				} else {
					matches = append(matches, name[nameIdx:])
					nameIdx = len(name)
				}
			}
		} else if wc.IsQuestion {
			if nameIdx < len(name) {
				matches = append(matches, string(name[nameIdx]))
				nameIdx++
			}
		}
	}

	return matches
}

func SubstituteWildcard(srcName, srcPattern, dstPattern string) string {
	srcWildcards := FindWildcards(srcPattern)
	dstWildcards := FindWildcards(dstPattern)

	if len(srcWildcards) == 0 || len(dstWildcards) == 0 {
		return dstPattern
	}

	matchedParts := ExtractMatches(srcName, srcPattern, srcWildcards)

	result := dstPattern
	for i, wc := range dstWildcards {
		if i < len(matchedParts) {
			part := matchedParts[i]
			if wc.IsStar {
				result = strings.Replace(result, "*", part, 1)
			} else {
				result = strings.Replace(result, "?", part, 1)
			}
		}
	}
	return result
}
