package corpus

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestNormalizeName(t *testing.T) {
	got := NormalizeName("  e\u0301lan  ")
	if got != "élan" {
		t.Fatalf("NormalizeName() = %q, want NFC %q", got, "élan")
	}
	if validName("e\u0301lan") {
		t.Fatal("validName accepted a decomposed spelling")
	}
	if validName(string([]byte{0xff})) {
		t.Fatal("validName accepted invalid UTF-8")
	}
	if validName("token-\u0085") {
		t.Fatal("validName accepted a Unicode control character")
	}
}

func TestCanonicalHashesIgnoreInputOrder(t *testing.T) {
	records := []Record{
		{ID: "b", Name: "token-beta", Categories: []string{"fixture"}, Genders: []Gender{}, SourceRefs: []SourceRef{{Revision: "rev", Path: "source.ts", Bucket: "generic", Index: 1}}},
		{ID: "a", Name: "token-alpha", Categories: []string{"fixture"}, Genders: []Gender{}, SourceRefs: []SourceRef{{Revision: "rev", Path: "source.ts", Bucket: "generic", Index: 0}}},
	}
	categories := []Category{{ID: "fixture", Label: "Fixture", Group: "language-region", SourceLocale: "xx", Scripts: []string{"Latin"}, SupportedGenders: []Gender{}, RecordCount: 2}}
	forward, err := RecordsHash(records)
	if err != nil {
		t.Fatal(err)
	}
	reverse, err := RecordsHash([]Record{records[1], records[0]})
	if err != nil {
		t.Fatal(err)
	}
	if forward != reverse {
		t.Fatalf("record hash depends on input order: %s != %s", forward, reverse)
	}
	categories = append(categories, Category{ID: "second", Label: "Second", Group: "language-region", SourceLocale: "yy", Scripts: []string{"Latin"}, SupportedGenders: []Gender{}, RecordCount: 1})
	bundleForward, err := BundleHash(records, categories)
	if err != nil {
		t.Fatal(err)
	}
	bundleReverse, err := BundleHash([]Record{records[1], records[0]}, []Category{categories[1], categories[0]})
	if err != nil {
		t.Fatal(err)
	}
	if bundleForward != bundleReverse {
		t.Fatalf("bundle hash depends on input order: %s != %s", bundleForward, bundleReverse)
	}
	data, err := CanonicalRecords(records)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte(`{"id":"a"`)) {
		t.Fatalf("canonical records are not sorted by ID: %s", data)
	}
	if categories[0].Scripts[0] != "Latin" {
		t.Fatal("canonical serialization modified input slices")
	}
}

func TestSelectGenderAndCategorySemantics(t *testing.T) {
	records := []Record{
		selectionRecord("u", "token-unspecified", []string{"second", "first"}, nil),
		selectionRecord("m", "token-masculine", []string{"first", "second"}, []Gender{GenderMasculine}),
		selectionRecord("f", "token-feminine", []string{"first", "second"}, []Gender{GenderFeminine}),
		selectionRecord("x", "token-dual", []string{"first", "second"}, []Gender{GenderFeminine, GenderMasculine}),
	}
	tests := []struct {
		filter GenderFilter
		want   []string
	}{
		{FilterAny, []string{"token-dual", "token-feminine", "token-masculine", "token-unspecified"}},
		{FilterMasculine, []string{"token-dual", "token-masculine"}},
		{FilterFeminine, []string{"token-dual", "token-feminine"}},
		{FilterUnisex, []string{"token-dual"}},
	}
	for _, test := range tests {
		t.Run(string(test.filter), func(t *testing.T) {
			pools, err := Select(records, []string{"second", "first", "first"}, test.filter)
			if err != nil {
				t.Fatal(err)
			}
			if len(pools) != 2 || pools[0].CategoryID != "first" || pools[1].CategoryID != "second" {
				t.Fatalf("category pools are not stably sorted: %#v", pools)
			}
			got := make([]string, 0, len(pools[0].Records))
			for _, record := range pools[0].Records {
				got = append(got, record.Name)
			}
			if strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("selected records = %v, want %v", got, test.want)
			}
			reversed := append([]Record(nil), records...)
			for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
				reversed[left], reversed[right] = reversed[right], reversed[left]
			}
			reordered, err := Select(reversed, []string{"first", "second"}, test.filter)
			if err != nil {
				t.Fatal(err)
			}
			var reorderedNames []string
			for _, record := range reordered[0].Records {
				reorderedNames = append(reorderedNames, record.Name)
			}
			if strings.Join(reorderedNames, ",") != strings.Join(got, ",") {
				t.Fatalf("selection depends on input record order: %v != %v", reorderedNames, got)
			}
		})
	}
	if _, err := Select(records, []string{"missing"}, FilterAny); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("empty category selection error = %v, want category ID", err)
	}
	if _, err := Select(records, nil, FilterAny); err == nil {
		t.Fatal("Select accepted an empty category selection")
	}
}

func TestLoadValidatesAndLoadsReadOnlyFilesystem(t *testing.T) {
	bundle, err := LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Records) != 3 || bundle.Categories[0].ID != "french" {
		t.Fatalf("unexpected embedded fixture bundle: %#v", bundle)
	}
	if _, err := fs.Stat(BuiltinFS(), "assets/builtin/licenses/FAKER-LICENSE"); err != nil {
		t.Fatalf("embedded license missing: %v", err)
	}
}

func TestLoadRejectsUnknownSchemaInvalidUTF8AndBadHashes(t *testing.T) {
	t.Run("unknown schema", func(t *testing.T) {
		bundle := testBundle(t)
		bundle.Manifest.SchemaVersion++
		if err := bundle.Validate(); err == nil || !strings.Contains(err.Error(), "unsupported corpus schema") {
			t.Fatalf("Validate() error = %v", err)
		}
	})
	t.Run("invalid UTF-8", func(t *testing.T) {
		files := validTestFiles(t)
		files["bundle/names.jsonl"] = &fstest.MapFile{Data: []byte{0xff, '\n'}}
		if _, err := Load(fstest.MapFS(files), "bundle"); err == nil || !strings.Contains(err.Error(), "UTF-8") {
			t.Fatalf("Load() error = %v, want invalid UTF-8", err)
		}
	})
	t.Run("hash mismatch", func(t *testing.T) {
		bundle := testBundle(t)
		bundle.Manifest.RecordsSHA256 = strings.Repeat("0", 64)
		if err := bundle.Validate(); err == nil || !strings.Contains(err.Error(), "records SHA-256 mismatch") {
			t.Fatalf("Validate() error = %v", err)
		}
	})
	t.Run("unknown fields", func(t *testing.T) {
		files := validTestFiles(t)
		files["bundle/manifest.json"] = &fstest.MapFile{Data: []byte(`{"schema_version":1,"unexpected":true}`)}
		if _, err := Load(fstest.MapFS(files), "bundle"); err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("Load() error = %v, want unknown field", err)
		}
	})
}

func TestLoadRejectsMissingLicense(t *testing.T) {
	files := validTestFiles(t)
	delete(files, "bundle/licenses/fixture-license")
	if _, err := Load(fstest.MapFS(files), "bundle"); err == nil || !strings.Contains(err.Error(), "license notice") {
		t.Fatalf("Load() error = %v, want missing license", err)
	}
}

func TestValidateRejectsDuplicateNormalizedSpellings(t *testing.T) {
	bundle := testBundle(t)
	duplicate := cloneRecord(bundle.Records[0])
	duplicate.ID = "fixture-2"
	duplicate.Name = "TOKEN-ALPHA"
	duplicate.SourceRefs[0].Index = 1
	bundle.Records = append(bundle.Records, duplicate)
	bundle.Manifest.RecordCount = 2
	bundle.Categories[0].RecordCount = 2
	var err error
	bundle.Manifest.RecordsSHA256, err = RecordsHash(bundle.Records)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Manifest.CategoriesSHA256, err = CategoriesHash(bundle.Categories)
	if err != nil {
		t.Fatal(err)
	}
	if err := bundle.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate normalized spelling") {
		t.Fatalf("Validate() error = %v, want duplicate spelling error", err)
	}
}

func selectionRecord(id, name string, categories []string, genders []Gender) Record {
	return Record{ID: id, Name: name, Categories: categories, Genders: genders}
}

func testBundle(t *testing.T) *Bundle {
	t.Helper()
	bundle := &Bundle{
		Manifest: Manifest{
			SchemaVersion: SchemaVersion, SourceRevision: "rev",
			Sources:          []SourceFile{{Path: "source.ts", SHA256: strings.Repeat("a", 64)}},
			Licenses:         []LicenseRef{{Name: "Fixture notice", Path: "licenses/fixture-license"}},
			ExtractorVersion: "test-extractor", NormalizationVersion: NormalizationVersion,
			RecordCount: 1,
		},
		Categories: []Category{{ID: "fixture", Label: "Token fixture", Group: "language-region", SourceLocale: "xx", Scripts: []string{"Latin"}, SupportedGenders: []Gender{}, RecordCount: 1}},
		Records:    []Record{{ID: "fixture-1", Name: "token-alpha", Categories: []string{"fixture"}, Genders: []Gender{}, SourceRefs: []SourceRef{{Revision: "rev", Path: "source.ts", Bucket: "generic", Index: 0}}}},
	}
	var err error
	bundle.Manifest.RecordsSHA256, err = RecordsHash(bundle.Records)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Manifest.CategoriesSHA256, err = CategoriesHash(bundle.Categories)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func validTestFiles(t *testing.T) map[string]*fstest.MapFile {
	t.Helper()
	bundle := testBundle(t)
	manifest, err := json.Marshal(bundle.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	categories, err := CanonicalCategories(bundle.Categories)
	if err != nil {
		t.Fatal(err)
	}
	records, err := CanonicalRecords(bundle.Records)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]*fstest.MapFile{
		"bundle/manifest.json":            {Data: append(manifest, '\n')},
		"bundle/categories.json":          {Data: categories},
		"bundle/names.jsonl":              {Data: records},
		"bundle/licenses/fixture-license": {Data: []byte("fixture notice\n")},
	}
}
