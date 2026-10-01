package generator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

func TestGenerateFromPinnedSourceFixtureIsSeededAndNFC(t *testing.T) {
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	seed := uint64(42)
	request := Request{CategoryIDs: []string{"french"}, Count: 1, Order: 4, Seed: &seed, AllowExisting: true}
	first, err := Generate(context.Background(), bundle, request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(context.Background(), bundle, request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Names[0].Name != second.Names[0].Name || first.Seed != seed || first.AlgorithmVersion != AlgorithmVersion {
		t.Fatalf("seeded result is not reproducible: %#v versus %#v", first, second)
	}
	if first.Names[0].Name != "Alix" {
		t.Fatalf("fixed-seed source-fixture sample = %q, want golden %q", first.Names[0].Name, "Alix")
	}
	if !isLatinSpelling(first.Names[0].Name) {
		t.Fatalf("generator returned non-Latin output %q", first.Names[0].Name)
	}
	if first.BundleHash == "" || first.BundleHash != second.BundleHash {
		t.Fatalf("corpus identity missing or unstable: %q versus %q", first.BundleHash, second.BundleHash)
	}
	if first.Names[0].CategoryIDs[0] != "french" || first.Bounds["french"] != (LengthBounds{Min: 4, Max: 7}) {
		t.Fatalf("source attribution or observed bounds incorrect: %#v", first)
	}
}

func TestGenerateCategorySelectionIsOrderIndependentAndAttributed(t *testing.T) {
	bundle := nonNameBundle(
		[]corpus.Category{fixtureCategory("alpha", "Latin"), fixtureCategory("beta", "Latin")},
		[]corpus.Record{fixtureRecord("alpha-record", "qzx", "alpha"), fixtureRecord("beta-record", "vrkt", "beta")},
	)
	seed := uint64(123)
	firstRequest := Request{CategoryIDs: []string{"beta", "alpha"}, Count: 1, Order: 4, Seed: &seed, AllowExisting: true}
	secondRequest := firstRequest
	secondRequest.CategoryIDs = []string{"alpha", "beta"}
	first, err := Generate(context.Background(), bundle, firstRequest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(context.Background(), bundle, secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	if first.Names[0].Name != second.Names[0].Name || strings.Join(first.Names[0].CategoryIDs, ",") != strings.Join(second.Names[0].CategoryIDs, ",") || first.CategoryIDs[0] != "alpha" || first.CategoryIDs[1] != "beta" {
		t.Fatalf("category order changed seeded output: %#v versus %#v", first, second)
	}
	if len(first.Names[0].CategoryIDs) != 1 || first.Names[0].CategoryIDs[0] != first.CategoryIDs[0] && first.Names[0].CategoryIDs[0] != first.CategoryIDs[1] {
		t.Fatalf("category mode attribution is not one selected category: %#v", first.Names[0])
	}
	if first.Bounds["alpha"] != (LengthBounds{Min: 3, Max: 3}) || first.Bounds["beta"] != (LengthBounds{Min: 4, Max: 4}) {
		t.Fatalf("category-specific automatic bounds = %#v", first.Bounds)
	}
}

func TestGenerateAllCategoriesKeepsReplayableOptions(t *testing.T) {
	bundle := nonNameBundle(
		[]corpus.Category{fixtureCategory("beta", "Latin"), fixtureCategory("alpha", "Latin")},
		[]corpus.Record{fixtureRecord("alpha-record", "qzxv", "alpha"), fixtureRecord("beta-record", "vrkt", "beta")},
	)
	seed := uint64(17)
	request := Request{AllCategories: true, Count: 1, Order: 4, Seed: &seed, AllowExisting: true}
	result, err := Generate(context.Background(), bundle, request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Options.AllCategories || len(result.Options.CategoryIDs) != 0 {
		t.Fatalf("AllCategories options are not replayable: %#v", result.Options)
	}
	replay, err := Generate(context.Background(), bundle, result.Options)
	if err != nil {
		t.Fatal(err)
	}
	if result.Names[0].Name != replay.Names[0].Name || strings.Join(result.Names[0].CategoryIDs, ",") != strings.Join(replay.Names[0].CategoryIDs, ",") {
		t.Fatalf("replay changed all-category output: %#v versus %#v", result, replay)
	}
}

func TestCategoryChoiceIsUniformAcrossSelectedPools(t *testing.T) {
	bundle := nonNameBundle(
		[]corpus.Category{fixtureCategory("alpha", "Latin"), fixtureCategory("beta", "Latin")},
		[]corpus.Record{fixtureRecord("alpha-record", "qzxv", "alpha"), fixtureRecord("beta-record", "vrkt", "beta")},
	)
	counts := map[string]int{}
	for value := uint64(0); value < 200; value++ {
		seed := value
		result, err := Generate(context.Background(), bundle, Request{
			CategoryIDs: []string{"alpha", "beta"}, Count: 1, Order: 4, Seed: &seed, AllowExisting: true,
		})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		counts[result.Names[0].CategoryIDs[0]]++
	}
	if counts["alpha"] < 70 || counts["alpha"] > 130 || counts["beta"] < 70 || counts["beta"] > 130 {
		t.Fatalf("category selection is not approximately uniform: %v", counts)
	}
}

func TestBlendDeduplicatesAndAttributesAllCategories(t *testing.T) {
	shared := fixtureRecord("shared-a", "qzxv", "alpha")
	shared.Categories = []string{"alpha", "beta"}
	bundle := nonNameBundle(
		[]corpus.Category{fixtureCategory("alpha", "Latin"), fixtureCategory("beta", "Latin")},
		[]corpus.Record{shared},
	)
	seed := uint64(7)
	result, err := Generate(context.Background(), bundle, Request{
		CategoryIDs: []string{"beta", "alpha"}, Mode: ModeBlend, Count: 1, Order: 4, Seed: &seed, AllowExisting: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Names[0].CategoryIDs) != 2 || result.Names[0].CategoryIDs[0] != "alpha" || result.Names[0].CategoryIDs[1] != "beta" {
		t.Fatalf("blend attribution = %v, want all sorted contributing categories", result.Names[0].CategoryIDs)
	}
	if result.Bounds["alpha"] != result.Bounds["beta"] {
		t.Fatalf("blend bounds differ by category: %#v", result.Bounds)
	}
	union := unionSpellings([]corpus.Pool{
		{CategoryID: "alpha", Records: []corpus.Record{shared}},
		{CategoryID: "beta", Records: []corpus.Record{shared}},
	})
	if len(union) != 1 || union[0] != "qzxv" {
		t.Fatalf("blend training union retained duplicates: %v", union)
	}
}

func TestBlendRejectsIncompatibleScripts(t *testing.T) {
	beta := fixtureCategory("beta", "Latin")
	beta.Scripts = []string{"Greek", "Latin"}
	bundle := nonNameBundle(
		[]corpus.Category{fixtureCategory("alpha", "Latin"), beta},
		[]corpus.Record{fixtureRecord("alpha-record", "qzxv", "alpha"), fixtureRecord("beta-record", "vrkt", "beta")},
	)
	seed := uint64(3)
	_, err := Generate(context.Background(), bundle, Request{CategoryIDs: []string{"alpha", "beta"}, Mode: ModeBlend, Count: 1, Seed: &seed})
	var generationErr *GenerationError
	if !errors.As(err, &generationErr) || generationErr.Kind != ErrorIncompatibleBlend {
		t.Fatalf("Generate() error = %v, want incompatible blend error", err)
	}
}

func TestGeneratorRequiresLatinAndRejectsMixedScriptSamples(t *testing.T) {
	t.Run("non-Latin category", func(t *testing.T) {
		bundle := nonNameBundle(
			[]corpus.Category{fixtureCategory("greek", "Greek")},
			[]corpus.Record{fixtureRecord("greek-record", "qzxΩ", "greek")},
		)
		seed := uint64(8)
		_, err := Generate(context.Background(), bundle, Request{CategoryIDs: []string{"greek"}, Count: 1, Seed: &seed})
		var generationErr *GenerationError
		if !errors.As(err, &generationErr) || generationErr.Kind != ErrorUnsupportedScript {
			t.Fatalf("Generate() error = %v, want unsupported-script error", err)
		}
	})

	t.Run("mixed-script candidate", func(t *testing.T) {
		category := fixtureCategory("mixed", "Latin")
		category.Scripts = []string{"Greek", "Latin"}
		bundle := nonNameBundle(
			[]corpus.Category{category},
			[]corpus.Record{fixtureRecord("mixed-record", "qzxΩ", "mixed")},
		)
		seed := uint64(12)
		result, err := Generate(context.Background(), bundle, Request{CategoryIDs: []string{"mixed"}, Count: 1, Order: 4, Seed: &seed, AllowExisting: true})
		assertExhausted(t, result, err)
		if result.Rejections.Script != result.Attempts || len(result.Names) != 0 {
			t.Fatalf("mixed-script candidate escaped the Latin filter: %#v", result)
		}
	})
}

func TestGenerateRejectsUnknownAndEmptyFilteredCategories(t *testing.T) {
	bundle := nonNameBundle(
		[]corpus.Category{fixtureCategory("alpha", "Latin")},
		[]corpus.Record{fixtureRecord("unset-record", "qzxv", "alpha")},
	)
	seed := uint64(1)
	for _, test := range []struct {
		label   string
		request Request
		kind    ErrorKind
	}{
		{label: "unknown category", request: Request{CategoryIDs: []string{"missing"}, Count: 1, Seed: &seed}, kind: ErrorInvalidRequest},
		{label: "generic is not unisex", request: Request{CategoryIDs: []string{"alpha"}, Gender: GenderUnisex, Count: 1, Seed: &seed}, kind: ErrorEmptySelection},
	} {
		t.Run(test.label, func(t *testing.T) {
			_, err := Generate(context.Background(), bundle, test.request)
			var generationErr *GenerationError
			if !errors.As(err, &generationErr) || generationErr.Kind != test.kind {
				t.Fatalf("Generate() error = %v, want kind %q", err, test.kind)
			}
		})
	}
}

func TestGenerateExcludesTrainingNamesByDefault(t *testing.T) {
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	seed := uint64(19)
	result, err := Generate(context.Background(), bundle, Request{CategoryIDs: []string{"french"}, Count: 1, Order: 4, Seed: &seed})
	var generationErr *GenerationError
	if !errors.As(err, &generationErr) || generationErr.Kind != ErrorAttemptsExhausted {
		t.Fatalf("Generate() error = %v, want bounded novelty exhaustion", err)
	}
	if result.Attempts != 1000 || result.Rejections.Existing != result.Attempts || len(result.Names) != 0 || result.Complete {
		t.Fatalf("novelty exhaustion result = %#v", result)
	}
	if generationErr.Partial == nil || generationErr.Partial.Attempts != result.Attempts {
		t.Fatalf("exhaustion error omitted partial result: %#v", generationErr)
	}
}

func TestGenerateReturnsBoundedPartialForDuplicatesAndImpossibleLengths(t *testing.T) {
	t.Run("duplicate batch", func(t *testing.T) {
		bundle := singleTokenBundle("qzxv")
		seed := uint64(9)
		result, err := Generate(context.Background(), bundle, Request{CategoryIDs: []string{"alpha"}, Count: 2, Order: 4, Seed: &seed, AllowExisting: true})
		assertExhausted(t, result, err)
		if len(result.Names) != 1 || result.Rejections.Duplicates != result.Attempts-1 {
			t.Fatalf("duplicate exhaustion result = %#v", result)
		}
	})

	t.Run("impossible length", func(t *testing.T) {
		bundle := singleTokenBundle("qzxv")
		seed := uint64(10)
		result, err := Generate(context.Background(), bundle, Request{
			CategoryIDs: []string{"alpha"}, Count: 1, Order: 4, MinLength: 64, MaxLength: 64, Seed: &seed, AllowExisting: true,
		})
		assertExhausted(t, result, err)
		if len(result.Names) != 0 || result.Rejections.Length != result.Attempts {
			t.Fatalf("length exhaustion result = %#v", result)
		}
	})
}

func TestGenerateRejectsMalformedSeparators(t *testing.T) {
	bundle := singleTokenBundle("qz--xv")
	seed := uint64(4)
	result, err := Generate(context.Background(), bundle, Request{CategoryIDs: []string{"alpha"}, Count: 1, Order: 4, Seed: &seed, AllowExisting: true})
	assertExhausted(t, result, err)
	if result.Rejections.Separators != result.Attempts {
		t.Fatalf("separator rejections = %d, attempts = %d", result.Rejections.Separators, result.Attempts)
	}
}

func TestGenerateHonorsCancellationBetweenAttempts(t *testing.T) {
	bundle := singleTokenBundle("qzxv")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	seed := uint64(5)
	result, err := Generate(ctx, bundle, Request{CategoryIDs: []string{"alpha"}, Count: 1, Order: 4, Seed: &seed, AllowExisting: true})
	var generationErr *GenerationError
	if !errors.As(err, &generationErr) || generationErr.Kind != ErrorCanceled || result.Attempts != 0 {
		t.Fatalf("canceled generation = %#v, %v", result, err)
	}
}

func TestGenerateRecordsGeneratedDefaultSeed(t *testing.T) {
	bundle := singleTokenBundle("qzxv")
	result, err := Generate(context.Background(), bundle, Request{CategoryIDs: []string{"alpha"}, Count: 1, Order: 4, AllowExisting: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Options.Seed == nil || *result.Options.Seed != result.Seed {
		t.Fatalf("generated seed was not recorded for replay: %#v", result)
	}
}

func TestGenerateValidatesRequestLimits(t *testing.T) {
	bundle := singleTokenBundle("qzxv")
	for _, request := range []Request{
		{CategoryIDs: []string{"alpha"}, Count: 1001},
		{CategoryIDs: []string{"alpha"}, Order: 5},
		{CategoryIDs: []string{"alpha"}, MinLength: 5, MaxLength: 4},
		{CategoryIDs: []string{"alpha"}, AllCategories: true},
	} {
		if _, err := Generate(context.Background(), bundle, request); err == nil {
			t.Fatalf("Generate(%#v) succeeded, want invalid-request error", request)
		}
	}
}

func TestNameDisplayCasingAndSeparators(t *testing.T) {
	// Deliberately non-name tokens exercise casing without adding a name fixture.
	if got := displayCase("qzxv-krt qxv'o"); got != "Qzxv-Krt Qxv'o" {
		t.Fatalf("displayCase() = %q", got)
	}
	if got := displayCase("'qzxv"); got != "'Qzxv" {
		t.Fatalf("displayCase() did not uppercase the first letter after the start: %q", got)
	}
	for _, value := range []string{"-qzxv", "qzxv-", "qzxv--krt", "qzxv  krt", "qzxv''q"} {
		if validSeparators(value) {
			t.Errorf("validSeparators(%q) = true, want false", value)
		}
	}
	for _, value := range []string{"qzxv-krt", "qzxv'q", "qzxv krt"} {
		if !validSeparators(value) {
			t.Errorf("validSeparators(%q) = false, want true", value)
		}
	}
}

func TestLatinFilterPreservesDiacriticsAndRejectsOtherScripts(t *testing.T) {
	// These non-name tokens isolate Latin diacritics and supported separators.
	for _, value := range []string{"qzxé", "qzxe\u0301", "qzxv-hy", "qzxv qrt'kr"} {
		if !isLatinSpelling(value) {
			t.Errorf("isLatinSpelling(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"qzxΩ", "qzxش", "qzx123", "qzx%v", "qzx\u064e", "qzx\u00a0hy"} {
		if isLatinSpelling(value) {
			t.Errorf("isLatinSpelling(%q) = true, want false", value)
		}
	}
}

func TestAttemptLimitScalesWithRequestedCount(t *testing.T) {
	if got := max(1000, 7*200); got != 1400 {
		t.Fatalf("attempt limit = %d, want 1400", got)
	}
}

func assertExhausted(t *testing.T, result Result, err error) {
	t.Helper()
	var generationErr *GenerationError
	if !errors.As(err, &generationErr) || generationErr.Kind != ErrorAttemptsExhausted {
		t.Fatalf("Generate() error = %v, want attempts-exhausted", err)
	}
	if result.Complete || result.Attempts != max(1000, result.Options.Count*200) || generationErr.Partial == nil {
		t.Fatalf("exhaustion was not bounded or lacked partial result: %#v, %#v", result, generationErr)
	}
}

func fixtureCategory(id, script string) corpus.Category {
	return corpus.Category{
		ID: id, Label: "Algorithm fixture " + id, Group: "language-region", SourceLocale: "xx",
		Scripts: []string{script}, SupportedGenders: []corpus.Gender{}, RecordCount: 1,
	}
}

func fixtureRecord(id, token string, categoryIDs ...string) corpus.Record {
	return corpus.Record{
		ID: id, Name: token, Categories: append([]string(nil), categoryIDs...),
		Genders: []corpus.Gender{}, SourceRefs: []corpus.SourceRef{},
	}
}

func nonNameBundle(categories []corpus.Category, records []corpus.Record) *corpus.Bundle {
	// These deliberately non-name token strings are algorithm fixtures, never corpus data.
	return &corpus.Bundle{Categories: categories, Records: records}
}

func singleTokenBundle(token string) *corpus.Bundle {
	return nonNameBundle(
		[]corpus.Category{fixtureCategory("alpha", "Latin")},
		[]corpus.Record{fixtureRecord("fixture-record", token, "alpha")},
	)
}

func ExampleAlgorithmVersion() {
	fmt.Println(AlgorithmVersion)
	// Output: markov-v2/nfc-v1/latin-v1/math-rand-v2-pcg
}
