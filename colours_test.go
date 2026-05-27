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
		{"not a header (has trailing text)", "Usage: foo bar", "\x1b[33mUsage:\x1b[0m \x1b[32mfoo\x1b[0m bar"},
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

func TestColourise_UsageLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple usage",
			in:   "Usage: csv_cut [OPTIONS]",
			want: "\x1b[33mUsage:\x1b[0m \x1b[32mcsv_cut\x1b[0m \x1b[36m[OPTIONS]\x1b[0m",
		},
		{
			name: "subcommand usage path-colouring stops at program token, but rule 3 colors flags",
			in:   "Usage: myapp serve --port=8080",
			want: "\x1b[33mUsage:\x1b[0m \x1b[32mmyapp\x1b[0m serve \x1b[32m--port\x1b[0m=8080",
		},
		{
			name: "rule 3 colors flags even in quoted strings (intended by design)",
			in:   "Run \"foo --help\" for more information.",
			want: "Run \"foo \x1b[32m--help\x1b[0m\" for more information.",
		},
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

func TestColourise_FlagTokens(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "short and long flag pair",
			in:   "  -v, --verbose          Display verbose debug output",
			want: "  \x1b[32m-v\x1b[0m, \x1b[32m--verbose\x1b[0m          Display verbose debug output",
		},
		{
			name: "long flag at line start",
			in:   "--help     Show context-sensitive help",
			want: "\x1b[32m--help\x1b[0m     Show context-sensitive help",
		},
		{
			name: "flag preceded by equals sign",
			in:   "see =--also-this token",
			want: "see =\x1b[32m--also-this\x1b[0m token",
		},
		{
			name: "no false-match inside placeholder (no leading boundary)",
			in:   "  <--FOO>  oddly-named placeholder",
			want: "  \x1b[36m<--FOO>\x1b[0m  oddly-named placeholder",
		},
		{
			name: "section header line untouched by flag rule",
			in:   "Flags:",
			want: "\x1b[33mFlags:\x1b[0m",
		},
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

func TestColourise_Placeholders(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "angle-bracket placeholder",
			in:   "  --sep <SEP>      CSV separator",
			want: "  \x1b[32m--sep\x1b[0m \x1b[36m<SEP>\x1b[0m      CSV separator",
		},
		{
			name: "bracketed uppercase placeholder",
			in:   "Usage: foo [OPTIONS]",
			want: "\x1b[33mUsage:\x1b[0m \x1b[32mfoo\x1b[0m \x1b[36m[OPTIONS]\x1b[0m",
		},
		{
			name: "lowercase bracketed text is not a placeholder",
			in:   "  --sep <SEP>      CSV separator [default: ,]",
			want: "  \x1b[32m--sep\x1b[0m \x1b[36m<SEP>\x1b[0m      CSV separator [default: ,]",
		},
		{
			name: "mixed-case bracketed (first char must be uppercase)",
			in:   "see [Examples] section",
			want: "see \x1b[36m[Examples]\x1b[0m section",
		},
		{
			name: "section header untouched",
			in:   "Arguments:",
			want: "\x1b[33mArguments:\x1b[0m",
		},
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
