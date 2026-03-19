package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func TestIntegration(t *testing.T) {
	files, err := filepath.Glob("*.bat")
	if err != nil {
		t.Fatal(err)
	}

	for _, batFile := range files {
		t.Run(batFile, func(t *testing.T) {
			content, err := os.ReadFile(batFile)
			if err != nil {
				t.Fatal(err)
			}

			expectedFile := strings.TrimSuffix(batFile, ".bat") + ".out"
			expected, err := os.ReadFile(expectedFile)
			if err != nil {
				t.Logf("Warning: No .out file for %s, skipping comparison", batFile)
				return
			}

			if batFile == "29_exe_mapping_path.bat" {
				t.Setenv("MSBATCH_DRIVE_C", "/usr/")
			}
			env := processor.NewEnvironment(true)
			var stdout bytes.Buffer
			proc := processor.New(env, []string{batFile, "A", "B", "C"}, executor.New())
			proc.Stdout = &stdout
			proc.Echo = false // Match @echo off behavior for comparison

			src := string(content)
			src = processor.Phase0ReadLine(src)
			nodes := processor.ParseExpanded(src)

			err = proc.Execute(nodes)
			if err != nil {
				t.Errorf("Execute failed: %v", err)
			}

			got := normalize(stdout.String())
			want := normalize(string(expected))

			if got != want {
				t.Errorf("Output mismatch for %s\nGOT:\n%s\nWANT:\n%s", batFile, got, want)
			}
		})
	}
}

func normalize(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \r\t")
		if trimmed != "" || len(result) > 0 {
			result = append(result, trimmed)
		}
	}
	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}
	return strings.Join(result, "\n")
}

func TestGawkCaseInsensitivePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping gawk test on Windows")
	}

	if _, err := os.Stat("/usr/bin/gawk"); os.IsNotExist(err) {
		t.Skip("gawk not available, skipping test")
	}

	tmpDir := t.TempDir()

	dataDir := filepath.Join(tmpDir, "DataFolder")
	if err := os.Mkdir(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	dataFile := filepath.Join(dataDir, "InputFile.txt")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(dataFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create input file: %v", err)
	}

	batContent := `@echo off
gawk "{print}" ` + filepath.Join(tmpDir, "datafolder", "inputfile.txt") + `
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err := proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	got := stdout.String()
	want := "line1\nline2\nline3\n"

	if got != want {
		t.Errorf("Gawk output mismatch\nGOT:\n%s\nWANT:\n%s", got, want)
	}
}

func TestGawkCaseInsensitiveRelativePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping gawk test on Windows")
	}

	if _, err := os.Stat("/usr/bin/gawk"); os.IsNotExist(err) {
		t.Skip("gawk not available, skipping test")
	}

	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	dataDir := filepath.Join(tmpDir, "MyData")
	if err := os.Mkdir(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	dataFile := filepath.Join(dataDir, "Records.txt")
	content := "apple\nbanana\ncherry\n"
	if err := os.WriteFile(dataFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create input file: %v", err)
	}

	batContent := `@echo off
gawk "{print}" ./mydata/records.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	got := stdout.String()
	want := "apple\nbanana\ncherry\n"

	if got != want {
		t.Errorf("Gawk output mismatch\nGOT:\n%s\nWANT:\n%s", got, want)
	}
}

func TestGawkAfterCdCaseInsensitive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping gawk test on Windows")
	}

	if _, err := os.Stat("/usr/bin/gawk"); os.IsNotExist(err) {
		t.Skip("gawk not available, skipping test")
	}

	tmpDir := t.TempDir()

	outerDir := filepath.Join(tmpDir, "OuterDir")
	dataDir := filepath.Join(outerDir, "DataFolder")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	dataFile := filepath.Join(dataDir, "InputFile.txt")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(dataFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create input file: %v", err)
	}

	batContent := `@echo off
cd ` + outerDir + `
gawk "{print}" datafolder/inputfile.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err := proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	got := stdout.String()
	want := "line1\nline2\nline3\n"

	if got != want {
		t.Errorf("Gawk output after cd mismatch\nGOT:\n%s\nWANT:\n%s", got, want)
	}
}

func TestGawkAfterPushdCaseInsensitive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping gawk test on Windows")
	}

	if _, err := os.Stat("/usr/bin/gawk"); os.IsNotExist(err) {
		t.Skip("gawk not available, skipping test")
	}

	tmpDir := t.TempDir()

	outerDir := filepath.Join(tmpDir, "ProjectRoot")
	dataDir := filepath.Join(outerDir, "SourceData")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	dataFile := filepath.Join(dataDir, "Records.txt")
	content := "alpha\nbeta\ngamma\n"
	if err := os.WriteFile(dataFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create input file: %v", err)
	}

	batContent := `@echo off
pushd ` + outerDir + `
gawk "{print}" sourcedata/records.txt
popd
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err := proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	got := stdout.String()
	want := "alpha\nbeta\ngamma\n"

	if got != want {
		t.Errorf("Gawk output after pushd mismatch\nGOT:\n%s\nWANT:\n%s", got, want)
	}
}

func TestGawkNestedCdCaseInsensitive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping gawk test on Windows")
	}

	if _, err := os.Stat("/usr/bin/gawk"); os.IsNotExist(err) {
		t.Skip("gawk not available, skipping test")
	}

	tmpDir := t.TempDir()

	level1 := filepath.Join(tmpDir, "LevelOne")
	level2 := filepath.Join(level1, "LevelTwo")
	dataDir := filepath.Join(level2, "FinalData")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create nested directories: %v", err)
	}

	dataFile := filepath.Join(dataDir, "File.txt")
	content := "nested content\n"
	if err := os.WriteFile(dataFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create input file: %v", err)
	}

	batContent := `@echo off
cd ` + level1 + `
cd leveltwo
gawk "{print}" finaldata/file.txt
`

	env := processor.NewEnvironment(true)
	var stdout bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err := proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	got := stdout.String()
	want := "nested content\n"

	if got != want {
		t.Errorf("Gawk output after nested cd mismatch\nGOT:\n%s\nWANT:\n%s", got, want)
	}
}
