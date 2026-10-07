package generator

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"golang.org/x/text/unicode/norm"
)

func TestSurnameAndFullBuiltinReplay(t *testing.T) {
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	seed := uint64(42)
	for _, nameType := range []NameType{NameSurname, NameFull} {
		for _, mode := range []Mode{ModeCategory, ModeBlend} {
			for _, category := range bundle.Surnames.Categories {
				t.Run(string(nameType)+"/"+string(mode)+"/"+category.ID, func(t *testing.T) {
					r := Request{NameType: nameType, CategoryIDs: []string{category.ID}, Mode: mode, Seed: &seed}
					result, err := Generate(context.Background(), bundle, r)
					if err != nil {
						t.Fatal(err)
					}
					assertTypedBatch(t, bundle, result)
					replay, err := Generate(context.Background(), bundle, result.Options)
					if err != nil || !reflect.DeepEqual(result, replay) {
						t.Fatalf("replay changed: %v", err)
					}
				})
			}
			r := Request{NameType: nameType, CategoryIDs: []string{"turkish", "italian", "french"}, Mode: mode, Seed: &seed, Gender: GenderFeminine, Surname: nil}
			if nameType == NameFull {
				r.Surname = &ComponentOptions{Order: 1, MinLength: 4, MaxLength: 12}
			}
			first, err := Generate(context.Background(), bundle, r)
			if err != nil {
				t.Fatal(err)
			}
			r.CategoryIDs = []string{"french", "italian", "turkish", "french"}
			second, err := Generate(context.Background(), bundle, r)
			if err != nil || !reflect.DeepEqual(first, second) {
				t.Fatalf("picker order changed: %v", err)
			}
			assertTypedBatch(t, bundle, first)
		}
	}
}

func assertTypedBatch(t *testing.T, bundle *corpus.Bundle, result Result) {
	t.Helper()
	if !result.Complete || len(result.Names) != result.Options.Count || result.NameType != result.Options.NameType || result.Options.Seed == nil || *result.Options.Seed != result.Seed {
		t.Fatal("incomplete reproduction metadata")
	}
	if result.NameType == NameFull && (result.SurnameBundleHash == "" || len(result.SurnameBounds) == 0 || result.ComponentOrder != "given-surname" || result.Separator != " " || result.Options.Surname == nil) {
		t.Fatal("missing composition metadata")
	}
	seen := map[string]bool{}
	for _, name := range result.Names {
		key := spellingKey(name.Name)
		if seen[key] || !isLatinSpelling(name.Name) || !norm.NFC.IsNormalString(name.Name) {
			t.Fatal("not unique NFC Latin output")
		}
		seen[key] = true
		if result.Mode == ModeCategory && len(name.CategoryIDs) != 1 || result.Mode == ModeBlend && !reflect.DeepEqual(name.CategoryIDs, result.CategoryIDs) {
			t.Fatal("incorrect attribution")
		}
		components := []struct {
			spelling string
			records  []corpus.Record
		}{{name.Name, bundle.Surnames.Records}}
		if result.NameType == NameFull {
			if name.Given == nil || name.Surname == nil || name.Name != name.Given.Name+" "+name.Surname.Name || !reflect.DeepEqual(name.Given.CategoryIDs, name.Surname.CategoryIDs) || !reflect.DeepEqual(name.CategoryIDs, name.Given.CategoryIDs) {
				t.Fatal("component pairing lost")
			}
			components = []struct {
				spelling string
				records  []corpus.Record
			}{{name.Given.Name, bundle.Records}, {name.Surname.Name, bundle.Surnames.Records}}
		}
		for _, component := range components {
			for _, record := range component.records {
				selected := false
				for _, id := range result.CategoryIDs {
					if record.Categories[0] == id {
						selected = true
					}
				}
				if selected && spellingKey(component.spelling) == spellingKey(record.Name) {
					t.Fatal("source spelling accepted with novelty enabled")
				}
			}
		}
	}
}

func TestNameTypeMissingDataAndGender(t *testing.T) {
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	seed := uint64(42)
	for _, nameType := range []NameType{NameSurname, NameFull} {
		for _, id := range []string{"greek", "arabic"} {
			_, err := Generate(context.Background(), bundle, Request{NameType: nameType, CategoryIDs: []string{"french", id}, Seed: &seed})
			var typed *GenerationError
			if !errors.As(err, &typed) || typed.Kind != ErrorEmptySelection || typed.CategoryID != id {
				t.Fatalf("missing component: %v", err)
			}
		}
	}
	_, err = Generate(context.Background(), bundle, Request{NameType: NameFull, AllCategories: true, Seed: &seed})
	if err == nil {
		t.Fatal("all categories silently dropped missing surnames")
	}
	all, err := Generate(context.Background(), bundle, Request{NameType: NameSurname, AllCategories: true, Gender: GenderUnisex, Seed: &seed})
	if err != nil || len(all.CategoryIDs) != 8 {
		t.Fatalf("surname unspecified gender excluded: %v", err)
	}
	for _, record := range bundle.Surnames.Records {
		if len(record.Genders) != 0 {
			t.Fatal("generic misclassified as unisex")
		}
	}
	_, err = Generate(context.Background(), bundle, Request{NameType: NameFull, CategoryIDs: []string{"french"}, Gender: GenderUnisex, Seed: &seed})
	if err == nil {
		t.Fatal("full-name given gender ignored")
	}
}

func TestFullNameComponentReuseExhaustionAndCancellation(t *testing.T) {
	// Clearly non-name token sequences, not authored training names.
	bundle := nonNameBundle([]corpus.Category{fixtureCategory("alpha", "Latin")}, []corpus.Record{fixtureRecord("g", "qzx", "alpha")})
	bundle.Records[0].Genders = []corpus.Gender{corpus.GenderFeminine}
	bundle.Surnames = nonNameBundle([]corpus.Category{fixtureCategory("alpha", "Latin")}, []corpus.Record{fixtureRecord("s1", "vkr", "alpha"), fixtureRecord("s2", "twp", "alpha")})
	seed := uint64(42)
	r := Request{NameType: NameFull, CategoryIDs: []string{"alpha"}, Gender: GenderFeminine, Order: 4, Count: 2, AllowExisting: true, Seed: &seed}
	result, err := Generate(context.Background(), bundle, r)
	if err != nil || len(result.Names) != 2 || result.Names[0].Given.Name != result.Names[1].Given.Name {
		t.Fatalf("component reuse failed: %v", err)
	}
	r.Count = 3
	partial, err := Generate(context.Background(), bundle, r)
	var typed *GenerationError
	if !errors.As(err, &typed) || typed.Kind != ErrorAttemptsExhausted || typed.Partial == nil || partial.Complete || len(partial.Names) != 2 || partial.Attempts != 1000 || partial.Rejections.Duplicates == 0 {
		t.Fatalf("bounded uniqueness: %#v %v", partial, err)
	}
	r.NameType, r.Count = NameSurname, 3
	partial, err = Generate(context.Background(), bundle, r)
	if !errors.As(err, &typed) || typed.Kind != ErrorAttemptsExhausted || partial.Attempts != 1000 {
		t.Fatalf("surname exhaustion: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, nameType := range []NameType{NameFull, NameSurname} {
		r.NameType = nameType
		partial, err = Generate(ctx, bundle, r)
		if !errors.As(err, &typed) || typed.Kind != ErrorCanceled || partial.Attempts != 0 {
			t.Fatalf("cancellation: %v", err)
		}
	}
}
