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
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/alecthomas/kong"
	"golang.org/x/term"
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
	reUsageLine     = regexp.MustCompile(`^(Usage: )(\S+)`)
	reFlagToken     = regexp.MustCompile(`(^|[\s,=])(--?[A-Za-z][A-Za-z0-9-]*)`)
	rePlaceholder   = regexp.MustCompile(`<[^>]+>|\[[A-Z][^\]]*\]`)
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
	line = reUsageLine.ReplaceAllString(line, ansiYellow+"Usage:"+ansiReset+" "+ansiGreen+"${2}"+ansiReset)
	line = reFlagToken.ReplaceAllString(line, "${1}"+ansiGreen+"${2}"+ansiReset)
	line = rePlaceholder.ReplaceAllStringFunc(line, func(m string) string {
		return ansiCyan + m + ansiReset
	})
	return line
}

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
