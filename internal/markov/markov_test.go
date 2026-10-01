package markov

import (
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

func TestTrainCountsWeightedRuneTransitions(t *testing.T) {
	// These are deliberately non-name algorithm tokens, not training names.
	model, err := Train([]string{"<fixture:alpha>", "<fixture:beta>", "<fixture:alpha>"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	start := model.transitions[contextKey([]int32{startToken}, 1)]
	if len(start) != 1 || start[0].token != '<' || start[0].weight != 2 {
		t.Fatalf("start transitions = %#v, want one '<' transition weighted by two distinct spellings", start)
	}
	common := model.transitions[contextKey([]int32{'i'}, 1)]
	if len(common) != 1 || common[0].token != 'x' || common[0].weight != 2 {
		t.Fatalf("weighted common transition = %#v, want x:2", common)
	}
}

func TestSampleFallsBackToShorterSuffixContext(t *testing.T) {
	// This is a non-name token fixture. The two-rune context x< is absent, but
	// its one-rune suffix < has the trained transition to f.
	model, err := Train([]string{"<fixture>"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	token, err := model.nextToken([]int32{'x', '<'}, rand.New(rand.NewPCG(1, 2)))
	if err != nil {
		t.Fatal(err)
	}
	if token != 'f' {
		t.Fatalf("backoff token = %q, want 'f'", rune(token))
	}
}

func TestSampleRuneLimitDoesNotTruncate(t *testing.T) {
	// This is a non-name fixture used only to exercise the sampler boundary.
	model, err := Train([]string{"<fixture:token>"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := model.Sample(rand.New(rand.NewPCG(7, 11)), 3)
	if !errors.Is(err, ErrTooLong) {
		t.Fatalf("Sample() error = %v, want ErrTooLong (partial %q)", err, sample)
	}
	if sample != "<fi" {
		t.Fatalf("partial sample = %q, want first three runes", sample)
	}
}

func TestTrainNormalizesUnicodeAndSeededSampleIsStable(t *testing.T) {
	// The decomposed/composed strings are Unicode tokens, not name examples.
	model, err := Train([]string{"e\u0301x", "éx"}, 4)
	if err != nil {
		t.Fatal(err)
	}
	first, err := model.Sample(rand.New(rand.NewPCG(42, 99)), 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := model.Sample(rand.New(rand.NewPCG(42, 99)), 2)
	if err != nil {
		t.Fatal(err)
	}
	if first != "éx" || second != first {
		t.Fatalf("seeded samples = %q and %q, want stable NFC spelling %q", first, second, "éx")
	}
}

func TestTrainValidatesOrderAndSpellings(t *testing.T) {
	for _, test := range []struct {
		label     string
		spellings []string
		order     int
	}{
		{label: "empty", order: 2},
		{label: "invalid order", spellings: []string{"<fixture>"}, order: 5},
		{label: "empty spelling", spellings: []string{"  "}, order: 2},
		{label: "control", spellings: []string{"fixture\nvalue"}, order: 2},
		{label: "invalid utf8", spellings: []string{string([]byte{0xff})}, order: 2},
	} {
		t.Run(test.label, func(t *testing.T) {
			if _, err := Train(test.spellings, test.order); err == nil {
				t.Fatal("Train() succeeded, want validation error")
			}
		})
	}
}

func TestSampleRequiresInitializedInputs(t *testing.T) {
	var model *Model
	if _, err := model.Sample(rand.New(rand.NewPCG(1, 2)), 1); err == nil {
		t.Fatal("nil model sampled without an error")
	}
	model, err := Train([]string{"<fixture>"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := model.Sample(nil, 1); err == nil {
		t.Fatal("nil RNG sampled without an error")
	}
	if _, err := model.Sample(rand.New(rand.NewPCG(1, 2)), 0); err == nil {
		t.Fatal("zero maximum length sampled without an error")
	}
}

func TestModelConcurrentSampling(t *testing.T) {
	// A single non-name fixture model is shared while each goroutine owns its RNG.
	model, err := Train([]string{"<fixture:alpha>", "<fixture:beta>"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	errors := make(chan error, 8)
	for i := range 8 {
		go func(seed uint64) {
			rng := rand.New(rand.NewPCG(seed, seed+1))
			for range 50 {
				if _, err := model.Sample(rng, 32); err != nil {
					errors <- err
					return
				}
			}
			errors <- nil
		}(uint64(i))
	}
	for range 8 {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
}

func BenchmarkTrainBuiltinSchemaFixture(b *testing.B) {
	spellings := benchmarkBuiltinSpellings(b)
	b.ResetTimer()
	for range b.N {
		if _, err := Train(spellings, 2); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(spellings)), "source-names")
}

func BenchmarkSample100BuiltinSchemaFixture(b *testing.B) {
	spellings := benchmarkBuiltinSpellings(b)
	model, err := Train(spellings, 2)
	if err != nil {
		b.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(42, 99))
	b.ResetTimer()
	for range b.N {
		for sampled, attempts := 0, 0; sampled < 100; attempts++ {
			if attempts >= 20000 {
				b.Fatal("100-sample benchmark exceeded its bounded retry limit")
			}
			if _, err := model.Sample(rng, MaxNameRunes); err != nil {
				if errors.Is(err, ErrTooLong) {
					continue
				}
				b.Fatal(err)
			}
			sampled++
		}
	}
	b.ReportMetric(float64(len(spellings)), "source-names")
	b.ReportMetric(100, "samples/op")
}

func benchmarkBuiltinSpellings(b *testing.B) []string {
	b.Helper()
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		b.Fatal(err)
	}
	spellings := make([]string, 0, len(bundle.Records))
	for _, record := range bundle.Records {
		spellings = append(spellings, record.Name)
	}
	return spellings
}

func TestNormalizeSpellingsIsInputOrderIndependent(t *testing.T) {
	first, err := normalizeSpellings([]string{"<fixture:b>", "<fixture:a>"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := normalizeSpellings([]string{"<fixture:a>", "<fixture:b>"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(first, "|") != strings.Join(second, "|") {
		t.Fatalf("normalized inputs differ by order: %v != %v", first, second)
	}
}

func TestTrainingAndSamplingAreIndependentOfInputOrder(t *testing.T) {
	// Reordered non-name tokens must yield the same seeded sample sequence.
	first, err := Train([]string{"<fixture:alpha>", "<fixture:beta>"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Train([]string{"<fixture:beta>", "<fixture:alpha>"}, 2)
	if err != nil {
		t.Fatal(err)
	}
	firstRNG := rand.New(rand.NewPCG(72, 81))
	secondRNG := rand.New(rand.NewPCG(72, 81))
	for i := range 20 {
		firstSample, firstErr := first.Sample(firstRNG, 32)
		secondSample, secondErr := second.Sample(secondRNG, 32)
		if firstErr != nil || secondErr != nil || firstSample != secondSample {
			t.Fatalf("sample %d depends on training order: %q (%v) != %q (%v)", i, firstSample, firstErr, secondSample, secondErr)
		}
	}
}
