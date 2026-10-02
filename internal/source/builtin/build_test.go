package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/source/faker"
)

type memoryCache map[string][]byte

func (c memoryCache) Read(key string) ([]byte, error) {
	b, ok := c[key]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return b, nil
}
func (c memoryCache) Write(key string, b []byte) error { c[key] = bytes.Clone(b); return nil }
func artifactFS(artifacts map[string][]byte) fstest.MapFS {
	f := fstest.MapFS{}
	for path, b := range artifacts {
		f[path] = &fstest.MapFile{Data: bytes.Clone(b)}
	}
	return f
}

// All fabricated parser/model inputs below are clearly non-name token sequences.
const entityFixture = `{"entities":{"Q1":{"id":"Q1","lastrevid":1,"claims":{
"P31":[{"id":"Q1$class","rank":"normal","mainsnak":{"snaktype":"value","datavalue":{"value":{"id":"Q12308941"}}}}],
"P407":[{"id":"Q1$language","rank":"normal","mainsnak":{"snaktype":"value","datavalue":{"value":{"id":"Q13955"}}}}],
"P1705":[{"id":"Q1$native","rank":"normal","mainsnak":{"snaktype":"value","datavalue":{"value":{"text":"ظظظ","language":"ar"}}}},
{"id":"Q1$latin","rank":"normal","mainsnak":{"snaktype":"value","datavalue":{"value":{"text":"qzxé","language":"mul"}}}}]
}}}}`

func fixture(t *testing.T) (Lock, faker.Lock, memoryCache) {
	t.Helper()
	cache := memoryCache{}
	original, err := faker.Pin(context.Background(), func(_ context.Context, path string) ([]byte, error) {
		if path == "LICENSE" {
			return []byte("non-name fixture notice\n"), nil
		}
		return []byte(`export default {female:['qzxé'],generic:['vrkt'],male:['QZXÉ']};`), nil
	}, cache)
	if err != nil {
		t.Fatal(err)
	}
	originalData, _ := Encode(original)
	var wiki strings.Builder
	for _, section := range []struct {
		Name  string
		Count int
	}{{"Ancient names", 280}, {"Biblical and Christian names", 242}} {
		wiki.WriteString("=== " + section.Name + " ===\n")
		for range section.Count {
			wiki.WriteString("#[[qzxé]] (ΩΩΩ)\n")
		}
	}
	lock, err := Pin(context.Background(), originalData, []byte(`{"items":[{"qid":"Q1","revision":1}]}`), func(_ context.Context, url string) ([]byte, error) {
		switch url {
		case GreekURL:
			return []byte(wiki.String()), nil
		case entityURL("Q1", "1"):
			return []byte(entityFixture), nil
		default:
			return []byte("non-name fixture notice\n"), nil
		}
	}, cache)
	if err != nil {
		t.Fatal(err)
	}
	return lock, original, cache
}

func TestBuildRebuildVerifyAndCorruption(t *testing.T) {
	lock, original, cache := fixture(t)
	a, err := Build(context.Background(), lock, original, cache)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(context.Background(), lock, original, cache)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("non-reproducible rebuild: %v", err)
	}
	root := artifactFS(a)
	if err := Verify(lock, original, root); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"licenses/CC0-1.0", "licenses/CC-BY-SA-4.0", "licenses/WIKIMEDIA-NOTICE", "licenses/FAKER-LICENSE"} {
		t.Run(path, func(t *testing.T) {
			f := artifactFS(a)
			f[faker.AssetDirectory+"/"+path].Data = []byte("truncated")
			if err := Verify(lock, original, f); err == nil {
				t.Fatal("corrupt notice accepted")
			}
		})
	}
	root[faker.ReportPath].Data = bytes.Replace(root[faker.ReportPath].Data, []byte(`"accepted_occurrences": 1`), []byte(`"accepted_occurrences": 2`), 1)
	if err := Verify(lock, original, root); err == nil {
		t.Fatal("corrupt quality accounting accepted")
	}
	cache[key(lock.Greek)] = []byte("corrupt")
	if _, err := Build(context.Background(), lock, original, cache); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("corrupt source: %v", err)
	}
	delete(cache, key(lock.Greek))
	if _, err := Build(context.Background(), lock, original, cache); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing source: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Build(ctx, lock, original, cache); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestWikidataRequiresClassificationAndKeepsLiteralProvenance(t *testing.T) {
	file := corpus.SourceFile{Path: "wikidata/Q1/1.json", Revision: "1"}
	entries, err := arabicOccurrences([]byte(entityFixture), file)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "qzxé" || entries[0].Ref.NativeName != "ظظظ" || entries[0].Ref.StatementID != "Q1$latin" || entries[0].Ref.Index != 1 || len(entries[0].Ref.Evidence) != 3 {
		t.Fatalf("lost statement evidence: %+v", entries)
	}
	for _, pair := range [][2]string{{`"lastrevid":1`, `"lastrevid":2`}, {`"Q13955"`, `"Q256"`}, {`"Q12308941"`, `"Q5"`}} {
		if _, err := arabicOccurrences([]byte(strings.Replace(entityFixture, pair[0], pair[1], 1)), file); err == nil {
			t.Fatalf("invalid evidence accepted: %v", pair)
		}
	}
	generic := strings.Replace(entityFixture, `"Q12308941"`, `"Q202444"`, 1)
	entries, err = arabicOccurrences([]byte(generic), file)
	if err != nil {
		t.Fatal(err)
	}
	records, _ := merge("arabic", entries)
	if len(records[0].Genders) != 0 {
		t.Fatal("unspecified became gendered")
	}
	dual := append(entries, entries[0])
	dual[0].Ref.Bucket = "male"
	dual[1].Ref.Bucket = "female"
	records, _ = merge("arabic", dual)
	if len(records[0].Genders) != 2 {
		t.Fatal("explicit gender evidence lost")
	}
	unsupported := strings.Replace(entityFixture, "qzxé", "qzxΩ", 1)
	entries, err = arabicOccurrences([]byte(unsupported), file)
	if err != nil || entries[0].Reason == "" {
		t.Fatal("non-Latin statement accepted")
	}
	wrongNative := strings.Replace(entityFixture, "ظظظ", "qzx", 1)
	entries, err = arabicOccurrences([]byte(wrongNative), file)
	if err != nil || entries[0].Reason == "" {
		t.Fatal("Latin native ar tag mistaken for Arabic script")
	}
}

func TestLiteralWikiSubsetAndNormalization(t *testing.T) {
	for input, want := range map[string]string{"[[qzx (disambiguation)]]": "qzx", "[[foreign|qzx]]/[[vrkt]]": "qzx/vrkt", "{{ill|qzx|el|ΩΩΩ}}": "qzx"} {
		got, err := wikiLabel(input)
		if err != nil || got != want {
			t.Fatalf("literal parse: %q %v", got, err)
		}
	}
	for _, input := range []string{"{{unreviewed|qzx}}", "[[qzx|vrkt|zzz]]", "[[qzx", "<script>qzx</script>"} {
		if _, err := wikiLabel(input); err == nil {
			t.Fatalf("unknown syntax accepted: %q", input)
		}
	}
	ref := corpus.SourceRef{Path: "wikipedia/Greek_name.wiki", Revision: "1", Bucket: "generic"}
	records, _ := merge("greek", []occurrence{{Name: "qzxe\u0301", Ref: ref}, {Name: "QZXÉ", Ref: ref}})
	if len(records) != 1 || records[0].Name != "qzxé" || len(records[0].SourceRefs) != 2 || len(records[0].Genders) != 0 {
		t.Fatal("NFC/dedup/unspecified contract failed")
	}
	if latinName("qzx\u05b0") || latinName("qzx\n") || latinName("qzx--vrkt") || latinName("qzx\u200e") {
		t.Fatal("unsupported characters accepted")
	}
}

func TestLockedFetchIsExplicitAndChecksumValidated(t *testing.T) {
	lock, _, populated := fixture(t)
	cache := memoryCache{}
	calls := 0
	remote := func(_ context.Context, url string) ([]byte, error) {
		calls++
		for _, file := range lock.Files() {
			if file.URL == url {
				return populated[key(file)], nil
			}
		}
		return nil, fs.ErrNotExist
	}
	if err := Fetch(context.Background(), lock, remote, cache); err != nil {
		t.Fatal(err)
	}
	if calls != len(lock.Files()) {
		t.Fatal("incorrect fetch inventory")
	}
	if err := Fetch(context.Background(), lock, func(context.Context, string) ([]byte, error) { t.Fatal("warm fetch used network"); return nil, nil }, cache); err != nil {
		t.Fatal(err)
	}
	cache[key(lock.Greek)] = []byte("bad")
	if err := Fetch(context.Background(), lock, remote, cache); err == nil {
		t.Fatal("corrupt cache accepted")
	}
	if err := Fetch(context.Background(), lock, func(context.Context, string) ([]byte, error) { return []byte("wrong"), nil }, memoryCache{}); err == nil {
		t.Fatal("corrupt download accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Fetch(ctx, lock, remote, populated); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled fetch: %v", err)
	}
	data, _ := Encode(lock)
	if _, err := DecodeLock(data); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeLock(bytes.Replace(data, []byte(`"extractor_version":`), []byte(`"extractor_version":"bad", "extractor_version":`), 1)); err == nil {
		t.Fatal("duplicate lock fields accepted")
	}
	lock.Arabic[0].URL = "https://unreviewed.example/private"
	if err := lock.Validate(); err == nil {
		t.Fatal("unreviewed URL accepted")
	}
}

func TestHTTPRemoteBoundsRetriesAndCancellation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(503) }))
	defer server.Close()
	_, err := Remote(server.Client())(context.Background(), server.URL)
	if err == nil || calls != 3 {
		t.Fatalf("retries: %d %v", calls, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = Remote(server.Client())(ctx, server.URL)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestCommittedSourcesVerifyOffline(t *testing.T) {
	root := os.DirFS("../../..")
	data, err := fs.ReadFile(root, "data/sources.lock.json")
	if err != nil {
		t.Fatal(err)
	}
	lock, err := DecodeLock(data)
	if err != nil {
		t.Fatal(err)
	}
	original, err := FakerLock(lock, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(lock, original, root); err != nil {
		t.Fatal(err)
	}
	bundle, err := corpus.Load(root, faker.AssetDirectory)
	if err != nil {
		t.Fatal(err)
	}
	greek, _ := bundle.Category("greek")
	if greek.RecordCount != 486 || len(greek.SupportedGenders) != 0 {
		t.Fatal("Greek source evidence changed")
	}
	arabic, _ := bundle.Category("arabic")
	if arabic.RecordCount != 109 {
		t.Fatal("Arabic coverage changed")
	}
	for _, record := range bundle.Records {
		if record.Categories[0] == "greek" || record.Categories[0] == "arabic" {
			for _, ref := range record.SourceRefs {
				if ref.Revision == faker.Revision {
					t.Fatal("native Faker record in replacement pack")
				}
				if _, err := strconv.ParseUint(ref.Revision, 10, 64); err != nil {
					t.Fatal("unversioned source reference")
				}
			}
		}
	}
	fixtureBytes, err := fs.ReadFile(root, FixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fixture corpus.Bundle
	if err := json.Unmarshal(fixtureBytes, &fixture); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Validate(); err != nil {
		t.Fatal(err)
	}
}
