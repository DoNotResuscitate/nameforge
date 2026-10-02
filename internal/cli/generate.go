package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/export"
	"github.com/DoNotResuscitate/nameforge/internal/generator"
)

const generateUsage = `Usage: nameforge generate --category <id> [--category <id> ...] [options]
       nameforge generate --all-categories [options]

Options:
  --category <id>        Repeatable category selection; see "nameforge data list"
  --all-categories      Explicitly select every category; exclusive with --category
  --mode <mode>         category (default) or blend
  --gender <filter>     any (default), masculine, feminine, or unisex
  --count <n>           Number of distinct names, 1..1000 (default 20)
  --order <n>           Markov order, 1..4 (default 2)
  --min-length <n>      Minimum NFC rune length, 1..64 (default observed minimum)
  --max-length <n>      Maximum NFC rune length, 1..64 (default observed maximum)
  --seed <uint64>       Decimal replay seed (default cryptographically random)
  --allow-existing     Permit exact training spellings; batch remains unique
  --format <format>    text (default) or json
  --help, -h           Show this help

Text writes one name per line to stdout and JSON reproduction metadata to stderr.
JSON writes one schema_version=1 result to stdout. Failures write no partial batch.
Category mode chooses each name's category uniformly; blend trains on the union.
Only Latin-script generation is supported; --all-categories currently fails because
Greek and Arabic have no Latin profile. Select supported categories explicitly.
`

func parseGenerate(args []string) (generator.Request, string, error) {
	var request generator.Request
	var mode, gender, seed string
	format := "text"
	flags := flag.NewFlagSet("generate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Func("category", "category ID", func(id string) error {
		request.CategoryIDs = append(request.CategoryIDs, id)
		return nil
	})
	flags.BoolVar(&request.AllCategories, "all-categories", false, "select all")
	flags.StringVar(&mode, "mode", "category", "generation mode")
	flags.StringVar(&gender, "gender", "any", "source gender filter")
	flags.IntVar(&request.Count, "count", 20, "number of names")
	flags.IntVar(&request.Order, "order", 2, "Markov order")
	flags.IntVar(&request.MinLength, "min-length", 0, "minimum rune length")
	flags.IntVar(&request.MaxLength, "max-length", 0, "maximum rune length")
	flags.StringVar(&seed, "seed", "", "unsigned decimal seed")
	flags.BoolVar(&request.AllowExisting, "allow-existing", false, "allow source spellings")
	flags.StringVar(&format, "format", "text", "output format")
	if err := flags.Parse(args); err != nil {
		return request, format, err
	}
	if flags.NArg() != 0 {
		return request, format, fmt.Errorf("unexpected argument %q; use flags for all generation options", flags.Arg(0))
	}
	var optionErr error
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "count", "order", "min-length", "max-length":
			if f.Value.String() == "0" {
				optionErr = fmt.Errorf("--%s must be at least 1 when explicitly set", f.Name)
			}
		case "seed":
			value, err := strconv.ParseUint(seed, 10, 64)
			if err != nil {
				optionErr = fmt.Errorf("--seed must be an unsigned 64-bit decimal integer")
			} else {
				request.Seed = &value
			}
		}
	})
	if optionErr != nil {
		return request, format, optionErr
	}
	if format != "text" && format != "json" {
		return request, format, fmt.Errorf("--format must be text or json")
	}
	if mode == "" || gender == "" {
		return request, format, fmt.Errorf("--mode and --gender must not be empty")
	}
	request.Mode = generator.Mode(mode)
	request.Gender = generator.GenderFilter(gender)
	request, err := generator.NormalizeRequest(request)
	return request, format, err
}

func runGenerate(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	request, format, err := parseGenerate(args)
	if errors.Is(err, flag.ErrHelp) {
		return writeOutput(stdout, stderr, generateUsage)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "generate: %v\nUse 'nameforge generate --help' for options.\n", err)
		return 2
	}
	if ctx.Err() != nil {
		_, _ = fmt.Fprintln(stderr, "canceled: generation interrupted")
		return 130
	}
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load bundled corpus: %v\n", err)
		return 1
	}
	result, err := generator.Generate(ctx, bundle, request)
	if err != nil {
		var typed *generator.GenerationError
		if errors.As(err, &typed) {
			_, _ = fmt.Fprintf(stderr, "%s: %s\n", typed.Kind, typed.Message)
			switch typed.Kind {
			case generator.ErrorInvalidRequest, generator.ErrorIncompatibleBlend:
				return 2
			case generator.ErrorCanceled:
				return 130
			case generator.ErrorAttemptsExhausted:
				_, _ = fmt.Fprintln(stderr, "Try a smaller --count, broader length bounds, a lower --order, or --allow-existing. No partial batch was written.")
			case generator.ErrorEmptySelection:
				_, _ = fmt.Fprintln(stderr, "Inspect source gender coverage with 'nameforge data inspect --category <id>' or use --gender any.")
			case generator.ErrorUnsupportedScript:
				_, _ = fmt.Fprintln(stderr, "Use 'nameforge data list' to choose categories with a Latin script profile.")
			}
		} else {
			_, _ = fmt.Fprintf(stderr, "generate: %v\n", err)
		}
		return 1
	}
	if ctx.Err() != nil {
		_, _ = fmt.Fprintln(stderr, "canceled: generation interrupted")
		return 130
	}
	if format == "json" {
		err = export.WriteJSON(stdout, result)
	} else {
		err = export.WriteMetadata(stderr, result)
		if err == nil {
			err = export.WriteText(stdout, result)
		}
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "write generation output: %v\n", err)
		return 1
	}
	return 0
}
