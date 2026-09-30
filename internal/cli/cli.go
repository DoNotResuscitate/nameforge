package cli

import (
	"fmt"
	"io"
)

var (
	version = "dev"
	commit  = "unknown"
)

const usage = `Nameforge generates TTRPG character and NPC names from sourced name lists.

Usage:
  nameforge <command> [options]

Commands:
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
	return (len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help")) ||
		(len(args) > 1 && (args[1] == "--help" || args[1] == "-h"))
}
