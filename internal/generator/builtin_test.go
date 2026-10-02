package generator

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

// Smoke uses only the licensed embedded arrays and never commits generated names.
func TestBuiltinCategoryGenerationSmoke(t *testing.T) {
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	for _, category := range bundle.Categories {
		t.Run(category.ID, func(t *testing.T) {
			seed := uint64(42)
			request := Request{CategoryIDs: []string{category.ID}, Seed: &seed}
			first, err := Generate(context.Background(), bundle, request)
			if category.ID == "greek" || category.ID == "arabic" {
				var typed *GenerationError
				if !errors.As(err, &typed) || typed.Kind != ErrorUnsupportedScript {
					t.Fatalf("non-Latin category: %v", err)
				}
				t.Logf("%s: explicit %s", category.ID, typed.Kind)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !first.Complete || len(first.Names) != 20 {
				t.Fatalf("default batch incomplete: %#v", first)
			}
			second, err := Generate(context.Background(), bundle, request)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatal("seeded smoke changed across runs")
			}
			for _, name := range first.Names {
				if !isLatinSpelling(name.Name) {
					t.Fatal("non-Latin sample accepted")
				}
			}
			t.Logf("%s: 20/20 novel names, seed 42, order %d, bounds %+v, attempts %d, rejections %+v", category.ID, first.Options.Order, first.Bounds[category.ID], first.Attempts, first.Rejections)
		})
	}
	for _, mode := range []Mode{ModeCategory, ModeBlend} {
		t.Run(string(mode), func(t *testing.T) {
			seed := uint64(42)
			request := Request{CategoryIDs: []string{"french", "italian"}, Mode: mode, Seed: &seed}
			first, err := Generate(context.Background(), bundle, request)
			if err != nil {
				t.Fatal(err)
			}
			request.CategoryIDs = []string{"italian", "french"}
			second, err := Generate(context.Background(), bundle, request)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatal("picker order changed real-corpus output")
			}
		})
	}
}

func BenchmarkGenerate100BuiltinFrench(b *testing.B) {
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		b.Fatal(err)
	}
	seed := uint64(42)
	request := Request{CategoryIDs: []string{"french"}, Count: 100, Seed: &seed}
	b.ResetTimer()
	for range b.N {
		result, err := Generate(context.Background(), bundle, request)
		if err != nil || !result.Complete {
			b.Fatalf("100-distinct-name generation: %v", err)
		}
	}
	b.ReportMetric(931, "source-names")
	b.ReportMetric(100, "names/op")
}
