package faker

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

type memoryCache map[string][]byte

func (cache memoryCache) Read(key string) ([]byte, error) {
	data, ok := cache[key]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return data, nil
}
func (cache memoryCache) Write(key string, data []byte) error {
	cache[key] = bytes.Clone(data)
	return nil
}

func TestExtractionNormalizationMergingAndRejections(t *testing.T) {
	// Algorithm fixtures are non-name tokens. They never enter public assets.
	target := Targets()[0]
	records, rejects := extract(target, map[string][]string{
		"female": {" qzxe\u0301 ", "qzxé"}, "male": {"QZXÉ"},
		"generic": {"vrkt", "", "qz\nv", "qzx\u200cv", "qzx123", "-qzx", "qzx--vrk", "\u0301qzx"},
	}, Revision)
	if len(records) != 2 || len(rejects) != 7 {
		t.Fatalf("records=%#v rejects=%#v", records, rejects)
	}
	if records[0].Name != "qzxé" || len(records[0].SourceRefs) != 3 || !reflect.DeepEqual(records[0].Genders, []corpus.Gender{corpus.GenderFeminine, corpus.GenderMasculine}) {
		t.Fatalf("merged NFC/case spelling lost provenance/gender: %#v", records[0])
	}
	if records[1].Name != "vrkt" || len(records[1].Genders) != 0 {
		t.Fatalf("generic became gendered: %#v", records[1])
	}
	if got := invalidSpelling(string([]byte{0xff}), "qzx"); got != "invalid UTF-8" {
		t.Fatal(got)
	}
}

func fixtureLock(t *testing.T) (Lock, memoryCache) {
	t.Helper()
	cache := memoryCache{}
	lock, err := Pin(context.Background(), func(_ context.Context, path string) ([]byte, error) {
		if path == "LICENSE" {
			return []byte("non-name test notice\n"), nil
		}
		return []byte(`export default {female: ['qzxé'], male: ['QZXÉ'], generic: ['vrkt','qz\nv']};`), nil
	}, cache)
	if err != nil {
		t.Fatal(err)
	}
	return lock, cache
}
func artifactFS(artifacts map[string][]byte) fstest.MapFS {
	result := fstest.MapFS{}
	for path, data := range artifacts {
		result[path] = &fstest.MapFile{Data: bytes.Clone(data)}
	}
	return result
}

func TestBuildIsCanonicalOfflineAndTraceable(t *testing.T) {
	lock, cache := fixtureLock(t)
	first, err := Build(context.Background(), lock, cache)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(context.Background(), lock, cache)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("rebuild bytes differ")
	}
	root := artifactFS(first)
	if err := Verify(lock, root); err != nil {
		t.Fatal(err)
	}
	if err := Compare(second, root); err != nil {
		t.Fatal(err)
	}
	bundle, err := corpus.Load(root, AssetDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Records) != 20 {
		t.Fatalf("coverage = %d", len(bundle.Records))
	}
	for _, record := range bundle.Records {
		if record.Name == "vrkt" && len(record.Genders) != 0 {
			t.Fatal("generic became unisex")
		}
	}
	for label, mutate := range map[string]func(fstest.MapFS){
		"notice": func(root fstest.MapFS) { root[AssetDirectory+"/licenses/FAKER-LICENSE"].Data = []byte("truncated") },
		"report": func(root fstest.MapFS) {
			root[ReportPath].Data = bytes.Replace(root[ReportPath].Data, []byte(`"distinct_names": 2`), []byte(`"distinct_names": 3`), 1)
		},
		"reference": func(root fstest.MapFS) {
			root[ReportPath].Data = bytes.Replace(root[ReportPath].Data, []byte(`"index": 1`), []byte(`"index": 99`), 1)
		},
		"missing category": func(root fstest.MapFS) { delete(root, AssetDirectory+"/categories.json") },
	} {
		t.Run(label, func(t *testing.T) {
			root := artifactFS(first)
			mutate(root)
			if err := Verify(lock, root); err == nil {
				t.Fatal("corruption accepted")
			}
		})
	}
	cache[cacheKey(lock.Targets[0].Path)] = []byte("corrupt")
	if _, err := Build(context.Background(), lock, cache); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("corrupt source error=%v", err)
	}
	delete(cache, cacheKey(lock.Targets[0].Path))
	if _, err := Build(context.Background(), lock, cache); err == nil || !strings.Contains(err.Error(), "data:fetch") {
		t.Fatalf("missing array error=%v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Build(ctx, lock, cache); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled build=%v", err)
	}
}

func TestFetchValidatesChecksumsRetriesAndCancellation(t *testing.T) {
	lock, populated := fixtureLock(t)
	cache := memoryCache{}
	calls := 0
	remote := func(_ context.Context, path string) ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("transient")
		}
		return populated[cacheKey(path)], nil
	}
	if err := Fetch(context.Background(), lock, remote, cache); err != nil {
		t.Fatal(err)
	}
	if calls != len(lock.Files())+1 {
		t.Fatalf("calls = %d", calls)
	}
	if err := Fetch(context.Background(), lock, func(context.Context, string) ([]byte, error) {
		t.Fatal("valid cached fetch hit network")
		return nil, nil
	}, cache); err != nil {
		t.Fatal(err)
	}
	cache[cacheKey(lock.Targets[0].Path)] = []byte("corrupt")
	if err := Fetch(context.Background(), lock, remote, cache); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("cached mismatch=%v", err)
	}
	if err := Fetch(context.Background(), lock, func(context.Context, string) ([]byte, error) { return []byte("wrong"), nil }, memoryCache{}); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("remote mismatch=%v", err)
	}
	calls = 0
	err := Fetch(context.Background(), lock, func(context.Context, string) ([]byte, error) { calls++; return nil, errors.New("unavailable") }, memoryCache{})
	if err == nil || calls != 3 {
		t.Fatalf("unbounded retry: calls=%d error=%v", calls, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Fetch(ctx, lock, remote, memoryCache{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled fetch=%v", err)
	}
}

func TestLockFailsOnUnsupportedMetadataAndDuplicateFields(t *testing.T) {
	lock, _ := fixtureLock(t)
	data, _ := encode(lock)
	if _, err := DecodeLock(data); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeLock(bytes.Replace(data, []byte(`"schema_version": 1`), []byte(`"schema_version": 0, "schema_version": 1`), 1)); err == nil {
		t.Fatal("duplicate fields accepted")
	}
	lock.Targets[0].Path = "../private"
	if err := lock.Validate(); err == nil {
		t.Fatal("unreviewed source accepted")
	}
}
