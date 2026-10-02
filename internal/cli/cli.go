package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/legal"
)

var (
	version = "dev"
	commit  = "unknown"
)

const usage = `Nameforge generates TTRPG character and NPC names from sourced name lists.

Usage:
  nameforge <command> [options]

Commands:
  tui        Open the offline interactive category picker (default with a TTY)
  generate   Generate reproducible names from selected bundled categories
  data       List and inspect bundled corpus categories
  licenses   Display GPL, dependency and corpus license notices
  version    Show version information

Use "nameforge <command> --help" for command-specific help.
`

// Run routes one Nameforge command and returns its process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	return RunContext(context.Background(), args, stdout, stderr)
}

// RunContext propagates process cancellation into generation.
func RunContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	return RunWithInput(ctx, args, os.Stdin, stdout, stderr)
}

// RunWithInput keeps terminal detection and input explicit for frontends/tests.
func RunWithInput(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		if terminal(stdin) && terminal(stdout) {
			return runTUI(ctx, nil, stdin, stdout, stderr)
		}
		_, _ = io.WriteString(stderr, usage)
		return 2
	}

	if len(args) == 1 && isHelp(args) {
		return writeOutput(stdout, stderr, usage)
	}

	switch args[0] {
	case "tui":
		return runTUI(ctx, args[1:], stdin, stdout, stderr)
	case "generate":
		return runGenerate(ctx, args[1:], stdout, stderr)
	case "data":
		if isHelp(args[1:]) {
			return writeOutput(stdout, stderr, "Usage: nameforge data list\n       nameforge data inspect --category <id>\n")
		}
		return runData(args[1:], stdout, stderr)
	case "licenses":
		if isHelp(args[1:]) {
			return writeOutput(stdout, stderr, "Usage: nameforge licenses\nDisplay complete embedded GPL, dependency and corpus license notices.\n")
		}
		if len(args) != 1 {
			_, _ = fmt.Fprintln(stderr, "licenses does not accept arguments")
			return 2
		}
		return runLicenses(stdout, stderr)
	case "version":
		if isHelp(args[1:]) {
			return writeOutput(stdout, stderr, usage)
		}
		if len(args) != 1 {
			_, _ = fmt.Fprintf(stderr, "version does not accept arguments\n\n%s", usage)
			return 2
		}
		return writeOutput(stdout, stderr, fmt.Sprintf("nameforge version %s (commit %s)\n", version, commit))
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func writeOutput(stdout, stderr io.Writer, text string) int {
	if _, err := io.WriteString(stdout, text); err != nil {
		_, _ = fmt.Fprintf(stderr, "write output: %v\n", err)
		return 1
	}
	return 0
}

func runLicenses(stdout, stderr io.Writer) int {
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load bundled corpus: %v\n", err)
		return 1
	}
	output, err := legal.Text(bundle)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return writeOutput(stdout, stderr, output)
}

func isHelp(args []string) bool {
	for i, arg := range args {
		if arg == "--help" || arg == "-h" || (i == 0 && arg == "help") {
			return true
		}
	}
	return false
}

func runData(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "data requires list or inspect")
		return 2
	}
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load bundled corpus: %v\n", err)
		return 1
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			_, _ = fmt.Fprintln(stderr, "data list does not accept arguments")
			return 2
		}
		writer := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(writer, "ID\tLABEL\tLOCALE\tRECORDS\tSCRIPTS\tGENDERS"); err != nil {
			_, _ = fmt.Fprintf(stderr, "write data list: %v\n", err)
			return 1
		}
		categories := append([]corpus.Category(nil), bundle.Categories...)
		sort.Slice(categories, func(i, j int) bool { return categories[i].ID < categories[j].ID })
		for _, category := range categories {
			if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%s\t%s\n", category.ID, category.Label,
				category.SourceLocale, category.RecordCount, strings.Join(category.Scripts, ","), formatGenders(category.SupportedGenders)); err != nil {
				_, _ = fmt.Fprintf(stderr, "write data list: %v\n", err)
				return 1
			}
		}
		if err := writer.Flush(); err != nil {
			_, _ = fmt.Fprintf(stderr, "write data list: %v\n", err)
			return 1
		}
		return 0
	case "inspect":
		if len(args) != 3 || args[1] != "--category" || args[2] == "" {
			_, _ = fmt.Fprintln(stderr, "usage: nameforge data inspect --category <id>")
			return 2
		}
		category, ok := bundle.Category(args[2])
		if !ok {
			_, _ = fmt.Fprintf(stderr, "unknown category %q\n", args[2])
			return 2
		}
		if _, err := fmt.Fprintf(stdout, "ID: %s\nLabel: %s\nGroup: %s\nSource locale: %s\nRecords: %d\nScripts: %s\nSupported genders: %s\n",
			category.ID, category.Label, category.Group, category.SourceLocale, category.RecordCount,
			strings.Join(category.Scripts, ", "), formatGenders(category.SupportedGenders)); err != nil {
			_, _ = fmt.Fprintf(stderr, "write data inspect: %v\n", err)
			return 1
		}
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown data command %q\n", args[0])
		return 2
	}
}

func formatGenders(genders []corpus.Gender) string {
	if len(genders) == 0 {
		return "unspecified"
	}
	values := make([]string, len(genders))
	for i, gender := range genders {
		values[i] = string(gender)
	}
	return strings.Join(values, ",")
}
