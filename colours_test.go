package helpcolours

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

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

func TestColourise_FullHelp(t *testing.T) {
	in := strings.Join([]string{
		"Usage: csv_cut [OPTIONS] --fields <FIELDS> [CSVFile]",
		"",
		"Arguments:",
		"  [CSVFile]  CSV file to be processed",
		"",
		"Flags:",
		"  -h, --help             Show context-sensitive help.",
		"  -t, --sep <SEP>        CSV separator [default: ,]",
		"  -f, --fields <FIELDS>  Comma-separated list of field names",
	}, "\n") + "\n"

	want := strings.Join([]string{
		"\x1b[33mUsage:\x1b[0m \x1b[32mcsv_cut\x1b[0m \x1b[36m[OPTIONS]\x1b[0m \x1b[32m--fields\x1b[0m \x1b[36m<FIELDS>\x1b[0m \x1b[36m[CSVFile]\x1b[0m",
		"",
		"\x1b[33mArguments:\x1b[0m",
		"  \x1b[36m[CSVFile]\x1b[0m  CSV file to be processed",
		"",
		"\x1b[33mFlags:\x1b[0m",
		"  \x1b[32m-h\x1b[0m, \x1b[32m--help\x1b[0m             Show context-sensitive help.",
		"  \x1b[32m-t\x1b[0m, \x1b[32m--sep\x1b[0m \x1b[36m<SEP>\x1b[0m        CSV separator [default: ,]",
		"  \x1b[32m-f\x1b[0m, \x1b[32m--fields\x1b[0m \x1b[36m<FIELDS>\x1b[0m  Comma-separated list of field names",
	}, "\n") + "\n"

	got := colourise(in)
	if got != want {
		t.Errorf("colourise(...) =\n  got:  %q\n  want: %q", got, want)
	}
}

func TestColourise_EqualsPlaceholder(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "bare uppercase placeholder after equals",
			in:   "  --sep=STRING    Field separator.",
			want: "  \x1b[32m--sep\x1b[0m=\x1b[36mSTRING\x1b[0m    Field separator.",
		},
		{
			name: "slice placeholder with trailing comma-ellipsis",
			in:   "  --fields=FIELDS,...  List of fields.",
			want: "  \x1b[32m--fields\x1b[0m=\x1b[36mFIELDS\x1b[0m,...  List of fields.",
		},
		{
			name: "lowercase after equals is not coloured",
			in:   "  --mode=fast    Speed mode.",
			want: "  \x1b[32m--mode\x1b[0m=fast    Speed mode.",
		},
		{
			name: "digits after equals not coloured",
			in:   "  --port=8080    Listen port.",
			want: "  \x1b[32m--port\x1b[0m=8080    Listen port.",
		},
		{
			name: "angle-bracket form still works (rule 4 original)",
			in:   "  --sep=<SEP>    Field separator.",
			want: "  \x1b[32m--sep\x1b[0m=\x1b[36m<SEP>\x1b[0m    Field separator.",
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

func TestShouldColour_NoColorAlwaysWins(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORCE_COLOR", "1")
	// Even with a real TTY-like file, NO_COLOR must win.
	if shouldColour(os.Stdout) {
		t.Fatal("NO_COLOR set: expected shouldColour=false")
	}
}

func TestShouldColour_ForceColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")

	// A pipe is definitely not a TTY, but FORCE_COLOR overrides.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	if !shouldColour(w) {
		t.Fatal("FORCE_COLOR set: expected shouldColour=true")
	}
}

func TestShouldColour_NoTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	if shouldColour(w) {
		t.Fatal("no TTY, no FORCE_COLOR: expected shouldColour=false")
	}
}

func TestShouldColour_NonFileWriter(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")

	// A bytes.Buffer is not *os.File — should fall through to false.
	var buf bytes.Buffer
	if shouldColour(&buf) {
		t.Fatal("non-*os.File writer, no FORCE_COLOR: expected shouldColour=false")
	}
}
