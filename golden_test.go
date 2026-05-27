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
	var buf bytes.Buffer
	var exitCalled bool
	allOpts := append([]kong.Option{
		kong.Name("demo"),
		kong.Exit(func(int) { exitCalled = true }),
		kong.Writers(&buf, &buf),
	}, opts...)
	k, err := kong.New(cli, allOpts...)
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	// k.Stdout is already set to &buf via kong.Writers; keep it in sync.
	k.Stdout = &buf
	if _, err := k.Parse([]string{"--help"}); err != nil {
		// For CLIs with required subcommands, kong prints top-level help
		// (triggering Exit) and then returns an error. Treat that as success.
		if !exitCalled {
			t.Fatalf("Parse(--help): %v", err)
		}
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

// --- groups fixture ---

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

// --- subcommands fixture ---

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

// --- compact fixture ---

func TestGolden_Compact(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "compact",
		cli:  &subcommandsCLI{},
		extraOpts: []kong.Option{
			kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		},
	})
}

// --- tree fixture ---

func TestGolden_Tree(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "tree",
		cli:  &subcommandsCLI{},
		extraOpts: []kong.Option{
			kong.ConfigureHelp(kong.HelpOptions{Tree: true}),
		},
	})
}

// --- flags-last fixture ---

func TestGolden_FlagsLast(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "flags-last",
		cli:  &subcommandsCLI{},
		extraOpts: []kong.Option{
			kong.ConfigureHelp(kong.HelpOptions{FlagsLast: true}),
		},
	})
}

// --- env-vars fixture ---

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

// --- negatable fixture ---

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

// --- defaults fixture ---

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

// --- long-help fixture ---

type longHelpCLI struct {
	Format string `help:"Output format. Use 'csv' for comma-separated values, 'tsv' for tab-separated values, or 'json' for JSON Lines. CSV is the default and is suitable for most spreadsheet tools." default:"csv"`
}

func TestGolden_LongHelp(t *testing.T) {
	runGolden(t, goldenFixture{
		name: "long-help",
		cli:  &longHelpCLI{},
	})
}

var reANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestGolden_StrippedAnsiMatchesPlain(t *testing.T) {
	fixtures := []string{
		"basic", "groups", "subcommands", "compact", "tree",
		"flags-last", "env-vars", "negatable", "defaults", "long-help",
	}
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
