package helpcolours

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

type fixtureCLI struct {
	Verbose bool   `short:"v" help:"Enable verbose output."`
	Sep     string `short:"t" placeholder:"SEP" default:"," help:"CSV separator."`
}

// parseHelp invokes a tiny kong CLI with --help, routing stdout to buf.
// exit is overridden so kong doesn't call os.Exit when the help flag fires.
func parseHelp(t *testing.T, buf *bytes.Buffer, opts ...kong.Option) {
	t.Helper()
	var cli fixtureCLI
	allOpts := append([]kong.Option{
		kong.Name("demo"),
		kong.Exit(func(int) {}),
	}, opts...)
	k, err := kong.New(&cli, allOpts...)
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	k.Stdout = buf
	if _, err := k.Parse([]string{"--help"}); err != nil {
		t.Fatalf("Parse(--help): %v", err)
	}
}

func TestHelp_AddsAnsiWhenForced(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")

	var buf bytes.Buffer
	parseHelp(t, &buf, kong.Help(Help))

	out := buf.String()
	if !strings.Contains(out, "\x1b[33m") {
		t.Errorf("expected yellow ANSI in output; got:\n%s", out)
	}
	if !strings.Contains(out, "\x1b[32m--verbose\x1b[0m") {
		t.Errorf("expected --verbose coloured green; got:\n%s", out)
	}
}

func TestHelp_NoColorMatchesDefault(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORCE_COLOR", "")

	var coloured, plain bytes.Buffer
	parseHelp(t, &coloured, kong.Help(Help))
	parseHelp(t, &plain, kong.Help(kong.DefaultHelpPrinter))

	if coloured.String() != plain.String() {
		t.Errorf("with NO_COLOR set, Help output must match DefaultHelpPrinter exactly\n  coloured: %q\n  plain:    %q",
			coloured.String(), plain.String())
	}
}

func TestShortHelp_AddsAnsiWhenForced(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")

	var cli fixtureCLI
	k, err := kong.New(&cli,
		kong.Name("demo"),
		kong.Exit(func(int) {}),
		kong.ShortHelp(ShortHelp),
	)
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	var buf bytes.Buffer
	k.Stdout = &buf

	// ShortHelp is normally fired by kong on parse errors. To exercise our
	// wrapper without contriving a parse failure, call it directly with a
	// context built via kong.Trace.
	ctx, err := kong.Trace(k, []string{})
	if err != nil {
		t.Fatalf("kong.Trace: %v", err)
	}
	if err := ShortHelp(kong.HelpOptions{}, ctx); err != nil {
		t.Fatalf("ShortHelp: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "\x1b[33mUsage:\x1b[0m") {
		t.Errorf("expected coloured Usage: in short help; got:\n%s", out)
	}
}

func TestShortHelp_NoColorMatchesDefault(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORCE_COLOR", "")

	run := func(printer kong.HelpPrinter) string {
		var cli fixtureCLI
		k, err := kong.New(&cli,
			kong.Name("demo"),
			kong.Exit(func(int) {}),
			kong.ShortHelp(printer),
		)
		if err != nil {
			t.Fatalf("kong.New: %v", err)
		}
		var buf bytes.Buffer
		k.Stdout = &buf
		ctx, err := kong.Trace(k, []string{})
		if err != nil {
			t.Fatalf("kong.Trace: %v", err)
		}
		if err := printer(kong.HelpOptions{}, ctx); err != nil {
			t.Fatalf("printer: %v", err)
		}
		return buf.String()
	}

	if got, want := run(ShortHelp), run(kong.DefaultShortHelpPrinter); got != want {
		t.Errorf("with NO_COLOR set, ShortHelp output must match DefaultShortHelpPrinter exactly\n  got:  %q\n  want: %q", got, want)
	}
}

func TestHelp_PropagatesTerminalWidth(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("COLUMNS", "120")

	type wideCLI struct {
		Format string `help:"Output format. Use 'csv' for comma-separated values, 'tsv' for tab-separated, or 'json' for JSON Lines. CSV is the default and is suitable for most spreadsheet tools." default:"csv"`
	}
	var cli wideCLI
	k, err := kong.New(&cli,
		kong.Name("demo"),
		kong.Exit(func(int) {}),
		kong.Help(Help),
	)
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	var buf bytes.Buffer
	k.Stdout = &buf
	if _, err := k.Parse([]string{"--help"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Strip ANSI to compare line widths.
	reANSI := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	plain := reANSI.ReplaceAllString(buf.String(), "")

	// The long help should not be wrapped near 80 cols when COLUMNS=120.
	// Find the longest line:
	var maxLen int
	for _, line := range strings.Split(strings.TrimRight(plain, "\n"), "\n") {
		if l := len(line); l > maxLen {
			maxLen = l
		}
	}
	if maxLen <= 80 {
		t.Errorf("expected at least one line longer than 80 cols when COLUMNS=120, max was %d\noutput:\n%s", maxLen, plain)
	}
}
