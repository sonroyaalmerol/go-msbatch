package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sonroyaalmerol/go-msbatch/pkg/executor"
	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

func TestEchoRedirectThenCopyWildcardDebug(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo line1 >> test.sum
`

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	t.Logf("Files in directory after echo redirect:")
	for _, e := range entries {
		info, _ := e.Info()
		t.Logf("  %s (%d bytes)", e.Name(), info.Size())
	}

	if _, err := os.Stat("test.sum"); err != nil {
		t.Fatalf("test.sum was not created: %v", err)
	}
}

func TestEchoRedirectWithCopy(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	destDir := "dest"
	if err := os.Mkdir(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	batContent := `@echo off
echo line1 >> test.sum
echo line2 >> test.sum
copy *.sum dest\
`

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	out := stdout.String()
	errOut := stderr.String()
	t.Logf("stdout: %s", out)
	t.Logf("stderr: %s", errOut)

	if !strings.Contains(out, "1 file(s) copied") {
		t.Errorf("Expected '1 file(s) copied', got stdout: %q, stderr: %q", out, errOut)
	}

	if _, err := os.Stat(filepath.Join(destDir, "test.sum")); err != nil {
		t.Errorf("test.sum was not copied to dest: %v", err)
	}
}

func TestEchoRedirectWithCopyPlus(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo line1 >> file1.sum
echo line2 >> file2.sum
copy file1.sum + file2.sum combined.sum
`

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	out := stdout.String()
	errOut := stderr.String()
	t.Logf("stdout: %s", out)
	t.Logf("stderr: %s", errOut)

	if !strings.Contains(out, "1 file(s) copied") {
		t.Errorf("Expected '1 file(s) copied', got stdout: %q, stderr: %q", out, errOut)
	}

	content, err := os.ReadFile("combined.sum")
	if err != nil {
		t.Errorf("combined.sum was not created: %v", err)
	}
	if !strings.Contains(string(content), "line1") || !strings.Contains(string(content), "line2") {
		t.Errorf("combined.sum does not contain expected content: %q", string(content))
	}
}

func TestEchoRedirectMultipleWithCopyWildcard(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	destDir := "output"
	if err := os.Mkdir(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	batContent := `@echo off
echo test1 >> GRV1_TA-Flight-1026.sum
echo test2 >> GRV2_TA-Flight-1027.sum
echo test3 >> GRV3_TA-Flight-1028.sum
copy *.sum output\
`

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	out := stdout.String()
	errOut := stderr.String()
	t.Logf("stdout: %s", out)
	t.Logf("stderr: %s", errOut)

	if !strings.Contains(out, "3 file(s) copied") {
		t.Errorf("Expected '3 file(s) copied', got stdout: %q, stderr: %q", out, errOut)
	}

	for _, f := range []string{"GRV1_TA-Flight-1026.sum", "GRV2_TA-Flight-1027.sum", "GRV3_TA-Flight-1028.sum"} {
		if _, err := os.Stat(filepath.Join(destDir, f)); err != nil {
			t.Errorf("%s was not copied to output/: %v", f, err)
		}
	}
}

func TestEchoRedirectWithCopyPlusMissingDestination(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo line1 >> test.sum
set OUTPUTFILE=test.sum
echo line2 >> temp_vars_output.txt
copy %OUTPUTFILE% + temp_vars_output.txt
`

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	out := stdout.String()
	errOut := stderr.String()
	t.Logf("stdout: %s", out)
	t.Logf("stderr: %s", errOut)

	if !strings.Contains(out, "1 file(s) copied") {
		t.Errorf("Expected '1 file(s) copied', got stdout: %q, stderr: %q", out, errOut)
	}
}

func TestCopyWildcardGlobFindsFile(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
echo content >> GRV1_TA-Flight-1026.sum
dir *.sum /b
copy *.sum dest\
`

	destDir := "dest"
	if err := os.Mkdir(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	out := stdout.String()
	errOut := stderr.String()
	t.Logf("stdout: %s", out)
	t.Logf("stderr: %s", errOut)

	if !strings.Contains(out, "GRV1_TA-Flight-1026.sum") {
		t.Errorf("dir output should contain the .sum file, got: %q", out)
	}

	if !strings.Contains(out, "1 file(s) copied") {
		t.Errorf("Expected '1 file(s) copied', got stdout: %q, stderr: %q", out, errOut)
	}

	if _, err := os.Stat(filepath.Join(destDir, "GRV1_TA-Flight-1026.sum")); err != nil {
		t.Errorf("file was not copied to dest: %v", err)
	}
}

func TestCopyPlusConcatenationFindsCreatedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	batContent := `@echo off
set OUTPUTFILE=test.sum
echo first >> %OUTPUTFILE%
echo second >> temp.txt
copy %OUTPUTFILE% + temp.txt combined.txt
`

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	out := stdout.String()
	errOut := stderr.String()
	t.Logf("stdout: %s", out)
	t.Logf("stderr: %s", errOut)

	if !strings.Contains(out, "1 file(s) copied") {
		t.Errorf("Expected '1 file(s) copied', got stdout: %q, stderr: %q", out, errOut)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "combined.txt"))
	if err != nil {
		t.Fatalf("combined.txt was not created: %v", err)
	}
	if !strings.Contains(string(content), "first") || !strings.Contains(string(content), "second") {
		t.Errorf("combined.txt does not contain expected content: %q", string(content))
	}
}

func TestEchoRedirectWithCDAndCopy(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	subDir := "subdir"
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	destDir := "procfiles"
	if err := os.Mkdir(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	batContent := `@echo off
cd subdir
echo line1 >> test.sum
cd ..
copy subdir\*.sum procfiles\
`

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	out := stdout.String()
	errOut := stderr.String()
	t.Logf("stdout: %s", out)
	t.Logf("stderr: %s", errOut)

	if !strings.Contains(out, "1 file(s) copied") {
		t.Errorf("Expected '1 file(s) copied', got stdout: %q, stderr: %q", out, errOut)
	}

	if _, err := os.Stat(filepath.Join(destDir, "test.sum")); err != nil {
		t.Errorf("test.sum was not copied to procfiles: %v", err)
	}
}

func TestEchoRedirectWithSETLOCALAndCD(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	subDir := "subdir"
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	destDir := "procfiles"
	if err := os.Mkdir(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	batContent := `@echo off
setlocal
cd subdir
echo line1 >> test.sum
endlocal
copy *.sum ..\procfiles\
`

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	out := stdout.String()
	errOut := stderr.String()
	t.Logf("stdout: %s", out)
	t.Logf("stderr: %s", errOut)

	t.Logf("Files in %s:", tmpDir)
	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		t.Logf("  %s", e.Name())
	}
	t.Logf("Files in %s:", filepath.Join(tmpDir, subDir))
	entries2, _ := os.ReadDir(filepath.Join(tmpDir, subDir))
	for _, e := range entries2 {
		t.Logf("  %s", e.Name())
	}
	t.Logf("Files in %s:", filepath.Join(tmpDir, destDir))
	entries3, _ := os.ReadDir(filepath.Join(tmpDir, destDir))
	for _, e := range entries3 {
		t.Logf("  %s", e.Name())
	}

	if !strings.Contains(out, "1 file(s) copied") {
		t.Errorf("ENDLOCAL should NOT restore working directory (Windows CMD behavior) - got stdout: %q, stderr: %q", out, errOut)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, destDir, "test.sum")); err != nil {
		t.Errorf("test.sum should be copied to procfiles: %v", err)
	}
}

func TestEchoRedirectWithSETLOCALPreservesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	subDir := "subdir"
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	batContent := `@echo off
setlocal
cd subdir
echo line1 >> test.sum
endlocal
cd subdir
copy *.sum ..\
`

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	out := stdout.String()
	errOut := stderr.String()
	t.Logf("stdout: %s", out)
	t.Logf("stderr: %s", errOut)

	if !strings.Contains(out, "1 file(s) copied") {
		t.Errorf("Expected '1 file(s) copied', got stdout: %q, stderr: %q", out, errOut)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "test.sum")); err != nil {
		t.Errorf("test.sum was not copied to parent directory: %v", err)
	}
}

func TestENDLOCALRestoresVarsNotDir(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	subDir := "subdir"
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	batContent := `@echo off
set OUTER_VAR=outer_value
setlocal
set OUTER_VAR=inner_value
set INNER_VAR=inner_only
cd subdir
echo %OUTER_VAR% %INNER_VAR% >> test.txt
endlocal
echo After ENDLOCAL: OUTER_VAR=%OUTER_VAR% INNER_VAR=%INNER_VAR% >> result.txt
`

	env := processor.NewEnvironment(true)
	var stdout, stderr bytes.Buffer
	proc := processor.New(env, []string{"test.bat"}, executor.New())
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	proc.Echo = false

	src := processor.Phase0ReadLine(batContent)
	nodes := processor.ParseExpanded(src)

	err = proc.Execute(nodes)
	if err != nil {
		t.Errorf("Execute failed: %v", err)
	}

	testContent, err := os.ReadFile(filepath.Join(tmpDir, subDir, "test.txt"))
	if err != nil {
		t.Fatalf("test.txt not found in subdir: %v", err)
	}
	if !strings.Contains(string(testContent), "inner_value") {
		t.Errorf("test.txt should contain 'inner_value', got: %q", string(testContent))
	}

	resultContent, err := os.ReadFile(filepath.Join(tmpDir, subDir, "result.txt"))
	if err != nil {
		t.Fatalf("result.txt not found - working directory should still be subdir: %v", err)
	}
	resultStr := string(resultContent)
	t.Logf("result.txt content: %s", resultStr)

	if !strings.Contains(resultStr, "OUTER_VAR=outer_value") {
		t.Errorf("OUTER_VAR should be restored to 'outer_value', got: %q", resultStr)
	}
	if strings.Contains(resultStr, "INNER_VAR=inner_only") {
		t.Errorf("INNER_VAR should not exist after ENDLOCAL, got: %q", resultStr)
	}
}
