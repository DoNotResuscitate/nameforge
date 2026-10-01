package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

var (
	version = "dev"
	commit  = "unknown"
)

const usage = `Nameforge generates TTRPG character and NPC names from sourced name lists.

Usage:
  nameforge <command> [options]

Commands:
  data       List and inspect bundled corpus categories
  version    Show version information

Use "nameforge <command> --help" for command-specific help.
`

// Run routes one Nameforge command and returns its process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = io.WriteString(stderr, usage)
		return 2
	}

	if isHelp(args) {
		_, _ = io.WriteString(stdout, usage)
		return 0
	}

	switch args[0] {
	case "data":
		return runData(args[1:], stdout, stderr)
	case "version":
		if len(args) != 1 {
			_, _ = fmt.Fprintf(stderr, "version does not accept arguments\n\n%s", usage)
			return 2
		}
		_, _ = fmt.Fprintf(stdout, "nameforge version %s (commit %s)\n", version, commit)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
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
		_, _ = fmt.Fprintln(writer, "ID\tLABEL\tLOCALE\tRECORDS\tSCRIPTS\tGENDERS")
		categories := append([]corpus.Category(nil), bundle.Categories...)
		sort.Slice(categories, func(i, j int) bool { return categories[i].ID < categories[j].ID })
		for _, category := range categories {
			_, _ = fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%s\t%s\n", category.ID, category.Label,
				category.SourceLocale, category.RecordCount, strings.Join(category.Scripts, ","), formatGenders(category.SupportedGenders))
		}
		_ = writer.Flush()
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
		_, _ = fmt.Fprintf(stdout, "ID: %s\nLabel: %s\nGroup: %s\nSource locale: %s\nRecords: %d\nScripts: %s\nSupported genders: %s\n",
			category.ID, category.Label, category.Group, category.SourceLocale, category.RecordCount,
			strings.Join(category.Scripts, ", "), formatGenders(category.SupportedGenders))
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
