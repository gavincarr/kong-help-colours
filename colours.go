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

import (
	"regexp"
	"strings"
)

// ANSI SGR sequences used by the colouriser.
const (
	ansiReset  = "\x1b[0m"
	ansiYellow = "\x1b[33m"
	ansiGreen  = "\x1b[32m"
	ansiCyan   = "\x1b[36m"
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
