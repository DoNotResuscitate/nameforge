package faker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

const AssetDirectory = "internal/corpus/assets/builtin"
const ReportPath = "data/quality.json"

type Rejection struct {
	Ref    corpus.SourceRef `json:"source_ref"`
	Reason string           `json:"reason"`
}
type Coverage struct {
	CategoryID   string         `json:"category_id"`
	SourceCounts map[string]int `json:"source_counts"`
	Accepted     int            `json:"accepted_occurrences"`
	Rejected     int            `json:"rejected_occurrences"`
	Duplicates   int            `json:"merged_occurrences"`
	Distinct     int            `json:"distinct_names"`
	GenderCounts map[string]int `json:"gender_counts"`
	Scripts      []string       `json:"scripts"`
	MinLength    int            `json:"min_length_runes"`
	MaxLength    int            `json:"max_length_runes"`
}
type Report struct {
	Revision         string      `json:"source_revision"`
	ExtractorVersion string      `json:"extractor_version"`
	BundleHash       string      `json:"bundle_hash"`
	Categories       []Coverage  `json:"categories"`
	Rejections       []Rejection `json:"rejections"`
}

// Build reads only locked cache entries. Artifacts contains exactly the public
// redistributable files to write or compare; raw files remain outside this set.
func Build(ctx context.Context, lock Lock, cache Cache) (map[string][]byte, error) {
	if err := lock.Validate(); err != nil {
		return nil, err
	}
	bundle := &corpus.Bundle{Manifest: corpus.Manifest{
		SchemaVersion: corpus.SchemaVersion, SourceRevision: lock.Revision,
		ExtractorVersion: ExtractorVersion, NormalizationVersion: corpus.NormalizationVersion,
		Licenses: []corpus.LicenseRef{{Name: "Faker (MIT)", Path: "licenses/FAKER-LICENSE"}},
	}}
	rejected := []Rejection{}
	for _, target := range lock.Targets {
		source := corpus.SourceFile{Path: target.Path, SHA256: target.SHA256}
		data, err := readLocked(ctx, cache, source)
		if err != nil {
			return nil, err
		}
		buckets, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", target.Path, err)
		}
		records, rejects := extract(target, buckets, lock.Revision)
		if len(records) == 0 {
			return nil, fmt.Errorf("category %s has no accepted names", target.ID)
		}
		rejected = append(rejected, rejects...)
		bundle.Records = append(bundle.Records, records...)
		bundle.Manifest.Sources = append(bundle.Manifest.Sources, source)
		category := corpus.Category{ID: target.ID, Label: target.Label, Group: "language-region", SourceLocale: target.Locale, RecordCount: len(records), Scripts: scripts(records), SupportedGenders: []corpus.Gender{}}
		for _, gender := range []corpus.Gender{corpus.GenderFeminine, corpus.GenderMasculine} {
			for _, record := range records {
				if containsGender(record.Genders, gender) {
					category.SupportedGenders = append(category.SupportedGenders, gender)
					break
				}
			}
		}
		bundle.Categories = append(bundle.Categories, category)
	}
	bundle.Manifest.RecordCount = len(bundle.Records)
	var err error
	bundle.Manifest.RecordsSHA256, err = corpus.RecordsHash(bundle.Records)
	if err != nil {
		return nil, err
	}
	bundle.Manifest.CategoriesSHA256, err = corpus.CategoriesHash(bundle.Categories)
	if err != nil {
		return nil, err
	}
	if err := bundle.Validate(); err != nil {
		return nil, err
	}
	license, err := readLocked(ctx, cache, lock.License)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(license)) == 0 {
		return nil, fmt.Errorf("empty upstream license")
	}
	report, err := quality(lock, bundle, rejected)
	if err != nil {
		return nil, err
	}
	manifestData, err := encode(bundle.Manifest)
	if err != nil {
		return nil, err
	}
	reportData, err := encode(report)
	if err != nil {
		return nil, err
	}
	recordData, err := corpus.CanonicalRecords(bundle.Records)
	if err != nil {
		return nil, err
	}
	categoryData, err := corpus.CanonicalCategories(bundle.Categories)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		AssetDirectory + "/manifest.json":          manifestData,
		AssetDirectory + "/categories.json":        categoryData,
		AssetDirectory + "/names.jsonl":            recordData,
		AssetDirectory + "/licenses/FAKER-LICENSE": license,
		ReportPath: reportData,
	}, nil
}

func extract(target Target, buckets map[string][]string, revision string) ([]corpus.Record, []Rejection) {
	merged := map[string]*corpus.Record{}
	rejected := []Rejection{}
	// Deterministic bucket order also chooses the retained upstream display form.
	for _, bucket := range []string{"female", "generic", "male"} {
		for index, raw := range buckets[bucket] {
			ref := corpus.SourceRef{Revision: revision, Path: target.Path, Bucket: bucket, Index: index}
			name := corpus.NormalizeName(raw)
			if reason := invalidSpelling(raw, name); reason != "" {
				rejected = append(rejected, Rejection{Ref: ref, Reason: reason})
				continue
			}
			key := strings.ToLower(name)
			record := merged[key]
			if record == nil {
				record = &corpus.Record{ID: "faker:" + target.Locale + ":" + hash([]byte(key)), Name: name, Categories: []string{target.ID}, Genders: []corpus.Gender{}, SourceRefs: []corpus.SourceRef{}}
				merged[key] = record
			}
			record.SourceRefs = append(record.SourceRefs, ref)
			var gender corpus.Gender
			if bucket == "female" {
				gender = corpus.GenderFeminine
			}
			if bucket == "male" {
				gender = corpus.GenderMasculine
			}
			if gender != "" && !containsGender(record.Genders, gender) {
				record.Genders = append(record.Genders, gender)
			}
		}
	}
	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	records := make([]corpus.Record, 0, len(keys))
	for _, key := range keys {
		record := merged[key]
		sort.Slice(record.Genders, func(i, j int) bool { return record.Genders[i] < record.Genders[j] })
		records = append(records, *record)
	}
	return records, rejected
}

func invalidSpelling(raw, name string) string {
	if !utf8.ValidString(raw) {
		return "invalid UTF-8"
	}
	for _, r := range raw {
		if unicode.IsControl(r) {
			return "control character"
		}
		if unicode.Is(unicode.Cf, r) {
			return "unsupported format character"
		}
	}
	if name == "" {
		return "empty spelling"
	}
	letter, separator := false, true
	for _, r := range name {
		if strings.ContainsRune(" '-’ʼ", r) {
			if separator {
				return "malformed separator placement"
			}
			separator = true
			continue
		}
		if unicode.IsLetter(r) {
			letter = true
			separator = false
			continue
		}
		if unicode.IsMark(r) && !separator {
			continue
		}
		return "unsupported name character"
	}
	if !letter || separator {
		return "malformed separator placement"
	}
	return ""
}
func containsGender(values []corpus.Gender, want corpus.Gender) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
func scripts(records []corpus.Record) []string {
	observed := map[string]bool{}
	for _, record := range records {
		for _, r := range record.Name {
			if strings.ContainsRune(" '-’ʼ", r) || !unicode.IsLetter(r) {
				continue
			}
			for name, table := range unicode.Scripts {
				if unicode.Is(table, r) {
					observed[name] = true
					break
				}
			}
		}
	}
	result := make([]string, 0, len(observed))
	for name := range observed {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
func encode(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	return append(data, '\n'), err
}

// quality independently accounts for every occurrence, catching incorrect
// indices, cross-locale references and gender leakage between categories.
func quality(lock Lock, bundle *corpus.Bundle, rejected []Rejection) (Report, error) {
	identity, err := corpus.BundleHash(bundle.Records, bundle.Categories)
	if err != nil {
		return Report{}, err
	}
	report := Report{Revision: lock.Revision, ExtractorVersion: ExtractorVersion, BundleHash: identity, Categories: []Coverage{}, Rejections: rejected}
	if rejected == nil {
		report.Rejections = []Rejection{}
	}
	knownPaths := map[string]bool{}
	for _, target := range lock.Targets {
		knownPaths[target.Path] = true
	}
	for _, reject := range rejected {
		if !knownPaths[reject.Ref.Path] || reject.Ref.Revision != lock.Revision || reject.Ref.Index < 0 || reject.Reason == "" {
			return Report{}, fmt.Errorf("invalid rejection reference")
		}
	}
	for _, target := range lock.Targets {
		category, ok := bundle.Category(target.ID)
		if !ok || category.Label != target.Label || category.SourceLocale != target.Locale || category.Group != "language-region" {
			return Report{}, fmt.Errorf("missing/incorrect target category %s", target.ID)
		}
		coverage := Coverage{CategoryID: target.ID, SourceCounts: map[string]int{}, GenderCounts: map[string]int{"masculine": 0, "feminine": 0, "unisex": 0, "unspecified": 0}, MinLength: 1 << 30}
		seen := map[string]map[int]bool{"male": {}, "female": {}, "generic": {}}
		add := func(ref corpus.SourceRef) error {
			indices, ok := seen[ref.Bucket]
			if !ok || ref.Index < 0 || ref.Revision != lock.Revision || ref.Path != target.Path || indices[ref.Index] {
				return fmt.Errorf("invalid/duplicate source occurrence %+v", ref)
			}
			indices[ref.Index] = true
			coverage.SourceCounts[ref.Bucket]++
			return nil
		}
		records := []corpus.Record{}
		for _, record := range bundle.Records {
			if len(record.Categories) != 1 || record.Categories[0] != target.ID {
				continue
			}
			if record.ID != "faker:"+target.Locale+":"+hash([]byte(strings.ToLower(record.Name))) || invalidSpelling(record.Name, record.Name) != "" {
				return Report{}, fmt.Errorf("invalid extracted record %s", record.ID)
			}
			records = append(records, record)
			coverage.Distinct++
			length := utf8.RuneCountInString(record.Name)
			coverage.MinLength = min(coverage.MinLength, length)
			coverage.MaxLength = max(coverage.MaxLength, length)
			for _, ref := range record.SourceRefs {
				if err := add(ref); err != nil {
					return Report{}, err
				}
				coverage.Accepted++
			}
			for _, gender := range record.Genders {
				coverage.GenderCounts[string(gender)]++
			}
			if len(record.Genders) == 0 {
				coverage.GenderCounts["unspecified"]++
			}
			if len(record.Genders) == 2 {
				coverage.GenderCounts["unisex"]++
			}
		}
		for _, reject := range rejected {
			if reject.Ref.Path == target.Path {
				if err := add(reject.Ref); err != nil {
					return Report{}, err
				}
				coverage.Rejected++
			}
		}
		for bucket, indices := range seen {
			for index := 0; index < len(indices); index++ {
				if !indices[index] {
					return Report{}, fmt.Errorf("non-contiguous source indices in %s/%s", target.ID, bucket)
				}
			}
		}
		coverage.Duplicates = coverage.Accepted - coverage.Distinct
		coverage.Scripts = scripts(records)
		if coverage.Distinct != category.RecordCount || strings.Join(coverage.Scripts, ",") != strings.Join(category.Scripts, ",") {
			return Report{}, fmt.Errorf("coverage mismatch for %s", target.ID)
		}
		report.Categories = append(report.Categories, coverage)
	}
	if len(bundle.Categories) != len(lock.Targets) {
		return Report{}, fmt.Errorf("unexpected extra categories")
	}
	for _, record := range bundle.Records {
		if len(record.Categories) != 1 {
			return Report{}, fmt.Errorf("extracted records must preserve locale-specific gender semantics")
		}
	}
	return report, nil
}

// Verify checks committed assets and coverage offline without requiring a raw
// cache. Call Build and Compare separately to verify a locked-cache rebuild.
func Verify(lock Lock, root fs.FS) error {
	if err := lock.Validate(); err != nil {
		return err
	}
	bundle, err := corpus.Load(root, AssetDirectory)
	if err != nil {
		return err
	}
	if bundle.Manifest.SourceRevision != lock.Revision || bundle.Manifest.ExtractorVersion != lock.ExtractorVersion {
		return fmt.Errorf("bundle does not match source lock")
	}
	wantSources := []corpus.SourceFile{}
	for _, target := range lock.Targets {
		wantSources = append(wantSources, corpus.SourceFile{Path: target.Path, SHA256: target.SHA256})
	}
	want, _ := encode(wantSources)
	got, _ := encode(bundle.Manifest.Sources)
	if !bytes.Equal(want, got) {
		return fmt.Errorf("manifest sources do not match lock")
	}
	if len(bundle.Manifest.Licenses) != 1 || bundle.Manifest.Licenses[0] != (corpus.LicenseRef{Name: "Faker (MIT)", Path: "licenses/FAKER-LICENSE"}) {
		return fmt.Errorf("incorrect license reference")
	}
	license, err := fs.ReadFile(root, AssetDirectory+"/licenses/FAKER-LICENSE")
	if err != nil {
		return err
	}
	if hash(license) != lock.License.SHA256 {
		return fmt.Errorf("license checksum mismatch")
	}
	reportData, err := fs.ReadFile(root, ReportPath)
	if err != nil {
		return err
	}
	var report Report
	d := json.NewDecoder(bytes.NewReader(reportData))
	d.DisallowUnknownFields()
	if err := d.Decode(&report); err != nil {
		return err
	}
	computed, err := quality(lock, bundle, report.Rejections)
	if err != nil {
		return err
	}
	want, err = encode(computed)
	if err != nil {
		return err
	}
	if !bytes.Equal(want, reportData) {
		return fmt.Errorf("quality report mismatch or noncanonical report")
	}
	return nil
}

func Compare(artifacts map[string][]byte, root fs.FS) error {
	paths := make([]string, 0, len(artifacts))
	for path := range artifacts {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		got, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, artifacts[path]) {
			return fmt.Errorf("locked-cache rebuild differs: %s", path)
		}
	}
	return nil
}
