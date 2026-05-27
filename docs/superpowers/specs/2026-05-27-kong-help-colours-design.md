# kong-help-colours — design

Date: 2026-05-27
Status: approved (brainstorming)

## Goal

Add colourised `--help` output to CLIs built with
[`github.com/alecthomas/kong`](https://github.com/alecthomas/kong),
delivered as a standalone module that drops in alongside kong — no fork,
no patches.

The target look is the one clap/Rust CLIs ship with by default
(reference: `csv_cut -h` from the `csv_rstk` package). Three colours,
applied to a few fixed token classes, no configuration knobs.

## Non-goals

- Custom colour schemes or per-element opt-in/out.
- 256-colour or truecolour palettes (basic 8-colour ANSI only — works
  everywhere).
- Reimplementing kong's help layout (two-column wrapping, group titles,
  command trees, compact mode, etc.).
- Tracking kong internals across versions beyond the public
  `HelpPrinter` / `HelpOptions` / `Context` surface.

## Approach

`kong.Help(HelpPrinter)` is the official extension point — a
`HelpPrinter` is `func(HelpOptions, *Context) error`, and the printer
writes to `ctx.Stdout`.

Rather than reimplement the layout, the package **wraps the default
printer and post-processes its output**:

1. Decide whether colour is enabled (see "Colour gating" below).
   If not, delegate straight through to `kong.DefaultHelpPrinter` and
   return.
2. Save `ctx.Stdout`, replace it with a `bytes.Buffer`.
3. Call `kong.DefaultHelpPrinter(options, ctx)` — kong does the
   wrapping, alignment, and group handling.
4. Restore `ctx.Stdout`.
5. Run the buffered text through the colourisation pipeline (regex
   substitutions, see below).
6. Write the colourised text to the original `ctx.Stdout`.

ANSI escape sequences add bytes but no visible columns, so kong's
column alignment is unaffected by colourisation that happens after
layout.

The same wrapping pattern is used for `ShortHelp`, delegating to
`kong.DefaultShortHelpPrinter`.

## Public API

```go
package helpcolours // import "github.com/gavincarr/kong-help-colours"

// Help is a kong.HelpPrinter that delegates to kong.DefaultHelpPrinter
// and colourises the output when stdout is a TTY (or FORCE_COLOR is set)
// and NO_COLOR is unset.
var Help kong.HelpPrinter

// ShortHelp is the equivalent wrapper around kong.DefaultShortHelpPrinter,
// for use with kong.ShortHelp(...).
var ShortHelp kong.HelpPrinter
```

Usage:

```go
import (
    "github.com/alecthomas/kong"
    helpcolours "github.com/gavincarr/kong-help-colours"
)

kong.Parse(&cli,
    kong.Help(helpcolours.Help),
    kong.ShortHelp(helpcolours.ShortHelp), // optional
)
```

No configuration types, no constructors, no options — the package
exports two `kong.HelpPrinter` values and nothing else.

## Colour scheme

Verified against `csv_cut -h` (run with a PTY via
`script -qc 'csv_cut -h' /dev/null`) to mirror clap's defaults exactly:

| Element                          | ANSI       | Visible           | Examples                              |
| -------------------------------- | ---------- | ----------------- | ------------------------------------- |
| Section headers                  | `\e[33m`   | yellow            | `Usage:`, `Arguments:`, `Flags:`, `Commands:`, custom group titles |
| Program name + flag names        | `\e[32m`   | green             | `csv_cut`, `--fields`, `-f`           |
| Placeholders                     | `\e[36m`   | cyan              | `<FIELDS>`, `[CSVFile]`, `[OPTIONS]`  |

Reset is `\e[0m` after each coloured span.

Only these three colours are used. The choice of which token gets
which colour is fixed.

## Colourisation pipeline

The buffered help text is processed once, line by line, applying the
following rules in order. Each rule wraps its match in
`\e[<code>m...\e[0m`.

1. **Section header line.** A line whose trimmed content matches
   `^[A-Z][A-Za-z0-9 ]*:$` → wrap the whole line content in yellow.
   This covers kong's built-in headers (`Usage:`, `Arguments:`,
   `Flags:`, `Commands:`) and user-defined group titles.
2. **Usage line program name.** On a line beginning `Usage: `, colour
   the first whitespace-delimited token after `Usage: ` in green.
   (The `Usage:` label itself is coloured by rule 1.)
3. **Flag tokens.** Match `(?:^|[\s,=])(--?[A-Za-z][A-Za-z0-9-]*)` and
   wrap the captured flag in green. Anchoring on a leading boundary
   prevents matches inside placeholders or default values.
4. **Placeholders.** Match `<[^>]+>` and `\[[A-Z][^\]]*\]` and wrap in
   cyan. The uppercase-leading character constraint on `[...]` avoids
   colouring kong's `[default: ...]` annotations.

Rules 3 and 4 are applied to all non-header lines (header lines are
already wrapped in yellow by rule 1 and skipped from further
processing).

Edge cases the pipeline is *not* trying to handle:

- Arbitrary user-written help strings that happen to contain
  `--foo`-shaped text will be coloured as flags. Acceptable —
  consistent with how clap behaves.
- Multi-byte runes in flag names are not supported (matches kong's
  own validation — flag names are ASCII).
- Colour codes that span wrapped lines: kong wraps before we
  colourise, and every wrapped fragment is processed independently,
  so there are no torn escape sequences.

## Colour gating

Colour is enabled iff **all** of the following hold:

- `NO_COLOR` env var is **not** set (https://no-color.org).
- One of:
  - `FORCE_COLOR` env var is set (any non-empty value), **or**
  - `ctx.Stdout` is a TTY.

TTY detection: type-assert `ctx.Stdout` to `*os.File`; if it succeeds,
call `golang.org/x/term.IsTerminal(int(f.Fd()))`. If the assertion
fails (the user has wired stdout to a non-file writer), treat it as
not-a-TTY — `FORCE_COLOR` is the escape hatch for that case.

When colour is disabled the package delegates directly to the
corresponding `kong.Default*HelpPrinter` and returns. No buffer, no
regex work, zero overhead beyond a function call.

## Dependencies

- `github.com/alecthomas/kong` (the library we extend)
- `golang.org/x/term` (TTY detection only)

Standard library otherwise.

## Module layout

```
go.mod
go.sum
colours.go            # Help, ShortHelp, internal colourise() and gating
colours_test.go       # unit tests
example_test.go       # runnable godoc example
README.md
cmd/demo/main.go      # small kong app for visual confirmation
```

`colours.go` is the only non-test source file. Helpers (`colourise`,
`shouldColour`, regex vars) are unexported.

## Testing strategy

- **Pipeline unit tests** in `colours_test.go`: drive `colourise()`
  with hand-crafted help-shaped strings and assert exact ANSI output.
  Cover each rule plus the "header lines are skipped from rule 3/4"
  case.
- **Gating unit tests**: assert that `shouldColour` returns the right
  decision for the matrix of (NO_COLOR set/unset) × (FORCE_COLOR
  set/unset) × (stdout is TTY/not). Inject env via `t.Setenv` and use
  a `*os.File` from `os.Pipe()` for the non-TTY case.
- **End-to-end test**: build a tiny `kong.New(&struct{ ... }{})` CLI
  inside the test, call `Help` with `ctx.Stdout` pointing at a
  `bytes.Buffer`, set `FORCE_COLOR=1`, and assert the output contains
  the expected ANSI sequences for known section headers and flags.
  Also assert that the same call with `NO_COLOR=1` produces output
  identical to `kong.DefaultHelpPrinter`.
- **`cmd/demo`**: not automated — exists so the developer can run
  `go run ./cmd/demo --help` and eyeball the result against `csv_cut -h`.

## Open risks

- **kong upstream changes to header text.** If kong renames `Flags:`
  to e.g. `Options:`, the section-header regex still matches (it's
  generic), so no breakage. If kong starts emitting headers with
  leading indentation, the regex would need adjustment — covered by
  tests against kong's actual output.
- **User group titles that don't end in `:`.** They wouldn't be
  coloured as headers. Acceptable; kong's own built-in titles all end
  in `:` and most user titles follow suit.
- **Help text containing literal text shaped like flags or
  placeholders.** Will be coloured. This is what clap does too.
