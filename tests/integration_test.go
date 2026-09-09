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

func TestIntegration(t *testing.T) {
	files, err := filepath.Glob("*.bat")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no .bat fixtures found")
	}

	fixtures, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	for _, batFile := range files {
		t.Run(batFile, func(t *testing.T) {
			content, err := os.ReadFile(batFile)
			if err != nil {
				t.Fatal(err)
			}

			stem := strings.TrimSuffix(batFile, ".bat")
			expectedFile := stem + ".out"
			expected, err := os.ReadFile(expectedFile)
			containsMode := false
			if err != nil {
				expectedFile = stem + ".contains"
				expected, err = os.ReadFile(expectedFile)
				containsMode = true
			}
			if err != nil {
				t.Errorf("no golden %s (.out or .contains) for fixture %s", expectedFile, batFile)
				return
			}

			workDir := t.TempDir()
			for _, e := range fixtures {
				data, err := os.ReadFile(e.Name())
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(workDir, e.Name()), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(workDir)

			if batFile == "29_exe_mapping_path.bat" {
				t.Setenv("MSBATCH_DRIVE_C", "/usr/")
			}
			env := processor.NewEnvironment(true)
			var stdout, stderr bytes.Buffer
			proc := processor.New(env, []string{batFile, "A", "B", "C"}, executor.New())
			proc.Stdout = &stdout
			proc.Stderr = &stderr
			proc.Echo = false

			src := string(content)
			src = processor.Phase0ReadLine(src)
			nodes := processor.ParseExpanded(src)

			err = proc.Execute(nodes)
			if err != nil {
				t.Errorf("Execute failed: %v", err)
			}

			got := normalize(stdout.String())

			if containsMode {
				for marker := range strings.SplitSeq(normalize(string(expected)), "\n") {
					if marker != "" && !strings.Contains(got, marker) {
						t.Errorf("output for %s missing marker %q", batFile, marker)
					}
				}
				return
			}

			want := normalize(string(expected))

			if got != want {
				t.Errorf("Output mismatch for %s\nGOT:\n%s\nWANT:\n%s", batFile, got, want)
			}

			if errExpected, err := os.ReadFile(filepath.Join("..", expectedFile[:len(expectedFile)-4]+".err")); err == nil {
				if gotErr := normalize(stderr.String()); gotErr != normalize(string(errExpected)) {
					t.Errorf("Stderr mismatch for %s\nGOT:\n%s\nWANT:\n%s", batFile, gotErr, normalize(string(errExpected)))
				}
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

// toolCase covers built-in tool interactions: gawk program quoting and
// redirects, case-insensitive wildcards for del/copy/if-exist, and argument
// quoting for external programs. %TESTDIR% in a script is replaced with the
// case's private directory, reproducing the old absolute-path cases.
type toolCase struct {
	name              string
	needsGawk         bool
	setupDirs         []string
	setupFiles        map[string]string
	script            string
	wantStdout        string
	wantFiles         map[string]string
	wantFilesContains map[string]string
	wantFileMinLen    map[string]int
	wantRemaining     []string
}

func TestToolInteractions(t *testing.T) {
	_, gawkErr := os.Stat("/usr/bin/gawk")

	cases := []toolCase{
		{
			name:       "gawk_absolute_path_wrong_case",
			needsGawk:  true,
			setupFiles: map[string]string{"DataFolder/InputFile.txt": "line1\nline2\nline3\n"},
			script:     "@echo off\ngawk \"{print}\" %TESTDIR%/datafolder/inputfile.txt\n",
			wantStdout: "line1\nline2\nline3\n",
		},
		{
			name:       "gawk_relative_path_wrong_case",
			needsGawk:  true,
			setupFiles: map[string]string{"MyData/Records.txt": "apple\nbanana\ncherry\n"},
			script:     "@echo off\ngawk \"{print}\" ./mydata/records.txt\n",
			wantStdout: "apple\nbanana\ncherry\n",
		},
		{
			name:       "gawk_after_cd_wrong_case",
			needsGawk:  true,
			setupFiles: map[string]string{"OuterDir/DataFolder/InputFile.txt": "line1\nline2\nline3\n"},
			script:     "@echo off\ncd %TESTDIR%/OuterDir\ngawk \"{print}\" datafolder/inputfile.txt\n",
			wantStdout: "line1\nline2\nline3\n",
		},
		{
			name:       "gawk_after_pushd_wrong_case",
			needsGawk:  true,
			setupFiles: map[string]string{"ProjectRoot/SourceData/Records.txt": "alpha\nbeta\ngamma\n"},
			script:     "@echo off\npushd %TESTDIR%/ProjectRoot\ngawk \"{print}\" sourcedata/records.txt\npopd\n",
			wantStdout: "alpha\nbeta\ngamma\n",
		},
		{
			name:       "gawk_after_nested_cd_wrong_case",
			needsGawk:  true,
			setupFiles: map[string]string{"LevelOne/LevelTwo/FinalData/File.txt": "nested content\n"},
			script:     "@echo off\ncd %TESTDIR%/LevelOne\ncd leveltwo\ngawk \"{print}\" finaldata/file.txt\n",
			wantStdout: "nested content\n",
		},
		{
			name: "del_wildcard_case_insensitive",
			setupFiles: map[string]string{
				"File1.txt": "content",
				"File2.TXT": "content",
				"File3.log": "content",
				"Other.txt": "content",
			},
			script:        "@echo off\ndel *.txt\n",
			wantRemaining: []string{"File3.log"},
		},
		{
			name:      "copy_wildcard_case_insensitive",
			setupDirs: []string{"DestDir"},
			setupFiles: map[string]string{
				"SourceDir/File1.txt": "content",
				"SourceDir/File2.TXT": "content",
			},
			script:     "@echo off\ncopy sourcedir\\*.txt destdir\\\n",
			wantStdout: "File1.txt\nFile2.TXT\n        2 file(s) copied.\n",
			wantFiles: map[string]string{
				"DestDir/File1.txt": "content",
				"DestDir/File2.TXT": "content",
			},
		},
		{
			name:       "if_exist_wildcard_case_insensitive",
			setupFiles: map[string]string{"DataFolder/gpsfile1.lst": "content", "DataFolder/gpsfile2.lst": "content"},
			script:     "@echo off\nif exist DATAFOLDER\\gps*.lst (\n    echo found\n) else (\n    echo not found\n)\n",
			wantStdout: "found\n",
		},
		{
			name:       "if_exist_wildcard_no_match",
			setupFiles: map[string]string{"DataFolder/other1.txt": "content", "DataFolder/other2.txt": "content"},
			script:     "@echo off\nif exist DATAFOLDER\\gps*.lst (\n    echo found\n) else (\n    echo not found\n)\n",
			wantStdout: "not found\n",
		},
		{
			name:           "gawk_quoted_program_to_file",
			needsGawk:      true,
			script:         "@echo off\ngawk \"BEGIN {print systime()}\" > timetemp.txt\n",
			wantFileMinLen: map[string]int{"timetemp.txt": 10},
		},
		{
			name:      "echo_quoted_arg_to_file",
			script:    "@echo off\necho \"hello world\" > output.txt\n",
			wantFiles: map[string]string{"output.txt": "\"hello world\" \r\n"},
		},
		{
			name:       "gawk_quoted_program_to_console",
			needsGawk:  true,
			script:     "@echo off\ngawk \"BEGIN {print 12345}\"\n",
			wantStdout: "12345\n",
		},
		{
			name:              "gawk_append_with_nested_quotes",
			needsGawk:         true,
			setupFiles:        map[string]string{"timetemp.txt": "1234567890\n"},
			script:            "@echo off\ngawk \"{print systime()-$1 \\\" seconds to process GRV1_TA\\\" }  \" timetemp.txt >> time.txt\n",
			wantFilesContains: map[string]string{"time.txt": "seconds to process GRV1_TA"},
		},
		{
			name:       "embedded_quotes_preserved_for_external",
			setupFiles: map[string]string{"print_args.sh": "#!/bin/bash\nfor arg in \"$@\"; do echo \"ARG: [$arg]\"; done\n"},
			script:     "@echo off\nbash print_args.sh Instrument=\"King Radar\"\n",
			wantStdout: "ARG: [Instrument=\"King Radar\"]\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.needsGawk && gawkErr != nil {
				t.Skip("system gawk not available")
			}
			dir := isolate(t)
			for _, d := range tc.setupDirs {
				mkdir(t, d)
			}
			for name, content := range tc.setupFiles {
				writeFile(t, name, content)
			}

			got := runScript(t, strings.ReplaceAll(tc.script, "%TESTDIR%", dir))
			if got.stdout != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", got.stdout, tc.wantStdout)
			}
			for name, want := range tc.wantFiles {
				assertFile(t, name, want)
			}
			for name, want := range tc.wantFilesContains {
				assertFileContains(t, name, want)
			}
			for name, minLen := range tc.wantFileMinLen {
				assertFileMinLen(t, name, minLen)
			}
			if tc.wantRemaining != nil {
				assertDirEntries(t, ".", tc.wantRemaining)
			}
		})
	}
}
