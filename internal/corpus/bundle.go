package corpus

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

const SchemaVersion = 1

// Bundle is a validated, self-contained corpus catalog and its name records.
type Bundle struct {
	// Surnames is a separate, independently validated training bundle. It is
	// attached by the runtime catalog loader, never mixed into given-name hashes.
	Surnames   *Bundle    `json:"-"`
	Manifest   Manifest   `json:"manifest"`
	Categories []Category `json:"categories"`
	Records    []Record   `json:"records"`
}

// Category returns a category by stable ID.
func (bundle *Bundle) Category(id string) (Category, bool) {
	if bundle == nil {
		return Category{}, false
	}
	for _, category := range bundle.Categories {
		if category.ID == id {
			return cloneCategory(category), true
		}
	}
	return Category{}, false
}

// Validate checks schema, provenance, references, and canonical content hashes.
func (bundle *Bundle) Validate() error {
	manifest := bundle.Manifest
	if manifest.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported corpus schema version %d (supported: %d)", manifest.SchemaVersion, SchemaVersion)
	}
	if manifest.SourceRevision == "" || manifest.ExtractorVersion == "" || manifest.NormalizationVersion != NormalizationVersion {
		return errors.New("manifest requires a source revision, extractor version, and supported normalization version")
	}
	if len(manifest.Sources) == 0 {
		return errors.New("manifest has no source files")
	}
	sourcePaths := make(map[string]struct{}, len(manifest.Sources))
	for _, source := range manifest.Sources {
		if !fs.ValidPath(source.Path) || source.Path == "." || !isSHA256(source.SHA256) {
			return fmt.Errorf("invalid source file entry %q", source.Path)
		}
		if _, exists := sourcePaths[source.Path]; exists {
			return fmt.Errorf("duplicate source file %q", source.Path)
		}
		sourcePaths[source.Path] = struct{}{}
	}
	licensePaths := make(map[string]struct{}, len(manifest.Licenses))
	for _, license := range manifest.Licenses {
		if license.Name == "" || !fs.ValidPath(license.Path) || license.Path == "." {
			return fmt.Errorf("invalid license reference %q", license.Path)
		}
		if _, exists := licensePaths[license.Path]; exists {
			return fmt.Errorf("duplicate license reference %q", license.Path)
		}
		licensePaths[license.Path] = struct{}{}
	}
	if len(licensePaths) == 0 {
		return errors.New("manifest has no license references")
	}

	categories := make(map[string]Category, len(bundle.Categories))
	for _, category := range bundle.Categories {
		if !validIdentifier(category.ID) || category.Label == "" || category.Group == "" || category.SourceLocale == "" {
			return fmt.Errorf("invalid category metadata for %q", category.ID)
		}
		if _, exists := categories[category.ID]; exists {
			return fmt.Errorf("duplicate category %q", category.ID)
		}
		if category.RecordCount < 1 || len(category.Scripts) == 0 || category.SupportedGenders == nil {
			return fmt.Errorf("category %q must have scripts, supported genders, and a positive record count", category.ID)
		}
		if err := validateUniqueSortedStrings("scripts", category.Scripts); err != nil {
			return fmt.Errorf("category %q: %w", category.ID, err)
		}
		if err := validateGenders(category.SupportedGenders); err != nil {
			return fmt.Errorf("category %q: %w", category.ID, err)
		}
		categories[category.ID] = category
	}
	if len(categories) == 0 {
		return errors.New("corpus has no categories")
	}

	if manifest.RecordCount != len(bundle.Records) {
		return fmt.Errorf("manifest record count is %d, found %d", manifest.RecordCount, len(bundle.Records))
	}
	ids := make(map[string]struct{}, len(bundle.Records))
	categoryNames := make(map[string]map[string]struct{}, len(categories))
	categoryGenders := make(map[string]map[Gender]struct{}, len(categories))
	for id := range categories {
		categoryNames[id] = make(map[string]struct{})
		categoryGenders[id] = make(map[Gender]struct{})
	}
	for _, record := range bundle.Records {
		if record.ID == "" || !validName(record.Name) {
			return fmt.Errorf("record %q has an empty ID or invalid/non-normalized name", record.ID)
		}
		if _, exists := ids[record.ID]; exists {
			return fmt.Errorf("duplicate record ID %q", record.ID)
		}
		ids[record.ID] = struct{}{}
		if len(record.Categories) == 0 {
			return fmt.Errorf("record %q has no categories", record.ID)
		}
		if record.Genders == nil {
			return fmt.Errorf("record %q must encode unspecified genders as an empty array", record.ID)
		}
		if err := validateUniqueSortedStrings("categories", record.Categories); err != nil {
			return fmt.Errorf("record %q: %w", record.ID, err)
		}
		if err := validateGenders(record.Genders); err != nil {
			return fmt.Errorf("record %q: %w", record.ID, err)
		}
		if len(record.SourceRefs) == 0 {
			return fmt.Errorf("record %q has no source references", record.ID)
		}
		type occurrenceID struct {
			Revision, Path, Bucket, StatementID string
			Index                               int
		}
		seenRefs := make(map[occurrenceID]struct{}, len(record.SourceRefs))
		fromMale, fromFemale := false, false
		for _, ref := range record.SourceRefs {
			if !fs.ValidPath(ref.Path) || ref.Path == "." || ref.Index < 0 {
				return fmt.Errorf("record %q has invalid source reference", record.ID)
			}
			if _, exists := sourcePaths[ref.Path]; !exists {
				return fmt.Errorf("record %q references undeclared source file %q", record.ID, ref.Path)
			}
			revision := manifest.SourceRevision
			for _, source := range manifest.Sources {
				if source.Path == ref.Path && source.Revision != "" {
					revision = source.Revision
				}
			}
			if ref.Revision != revision {
				return fmt.Errorf("record %q source revision mismatch", record.ID)
			}
			identity := occurrenceID{Revision: ref.Revision, Path: ref.Path, Bucket: ref.Bucket, StatementID: ref.StatementID, Index: ref.Index}
			if _, exists := seenRefs[identity]; exists {
				return fmt.Errorf("record %q has duplicate source reference", record.ID)
			}
			seenRefs[identity] = struct{}{}
			switch ref.Bucket {
			case "generic":
			case "male":
				fromMale = true
			case "female":
				fromFemale = true
			default:
				return fmt.Errorf("record %q references unsupported source bucket %q", record.ID, ref.Bucket)
			}
		}
		if hasGender(record.Genders, GenderMasculine) != fromMale || hasGender(record.Genders, GenderFeminine) != fromFemale {
			return fmt.Errorf("record %q gender labels do not match its source buckets", record.ID)
		}
		key := strings.ToLower(record.Name)
		for _, categoryID := range record.Categories {
			if _, exists := categories[categoryID]; !exists {
				return fmt.Errorf("record %q references unknown category %q", record.ID, categoryID)
			}
			if _, exists := categoryNames[categoryID][key]; exists {
				return fmt.Errorf("category %q has duplicate normalized spelling %q", categoryID, record.Name)
			}
			categoryNames[categoryID][key] = struct{}{}
			for _, gender := range record.Genders {
				categoryGenders[categoryID][gender] = struct{}{}
			}
		}
	}
	for id, category := range categories {
		if category.RecordCount != len(categoryNames[id]) {
			return fmt.Errorf("category %q record count is %d, found %d", id, category.RecordCount, len(categoryNames[id]))
		}
		observed := make([]Gender, 0, len(categoryGenders[id]))
		for gender := range categoryGenders[id] {
			observed = append(observed, gender)
		}
		sort.Slice(observed, func(i, j int) bool { return observed[i] < observed[j] })
		if !equalGenders(category.SupportedGenders, observed) {
			return fmt.Errorf("category %q supported genders do not match its records", id)
		}
	}

	recordsHash, err := RecordsHash(bundle.Records)
	if err != nil {
		return err
	}
	if !isSHA256(manifest.RecordsSHA256) || manifest.RecordsSHA256 != recordsHash {
		return fmt.Errorf("records SHA-256 mismatch: manifest %q, calculated %q", manifest.RecordsSHA256, recordsHash)
	}
	categoriesHash, err := CategoriesHash(bundle.Categories)
	if err != nil {
		return err
	}
	if !isSHA256(manifest.CategoriesSHA256) || manifest.CategoriesSHA256 != categoriesHash {
		return fmt.Errorf("categories SHA-256 mismatch: manifest %q, calculated %q", manifest.CategoriesSHA256, categoriesHash)
	}
	return nil
}

// Load reads a corpus v1 bundle from a read-only filesystem.
func Load(fsys fs.FS, directory string) (*Bundle, error) {
	if !fs.ValidPath(directory) || directory == "." {
		return nil, fmt.Errorf("invalid corpus directory %q", directory)
	}
	read := func(name string) ([]byte, error) {
		data, err := fs.ReadFile(fsys, path.Join(directory, name))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		return data, nil
	}
	manifestData, err := read("manifest.json")
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := decodeStrict(manifestData, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	categoriesData, err := read("categories.json")
	if err != nil {
		return nil, err
	}
	var categories []Category
	if err := decodeStrict(categoriesData, &categories); err != nil {
		return nil, fmt.Errorf("decode categories: %w", err)
	}
	recordsData, err := read("names.jsonl")
	if err != nil {
		return nil, err
	}
	records, err := decodeRecords(recordsData)
	if err != nil {
		return nil, fmt.Errorf("decode records: %w", err)
	}
	bundle := &Bundle{Manifest: manifest, Categories: categories, Records: records}
	if err := bundle.Validate(); err != nil {
		return nil, err
	}
	canonicalRecords, err := CanonicalRecords(records)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(recordsData, canonicalRecords) {
		return nil, errors.New("names.jsonl is not in canonical JSONL form")
	}
	canonicalCategories, err := CanonicalCategories(categories)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(categoriesData, canonicalCategories) {
		return nil, errors.New("categories.json is not in canonical form")
	}
	for _, license := range manifest.Licenses {
		if _, err := fs.Stat(fsys, path.Join(directory, license.Path)); err != nil {
			return nil, fmt.Errorf("license notice %q is missing: %w", license.Path, err)
		}
	}
	return bundle, nil
}

func decodeRecords(data []byte) ([]Record, error) {
	if len(data) == 0 {
		return nil, errors.New("names.jsonl is empty")
	}
	lines := bytes.Split(data, []byte{'\n'})
	if len(lines[len(lines)-1]) != 0 {
		return nil, errors.New("names.jsonl must end with a newline")
	}
	lines = lines[:len(lines)-1]
	records := make([]Record, 0, len(lines))
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			return nil, fmt.Errorf("line %d is empty", i+1)
		}
		var record Record
		if err := decodeStrict(line, &record); err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		records = append(records, record)
	}
	return records, nil
}

func decodeStrict(data []byte, target any) error {
	if !utf8.Valid(data) {
		return errors.New("input is not valid UTF-8")
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			if !ok {
				return errors.New("JSON object key is not a string")
			}
			if _, exists := keys[key]; exists {
				return fmt.Errorf("duplicate JSON object key %q", key)
			}
			keys[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return errors.New("malformed JSON object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return errors.New("malformed JSON array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	return nil
}

func validIdentifier(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	return true
}

func validateUniqueSortedStrings(label string, values []string) error {
	for i, value := range values {
		if value == "" {
			return fmt.Errorf("%s contains an empty value", label)
		}
		if i > 0 && values[i-1] >= value {
			return fmt.Errorf("%s must be sorted and unique", label)
		}
	}
	return nil
}

func validateGenders(genders []Gender) error {
	for i, gender := range genders {
		if gender != GenderMasculine && gender != GenderFeminine {
			return fmt.Errorf("unsupported gender label %q", gender)
		}
		if i > 0 && genders[i-1] >= gender {
			return errors.New("gender labels must be sorted and unique")
		}
	}
	return nil
}

func hasGender(genders []Gender, target Gender) bool {
	for _, gender := range genders {
		if gender == target {
			return true
		}
	}
	return false
}

func equalGenders(a, b []Gender) bool {
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

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && strings.ToLower(value) == value
}
