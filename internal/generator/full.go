package generator

import (
	"context"
	"fmt"
	"math/rand/v2"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

// Full names pair components within the same selected category. Blends train
// each component on its own union. Components may repeat; complete names may not.
func generateFull(ctx context.Context, bundle *corpus.Bundle, request Request) (Result, error) {
	r, ids, err := normalizeRequest(bundle, request)
	if err != nil {
		return Result{}, err
	}
	if bundle.Surnames == nil {
		return Result{}, generationError(ErrorEmptySelection, "surname corpus unavailable for full-name composition", "", nil)
	}
	seed, err := resolveSeed(r.Seed)
	if err != nil {
		return Result{}, generationError(ErrorInvalidRequest, "obtain random seed: "+err.Error(), "", nil)
	}
	r.Seed = &seed
	if r.Surname == nil {
		r.Surname = &ComponentOptions{Order: r.Order, MinLength: r.MinLength, MaxLength: r.MaxLength, AllowExisting: r.AllowExisting}
	}
	result := Result{NameType: NameFull, Seed: seed, AlgorithmVersion: AlgorithmVersion + "/full-v1", CategoryIDs: ids, Mode: r.Mode, Options: cloneRequest(r), Bounds: map[string]LengthBounds{}, SurnameBounds: map[string]LengthBounds{}, ComponentOrder: "given-surname", Separator: " ", Names: make([]GeneratedName, 0, r.Count)}
	result.BundleHash, err = corpus.BundleHash(bundle.Records, bundle.Categories)
	if err != nil {
		return result, err
	}
	result.SurnameBundleHash, err = corpus.BundleHash(bundle.Surnames.Records, bundle.Surnames.Categories)
	if err != nil {
		return result, err
	}
	if ctx.Err() != nil {
		return result, generationError(ErrorCanceled, "generation canceled", "", &result)
	}
	givenRequest := r
	givenRequest.NameType, givenRequest.Surname = NameGiven, nil
	given, givenExisting, err := prepareModels(bundle, givenRequest, ids, result.Bounds)
	if err != nil {
		return result, err
	}
	surnameRequest := givenRequest
	surnameRequest.NameType, surnameRequest.Gender = NameSurname, GenderAny
	surnameRequest.Order, surnameRequest.MinLength, surnameRequest.MaxLength, surnameRequest.AllowExisting = r.Surname.Order, r.Surname.MinLength, r.Surname.MaxLength, r.Surname.AllowExisting
	surname, surnameExisting, err := prepareModels(bundle.Surnames, surnameRequest, ids, result.SurnameBounds)
	if err != nil {
		return result, err
	}
	rng := rand.New(rand.NewPCG(seed, seed^seedXOR))
	limit := max(1000, r.Count*200)
	sample := func(model categoryModel, allow bool, existing map[string]struct{}) (string, error) {
		for {
			if ctx.Err() != nil {
				return "", generationError(ErrorCanceled, "full-name generation canceled", model.id, &result)
			}
			if result.Attempts >= limit {
				return "", generationError(ErrorAttemptsExhausted, fmt.Sprintf("full-name generation reached the %d-component-attempt limit after producing %d of %d names", limit, len(result.Names), r.Count), model.id, &result)
			}
			result.Attempts++
			if candidate, ok := sampleCandidate(model, rng, allow, existing, &result.Rejections); ok {
				return candidate, nil
			}
		}
	}
	for len(result.Names) < r.Count {
		i := 0
		if r.Mode == ModeCategory {
			i = rng.IntN(len(given))
		}
		for {
			g, err := sample(given[i], r.AllowExisting, givenExisting)
			if err != nil {
				return result, err
			}
			s, err := sample(surname[i], r.Surname.AllowExisting, surnameExisting)
			if err != nil {
				return result, err
			}
			name := g + " " + s
			if containsKey(result.Names, spellingKey(name)) {
				result.Rejections.Duplicates++
				continue
			}
			attribution := []string{given[i].id}
			if r.Mode == ModeBlend {
				attribution = append([]string(nil), ids...)
			}
			result.Names = append(result.Names, GeneratedName{Name: name, CategoryIDs: attribution, Given: &NameComponent{Name: g, CategoryIDs: append([]string(nil), attribution...)}, Surname: &NameComponent{Name: s, CategoryIDs: append([]string(nil), attribution...)}})
			break
		}
	}
	result.Complete = true
	return result, nil
}
