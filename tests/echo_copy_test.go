package tests

import (
	"strings"
	"testing"
)

type echoCopyCase struct {
	name               string
	setupDirs          []string
	script             string
	wantStdoutContains []string
	wantFiles          map[string]string
	wantFilesExist     []string
}

func TestEchoRedirectAndCopy(t *testing.T) {
	cases := []echoCopyCase{
		{
			name:      "echo_append_creates_file",
			script:    "@echo off\necho line1 >> test.sum\n",
			wantFiles: map[string]string{"test.sum": "line1 \r\n"},
		},
		{
			name:               "copy_wildcard_into_directory",
			setupDirs:          []string{"dest"},
			script:             "@echo off\necho line1 >> test.sum\necho line2 >> test.sum\ncopy *.sum dest\\\n",
			wantStdoutContains: []string{"1 file(s) copied"},
			wantFilesExist:     []string{"dest/test.sum"},
		},
		{
			name:               "copy_plus_concatenates_in_text_mode",
			script:             "@echo off\necho line1 >> file1.sum\necho line2 >> file2.sum\ncopy file1.sum + file2.sum combined.sum\n",
			wantStdoutContains: []string{"file1.sum", "file2.sum", "1 file(s) copied"},
			wantFiles:          map[string]string{"combined.sum": "line1 \r\nline2 \r\n\x1a"},
		},
		{
			name:               "copy_plus_binary_mode_preserves_bytes",
			script:             "@echo off\necho line1 >> file1.sum\necho line2 >> file2.sum\ncopy /b file1.sum + file2.sum combined.sum\n",
			wantStdoutContains: []string{"file1.sum", "file2.sum", "1 file(s) copied"},
			wantFiles:          map[string]string{"combined.sum": "line1 \r\nline2 \r\n"},
		},
		{
			name:               "copy_wildcard_multiple_files",
			setupDirs:          []string{"output"},
			script:             "@echo off\necho test1 >> GRV1_TA-Flight-1026.sum\necho test2 >> GRV2_TA-Flight-1027.sum\necho test3 >> GRV3_TA-Flight-1028.sum\ncopy *.sum output\\\n",
			wantStdoutContains: []string{"3 file(s) copied"},
			wantFilesExist: []string{
				"output/GRV1_TA-Flight-1026.sum",
				"output/GRV2_TA-Flight-1027.sum",
				"output/GRV3_TA-Flight-1028.sum",
			},
		},
		{
			name:               "copy_plus_without_destination_writes_first_arg",
			script:             "@echo off\necho line1 >> test.sum\nset OUTPUTFILE=test.sum\necho line2 >> temp_vars_output.txt\ncopy %OUTPUTFILE% + temp_vars_output.txt\n",
			wantStdoutContains: []string{"1 file(s) copied"},
		},
		{
			name:               "copy_wildcard_after_dir_listing",
			setupDirs:          []string{"dest"},
			script:             "@echo off\necho content >> GRV1_TA-Flight-1026.sum\ndir *.sum /b\ncopy *.sum dest\\\n",
			wantStdoutContains: []string{"GRV1_TA-Flight-1026.sum", "1 file(s) copied"},
			wantFilesExist:     []string{"dest/GRV1_TA-Flight-1026.sum"},
		},
		{
			name:               "copy_plus_with_variable_source",
			script:             "@echo off\nset OUTPUTFILE=test.sum\necho first >> %OUTPUTFILE%\necho second >> temp.txt\ncopy %OUTPUTFILE% + temp.txt combined.txt\n",
			wantStdoutContains: []string{"1 file(s) copied"},
			wantFiles:          map[string]string{"combined.txt": "first \r\nsecond \r\n\x1a"},
		},
		{
			name:               "copy_after_cd_roundtrip",
			setupDirs:          []string{"subdir", "procfiles"},
			script:             "@echo off\ncd subdir\necho line1 >> test.sum\ncd ..\ncopy subdir\\*.sum procfiles\\\n",
			wantStdoutContains: []string{"1 file(s) copied"},
			wantFilesExist:     []string{"procfiles/test.sum"},
		},
		{
			name:               "endlocal_restores_working_directory",
			setupDirs:          []string{"subdir", "procfiles"},
			script:             "@echo off\nsetlocal\ncd subdir\necho line1 >> test.sum\nendlocal\ncopy subdir\\*.sum procfiles\\\n",
			wantStdoutContains: []string{"1 file(s) copied"},
			wantFilesExist:     []string{"procfiles/test.sum"},
		},
		{
			name:               "setlocal_scope_keeps_created_files",
			setupDirs:          []string{"subdir"},
			script:             "@echo off\nsetlocal\ncd subdir\necho line1 >> test.sum\nendlocal\ncd subdir\ncopy *.sum ..\\\n",
			wantStdoutContains: []string{"1 file(s) copied"},
			wantFilesExist:     []string{"test.sum"},
		},
		{
			name:      "endlocal_restores_vars_and_directory",
			setupDirs: []string{"subdir"},
			script: "@echo off\n" +
				"set OUTER_VAR=outer_value\n" +
				"setlocal\n" +
				"set OUTER_VAR=inner_value\n" +
				"set INNER_VAR=inner_only\n" +
				"cd subdir\n" +
				"echo %OUTER_VAR% %INNER_VAR% >> test.txt\n" +
				"endlocal\n" +
				"echo After ENDLOCAL: OUTER_VAR=%OUTER_VAR% INNER_VAR=%INNER_VAR% >> result.txt\n",
			wantFiles: map[string]string{
				"subdir/test.txt": "inner_value inner_only \r\n",
				"result.txt":      "After ENDLOCAL: OUTER_VAR=outer_value INNER_VAR= \r\n",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolate(t)
			for _, dir := range tc.setupDirs {
				mkdir(t, dir)
			}

			got := runScript(t, tc.script)
			for _, want := range tc.wantStdoutContains {
				if !strings.Contains(got.stdout, want) {
					t.Errorf("stdout %q does not contain %q (stderr %q)", got.stdout, want, got.stderr)
				}
			}
			for name, want := range tc.wantFiles {
				assertFile(t, name, want)
			}
			for _, name := range tc.wantFilesExist {
				assertFileExists(t, name)
			}
		})
	}
}
