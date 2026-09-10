package tools

import (
	"strings"
	"testing"
)

func TestChoiceDefaults(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		stdin      string
		wantLevel  string
		wantStdout string
		wantStderr string
	}{
		{name: "yes_default_list", stdin: "y", wantLevel: "1", wantStdout: " [Y,N]?y\n"},
		{name: "no_default_list", stdin: "n", wantLevel: "2", wantStdout: " [Y,N]?n\n"},
		{name: "case_insensitive_default", stdin: "Y", wantLevel: "1", wantStdout: " [Y,N]?Y\n"},
		{name: "custom_list_third_key", args: []string{"/C", "abc", "/M", "pick"}, stdin: "c", wantLevel: "3", wantStdout: "pick [a,b,c]?c\n"},
		{name: "skips_newline_before_valid_key", stdin: "\r\nq", args: []string{"/C", "q"}, wantLevel: "1", wantStdout: " [q]?q\n"},
		{name: "invalid_then_valid", stdin: "xY", wantLevel: "1", wantStdout: " [Y,N]?Y\n"},
		{name: "eof_fails", stdin: "", wantLevel: "255", wantStdout: " [Y,N]?\n"},
		{name: "n_hides_list", args: []string{"/N"}, stdin: "y", wantLevel: "1", wantStdout: "y\n"},
		{name: "cs_mismatch_is_invalid", args: []string{"/CS", "/C", "YN"}, stdin: "y", wantLevel: "255", wantStdout: " [Y,N]?\n"},
		{name: "bad_switch_fails", args: []string{"bogus"}, wantLevel: "255", wantStderr: "ERROR: Invalid argument/option - 'bogus'.\nType \"CHOICE /?\" for usage.\n"},
		{name: "t_without_d_fails", args: []string{"/T", "5"}, wantLevel: "255", wantStderr: "ERROR: Invalid argument/option - '/D'.\nType \"CHOICE /?\" for usage.\n"},
		{name: "d_not_in_choices_fails", args: []string{"/C", "ab", "/D", "z"}, wantLevel: "255", wantStderr: "ERROR: Invalid argument/option - '/D'.\nType \"CHOICE /?\" for usage.\n"},
		{name: "timeout_picks_default", args: []string{"/T", "1", "/D", "n"}, stdin: "", wantLevel: "2", wantStdout: " [Y,N]?\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, out, errOut := newProc(strings.NewReader(tc.stdin))
			if err := Choice(p, cmd("choice", tc.args...)); err != nil {
				t.Fatalf("Choice returned error: %v", err)
			}
			if got := errorLevel(p); got != tc.wantLevel {
				t.Errorf("ERRORLEVEL = %q, want %q", got, tc.wantLevel)
			}
			if got := out.String(); got != tc.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tc.wantStdout)
			}
			if got := errOut.String(); got != tc.wantStderr {
				t.Errorf("stderr = %q, want %q", got, tc.wantStderr)
			}
		})
	}
}
