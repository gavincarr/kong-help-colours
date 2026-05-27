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
| Placeholders (`<FILE>`, `[OPTIONS]`, `=STRING`) | cyan |

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
