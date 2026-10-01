package corpus

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// CanonicalRecords encodes records as sorted JSONL, with sorted set-like fields.
func CanonicalRecords(records []Record) ([]byte, error) {
	canonical := make([]Record, len(records))
	for i, record := range records {
		canonical[i] = cloneRecord(record)
		sort.Strings(canonical[i].Categories)
		sort.Slice(canonical[i].Genders, func(a, b int) bool { return canonical[i].Genders[a] < canonical[i].Genders[b] })
		sort.Slice(canonical[i].SourceRefs, func(a, b int) bool {
			return sourceRefLess(canonical[i].SourceRefs[a], canonical[i].SourceRefs[b])
		})
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].ID < canonical[j].ID })

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	for _, record := range canonical {
		if err := encoder.Encode(record); err != nil {
			return nil, fmt.Errorf("encode record %q: %w", record.ID, err)
		}
	}
	return output.Bytes(), nil
}

// CanonicalCategories encodes the category catalog in stable ID order.
func CanonicalCategories(categories []Category) ([]byte, error) {
	canonical := make([]Category, len(categories))
	for i, category := range categories {
		canonical[i] = cloneCategory(category)
		sort.Strings(canonical[i].Scripts)
		sort.Slice(canonical[i].SupportedGenders, func(a, b int) bool {
			return canonical[i].SupportedGenders[a] < canonical[i].SupportedGenders[b]
		})
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].ID < canonical[j].ID })

	data, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("encode categories: %w", err)
	}
	return append(data, '\n'), nil
}

// RecordsHash returns the SHA-256 digest of canonical JSONL records.
func RecordsHash(records []Record) (string, error) {
	data, err := CanonicalRecords(records)
	if err != nil {
		return "", err
	}
	return digest(data), nil
}

// CategoriesHash returns the SHA-256 digest of the canonical category catalog.
func CategoriesHash(categories []Category) (string, error) {
	data, err := CanonicalCategories(categories)
	if err != nil {
		return "", err
	}
	return digest(data), nil
}

// BundleHash identifies canonical records, categories, and normalization rules.
func BundleHash(records []Record, categories []Category) (string, error) {
	recordData, err := CanonicalRecords(records)
	if err != nil {
		return "", err
	}
	categoryData, err := CanonicalCategories(categories)
	if err != nil {
		return "", err
	}
	data := make([]byte, 0, len(NormalizationVersion)+1+len(recordData)+len(categoryData))
	data = append(data, NormalizationVersion...)
	data = append(data, '\n')
	data = append(data, recordData...)
	data = append(data, categoryData...)
	return digest(data), nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cloneRecord(record Record) Record {
	if record.Categories != nil {
		record.Categories = append(make([]string, 0, len(record.Categories)), record.Categories...)
	}
	if record.Genders != nil {
		record.Genders = append(make([]Gender, 0, len(record.Genders)), record.Genders...)
	}
	if record.SourceRefs != nil {
		record.SourceRefs = append(make([]SourceRef, 0, len(record.SourceRefs)), record.SourceRefs...)
	}
	return record
}

func cloneCategory(category Category) Category {
	if category.Scripts != nil {
		category.Scripts = append(make([]string, 0, len(category.Scripts)), category.Scripts...)
	}
	if category.SupportedGenders != nil {
		category.SupportedGenders = append(make([]Gender, 0, len(category.SupportedGenders)), category.SupportedGenders...)
	}
	return category
}

func sourceRefLess(a, b SourceRef) bool {
	if a.Revision != b.Revision {
		return a.Revision < b.Revision
	}
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.Bucket != b.Bucket {
		return a.Bucket < b.Bucket
	}
	return a.Index < b.Index
}
