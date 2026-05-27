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
