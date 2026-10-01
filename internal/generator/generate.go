package generator

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/markov"
	"golang.org/x/text/unicode/norm"
)

const (
	// AlgorithmVersion changes whenever seeded output behavior intentionally changes.
	AlgorithmVersion = "markov-v2/nfc-v1/latin-v1/math-rand-v2-pcg"
	defaultCount     = 20
	defaultOrder     = 2
	maxCount         = 1000
	seedXOR          = uint64(0x9e3779b97f4a7c15)
)

type categoryModel struct {
	id     string
	model  *markov.Model
	bounds LengthBounds
}

// Generate filters a validated corpus bundle, trains the selected model or
// models, and samples a bounded batch. It has no filesystem, terminal, or
// network dependencies. A nil Request.Seed is generated cryptographically and
// recorded in both Result.Seed and Result.Options.Seed.
func Generate(ctx context.Context, bundle *corpus.Bundle, request Request) (Result, error) {
	if ctx == nil {
		return Result{}, generationError(ErrorInvalidRequest, "context must not be nil", "", nil)
	}
	if bundle == nil {
		return Result{}, generationError(ErrorInvalidRequest, "corpus bundle must not be nil", "", nil)
	}

	request, categoryIDs, err := normalizeRequest(bundle, request)
	if err != nil {
		return Result{}, err
	}
	seed, err := resolveSeed(request.Seed)
	if err != nil {
		return Result{}, generationError(ErrorInvalidRequest, "obtain random seed: "+err.Error(), "", nil)
	}
	request.Seed = new(uint64)
	*request.Seed = seed
	result := Result{
		Seed:             seed,
		AlgorithmVersion: AlgorithmVersion,
		CategoryIDs:      append([]string(nil), categoryIDs...),
		Mode:             request.Mode,
		Options:          cloneRequest(request),
		Names:            make([]GeneratedName, 0, request.Count),
		Bounds:           make(map[string]LengthBounds, len(categoryIDs)),
	}
	result.BundleHash, err = corpus.BundleHash(bundle.Records, bundle.Categories)
	if err != nil {
		return result, generationError(ErrorInvalidRequest, "calculate corpus identity: "+err.Error(), "", nil)
	}
	if err := ctx.Err(); err != nil {
		return result, generationError(ErrorCanceled, "generation canceled", "", &result)
	}

	categoryCatalog := make(map[string]corpus.Category, len(bundle.Categories))
	for _, category := range bundle.Categories {
		categoryCatalog[category.ID] = category
	}
	pools := make([]corpus.Pool, 0, len(categoryIDs))
	for _, id := range categoryIDs {
		category, exists := categoryCatalog[id]
		if !exists {
			return result, generationError(ErrorInvalidRequest, fmt.Sprintf("unknown category %q", id), id, nil)
		}
		if !supportsLatin(category) {
			return result, generationError(ErrorUnsupportedScript,
				fmt.Sprintf("category %q has no Latin-script source data; this generator emits Latin-script output only", id), id, nil)
		}
		selected, selectErr := corpus.Select(bundle.Records, []string{id}, corpus.GenderFilter(request.Gender))
		if selectErr != nil {
			return result, generationError(ErrorEmptySelection, selectErr.Error(), id, nil)
		}
		pools = append(pools, selected[0])
	}
	if request.Mode == ModeBlend {
		if err := compatibleScripts(categoryIDs, categoryCatalog); err != nil {
			return result, generationError(ErrorIncompatibleBlend, err.Error(), "", nil)
		}
	}

	models := make([]categoryModel, 0, len(pools))
	existing := make(map[string]struct{})
	for _, pool := range pools {
		spellings := make([]string, 0, len(pool.Records))
		for _, record := range pool.Records {
			spellings = append(spellings, record.Name)
			existing[spellingKey(record.Name)] = struct{}{}
		}
		if request.Mode == ModeCategory {
			bounds := applyBounds(observedBounds(spellings), request)
			if err := validateBounds(bounds); err != nil {
				return result, generationError(ErrorInvalidRequest, fmt.Sprintf("category %q: %v", pool.CategoryID, err), pool.CategoryID, nil)
			}
			model, trainErr := markov.Train(spellings, request.Order)
			if trainErr != nil {
				return result, generationError(ErrorInvalidRequest, fmt.Sprintf("train category %q: %v", pool.CategoryID, trainErr), pool.CategoryID, nil)
			}
			models = append(models, categoryModel{id: pool.CategoryID, model: model, bounds: bounds})
			result.Bounds[pool.CategoryID] = bounds
		}
	}

	if request.Mode == ModeBlend {
		spellings := unionSpellings(pools)
		bounds := applyBounds(observedBounds(spellings), request)
		if err := validateBounds(bounds); err != nil {
			return result, generationError(ErrorInvalidRequest, "blend: "+err.Error(), "", nil)
		}
		model, trainErr := markov.Train(spellings, request.Order)
		if trainErr != nil {
			return result, generationError(ErrorInvalidRequest, "train blend: "+trainErr.Error(), "", nil)
		}
		for _, id := range categoryIDs {
			models = append(models, categoryModel{id: id, model: model, bounds: bounds})
			result.Bounds[id] = bounds
		}
	}

	rng := rand.New(rand.NewPCG(seed, seed^seedXOR))
	attemptLimit := max(1000, request.Count*200)
	for len(result.Names) < request.Count {
		modelIndex := 0
		if request.Mode == ModeCategory {
			modelIndex = rng.IntN(len(models))
		}
		selected := models[modelIndex]
		for {
			if err := ctx.Err(); err != nil {
				result.Complete = false
				return result, generationError(ErrorCanceled, "generation canceled", selected.id, &result)
			}
			if result.Attempts >= attemptLimit {
				result.Complete = false
				return result, generationError(ErrorAttemptsExhausted,
					fmt.Sprintf("generation reached the %d-attempt limit after producing %d of %d names", attemptLimit, len(result.Names), request.Count),
					selected.id, &result)
			}
			result.Attempts++
			candidate, sampleErr := selected.model.Sample(rng, selected.bounds.Max)
			if sampleErr != nil {
				if errors.Is(sampleErr, markov.ErrTooLong) {
					result.Rejections.Length++
				} else {
					result.Rejections.Exhausted++
				}
				continue
			}
			candidate = norm.NFC.String(displayCase(candidate))
			if !isLatinSpelling(candidate) {
				result.Rejections.Script++
				continue
			}
			if !validSeparators(candidate) {
				result.Rejections.Separators++
				continue
			}
			length := utf8.RuneCountInString(candidate)
			if length < selected.bounds.Min || length > selected.bounds.Max {
				result.Rejections.Length++
				continue
			}
			key := spellingKey(candidate)
			if !request.AllowExisting {
				if _, exists := existing[key]; exists {
					result.Rejections.Existing++
					continue
				}
			}
			if containsKey(result.Names, key) {
				result.Rejections.Duplicates++
				continue
			}
			attribution := []string{selected.id}
			if request.Mode == ModeBlend {
				attribution = append([]string(nil), categoryIDs...)
			}
			result.Names = append(result.Names, GeneratedName{Name: candidate, CategoryIDs: attribution})
			break
		}
	}
	result.Complete = true
	return result, nil
}

func normalizeRequest(bundle *corpus.Bundle, request Request) (Request, []string, error) {
	if request.Count == 0 {
		request.Count = defaultCount
	}
	if request.Count < 1 || request.Count > maxCount {
		return Request{}, nil, generationError(ErrorInvalidRequest, "count must be between 1 and 1000", "", nil)
	}
	if request.Order == 0 {
		request.Order = defaultOrder
	}
	if request.Order < markov.MinOrder || request.Order > markov.MaxOrder {
		return Request{}, nil, generationError(ErrorInvalidRequest, "order must be between 1 and 4", "", nil)
	}
	if request.Mode == "" {
		request.Mode = ModeCategory
	}
	if request.Mode != ModeCategory && request.Mode != ModeBlend {
		return Request{}, nil, generationError(ErrorInvalidRequest, fmt.Sprintf("unknown generation mode %q", request.Mode), "", nil)
	}
	if request.Gender == "" {
		request.Gender = GenderAny
	}
	switch request.Gender {
	case GenderAny, GenderMasculine, GenderFeminine, GenderUnisex:
	default:
		return Request{}, nil, generationError(ErrorInvalidRequest, fmt.Sprintf("unknown gender filter %q", request.Gender), "", nil)
	}
	if request.MinLength < 0 || request.MinLength > markov.MaxNameRunes || request.MaxLength < 0 || request.MaxLength > markov.MaxNameRunes {
		return Request{}, nil, generationError(ErrorInvalidRequest, "length bounds must be between 1 and 64 runes", "", nil)
	}
	if request.AllCategories && len(request.CategoryIDs) != 0 {
		return Request{}, nil, generationError(ErrorInvalidRequest, "select categories by IDs or --all-categories, not both", "", nil)
	}
	var categoryIDs []string
	if request.AllCategories {
		categoryIDs = make([]string, 0, len(bundle.Categories))
		for _, category := range bundle.Categories {
			categoryIDs = append(categoryIDs, category.ID)
		}
	} else {
		categoryIDs = append([]string(nil), request.CategoryIDs...)
	}
	sort.Strings(categoryIDs)
	categoryIDs = compact(categoryIDs)
	if len(categoryIDs) == 0 {
		return Request{}, nil, generationError(ErrorInvalidRequest, "select one or more categories or set AllCategories", "", nil)
	}
	if request.AllCategories {
		request.CategoryIDs = nil
	} else {
		request.CategoryIDs = append([]string(nil), categoryIDs...)
	}
	return request, categoryIDs, nil
}

func observedBounds(spellings []string) LengthBounds {
	minimum, maximum := markov.MaxNameRunes, 0
	for _, spelling := range spellings {
		length := utf8.RuneCountInString(norm.NFC.String(strings.TrimSpace(spelling)))
		if length < minimum {
			minimum = length
		}
		if length > maximum {
			maximum = length
		}
	}
	if maximum > markov.MaxNameRunes {
		maximum = markov.MaxNameRunes
	}
	if minimum > markov.MaxNameRunes {
		minimum = markov.MaxNameRunes
	}
	if minimum < 1 {
		minimum = 1
	}
	return LengthBounds{Min: minimum, Max: maximum}
}

func applyBounds(bounds LengthBounds, request Request) LengthBounds {
	if request.MinLength != 0 {
		bounds.Min = request.MinLength
	}
	if request.MaxLength != 0 {
		bounds.Max = request.MaxLength
	}
	return bounds
}

func validateBounds(bounds LengthBounds) error {
	if bounds.Min < 1 || bounds.Min > bounds.Max || bounds.Max > markov.MaxNameRunes {
		return fmt.Errorf("effective length bounds must satisfy 1 <= min <= max <= %d (got %d..%d)", markov.MaxNameRunes, bounds.Min, bounds.Max)
	}
	return nil
}

func unionSpellings(pools []corpus.Pool) []string {
	byKey := make(map[string]string)
	for _, pool := range pools {
		for _, record := range pool.Records {
			spelling := norm.NFC.String(strings.TrimSpace(record.Name))
			key := spellingKey(spelling)
			if previous, exists := byKey[key]; !exists || spelling < previous {
				byKey[key] = spelling
			}
		}
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	spellings := make([]string, 0, len(keys))
	for _, key := range keys {
		spellings = append(spellings, byKey[key])
	}
	return spellings
}

func compatibleScripts(categoryIDs []string, categories map[string]corpus.Category) error {
	var expected []string
	for i, id := range categoryIDs {
		scripts := append([]string(nil), categories[id].Scripts...)
		sort.Strings(scripts)
		scripts = compact(scripts)
		if len(scripts) == 0 {
			return fmt.Errorf("category %q has no script profile for blending", id)
		}
		if i == 0 {
			expected = scripts
			continue
		}
		if !equalStrings(expected, scripts) {
			return fmt.Errorf("cannot blend categories with incompatible script profiles (%s: %s; %s: %s)",
				categoryIDs[0], strings.Join(expected, ", "), id, strings.Join(scripts, ", "))
		}
	}
	return nil
}

func supportsLatin(category corpus.Category) bool {
	for _, script := range category.Scripts {
		if script == "Latin" {
			return true
		}
	}
	return false
}

// isLatinSpelling permits Latin-script letters, their supported combining
// diacritics, and ordinary name separators only.
func isLatinSpelling(value string) bool {
	hasLatinLetter := false
	previousLatinLetter := false
	for _, r := range value {
		switch {
		case isLatinSeparator(r):
			previousLatinLetter = false
		case unicode.IsLetter(r):
			if !unicode.In(r, unicode.Latin) {
				return false
			}
			hasLatinLetter = true
			previousLatinLetter = true
		case unicode.IsMark(r):
			if !previousLatinLetter || !isLatinMark(r) {
				return false
			}
		default:
			return false
		}
	}
	return hasLatinLetter
}

func isLatinMark(r rune) bool {
	if unicode.In(r, unicode.Latin) {
		return true
	}
	return r >= 0x0300 && r <= 0x036f ||
		r >= 0x1ab0 && r <= 0x1aff ||
		r >= 0x1dc0 && r <= 0x1dff ||
		r >= 0xfe20 && r <= 0xfe2f
}

func isLatinSeparator(r rune) bool {
	return r == ' ' || isHyphen(r) || r == '\'' || r == '\u2019' || r == '\u02bc'
}

func displayCase(value string) string {
	runes := []rune(value)
	capitalizeNext := true
	for i, r := range runes {
		if capitalizeNext && unicode.IsLetter(r) {
			runes[i] = unicode.ToUpper(r)
			capitalizeNext = false
		}
		if r == ' ' || isHyphen(r) {
			capitalizeNext = true
		}
	}
	return string(runes)
}

func validSeparators(value string) bool {
	runes := []rune(value)
	if len(runes) == 0 {
		return false
	}
	previousSeparator := true
	for i, r := range runes {
		if !isSeparator(r) {
			previousSeparator = false
			continue
		}
		if i == 0 || i == len(runes)-1 || previousSeparator {
			return false
		}
		previousSeparator = true
	}
	return true
}

func isSeparator(r rune) bool {
	return unicode.IsSpace(r) || isHyphen(r) || r == '\'' || r == '\u2019' || r == '\u02bc'
}

func isHyphen(r rune) bool {
	switch r {
	case '-', '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2212':
		return true
	default:
		return false
	}
}

func spellingKey(value string) string {
	return strings.ToLower(norm.NFC.String(strings.TrimSpace(value)))
}

func containsKey(names []GeneratedName, key string) bool {
	for _, name := range names {
		if spellingKey(name.Name) == key {
			return true
		}
	}
	return false
}

func compact(values []string) []string {
	if len(values) == 0 {
		return values
	}
	write := 1
	for read := 1; read < len(values); read++ {
		if values[read] != values[write-1] {
			values[write] = values[read]
			write++
		}
	}
	return values[:write]
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func resolveSeed(seed *uint64) (uint64, error) {
	if seed != nil {
		return *seed, nil
	}
	var data [8]byte
	if _, err := cryptorand.Read(data[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(data[:]), nil
}

func cloneRequest(request Request) Request {
	request.CategoryIDs = append([]string(nil), request.CategoryIDs...)
	if request.Seed != nil {
		seed := *request.Seed
		request.Seed = &seed
	}
	return request
}

func generationError(kind ErrorKind, message, categoryID string, partial *Result) *GenerationError {
	return &GenerationError{Kind: kind, Message: message, CategoryID: categoryID, Partial: partial}
}
