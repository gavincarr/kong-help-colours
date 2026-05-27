# kong-help-colours Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a standalone Go module that adds clap-style colourised `--help` output to CLIs built with `github.com/alecthomas/kong`, without forking kong.

**Architecture:** Wrap `kong.DefaultHelpPrinter` / `kong.DefaultShortHelpPrinter`: temporarily replace `ctx.Stdout` with a `bytes.Buffer`, let kong do all layout, then post-process the buffered text with four regex rules that inject ANSI colour codes. ANSI bytes add no visible width, so column alignment is preserved. Colour is gated by `NO_COLOR`, `FORCE_COLOR`, and TTY detection on the original stdout. Backed by a golden-test matrix that re-renders fixture CLIs through kong on every test run, so any upstream layout drift fails loudly with a readable diff.

**Tech Stack:**
- Go 1.25
- `github.com/alecthomas/kong v1.15.0`
- `golang.org/x/term` (TTY detection)
- Standard library only otherwise

**Spec:** `docs/superpowers/specs/2026-05-27-kong-help-colours-design.md`

---

## File Structure

| Path | Purpose |
|---|---|
| `colours.go` | All non-test source. Exports `Help`, `ShortHelp`. Internal: `colourise(string) string`, `shouldColour(io.Writer) bool`, ANSI constants, compiled regex package-vars. |
| `colours_test.go` | Unit tests for `colourise()` (each rule + interactions) and `shouldColour()` (env × TTY matrix). |
| `help_test.go` | Integration tests that drive a tiny kong CLI end-to-end through `Help` / `ShortHelp` with a buffered stdout. |
| `golden_test.go` | Golden test runner. Walks fixtures in `testdata/`, re-renders each through kong, compares against checked-in goldens. Supports `-update` flag. |
| `testdata/<name>.golden.txt` | Colourised expected output per fixture (ANSI escape sequences inline). |
| `testdata/<name>.plain.golden.txt` | Default-printer (un-coloured) expected output per fixture. |
| `example_test.go` | Runnable godoc example showing the typical kong.Parse wiring. |
| `cmd/demo/main.go` | Small CLI for visual confirmation: `go run ./cmd/demo --help` should look like `csv_cut -h`. |
| `README.md` | One-page usage doc. |
| `go.mod` / `go.sum` | Module metadata, kept current via `go mod tidy`. |

`colours.go` is intentionally the only non-test source file in the root package — keeps the surface tiny.

---

## Conventions for this plan

- All commands assume working directory `/home/gavin/work/kong-help-colours` unless stated otherwise.
- Test runs use `go test ./...` unless a more targeted invocation is shown.
- Commit messages follow Conventional Commits (per user preferences in CLAUDE.md).
- Do **not** `git push` at any point. Local commits only.

---

### Task 1: Project scaffolding

**Files:**
- Modify: `go.mod` (add `golang.org/x/term` dep via `go get`)
- Create: `colours.go` (skeleton — just package + imports so tests compile)

- [ ] **Step 1: Pull in the TTY-detection dep**

Run:
```bash
go get golang.org/x/term
go mod tidy
```
Expected: `go.mod` now lists `golang.org/x/term` as a direct dependency; `go.sum` updated.

- [ ] **Step 2: Create `colours.go` skeleton**

Minimal — just the package, the doc comment, and the ANSI constants. Imports are added as later tasks need them, to keep `go vet` happy at each commit.

```go
// Package helpcolours adds clap-style colourised --help output to CLIs
// built with github.com/alecthomas/kong. Wire it in with:
//
//	kong.Parse(&cli,
//	    kong.Help(helpcolours.Help),
//	    kong.ShortHelp(helpcolours.ShortHelp), // optional
//	)
//
// Colour is enabled when stdout is a terminal (or FORCE_COLOR is set)
// and NO_COLOR is unset.
package helpcolours

// ANSI SGR sequences used by the colouriser.
const (
	ansiReset  = "\x1b[0m"
	ansiYellow = "\x1b[33m"
	ansiGreen  = "\x1b[32m"
	ansiCyan   = "\x1b[36m"
)
```

- [ ] **Step 3: Verify it builds**

Run: `go build ./...`
Expected: no output (success).

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum colours.go
git commit -m "chore: scaffold module with kong + x/term deps"
```

---

### Task 2: Section-header rule (rule 1)

Rule: a line whose entire content matches `^[A-Z][A-Za-z0-9 ]*:$` is wrapped in yellow.

**Files:**
- Modify: `colours.go` (add `colourise()` and the header regex)
- Create: `colours_test.go`

- [ ] **Step 1: Write the failing test**

Create `colours_test.go`:

```go
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
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./... -run TestColourise_SectionHeader`
Expected: compile error — `undefined: colourise`.

- [ ] **Step 3: Implement `colourise` with rule 1 only**

Add to `colours.go` (introducing the `regexp` and `strings` imports):

```go
import (
	"regexp"
	"strings"
)

// Compiled regex rules used by colourise.
var (
	reSectionHeader = regexp.MustCompile(`^[A-Z][A-Za-z0-9 ]*:$`)
)

// colourise applies the colour rules to a block of help text and returns
// the result. Operates line-by-line; rules are applied in order.
func colourise(text string) string {
	if text == "" {
		return ""
	}
	hadTrailingNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = colouriseLine(line)
	}
	joined := strings.Join(out, "\n")
	if hadTrailingNewline {
		joined += "\n"
	}
	return joined
}

func colouriseLine(line string) string {
	if reSectionHeader.MatchString(line) {
		return ansiYellow + line + ansiReset
	}
	return line
}
```

Remove the obsolete `var ( _ = bytes.NewBuffer ... )` block and any imports no longer used (the `goimports` / `go vet` pass in step 4 will surface unused imports).

- [ ] **Step 4: Run the test and confirm it passes**

Run: `go test ./... -run TestColourise_SectionHeader -v`
Expected: all subtests PASS.

Also: `go vet ./...` → no warnings.

- [ ] **Step 5: Commit**

```bash
git add colours.go colours_test.go
git commit -m "feat: colourise section-header lines yellow"
```

---

### Task 3: Usage-line rule (rule 2)

Rule: on a line beginning with `Usage: `, colour `Usage:` yellow and the first whitespace-delimited token after it green. (This combined form means rule 1 doesn't need to fire on the Usage line — it wouldn't match anyway, since "Usage: foo bar" has content after the colon.)

**Files:**
- Modify: `colours.go`
- Modify: `colours_test.go`

- [ ] **Step 1: Add the failing test**

Append to `colours_test.go`:

```go
func TestColourise_UsageLine(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple usage",
			in:   "Usage: csv_cut [OPTIONS]",
			want: "\x1b[33mUsage:\x1b[0m \x1b[32mcsv_cut\x1b[0m [OPTIONS]",
		},
		{
			name: "subcommand usage path-colouring stops at program token",
			in:   "Usage: myapp serve --port=8080",
			want: "\x1b[33mUsage:\x1b[0m \x1b[32mmyapp\x1b[0m serve --port=8080",
		},
		{
			name: "non-usage line untouched by rule 2",
			in:   "Run \"foo --help\" for more information.",
			want: "Run \"foo --help\" for more information.",
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
```

Note: the second case will still leave `--port=8080` un-coloured because rules 3 and 4 don't exist yet. That's intentional — each rule's test asserts only that rule's effect.

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./... -run TestColourise_UsageLine -v`
Expected: assertions fail because rule 2 isn't implemented.

- [ ] **Step 3: Implement rule 2**

In `colours.go`, add a second regex and extend `colouriseLine`:

```go
var (
	reSectionHeader = regexp.MustCompile(`^[A-Z][A-Za-z0-9 ]*:$`)
	reUsageLine     = regexp.MustCompile(`^(Usage: )(\S+)`)
)

func colouriseLine(line string) string {
	if reSectionHeader.MatchString(line) {
		return ansiYellow + line + ansiReset
	}
	line = reUsageLine.ReplaceAllString(line, ansiYellow+"Usage:"+ansiReset+" "+ansiGreen+"${2}"+ansiReset)
	return line
}
```

- [ ] **Step 4: Run all colourise tests**

Run: `go test ./... -run TestColourise -v`
Expected: both `TestColourise_SectionHeader` and `TestColourise_UsageLine` PASS.

- [ ] **Step 5: Commit**

```bash
git add colours.go colours_test.go
git commit -m "feat: colourise Usage line (label yellow, program name green)"
```

---

### Task 4: Flag-token rule (rule 3)

Rule: match `(^|[\s,=])(--?[A-Za-z][A-Za-z0-9-]*)` and wrap the captured flag in green. Header lines and the part of the Usage line already coloured by rule 2 must not be re-coloured.

**Files:**
- Modify: `colours.go`
- Modify: `colours_test.go`

- [ ] **Step 1: Add the failing test**

Append to `colours_test.go`:

```go
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
			want: "  <--FOO>  oddly-named placeholder",
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
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./... -run TestColourise_FlagTokens -v`
Expected: most subtests fail (the header case may already pass).

- [ ] **Step 3: Implement rule 3**

Add a regex and extend `colouriseLine`:

```go
var (
	reSectionHeader = regexp.MustCompile(`^[A-Z][A-Za-z0-9 ]*:$`)
	reUsageLine     = regexp.MustCompile(`^(Usage: )(\S+)`)
	reFlagToken     = regexp.MustCompile(`(^|[\s,=])(--?[A-Za-z][A-Za-z0-9-]*)`)
)

func colouriseLine(line string) string {
	if reSectionHeader.MatchString(line) {
		return ansiYellow + line + ansiReset
	}
	line = reUsageLine.ReplaceAllString(line, ansiYellow+"Usage:"+ansiReset+" "+ansiGreen+"${2}"+ansiReset)
	line = reFlagToken.ReplaceAllString(line, "${1}"+ansiGreen+"${2}"+ansiReset)
	return line
}
```

- [ ] **Step 4: Run all colourise tests**

Run: `go test ./... -run TestColourise -v`
Expected: all subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add colours.go colours_test.go
git commit -m "feat: colourise flag tokens green"
```

---

### Task 5: Placeholder rule (rule 4)

Rule: match `<[^>]+>` and `\[[A-Z][^\]]*\]` and wrap in cyan. The uppercase-letter constraint inside `[...]` prevents matching kong's `[default: ...]` annotations.

**Files:**
- Modify: `colours.go`
- Modify: `colours_test.go`

- [ ] **Step 1: Add the failing test**

Append to `colours_test.go`:

```go
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
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./... -run TestColourise_Placeholders -v`
Expected: at least the placeholder subtests fail.

- [ ] **Step 3: Implement rule 4**

Add a regex and extend `colouriseLine`:

```go
var (
	reSectionHeader = regexp.MustCompile(`^[A-Z][A-Za-z0-9 ]*:$`)
	reUsageLine     = regexp.MustCompile(`^(Usage: )(\S+)`)
	reFlagToken     = regexp.MustCompile(`(^|[\s,=])(--?[A-Za-z][A-Za-z0-9-]*)`)
	rePlaceholder   = regexp.MustCompile(`<[^>]+>|\[[A-Z][^\]]*\]`)
)

func colouriseLine(line string) string {
	if reSectionHeader.MatchString(line) {
		return ansiYellow + line + ansiReset
	}
	line = reUsageLine.ReplaceAllString(line, ansiYellow+"Usage:"+ansiReset+" "+ansiGreen+"${2}"+ansiReset)
	line = reFlagToken.ReplaceAllString(line, "${1}"+ansiGreen+"${2}"+ansiReset)
	line = rePlaceholder.ReplaceAllStringFunc(line, func(m string) string {
		return ansiCyan + m + ansiReset
	})
	return line
}
```

- [ ] **Step 4: Run all colourise tests**

Run: `go test ./... -run TestColourise -v`
Expected: every subtest PASS.

- [ ] **Step 5: Commit**

```bash
git add colours.go colours_test.go
git commit -m "feat: colourise <PLACEHOLDER> and [OPTIONS]-style tokens cyan"
```

---

### Task 6: End-to-end pipeline test

A single test that drives `colourise()` with a realistic multi-line help block (modelled on `csv_cut -h`), asserting every rule fires together and they don't interact badly.

**Files:**
- Modify: `colours_test.go`

- [ ] **Step 1: Add the failing test**

Append to `colours_test.go`:

```go
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
```

Add `"strings"` to the test imports if it isn't already.

- [ ] **Step 2: Run the test**

Run: `go test ./... -run TestColourise_FullHelp -v`
Expected: PASS. If it fails, the diff tells us which rule misbehaves on real-world input.

- [ ] **Step 3: Run the whole package to confirm no regressions**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add colours_test.go
git commit -m "test: cover full multi-line help colourisation"
```

---

### Task 7: Colour gating (`shouldColour`)

Decide whether to colourise. Truth table:

| `NO_COLOR` set | `FORCE_COLOR` set | Stdout is TTY | Result |
|---|---|---|---|
| yes | * | * | false |
| no | yes | * | true |
| no | no | yes | true |
| no | no | no | false |

A non-`*os.File` stdout is treated as not-a-TTY; `FORCE_COLOR` is the escape hatch.

**Files:**
- Modify: `colours.go`
- Modify: `colours_test.go`

- [ ] **Step 1: Add the failing tests**

Extend the existing import block at the top of `colours_test.go` to include `"bytes"` and `"os"` alongside `"testing"`, then append to the file:

```go
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
```

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test ./... -run TestShouldColour -v`
Expected: compile error — `undefined: shouldColour`.

- [ ] **Step 3: Implement `shouldColour`**

Extend the existing import block in `colours.go` to add `"io"`, `"os"`, and `"golang.org/x/term"` alongside `"regexp"` and `"strings"`. The block should look like:

```go
import (
	"io"
	"os"
	"regexp"
	"strings"

	"golang.org/x/term"
)
```

Then append:

```go
// shouldColour decides whether to colourise output written to w.
// Rules: NO_COLOR disables unconditionally; otherwise enabled if
// FORCE_COLOR is set (any non-empty value) or w is a terminal *os.File.
func shouldColour(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" {
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./... -run TestShouldColour -v`
Expected: all four subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add colours.go colours_test.go
git commit -m "feat: add NO_COLOR / FORCE_COLOR / TTY gating"
```

---

### Task 8: `Help` and `ShortHelp` HelpPrinter wrappers

Wire it all together. Both functions delegate straight through to kong's default printer when colour is disabled (zero overhead path), otherwise buffer + colourise + write.

**Files:**
- Modify: `colours.go`
- Create: `help_test.go`

- [ ] **Step 1: Write the failing integration test**

Create `help_test.go`:

```go
package helpcolours

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

type fixtureCLI struct {
	Verbose bool   `short:"v" help:"Enable verbose output."`
	Sep     string `short:"t" placeholder:"SEP" default:"," help:"CSV separator."`
}

// parseHelp invokes a tiny kong CLI with --help, routing stdout to buf,
// and returns the bytes written. exit is overridden so kong doesn't call
// os.Exit when the help flag fires.
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
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./... -run "TestHelp|TestShortHelp" -v`
Expected: compile error — `undefined: Help`, `undefined: ShortHelp`.

- [ ] **Step 3: Implement `Help` and `ShortHelp`**

Extend the existing import block in `colours.go` to add `"bytes"` and the kong import. The block should now look like:

```go
import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/alecthomas/kong"
	"golang.org/x/term"
)
```

Then append:

```go
// Help is a kong.HelpPrinter that delegates to kong.DefaultHelpPrinter
// and post-processes the output to inject ANSI colour codes when colour
// is enabled for ctx.Stdout.
var Help kong.HelpPrinter = func(options kong.HelpOptions, ctx *kong.Context) error {
	return printWithColour(options, ctx, kong.DefaultHelpPrinter)
}

// ShortHelp is the equivalent wrapper around kong.DefaultShortHelpPrinter,
// for use with kong.ShortHelp(...).
var ShortHelp kong.HelpPrinter = func(options kong.HelpOptions, ctx *kong.Context) error {
	return printWithColour(options, ctx, kong.DefaultShortHelpPrinter)
}

// printWithColour routes the inner printer's output through colourise when
// shouldColour returns true; otherwise it calls inner directly with no
// buffering.
func printWithColour(options kong.HelpOptions, ctx *kong.Context, inner kong.HelpPrinter) error {
	target := ctx.Stdout
	if !shouldColour(target) {
		return inner(options, ctx)
	}
	var buf bytes.Buffer
	ctx.Stdout = &buf
	err := inner(options, ctx)
	ctx.Stdout = target
	if err != nil {
		return err
	}
	_, err = io.WriteString(target, colourise(buf.String()))
	return err
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./... -v`
Expected: all tests PASS, including the new integration tests.

- [ ] **Step 5: Commit**

```bash
git add colours.go help_test.go
git commit -m "feat: wire Help and ShortHelp kong.HelpPrinter wrappers"
```

---

### Task 9: Golden-test runner + `basic` fixture

This task establishes the structural-drift safety net. One harness, one fixture; later tasks add more fixtures by reusing the harness.

**Files:**
- Create: `golden_test.go`
- Create: `testdata/basic.golden.txt`
- Create: `testdata/basic.plain.golden.txt`

- [ ] **Step 1: Write the golden runner**

Create `golden_test.go`:

```go
package helpcolours

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
)

// -update regenerates golden files. Run with: go test ./... -update
var update = flag.Bool("update", false, "regenerate golden files")

// goldenFixture describes one CLI under test. cli is a pointer to a
// freshly-zeroed struct of the CLI type; extraOpts are passed to
// kong.New in addition to kong.Name and kong.Exit.
type goldenFixture struct {
	name      string
	cli       any
	extraOpts []kong.Option
}

func runGolden(t *testing.T, f goldenFixture) {
	t.Helper()

	// Coloured run: FORCE_COLOR so a buffered (non-TTY) stdout still gets ANSI.
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")
	coloured := renderHelp(t, f.cli, append([]kong.Option{kong.Help(Help)}, f.extraOpts...)...)

	// Plain run: NO_COLOR so even Help() delegates straight through to default.
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORCE_COLOR", "")
	plain := renderHelp(t, f.cli, append([]kong.Option{kong.Help(Help)}, f.extraOpts...)...)

	colouredPath := filepath.Join("testdata", f.name+".golden.txt")
	plainPath := filepath.Join("testdata", f.name+".plain.golden.txt")

	if *update {
		writeGolden(t, colouredPath, coloured)
		writeGolden(t, plainPath, plain)
		return
	}

	compareGolden(t, colouredPath, coloured)
	compareGolden(t, plainPath, plain)
}

func renderHelp(t *testing.T, cli any, opts ...kong.Option) []byte {
	t.Helper()
	allOpts := append([]kong.Option{
		kong.Name("demo"),
		kong.Exit(func(int) {}),
	}, opts...)
	k, err := kong.New(cli, allOpts...)
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	var buf bytes.Buffer
	k.Stdout = &buf
	if _, err := k.Parse([]string{"--help"}); err != nil {
		t.Fatalf("Parse(--help): %v", err)
	}
	return buf.Bytes()
}

func writeGolden(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func compareGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run `go test ./... -update` to create)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden mismatch for %s\n--- want\n%s\n--- got\n%s\n--- (run `go test ./... -update` to regenerate)",
			path, want, got)
	}
}

// --- fixtures ---

type basicCLI struct {
	Verbose bool   `short:"v" help:"Enable verbose output."`
	Sep     string `short:"t" placeholder:"SEP" default:"," help:"CSV separator."`
	File    string `arg:"" optional:"" help:"Input file."`
}

func TestGolden_Basic(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "basic",
		cli:  &basicCLI{},
	})
}
```

- [ ] **Step 2: Generate the goldens for the first time**

Run: `go test ./... -update -run TestGolden_Basic`
Expected: PASS, and `testdata/basic.golden.txt` + `testdata/basic.plain.golden.txt` appear.

- [ ] **Step 3: Inspect the goldens**

Run: `cat -v testdata/basic.golden.txt` (the `-v` makes ANSI escapes visible as `^[[33m` etc.) and verify the colours look right: yellow headers, green `demo` / flag names, cyan `<SEP>` placeholder, lowercase `[default: ,]` un-coloured.

Also run: `cat testdata/basic.plain.golden.txt` and verify it's normal kong output with zero ANSI bytes.

- [ ] **Step 4: Re-run without `-update` and confirm it stays green**

Run: `go test ./... -run TestGolden_Basic -v`
Expected: PASS (golden matches).

- [ ] **Step 5: Add a stripped-ANSI consistency assertion**

Add `"regexp"` to the existing import block at the top of `golden_test.go`, then append the following to the bottom of the file:

```go
var reANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestGolden_StrippedAnsiMatchesPlain(t *testing.T) {
	fixtures := []string{"basic"}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			coloured, err := os.ReadFile(filepath.Join("testdata", name+".golden.txt"))
			if err != nil {
				t.Fatal(err)
			}
			plain, err := os.ReadFile(filepath.Join("testdata", name+".plain.golden.txt"))
			if err != nil {
				t.Fatal(err)
			}
			stripped := reANSI.ReplaceAll(coloured, nil)
			if !bytes.Equal(stripped, plain) {
				t.Errorf("stripping ANSI from %s.golden.txt must equal %s.plain.golden.txt\n--- plain:\n%s\n--- stripped:\n%s",
					name, name, plain, stripped)
			}
		})
	}
}
```

Run: `go test ./... -run TestGolden -v`
Expected: both PASS.

(The `fixtures` slice will be extended in Task 10 as more goldens are added.)

- [ ] **Step 6: Commit**

```bash
git add golden_test.go testdata/basic.golden.txt testdata/basic.plain.golden.txt
git commit -m "test: add golden-test harness with basic CLI fixture"
```

---

### Task 10: Remaining golden fixtures

Add the rest of the matrix from the spec. Each fixture is independent — its `cli` struct and any `extraOpts` differ; the runner is reused.

**Files:**
- Modify: `golden_test.go` (add fixture structs + `TestGolden_*` functions + extend `fixtures` slice in `TestGolden_StrippedAnsiMatchesPlain`)
- Create: `testdata/<name>.golden.txt` and `testdata/<name>.plain.golden.txt` for each new fixture

- [ ] **Step 1: Add the `groups` fixture**

Append to `golden_test.go`:

```go
type groupsCLI struct {
	Verbose bool   `short:"v" help:"Enable verbose output."`
	Host    string `group:"Database" help:"Database host." default:"localhost"`
	Port    int    `group:"Database" help:"Database port." default:"5432"`
}

func TestGolden_Groups(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "groups",
		cli:  &groupsCLI{},
		extraOpts: []kong.Option{
			kong.ExplicitGroups([]kong.Group{
				{Key: "Database", Title: "Database options:"},
			}),
		},
	})
}
```

- [ ] **Step 2: Add the `subcommands` fixture**

```go
type subcommandsCLI struct {
	Verbose bool                `short:"v" help:"Enable verbose output."`
	Serve   subcommandsServeCmd `cmd:"" help:"Run the HTTP server."`
	Migrate subcommandsMigCmd   `cmd:"" help:"Run database migrations."`
}
type subcommandsServeCmd struct {
	Port int `help:"Listen port." default:"8080"`
}
type subcommandsMigCmd struct {
	Dir string `arg:"" help:"Migrations directory."`
}

func TestGolden_Subcommands(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "subcommands",
		cli:  &subcommandsCLI{},
	})
}
```

- [ ] **Step 3: Add the `compact` fixture**

```go
func TestGolden_Compact(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "compact",
		cli:  &subcommandsCLI{},
		extraOpts: []kong.Option{
			kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		},
	})
}
```

- [ ] **Step 4: Add the `tree` fixture**

```go
func TestGolden_Tree(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "tree",
		cli:  &subcommandsCLI{},
		extraOpts: []kong.Option{
			kong.ConfigureHelp(kong.HelpOptions{Tree: true}),
		},
	})
}
```

- [ ] **Step 5: Add the `flags-last` fixture**

```go
func TestGolden_FlagsLast(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "flags-last",
		cli:  &subcommandsCLI{},
		extraOpts: []kong.Option{
			kong.ConfigureHelp(kong.HelpOptions{FlagsLast: true}),
		},
	})
}
```

- [ ] **Step 6: Add the `env-vars` fixture**

```go
type envVarsCLI struct {
	DBHost string `env:"DB_HOST" help:"Database host." default:"localhost"`
	DBPort int    `env:"DB_PORT" help:"Database port." default:"5432"`
}

func TestGolden_EnvVars(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "env-vars",
		cli:  &envVarsCLI{},
	})
}
```

- [ ] **Step 7: Add the `negatable` fixture**

```go
type negatableCLI struct {
	Verbose bool `negatable:"" help:"Toggle verbose output."`
	Cache   bool `negatable:"no-cache" help:"Toggle response caching."`
}

func TestGolden_Negatable(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "negatable",
		cli:  &negatableCLI{},
	})
}
```

- [ ] **Step 8: Add the `defaults` fixture**

```go
type defaultsCLI struct {
	Sep     string `default:"," help:"Field separator."`
	Retries int    `default:"3" help:"Number of retries."`
	Output  string `default:"-" help:"Output path or '-' for stdout."`
}

func TestGolden_Defaults(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "defaults",
		cli:  &defaultsCLI{},
	})
}
```

- [ ] **Step 9: Add the `long-help` fixture**

```go
type longHelpCLI struct {
	Format string `help:"Output format. Use 'csv' for comma-separated values, 'tsv' for tab-separated values, or 'json' for JSON Lines. CSV is the default and is suitable for most spreadsheet tools." default:"csv"`
}

func TestGolden_LongHelp(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "long-help",
		cli:  &longHelpCLI{},
	})
}
```

- [ ] **Step 10: Extend the strip-consistency fixture list**

In `TestGolden_StrippedAnsiMatchesPlain`, replace:

```go
fixtures := []string{"basic"}
```

with:

```go
fixtures := []string{
	"basic", "groups", "subcommands", "compact", "tree",
	"flags-last", "env-vars", "negatable", "defaults", "long-help",
}
```

- [ ] **Step 11: Generate all the goldens**

Run: `go test ./... -update -run TestGolden`
Expected: PASS; nine new pairs of golden files appear in `testdata/`.

- [ ] **Step 12: Inspect a sample for correctness**

Run: `cat -v testdata/groups.golden.txt` — confirm the custom "Database options:" group title is yellow.

Run: `cat -v testdata/defaults.golden.txt` — confirm `[default: ...]` annotations are NOT cyan-coloured (lowercase `d`).

Run: `cat -v testdata/env-vars.golden.txt` — confirm env-var annotations like `($DB_HOST)` render in their default colour (we don't special-case env hints).

If anything looks wrong, that's a real bug — go back and fix the corresponding rule, then re-run with `-update`.

- [ ] **Step 13: Re-run all tests without `-update`**

Run: `go test ./...`
Expected: PASS across the board, including `TestGolden_StrippedAnsiMatchesPlain`.

- [ ] **Step 14: Commit**

```bash
git add golden_test.go testdata/
git commit -m "test: add golden fixtures covering kong's layout matrix"
```

---

### Task 11: Godoc example + README

**Files:**
- Create: `example_test.go`
- Create: `README.md`

- [ ] **Step 1: Write the godoc example**

Create `example_test.go`:

```go
package helpcolours_test

import (
	"github.com/alecthomas/kong"
	helpcolours "github.com/gavincarr/kong-help-colours"
)

func ExampleHelp() {
	var cli struct {
		Verbose bool   `short:"v" help:"Enable verbose output."`
		Out     string `short:"o" placeholder:"FILE" help:"Output file."`
	}
	_ = kong.Parse(&cli,
		kong.Help(helpcolours.Help),
		kong.ShortHelp(helpcolours.ShortHelp),
	)
}
```

(The example doesn't have `// Output:` markers because `kong.Parse` reads from `os.Args` and would be non-deterministic in a `go test` context. The `_` of `kong.Parse`'s return suppresses the unused-result warning.)

Run: `go vet ./... && go test ./...`
Expected: all green; the example compiles and runs in the test binary without producing output to compare against.

- [ ] **Step 2: Write the README**

Create `README.md`:

````markdown
# kong-help-colours

Drop-in colourised `--help` output for CLIs built with
[alecthomas/kong](https://github.com/alecthomas/kong). Three colours,
no configuration, just clap-style help.

## Install

```bash
go get github.com/gavincarr/kong-help-colours
```

## Use

```go
import (
    "github.com/alecthomas/kong"
    helpcolours "github.com/gavincarr/kong-help-colours"
)

func main() {
    var cli struct {
        Verbose bool   `short:"v" help:"Enable verbose output."`
        Out     string `short:"o" placeholder:"FILE" help:"Output file."`
    }
    ctx := kong.Parse(&cli,
        kong.Help(helpcolours.Help),
        kong.ShortHelp(helpcolours.ShortHelp), // optional
    )
    _ = ctx
}
```

## Colour scheme

| Element | Colour |
|---|---|
| Section headers (`Usage:`, `Flags:`, `Arguments:`, `Commands:`, group titles) | yellow |
| Program name in the usage line; flag tokens (`--foo`, `-f`) | green |
| Placeholders (`<FILE>`, `[OPTIONS]`) | cyan |

The scheme is fixed — no options, no configuration. It mirrors clap's
defaults so it'll look like every Rust CLI you've used.

## When colours are emitted

Colour is enabled when **all** of:

- `NO_COLOR` env var is **not** set ([no-color.org](https://no-color.org))
- Either `FORCE_COLOR` is set, or stdout is a terminal

When colour is disabled the package delegates straight through to
`kong.DefaultHelpPrinter` — zero overhead, byte-for-byte identical output.

## Try it

```bash
go run ./cmd/demo --help
```
````

- [ ] **Step 3: Commit**

```bash
git add example_test.go README.md
git commit -m "docs: add README and runnable godoc example"
```

---

### Task 12: Demo command + final verification

**Files:**
- Create: `cmd/demo/main.go`

- [ ] **Step 1: Write the demo CLI**

Create `cmd/demo/main.go`:

```go
// Command demo is a small kong-based CLI used to eyeball
// kong-help-colours output. Run with --help.
package main

import (
	"fmt"

	"github.com/alecthomas/kong"
	helpcolours "github.com/gavincarr/kong-help-colours"
)

type CLI struct {
	Verbose bool   `short:"v" help:"Enable verbose debug output."`
	Sep     string `short:"t" placeholder:"SEP" default:"," help:"Field separator."`
	Fields  string `short:"f" placeholder:"FIELDS" required:"" help:"Comma-separated list of field names to select."`
	Exclude bool   `short:"x" help:"Exclude the specified fields, instead of selecting them."`
	File    string `arg:"" optional:"" help:"Input CSV file (reads from stdin if omitted)."`
}

func main() {
	var cli CLI
	ctx := kong.Parse(&cli,
		kong.Name("demo"),
		kong.Description("Demo CLI for kong-help-colours — mirrors csv_cut's shape."),
		kong.Help(helpcolours.Help),
		kong.ShortHelp(helpcolours.ShortHelp),
	)
	fmt.Printf("Parsed: %+v (cmd=%q)\n", cli, ctx.Command())
}
```

- [ ] **Step 2: Build and run it under a PTY-equivalent**

Run: `go build ./cmd/demo && ./demo --help`
Expected: coloured help output similar in style to `csv_cut -h`. Yellow `Usage:`/`Flags:`/`Arguments:`, green `demo` and flag names, cyan `<SEP>`/`<FIELDS>`/`[OPTIONS]`. The `[default: ,]` annotation is **not** coloured.

For pipe testing:
- Run: `./demo --help | cat` — output should be plain (no ANSI), because piping makes stdout a non-TTY.
- Run: `FORCE_COLOR=1 ./demo --help | cat -v` — output should contain `^[[33m`, `^[[32m`, `^[[36m` escapes.
- Run: `NO_COLOR=1 ./demo --help` — output should be plain even in a terminal.

- [ ] **Step 3: Run the full test suite one more time**

Run: `go test ./... -v`
Expected: every test PASS.

- [ ] **Step 4: Tidy go.mod**

Run: `go mod tidy`
Expected: no diff, or only minor reordering. If anything substantive changes, investigate before committing.

- [ ] **Step 5: Clean up the build artifact**

```bash
rm -f demo
```

- [ ] **Step 6: Commit**

```bash
git add cmd/demo/main.go
git diff --cached --stat   # sanity check — should only show cmd/demo/main.go
git commit -m "feat: add cmd/demo for visual confirmation"
```

---

## Self-review checklist (run before opening for review)

- [ ] `go test ./...` passes from a clean checkout.
- [ ] `go vet ./...` is clean.
- [ ] `go build ./...` is clean.
- [ ] `go mod tidy` produces no diff.
- [ ] Every golden file is checked in and `TestGolden_StrippedAnsiMatchesPlain` is green for the full fixture list.
- [ ] `./demo --help`, `./demo --help | cat`, and `NO_COLOR=1 ./demo --help` all behave as described in Task 12.
- [ ] `go doc` of the package shows the doc comment from Task 1.
