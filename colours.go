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
