package helpcolours

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
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
