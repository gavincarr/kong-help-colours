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
