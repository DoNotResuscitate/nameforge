package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/tui"
	"golang.org/x/term"
)

const tuiUsage = `Usage: nameforge tui [--no-color] [--data-dir <path>]
       nameforge

Requires terminal stdin and stdout. Built-in data works offline without local state.
--no-color         Disable color and text styling (also NO_COLOR or TERM=dumb)
--data-dir <path>  Reserved local-state path; no local packs/state are loaded yet
Tab focuses categories/search/settings/results; ? shows controls and source labels.
Use "nameforge generate --help" for scriptable output without a terminal.
`

func terminal(stream any) bool {
	fd, ok := stream.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(int(fd.Fd()))
}

func runTUI(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tui", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var noColor bool
	var dataDir string
	flags.BoolVar(&noColor, "no-color", false, "disable styling")
	flags.StringVar(&dataDir, "data-dir", "", "reserved local-state directory")
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return writeOutput(stdout, stderr, tuiUsage)
	} else if err != nil {
		_, _ = fmt.Fprintf(stderr, "tui: %v\n%s", err, tuiUsage)
		return 2
	}
	if flags.NArg() != 0 {
		_, _ = fmt.Fprintf(stderr, "tui: unexpected argument %q\n%s", flags.Arg(0), tuiUsage)
		return 2
	}
	if !terminal(stdin) || !terminal(stdout) {
		_, _ = fmt.Fprint(stderr, tuiUsage)
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "installation error: load bundled corpus: %v\n", err)
		return 1
	}
	_, envNoColor := os.LookupEnv("NO_COLOR")
	err = tui.Run(ctx, bundle, stdin, stdout, noColor || envNoColor || os.Getenv("TERM") == "dumb")
	if ctx.Err() != nil || errors.Is(err, tui.ErrInterrupted) {
		return 130
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tui: %v\n", err)
		return 1
	}
	return 0
}
