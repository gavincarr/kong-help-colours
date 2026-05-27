package helpcolours

import "testing"

func TestColourise_SectionHeader(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"flags header", "Flags:", "\x1b[33mFlags:\x1b[0m"},
		{"arguments header", "Arguments:", "\x1b[33mArguments:\x1b[0m"},
		{"commands header", "Commands:", "\x1b[33mCommands:\x1b[0m"},
		{"custom group title with space", "Database options:", "\x1b[33mDatabase options:\x1b[0m"},
		{"not a header (has trailing text)", "Usage: foo bar", "Usage: foo bar"},
		{"not a header (lowercase start)", "flags:", "flags:"},
		{"not a header (indented)", "  Flags:", "  Flags:"},
		{"empty line untouched", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := colourise(tc.in)
			if got != tc.want {
				t.Errorf("colourise(%q) =\n  got:  %q\n  want: %q", tc.in, got, tc.want)
			}
		})
	}
}
