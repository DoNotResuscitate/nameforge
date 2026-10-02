package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/export"
	"github.com/DoNotResuscitate/nameforge/internal/generator"
)

func runCaptured(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	if strings.Contains(stdout.String()+stderr.String(), "\x1b") {
		t.Fatal("machine output contains ANSI escapes")
	}
	return code, stdout.String(), stderr.String()
}

func decodeResult(t *testing.T, text string) export.Result {
	t.Helper()
	var result export.Result
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatalf("expected exactly one JSON object, got %v", err)
	}
	return result
}

func TestGenerateReplayAcrossModesAndFormats(t *testing.T) {
	for _, mode := range []string{"category", "blend"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{"generate", "--category", "french", "--category", "italian", "--mode", mode, "--seed", "42"}
			code, stdout, stderr := runCaptured(t, append(args, "--format", "json")...)
			if code != 0 || stderr != "" {
				t.Fatalf("JSON failed: code=%d stderr=%s", code, stderr)
			}
			result := decodeResult(t, stdout)
			if result.SchemaVersion != 1 || !result.Complete || len(result.Names) != 20 || result.Seed != 42 || result.Options.Seed == nil || *result.Options.Seed != 42 {
				t.Fatalf("missing default/replay metadata: %+v", result)
			}
			if result.AlgorithmVersion != generator.AlgorithmVersion || len(result.BundleHash) != 64 || len(result.Bounds) != 2 || result.Attempts < 20 {
				t.Fatalf("missing model metadata: %+v", result)
			}
			if !reflect.DeepEqual(result.CategoryIDs, []string{"french", "italian"}) {
				t.Fatal("unsorted category attribution")
			}
			reordered := []string{"generate", "--category", "italian", "--category", "french", "--category", "french", "--mode", mode, "--seed", "42", "--format", "json"}
			code, replay, stderr := runCaptured(t, reordered...)
			if code != 0 || stderr != "" || replay != stdout {
				t.Fatalf("seeded bytes changed with picker order/duplicate IDs: %d %s", code, stderr)
			}
			code, text, metadata := runCaptured(t, args...)
			if code != 0 {
				t.Fatalf("text failed: %s", metadata)
			}
			var expected strings.Builder
			for _, name := range result.Names {
				fmt.Fprintln(&expected, name.Name)
				if mode == "category" && len(name.CategoryIDs) != 1 || mode == "blend" && !reflect.DeepEqual(name.CategoryIDs, result.CategoryIDs) {
					t.Fatalf("wrong attribution in %s", mode)
				}
			}
			if text != expected.String() {
				t.Fatal("text stdout contains metadata or differs from JSON names")
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(metadata), &fields); err != nil {
				t.Fatal(err)
			}
			if _, ok := fields["names"]; ok {
				t.Fatal("text metadata must omit names")
			}
			result.Names = nil
			if got := decodeResult(t, metadata); !reflect.DeepEqual(got, result) {
				t.Fatalf("text and JSON reproduction metadata differ: %+v", got)
			}
		})
	}
}

func TestGenerateInvalidOptions(t *testing.T) {
	tests := [][]string{
		{}, {"--bogus"}, {"--category"}, {"--category", ""}, {"--category", " "},
		{"--category", "french", "--all-categories"},
		{"--count", "0"}, {"--count", "-1"}, {"--count", "1001"}, {"--count", "abc"},
		{"--order", "0"}, {"--order", "5"},
		{"--min-length", "0"}, {"--max-length", "0"}, {"--min-length", "-1"}, {"--max-length", "65"},
		{"--min-length", "10", "--max-length", "2"},
		{"--seed", ""}, {"--seed", "-1"}, {"--seed", "18446744073709551616"}, {"--seed", "0x2a"},
		{"--mode", "invalid"}, {"--mode", ""}, {"--gender", "invalid"}, {"--gender", ""},
		{"--format", "invalid"}, {"--format", ""}, {"positional"}, {"--", "extra"},
	}
	for _, options := range tests {
		t.Run(strings.Join(options, " "), func(t *testing.T) {
			// A canceled context proves malformed options take precedence over work.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			args := append([]string{"generate", "--category", "french"}, options...)
			if len(options) == 0 {
				args = []string{"generate"}
			}
			var stdout, stderr bytes.Buffer
			if code := RunContext(ctx, args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.Len() == 0 {
				t.Fatalf("invalid options: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestGenerateSeedAndExplicitOptions(t *testing.T) {
	for _, seed := range []string{"0", "18446744073709551615"} {
		code, stdout, stderr := runCaptured(t, "generate", "--category=french", "--format=json", "--seed="+seed,
			"--gender=feminine", "--mode=blend", "--count=3", "--order=1", "--min-length=3", "--max-length=8", "--allow-existing")
		if code != 0 || stderr != "" {
			t.Fatalf("explicit options failed: %d %s", code, stderr)
		}
		result := decodeResult(t, stdout)
		if fmt.Sprint(result.Seed) != seed || result.Options.Gender != generator.GenderFeminine || !result.Options.AllowExisting || result.Options.Order != 1 || len(result.Names) != 3 || result.Bounds["french"] != (generator.LengthBounds{Min: 3, Max: 8}) {
			t.Fatalf("options not retained: %+v", result)
		}
	}
	code, stdout, stderr := runCaptured(t, "generate", "--category", "turkish", "--format", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("random seed failed: %d %s", code, stderr)
	}
	result := decodeResult(t, stdout)
	if result.Options.Seed == nil || *result.Options.Seed != result.Seed {
		t.Fatal("actual random seed is not replayable")
	}
	code, replay, stderr := runCaptured(t, "generate", "--category", "turkish", "--format", "json", "--seed", fmt.Sprint(result.Seed))
	if code != 0 || stderr != "" || stdout != replay {
		t.Fatal("random-seed batch failed replay")
	}
}

func TestGenerateBuiltinCategorySmoke(t *testing.T) {
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range bundle.Categories {
		t.Run(category.ID, func(t *testing.T) {
			code, stdout, stderr := runCaptured(t, "generate", "--category", category.ID, "--seed", "42", "--format", "json")
			if code != 0 || stderr != "" {
				t.Fatalf("category failed: %d %s", code, stderr)
			}
			result := decodeResult(t, stdout)
			seen := map[string]bool{}
			existing := map[string]bool{}
			for _, record := range bundle.Records {
				if reflect.DeepEqual(record.Categories, []string{category.ID}) {
					existing[strings.ToLower(corpus.NormalizeName(record.Name))] = true
				}
			}
			for _, name := range result.Names {
				key := strings.ToLower(corpus.NormalizeName(name.Name))
				if seen[key] {
					t.Fatal("duplicate output")
				}
				seen[key] = true
				if existing[key] {
					t.Fatal("existing spelling accepted by default")
				}
			}
			if !result.Complete || len(seen) != 20 {
				t.Fatal("default batch incomplete")
			}
		})
	}
}

func TestGenerateFailuresDoNotWritePartialStdout(t *testing.T) {
	tests := []struct {
		options []string
		code    int
		message string
	}{
		{[]string{"--category", "missing"}, 2, `unknown category "missing"`},
		{[]string{"--category", "french", "--gender", "unisex"}, 1, "empty_selection"},
		{[]string{"--category", "greek", "--gender", "masculine"}, 1, "empty_selection"},
		{[]string{"--category", "french", "--min-length", "64"}, 2, "effective length bounds"},
		{[]string{"--category", "french", "--order", "4", "--count", "20", "--min-length", "3", "--max-length", "3", "--allow-existing"}, 1, "Try a smaller --count"},
	}
	for _, tt := range tests {
		for _, format := range []string{"json", "text"} {
			t.Run(strings.Join(tt.options, " ")+"/"+format, func(t *testing.T) {
				args := append([]string{"generate", "--format", format, "--seed", "42"}, tt.options...)
				code, stdout, stderr := runCaptured(t, args...)
				if code != tt.code || stdout != "" || !strings.Contains(stderr, tt.message) {
					t.Fatalf("failure: code=%d stdout=%q stderr=%q", code, stdout, stderr)
				}
				if strings.Contains(stderr, "attempts_exhausted") && strings.Contains(stderr, "producing 0 of") {
					t.Fatal("exhaustion case must exercise suppression of a nonempty partial batch")
				}
			})
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("test write failure") }

func TestGenerateCancellationAndIO(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	args := []string{"generate", "--category", "french", "--seed", "42"}
	if code := RunContext(ctx, args, &stdout, &stderr); code != 130 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "canceled") {
		t.Fatalf("canceled generation: %d %q %q", code, stdout.String(), stderr.String())
	}
	for _, format := range []string{"text", "json"} {
		stderr.Reset()
		if code := Run(append(args, "--format", format), failingWriter{}, &stderr); code != 1 || !strings.Contains(stderr.String(), "write generation output") {
			t.Fatalf("stdout write failure: %d %q", code, stderr.String())
		}
	}
	stdout.Reset()
	if code := Run(args, &stdout, failingWriter{}); code != 1 || stdout.Len() != 0 {
		t.Fatalf("stderr write failure: %d %q", code, stdout.String())
	}
}

func TestLicensesCompleteAndHelp(t *testing.T) {
	code, stdout, stderr := runCaptured(t, "licenses")
	for _, path := range []string{"FAKER-LICENSE", "WIKIMEDIA-NOTICE", "CC0-1.0", "CC-BY-SA-4.0"} {
		notice, err := fs.ReadFile(corpus.BuiltinFS(), "assets/builtin/licenses/"+path)
		if err != nil {
			t.Fatal(err)
		}
		if code != 0 || stderr != "" || !strings.Contains(stdout, string(notice)) {
			t.Fatalf("licenses omitted or changed notice bytes: %s", path)
		}
	}
	for _, args := range [][]string{{"generate", "--help"}, {"generate", "-h"}, {"licenses", "--help"}, {"data", "--help"}} {
		code, stdout, stderr := runCaptured(t, args...)
		if code != 0 || stderr != "" || !strings.Contains(stdout, "Usage: nameforge "+args[0]) {
			t.Fatalf("help: %d %q %q", code, stdout, stderr)
		}
	}
	stderrBuffer := new(bytes.Buffer)
	if code := Run([]string{"licenses"}, failingWriter{}, stderrBuffer); code != 1 {
		t.Fatalf("license write failure: %d", code)
	}
}
