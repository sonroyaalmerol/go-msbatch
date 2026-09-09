package tests

import "testing"

// Every case runs in its own directory with stdout/stderr captured and files
// asserted byte-exact: redirected output is CRLF and keeps the whitespace that
// preceded the redirect operator, exactly like cmd.exe.
type redirectCase struct {
	name       string
	setupFiles map[string]string
	script     string
	wantStdout string
	wantStderr string
	wantFiles  map[string]string
	wantAbsent []string
}

func TestRedirects(t *testing.T) {
	cases := []redirectCase{
		{
			name:      "stdout_to_file",
			script:    "@echo off\necho hello world > out.txt\n",
			wantFiles: map[string]string{"out.txt": "hello world \r\n"},
		},
		{
			name:      "stdout_to_file_without_space",
			script:    "@echo off\necho hello world>out.txt\n",
			wantFiles: map[string]string{"out.txt": "hello world\r\n"},
		},
		{
			name:      "append_to_file",
			script:    "@echo off\necho line1> out.txt\necho line2>> out.txt\n",
			wantFiles: map[string]string{"out.txt": "line1\r\nline2\r\n"},
		},
		{
			name:       "stdin_from_file",
			setupFiles: map[string]string{"input.txt": "test input\n"},
			script:     "@echo off\ntype < input.txt\n",
			wantStdout: "test input\n",
		},
		{
			name:       "stderr_to_console",
			script:     "@echo off\necho error message >&2\n",
			wantStderr: "error message \n",
		},
		{
			name:       "stdout_to_named_err_file",
			script:     "@echo off\necho error message> err.txt\necho normal output\n",
			wantStdout: "normal output\n",
			wantFiles:  map[string]string{"err.txt": "error message\r\n"},
		},
		{
			name:       "stderr_to_nul",
			script:     "@echo off\necho error message 2>nul\necho normal output\n",
			wantStdout: "error message \nnormal output\n",
		},
		{
			name:       "stdout_to_nul",
			script:     "@echo off\necho normal output > nul\necho this shows\n",
			wantStdout: "this shows\n",
		},
		{
			name:       "both_streams_to_nul",
			script:     "@echo off\necho output> nul 2>&1\necho error 2>nul >&2\n",
			wantStdout: "",
		},
		{
			name:      "both_streams_to_file",
			script:    "@echo off\necho output > combined.txt 2>&1\n",
			wantFiles: map[string]string{"combined.txt": "output  \r\n"},
		},
		{
			name:       "separate_stdout_stderr_files",
			script:     "@echo off\necho stdout message > stdout.txt 2> stderr.txt\necho stderr message >&2\n",
			wantStderr: "stderr message \n",
			wantFiles: map[string]string{
				"stdout.txt": "stdout message  \r\n",
				"stderr.txt": "",
			},
		},
		{
			name:      "operator_at_end_of_line",
			script:    "@echo off\necho A B>C\n",
			wantFiles: map[string]string{"C": "A B\r\n"},
		},
		{
			name:      "operator_in_middle_of_line",
			script:    "@echo off\necho A>C B\n",
			wantFiles: map[string]string{"C": "A B\r\n"},
		},
		{
			name:      "operator_at_start_of_line",
			script:    "@echo off\n>C echo A B\n",
			wantFiles: map[string]string{"C": "A B\r\n"},
		},
		{
			name:      "trailing_number_not_a_descriptor",
			script:    "@echo off\nset _demo=abc 5\necho %_demo% >> demofile.txt\n",
			wantFiles: map[string]string{"demofile.txt": "abc 5 \r\n"},
		},
		{
			name:      "quoted_target",
			script:    "@echo off\necho hello world > \"my file.txt\"\n",
			wantFiles: map[string]string{"my file.txt": "hello world \r\n"},
		},
		{
			name:       "redirect_overwrites",
			setupFiles: map[string]string{"out.txt": "old content\n"},
			script:     "@echo off\necho new content > out.txt\n",
			wantFiles:  map[string]string{"out.txt": "new content \r\n"},
		},
		{
			name:      "block_to_file",
			script:    "@echo off\n(\necho sample text1\necho sample text2\n) > logfile.txt\n",
			wantFiles: map[string]string{"logfile.txt": "sample text1\r\nsample text2\r\n"},
		},
		{
			name:       "pipe_to_external",
			script:     "@echo off\necho hello | cat\n",
			wantStdout: "hello \n",
		},
		{
			name:      "concat_operator_writes_both",
			script:    "@echo off\necho first > a.txt & echo second > b.txt\n",
			wantFiles: map[string]string{"a.txt": "first  \r\n", "b.txt": "second \r\n"},
		},
		{
			name:      "and_then_operator_writes_both",
			script:    "@echo off\necho first > a.txt && echo second > b.txt\n",
			wantFiles: map[string]string{"a.txt": "first  \r\n", "b.txt": "second \r\n"},
		},
		{
			name:       "or_else_operator_skips_second",
			script:     "@echo off\necho first > a.txt || echo second > b.txt\n",
			wantFiles:  map[string]string{"a.txt": "first  \r\n"},
			wantAbsent: []string{"b.txt"},
		},
		{
			name:      "stderr_append_with_dup",
			script:    "@echo off\necho error1>> err.txt 2>&1\necho error2>> err.txt 2>&1\n",
			wantFiles: map[string]string{"err.txt": "error1 \r\nerror2 \r\n"},
		},
		{
			name:       "stdin_and_stdout_redirected",
			setupFiles: map[string]string{"names.txt": "Jones\nSmith\nWilson\n"},
			script:     "@echo off\ntype < names.txt > output.txt\n",
			wantFiles:  map[string]string{"output.txt": "Jones\r\nSmith\r\nWilson\r\n"},
		},
		{
			name:       "nul_lowercase",
			script:     "@echo off\necho test0 > nul\n",
			wantAbsent: []string{"nul"},
		},
		{
			name:       "nul_uppercase",
			script:     "@echo off\necho test1 > NUL\n",
			wantAbsent: []string{"NUL"},
		},
		{
			name:       "nul_mixed_case_1",
			script:     "@echo off\necho test2 > Nul\n",
			wantAbsent: []string{"Nul"},
		},
		{
			name:       "nul_mixed_case_2",
			script:     "@echo off\necho test3 > NuL\n",
			wantAbsent: []string{"NuL"},
		},
		{
			name:      "target_from_variable",
			script:    "@echo off\nset FILE=output.txt\necho hello > %FILE%\n",
			wantFiles: map[string]string{"output.txt": "hello \r\n"},
		},
		{
			name:       "stdin_duplicated_onto_zero",
			script:     "@echo off\necho test <&0\n",
			wantStdout: "test \n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolate(t)
			for name, content := range tc.setupFiles {
				writeFile(t, name, content)
			}

			got := runScript(t, tc.script)
			if got.stdout != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", got.stdout, tc.wantStdout)
			}
			if got.stderr != tc.wantStderr {
				t.Errorf("stderr = %q, want %q", got.stderr, tc.wantStderr)
			}
			for name, want := range tc.wantFiles {
				assertFile(t, name, want)
			}
			for _, name := range tc.wantAbsent {
				assertNoFile(t, name)
			}
		})
	}
}
