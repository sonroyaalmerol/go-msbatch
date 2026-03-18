package processor

import (
	"testing"
)

func TestSplitForSetItems(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []string
		expectEmpty bool
	}{
		{
			name:     "simple space separated",
			input:    "1 2 3",
			expected: []string{"1", "2", "3"},
		},
		{
			name:     "comma separated",
			input:    "alpha,beta,gamma",
			expected: []string{"alpha", "beta", "gamma"},
		},
		{
			name:     "semicolon separated",
			input:    "one;two;three",
			expected: []string{"one", "two", "three"},
		},
		{
			name:     "mixed delimiters",
			input:    "1 2,3;4",
			expected: []string{"1", "2", "3", "4"},
		},
		{
			name:     "tab separated",
			input:    "a\tb\tc",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "quoted string as single item",
			input:    `"hello world"`,
			expected: []string{`"hello world"`},
		},
		{
			name:     "quoted with other items",
			input:    `a "b c" d`,
			expected: []string{"a", `"b c"`, "d"},
		},
		{
			name:     "single quotes",
			input:    `'hello world'`,
			expected: []string{`'hello world'`},
		},
		{
			name:        "empty string",
			input:       "",
			expectEmpty: true,
		},
		{
			name:        "only delimiters",
			input:       "  , , ; ;  ",
			expectEmpty: true,
		},
		{
			name:     "trailing delimiters",
			input:    "a b c ",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "leading delimiters",
			input:    " a b c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "quoted with comma inside",
			input:    `"a,b,c" d e`,
			expected: []string{`"a,b,c"`, "d", "e"},
		},
		{
			name:     "path with spaces in quotes",
			input:    `"C:\Program Files" "C:\Users"`,
			expected: []string{`"C:\Program Files"`, `"C:\Users"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitForSetItems(tt.input)
			if tt.expectEmpty {
				if len(got) != 0 {
					t.Errorf("splitForSetItems(%q) = %v, want empty", tt.input, got)
				}
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("splitForSetItems(%q) = %v, want %v", tt.input, got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("splitForSetItems(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
				}
			}
		})
	}
}
