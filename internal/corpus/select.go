package corpus

import (
	"fmt"
	"sort"
	"strings"
)

// GenderFilter controls inclusion by source-supported labels.
type GenderFilter string

const (
	FilterAny       GenderFilter = "any"
	FilterMasculine GenderFilter = "masculine"
	FilterFeminine  GenderFilter = "feminine"
	FilterUnisex    GenderFilter = "unisex"
)

// Pool is the stable, filtered set of records for one selected category.
type Pool struct {
	CategoryID string
	Records    []Record
}

// Select returns sorted category pools. Category IDs are deduplicated and sorted
// before processing, and records are ordered by normalized spelling and ID.
func Select(records []Record, categoryIDs []string, gender GenderFilter) ([]Pool, error) {
	if gender == "" {
		gender = FilterAny
	}
	switch gender {
	case FilterAny, FilterMasculine, FilterFeminine, FilterUnisex:
	default:
		return nil, fmt.Errorf("unknown gender filter %q", gender)
	}
	ids := append([]string(nil), categoryIDs...)
	sort.Strings(ids)
	ids = compactStrings(ids)
	if len(ids) == 0 {
		return nil, fmt.Errorf("select at least one category")
	}
	for _, id := range ids {
		if !validIdentifier(id) {
			return nil, fmt.Errorf("invalid category ID %q", id)
		}
	}

	pools := make([]Pool, 0, len(ids))
	for _, id := range ids {
		pool := Pool{CategoryID: id}
		type candidate struct {
			record Record
			key    string
		}
		candidates := make([]candidate, 0)
		for _, record := range records {
			if !contains(record.Categories, id) || !matchesGender(record.Genders, gender) {
				continue
			}
			key := strings.ToLower(NormalizeName(record.Name))
			candidates = append(candidates, candidate{record: record, key: key})
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].key != candidates[j].key {
				return candidates[i].key < candidates[j].key
			}
			return candidates[i].record.ID < candidates[j].record.ID
		})
		previous := ""
		for _, candidate := range candidates {
			if len(pool.Records) > 0 && candidate.key == previous {
				continue
			}
			pool.Records = append(pool.Records, cloneRecord(candidate.record))
			previous = candidate.key
		}
		if len(pool.Records) == 0 {
			return nil, fmt.Errorf("category %q has no records matching gender %q", id, gender)
		}
		pools = append(pools, pool)
	}
	return pools, nil
}

func compactStrings(values []string) []string {
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

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func matchesGender(genders []Gender, filter GenderFilter) bool {
	switch filter {
	case FilterAny:
		return true
	case FilterMasculine:
		return hasGender(genders, GenderMasculine)
	case FilterFeminine:
		return hasGender(genders, GenderFeminine)
	case FilterUnisex:
		return hasGender(genders, GenderMasculine) && hasGender(genders, GenderFeminine)
	default:
		return false
	}
}
