# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A single-package Go library (`package helpcolours`) that gives
[alecthomas/kong](https://github.com/alecthomas/kong) CLIs clap-style
coloured `--help`. The whole public API is two `kong.HelpPrinter` values,
`Help` and `ShortHelp`. There are deliberately no options or configuration:
custom schemes, per-element toggles and 256/truecolour are non-goals (see
`docs/superpowers/specs/2026-05-27-kong-help-colours-design.md`). Don't add
knobs without discussing it first.

## Commands

```bash
go test -race ./...                      # what CI runs, alongside go vet
go vet ./...
go test -run TestGolden_Negatable ./...  # run a single test
go test ./... -update                    # regenerate testdata/*.golden.txt
go run ./cmd/demo --help                 # eyeball real TTY output
go mod tidy && git diff --exit-code go.mod go.sum   # CI tidy check
```

CI (`.github/workflows/ci.yml`) runs Go 1.23, 1.24 and stable on Linux,
macOS and Windows, so `go.mod` stays at `go 1.23.0`.

## Architecture

Everything is in `colours.go`. It wraps kong's printer rather than
reimplementing it:

1. `printWithColour` checks `shouldColour(ctx.Stdout)`. When colour is off
   (`NO_COLOR` set, or no `FORCE_COLOR` and stdout isn't a TTY), it calls
   `kong.DefaultHelpPrinter` / `DefaultShortHelpPrinter` directly. Output
   must then match kong's byte for byte, and tests enforce this.
2. Otherwise it swaps `ctx.Stdout` for a `bytes.Buffer`, runs kong's
   default printer, restores stdout, and writes `colourise(buf)`. ANSI codes
   take up no visible columns, so kong's layout survives.
3. **Width gotcha:** kong's `guessWidth` type-asserts `ctx.Stdout` to
   `*os.File`, so it falls back to 80 cols once stdout is a buffer. To
   avoid that, `printWithColour` sets `COLUMNS` from the real terminal
   size for the duration of the call, unless the user already set it. That
   makes it unsafe to call concurrently. On Windows kong ignores `COLUMNS`
   entirely, which is why `TestHelp_HonoursCOLUMNS` skips there.

`colourise` works one line at a time with a fixed list of regex rules,
applied in order: section header (yellow; header lines skip the remaining
rules), usage-line program name (green), flag tokens including the
negatable `--[no-]x` and `--x/no-x` forms (green), `<...>`/`[UPPER...]`
placeholders (cyan), `=UPPER` placeholders (cyan). Rule order and the regex
details are subtle: `[default: ...]` must stay uncoloured, and a flag needs
a leading boundary so it doesn't match inside values. The spec explains the
reasoning behind each rule.

## Tests

- `colours_test.go`: table tests for `colourise` and the individual rules.
- `help_test.go`: end-to-end through `kong.New`/`Parse`, covering the
  NO_COLOR passthrough equivalence, ShortHelp via `kong.Trace`, and
  `COLUMNS`.
- `golden_test.go`: each fixture writes a coloured golden (`FORCE_COLOR=1`)
  and a plain golden (`NO_COLOR=1`) to `testdata/`.
  `TestGolden_StrippedAnsiMatchesPlain` checks that removing ANSI from every
  coloured golden yields its plain twin. When you add a fixture, also add
  its name to that test's `fixtures` list.
- `.gitattributes` marks the goldens `-text` so Windows checkouts keep LF
  line endings. Keep it that way.
- The tests use `t.Setenv` for `NO_COLOR`/`FORCE_COLOR`/`COLUMNS`, so they
  can't use `t.Parallel()`.
- After changing a colour rule, run `-update` and read the golden diff
  before committing it.
