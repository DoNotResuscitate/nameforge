package cli

import (
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

func TestAllCategoriesLatinNovelReplayAndAttribution(t *testing.T) {
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	existing := map[string]bool{}
	for _, record := range bundle.Records {
		existing[strings.ToLower(corpus.NormalizeName(record.Name))] = true
	}
	for _, mode := range []string{"category", "blend"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{"generate", "--all-categories", "--mode", mode, "--seed", "42"}
			code, stdout, stderr := runCaptured(t, append(args, "--format", "json")...)
			if code != 0 || stderr != "" {
				t.Fatalf("all-category generation: %d %s", code, stderr)
			}
			result := decodeResult(t, stdout)
			if !result.Complete || len(result.Names) != 20 || len(result.CategoryIDs) != 10 || len(result.Bounds) != 10 || !result.Options.AllCategories {
				t.Fatal("incomplete all-category metadata")
			}
			t.Logf("%s: 20/20 novel Latin-only names, 10 categories, seed 42, attempts %d, rejections %+v", mode, result.Attempts, result.Rejections)
			code, replay, stderr := runCaptured(t, append(args, "--format", "json")...)
			if code != 0 || replay != stdout || stderr != "" {
				t.Fatal("JSON replay changed")
			}
			var text strings.Builder
			seen := map[string]bool{}
			for _, name := range result.Names {
				key := strings.ToLower(corpus.NormalizeName(name.Name))
				if existing[key] || seen[key] {
					t.Fatal("non-novel or duplicate all-category output")
				}
				seen[key] = true
				for _, r := range name.Name {
					if unicode.IsLetter(r) && !unicode.Is(unicode.Latin, r) {
						t.Fatal("non-Latin output")
					}
				}
				if mode == "category" {
					if len(name.CategoryIDs) != 1 {
						t.Fatal("category attribution")
					}
					if _, ok := bundle.Category(name.CategoryIDs[0]); !ok {
						t.Fatal("unknown attributed category")
					}
				} else if !reflect.DeepEqual(name.CategoryIDs, result.CategoryIDs) {
					t.Fatal("blend attribution")
				}
				text.WriteString(name.Name + "\n")
			}
			code, actual, metadata := runCaptured(t, args...)
			if code != 0 || actual != text.String() {
				t.Fatal("text differs from JSON names")
			}
			result.Names = nil
			if !reflect.DeepEqual(decodeResult(t, metadata), result) {
				t.Fatal("text metadata differs")
			}
			code, replay, replayMetadata := runCaptured(t, args...)
			if code != 0 || replay != actual || metadata != replayMetadata {
				t.Fatal("text replay changed")
			}
			explicit := []string{"generate", "--mode", mode, "--seed", "42", "--format", "json"}
			for i := len(bundle.Categories) - 1; i >= 0; i-- {
				explicit = append(explicit, "--category", bundle.Categories[i].ID)
			}
			code, explicitOutput, stderr := runCaptured(t, explicit...)
			if code != 0 || stderr != "" {
				t.Fatal("explicit category selection failed")
			}
			explicitResult := decodeResult(t, explicitOutput)
			if !reflect.DeepEqual(explicitResult.Names, decodeResult(t, stdout).Names) {
				t.Fatal("selection order/all-category flag changed names")
			}
		})
	}
}
