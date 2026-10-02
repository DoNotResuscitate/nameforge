package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"testing/fstest"
	"unicode/utf8"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/source/faker"
)

const FixturePath = "internal/generator/testdata/nonlatin.json"
const Attribution = `Greek training records are adapted from "Greek name", Wikipedia contributors:
https://en.wikipedia.org/w/index.php?title=Greek_name&oldid=1377658475
Contributor history: https://en.wikipedia.org/w/index.php?title=Greek_name&action=history
Licensed under Creative Commons Attribution-ShareAlike 4.0 International:
https://creativecommons.org/licenses/by-sa/4.0/
Changes: select Ancient names and Biblical and Christian names lists, extract
supplied Latin display spellings, remove link disambiguators, separate supplied
alternatives, exclude reviewed unsuitable rows, NFC-normalize and deduplicate.
The Wikipedia-derived Greek records and adaptations remain CC BY-SA 4.0.
Other source records retain their separate notices; source licenses do not claim
linguistic accuracy or historical frequency. Generated names are not source text.

Arabic training records are adapted from revision-pinned Wikidata structured data.
Wikidata contributors release structured data under CC0 1.0:
https://www.wikidata.org/wiki/Wikidata:Licensing
https://creativecommons.org/publicdomain/zero/1.0/
Each record retains its entity/revision path, Latin statement ID, Arabic native
name associations and classification evidence. Entity URLs and raw checksums
are locked in data/sources.lock.json. No uniform romanization standard is claimed.
Changes: select explicit Arabic-language given-name entities with source-supplied
mul Latin native-label statements, hold out reviewed ambiguities, NFC-normalize
and deduplicate. No project transliteration or generated training data is used.
`

func FakerLock(lock Lock, root fs.FS) (faker.Lock, error) {
	b, err := fs.ReadFile(root, lock.FakerLock.Path)
	if err != nil {
		return faker.Lock{}, err
	}
	if Hash(b) != lock.FakerLock.SHA256 {
		return faker.Lock{}, fmt.Errorf("Faker lock checksum mismatch")
	}
	return faker.DecodeLock(b)
}
func Build(ctx context.Context, lock Lock, original faker.Lock, cache faker.Cache) (map[string][]byte, error) {
	if err := lock.Validate(); err != nil {
		return nil, err
	}
	artifacts, err := faker.Build(ctx, original, cache)
	if err != nil {
		return nil, err
	}
	root := fstest.MapFS{}
	for path, data := range artifacts {
		root[path] = &fstest.MapFile{Data: data}
	}
	bundle, err := corpus.Load(root, faker.AssetDirectory)
	if err != nil {
		return nil, err
	}
	fixture, err := nonLatinFixture(bundle)
	if err != nil {
		return nil, err
	}
	artifacts[FixturePath] = fixture
	var oldReport faker.Report
	if err := json.Unmarshal(artifacts[faker.ReportPath], &oldReport); err != nil {
		return nil, err
	}
	bundle.Records = filterRecords(bundle.Records)
	bundle.Categories = filterCategories(bundle.Categories)
	sources := []corpus.SourceFile{}
	for _, source := range bundle.Manifest.Sources {
		if !strings.Contains(source.Path, "/ar/") && !strings.Contains(source.Path, "/el/") {
			sources = append(sources, source)
		}
	}
	bundle.Manifest.Sources = append(sources, lock.Greek)
	bundle.Manifest.Sources = append(bundle.Manifest.Sources, lock.Arabic...)
	bundle.Manifest.ExtractorVersion = Version
	bundle.Manifest.Licenses = append(bundle.Manifest.Licenses, corpus.LicenseRef{Name: "Wikimedia source attribution", Path: "licenses/WIKIMEDIA-NOTICE"}, corpus.LicenseRef{Name: "Wikipedia Greek records (CC BY-SA 4.0)", Path: "licenses/CC-BY-SA-4.0"}, corpus.LicenseRef{Name: "Wikidata Arabic records (CC0 1.0)", Path: "licenses/CC0-1.0"})
	greekRaw, err := read(ctx, cache, lock.Greek)
	if err != nil {
		return nil, err
	}
	greek, err := greekOccurrences(greekRaw, lock.Greek)
	if err != nil {
		return nil, err
	}
	arabic := []occurrence{}
	for _, file := range lock.Arabic {
		raw, err := read(ctx, cache, file)
		if err != nil {
			return nil, err
		}
		entries, err := arabicOccurrences(raw, file)
		if err != nil {
			return nil, err
		}
		arabic = append(arabic, entries...)
	}
	for _, selection := range []struct {
		ID, Label, Locale string
		Occurrences       []occurrence
	}{{"arabic", "Arabic (romanized; broad Wikidata list)", "ar", arabic}, {"greek", "Greek (romanized; ancient and modern)", "el", greek}} {
		records, rejects := merge(selection.ID, selection.Occurrences)
		if len(records) == 0 {
			return nil, fmt.Errorf("no accepted %s names", selection.ID)
		}
		bundle.Records = append(bundle.Records, records...)
		oldReport.Rejections = append(oldReport.Rejections, rejects...)
		category := corpus.Category{ID: selection.ID, Label: selection.Label, SourceLocale: selection.Locale, Group: "language-region", Scripts: []string{"Latin"}, SupportedGenders: []corpus.Gender{}, RecordCount: len(records)}
		for _, g := range []corpus.Gender{corpus.GenderFeminine, corpus.GenderMasculine} {
			for _, r := range records {
				if hasGender(r, g) {
					category.SupportedGenders = append(category.SupportedGenders, g)
					break
				}
			}
		}
		bundle.Categories = append(bundle.Categories, category)
	}
	sort.Slice(bundle.Categories, func(i, j int) bool { return bundle.Categories[i].ID < bundle.Categories[j].ID })
	bundle.Manifest.RecordCount = len(bundle.Records)
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
	artifacts[faker.AssetDirectory+"/manifest.json"], err = Encode(bundle.Manifest)
	if err != nil {
		return nil, err
	}
	artifacts[faker.AssetDirectory+"/names.jsonl"], err = corpus.CanonicalRecords(bundle.Records)
	if err != nil {
		return nil, err
	}
	artifacts[faker.AssetDirectory+"/categories.json"], err = corpus.CanonicalCategories(bundle.Categories)
	if err != nil {
		return nil, err
	}
	report, err := quality(bundle, oldReport.Rejections)
	if err != nil {
		return nil, err
	}
	artifacts[faker.ReportPath], err = Encode(report)
	if err != nil {
		return nil, err
	}
	artifacts[faker.AssetDirectory+"/licenses/WIKIMEDIA-NOTICE"] = []byte(Attribution)
	for _, notice := range lock.Notices {
		data, err := read(ctx, cache, notice)
		if err != nil {
			return nil, err
		}
		artifacts[faker.AssetDirectory+"/licenses/"+notice.Path] = data
	}
	return artifacts, nil
}
func filterRecords(records []corpus.Record) []corpus.Record {
	result := []corpus.Record{}
	for _, r := range records {
		if r.Categories[0] != "arabic" && r.Categories[0] != "greek" {
			result = append(result, r)
		}
	}
	return result
}
func filterCategories(categories []corpus.Category) []corpus.Category {
	result := []corpus.Category{}
	for _, c := range categories {
		if c.ID != "arabic" && c.ID != "greek" {
			result = append(result, c)
		}
	}
	return result
}
func hasGender(r corpus.Record, g corpus.Gender) bool {
	for _, v := range r.Genders {
		if v == g {
			return true
		}
	}
	return false
}

func quality(bundle *corpus.Bundle, rejections []faker.Rejection) (faker.Report, error) {
	hash, err := corpus.BundleHash(bundle.Records, bundle.Categories)
	if err != nil {
		return faker.Report{}, err
	}
	report := faker.Report{Revision: bundle.Manifest.SourceRevision, ExtractorVersion: Version, BundleHash: hash, Categories: []faker.Coverage{}, Rejections: rejections}
	for _, category := range bundle.Categories {
		c := faker.Coverage{CategoryID: category.ID, SourceCounts: map[string]int{}, GenderCounts: map[string]int{"masculine": 0, "feminine": 0, "unisex": 0, "unspecified": 0}, Scripts: category.Scripts, MinLength: 1 << 30}
		for _, r := range bundle.Records {
			if r.Categories[0] != category.ID {
				continue
			}
			c.Distinct++
			n := utf8.RuneCountInString(r.Name)
			c.MinLength = min(c.MinLength, n)
			c.MaxLength = max(c.MaxLength, n)
			for _, ref := range r.SourceRefs {
				c.Accepted++
				c.SourceCounts[ref.Bucket]++
			}
			for _, g := range r.Genders {
				c.GenderCounts[string(g)]++
			}
			if len(r.Genders) == 0 {
				c.GenderCounts["unspecified"]++
			}
			if len(r.Genders) == 2 {
				c.GenderCounts["unisex"]++
			}
		}
		for _, rejection := range rejections {
			if categoryForPath(rejection.Ref.Path) == category.ID {
				c.Rejected++
				c.SourceCounts[rejection.Ref.Bucket]++
			}
		}
		c.Duplicates = c.Accepted - c.Distinct
		report.Categories = append(report.Categories, c)
	}
	return report, nil
}
func categoryForPath(path string) string {
	if strings.HasPrefix(path, "wikidata/") {
		return "arabic"
	}
	if strings.HasPrefix(path, "wikipedia/") {
		return "greek"
	}
	for _, t := range faker.Targets() {
		if t.Path == path {
			return t.ID
		}
	}
	return ""
}

func Verify(lock Lock, original faker.Lock, root fs.FS) error {
	if err := lock.Validate(); err != nil {
		return err
	}
	if err := original.Validate(); err != nil {
		return err
	}
	b, err := corpus.Load(root, faker.AssetDirectory)
	if err != nil {
		return err
	}
	if b.Manifest.ExtractorVersion != Version || b.Manifest.SourceRevision != original.Revision {
		return fmt.Errorf("bundle version mismatch")
	}
	wantSources := []corpus.SourceFile{}
	for _, t := range original.Targets {
		if t.ID != "greek" && t.ID != "arabic" {
			wantSources = append(wantSources, corpus.SourceFile{Path: t.Path, SHA256: t.SHA256})
		}
	}
	wantSources = append(wantSources, lock.Greek)
	wantSources = append(wantSources, lock.Arabic...)
	if !sameJSON(b.Manifest.Sources, wantSources) {
		return fmt.Errorf("manifest sources do not match lock")
	}
	wantLicenses := []corpus.LicenseRef{{Name: "Faker (MIT)", Path: "licenses/FAKER-LICENSE"}, {Name: "Wikimedia source attribution", Path: "licenses/WIKIMEDIA-NOTICE"}, {Name: "Wikipedia Greek records (CC BY-SA 4.0)", Path: "licenses/CC-BY-SA-4.0"}, {Name: "Wikidata Arabic records (CC0 1.0)", Path: "licenses/CC0-1.0"}}
	if !sameJSON(b.Manifest.Licenses, wantLicenses) {
		return fmt.Errorf("incorrect license references")
	}
	notices := append([]corpus.SourceFile{{Path: "FAKER-LICENSE", SHA256: original.License.SHA256}, {Path: "WIKIMEDIA-NOTICE", SHA256: Hash([]byte(Attribution))}}, lock.Notices...)
	for _, n := range notices {
		data, err := fs.ReadFile(root, faker.AssetDirectory+"/licenses/"+n.Path)
		if err != nil {
			return err
		}
		if Hash(data) != n.SHA256 {
			return fmt.Errorf("notice checksum mismatch: %s", n.Path)
		}
	}
	if len(b.Categories) != 10 {
		return fmt.Errorf("expected ten categories")
	}
	for _, c := range b.Categories {
		if !sameJSON(c.Scripts, []string{"Latin"}) {
			return fmt.Errorf("non-Latin category %s", c.ID)
		}
		label, locale := "", ""
		for _, t := range original.Targets {
			if t.ID == c.ID {
				label = t.Label
				locale = t.Locale
			}
		}
		if c.ID == "greek" {
			label = "Greek (romanized; ancient and modern)"
		}
		if c.ID == "arabic" {
			label = "Arabic (romanized; broad Wikidata list)"
		}
		if label == "" || c.Label != label || c.SourceLocale != locale || c.Group != "language-region" {
			return fmt.Errorf("incorrect reviewed category %s", c.ID)
		}
	}
	for _, r := range b.Records {
		if len(r.Categories) != 1 || !latinName(r.Name) {
			return fmt.Errorf("invalid training spelling %s", r.ID)
		}
		id := r.Categories[0]
		prefix := recordPrefix(id)
		if id != "arabic" && id != "greek" {
			c, _ := b.Category(id)
			prefix = "faker:" + c.SourceLocale + ":"
		}
		if r.ID != prefix+Hash([]byte(strings.ToLower(r.Name))) {
			return fmt.Errorf("incorrect stable ID")
		}
		for _, ref := range r.SourceRefs {
			if categoryForPath(ref.Path) != id {
				return fmt.Errorf("cross-category reference")
			}
			if err := verifyRef(ref, id); err != nil {
				return err
			}
		}
	}
	data, err := fs.ReadFile(root, faker.ReportPath)
	if err != nil {
		return err
	}
	var report faker.Report
	if err := json.Unmarshal(data, &report); err != nil {
		return err
	}
	if err := verifyLedger(b, report.Rejections); err != nil {
		return err
	}
	for _, r := range report.Rejections {
		if r.Reason == "" {
			return fmt.Errorf("empty rejection reason")
		}
		if categoryForPath(r.Ref.Path) == "" {
			return fmt.Errorf("unknown rejected source")
		}
		if err := verifyRef(r.Ref, categoryForPath(r.Ref.Path)); err != nil {
			return err
		}
	}
	want, err := quality(b, report.Rejections)
	if err != nil {
		return err
	}
	encoded, _ := Encode(want)
	if string(data) != string(encoded) {
		return fmt.Errorf("quality report mismatch")
	}
	return nil
}

// Account for every source occurrence once, including held-out rows. This keeps
// missing files/rows and fabricated report references from passing offline checks.
func verifyLedger(bundle *corpus.Bundle, rejected []faker.Rejection) error {
	sources := map[string]corpus.SourceFile{}
	for _, source := range bundle.Manifest.Sources {
		sources[source.Path] = source
	}
	seen := map[string]bool{}
	indices := map[string]map[int]bool{}
	rows := map[string]map[int]map[int]bool{}
	used := map[string]bool{}
	add := func(ref corpus.SourceRef, accepted bool) error {
		source, ok := sources[ref.Path]
		revision := source.Revision
		if revision == "" {
			revision = bundle.Manifest.SourceRevision
		}
		if !ok || ref.Revision != revision || ref.Index < 0 || ref.Bucket != "male" && ref.Bucket != "female" && ref.Bucket != "generic" {
			return fmt.Errorf("invalid occurrence source")
		}
		identity := fmt.Sprintf("%s/%s/%d/%s", ref.Path, ref.Bucket, ref.Index, ref.StatementID)
		if seen[identity] {
			return fmt.Errorf("duplicate occurrence %s", identity)
		}
		seen[identity] = true
		used[ref.Path] = true
		if strings.HasPrefix(ref.Path, "src/") {
			key := ref.Path + "/" + ref.Bucket
			if indices[key] == nil {
				indices[key] = map[int]bool{}
			}
			indices[key][ref.Index] = true
		}
		if strings.HasPrefix(ref.Path, "wikipedia/") {
			parts := strings.Split(ref.StatementID, "/")
			if len(parts) != 3 {
				return fmt.Errorf("invalid wiki occurrence")
			}
			variant, err := strconv.Atoi(parts[2])
			if err != nil || variant < 0 {
				return fmt.Errorf("invalid wiki alternative")
			}
			if rows[parts[0]] == nil {
				rows[parts[0]] = map[int]map[int]bool{}
			}
			if rows[parts[0]][ref.Index] == nil {
				rows[parts[0]][ref.Index] = map[int]bool{}
			}
			rows[parts[0]][ref.Index][variant] = true
			if accepted && greekHoldout(parts[0], ref.Index) != "" {
				return fmt.Errorf("held-out Greek row accepted")
			}
		}
		if strings.HasPrefix(ref.Path, "wikidata/") && accepted {
			qid := strings.Split(ref.Path, "/")[1]
			if arabicHoldout(qid) != "" || arabicStatementHoldout(ref.StatementID) != "" || ref.NativeName == "" {
				return fmt.Errorf("held-out or unassociated Arabic statement accepted")
			}
		}
		return nil
	}
	for _, record := range bundle.Records {
		for _, ref := range record.SourceRefs {
			if err := add(ref, true); err != nil {
				return err
			}
		}
	}
	for _, rejection := range rejected {
		if err := add(rejection.Ref, false); err != nil {
			return err
		}
	}
	for path := range sources {
		if !used[path] {
			return fmt.Errorf("unaccounted source file %s", path)
		}
	}
	for _, bucket := range indices {
		for i := 0; i < len(bucket); i++ {
			if !bucket[i] {
				return fmt.Errorf("non-contiguous Faker indices")
			}
		}
	}
	for section, count := range map[string]int{"Ancient names": 280, "Biblical and Christian names": 242} {
		if len(rows[section]) != count {
			return fmt.Errorf("missing Greek source rows")
		}
		for i := 0; i < count; i++ {
			variants := rows[section][i]
			if len(variants) == 0 {
				return fmt.Errorf("missing Greek row")
			}
			for v := 0; v < len(variants); v++ {
				if !variants[v] {
					return fmt.Errorf("missing Greek alternative")
				}
			}
		}
	}
	return nil
}
func verifyRef(ref corpus.SourceRef, category string) error {
	if category == "greek" {
		parts := strings.Split(ref.StatementID, "/")
		if len(parts) != 3 || ref.Bucket != "generic" || ref.NativeName == "" || parts[1] != strconv.Itoa(ref.Index) {
			return fmt.Errorf("invalid Greek provenance")
		}
		limit := 0
		switch parts[0] {
		case "Ancient names":
			limit = 280
		case "Biblical and Christian names":
			limit = 242
		}
		if ref.Index < 0 || ref.Index >= limit {
			return fmt.Errorf("invalid Greek row")
		}
	}
	if category == "arabic" {
		m := entityPath.FindStringSubmatch(ref.Path)
		if m == nil || !strings.EqualFold(strings.Split(ref.StatementID, "$")[0], m[1]) || len(ref.Evidence) == 0 {
			return fmt.Errorf("invalid Arabic statement provenance")
		}
		for _, id := range ref.Evidence {
			if !strings.EqualFold(strings.Split(id, "$")[0], m[1]) {
				return fmt.Errorf("cross-entity evidence")
			}
		}
	}
	return nil
}

// Two exact sourced native-script records remain as unsupported-script fixtures,
// separate from the runtime training packs. Rebuilds reproduce this fixture too.
func nonLatinFixture(original *corpus.Bundle) ([]byte, error) {
	fixture := &corpus.Bundle{Manifest: original.Manifest}
	fixture.Manifest.Sources = []corpus.SourceFile{}
	for _, id := range []string{"arabic", "greek"} {
		c, _ := original.Category(id)
		for _, r := range original.Records {
			if r.Categories[0] == id {
				fixture.Records = append(fixture.Records, r)
				c.RecordCount = 1
				c.SupportedGenders = r.Genders
				fixture.Categories = append(fixture.Categories, c)
				for _, s := range original.Manifest.Sources {
					if s.Path == r.SourceRefs[0].Path {
						fixture.Manifest.Sources = append(fixture.Manifest.Sources, s)
					}
				}
				break
			}
		}
	}
	fixture.Manifest.RecordCount = len(fixture.Records)
	var err error
	fixture.Manifest.RecordsSHA256, err = corpus.RecordsHash(fixture.Records)
	if err != nil {
		return nil, err
	}
	fixture.Manifest.CategoriesSHA256, err = corpus.CategoriesHash(fixture.Categories)
	if err != nil {
		return nil, err
	}
	if err := fixture.Validate(); err != nil {
		return nil, err
	}
	return Encode(fixture)
}
