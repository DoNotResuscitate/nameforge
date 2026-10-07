package faker

import (
	"context"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

func TestCommittedSurnameCorpus(t *testing.T) {
	root := os.DirFS("../../..")
	data, err := fs.ReadFile(root, "data/surnames.lock.json")
	if err != nil {
		t.Fatal(err)
	}
	lock, err := DecodeLock(data)
	if err != nil {
		t.Fatal(err)
	}
	if lock.NameType != "surname" {
		t.Fatal("wrong source type")
	}
	if err := Verify(lock, root); err != nil {
		t.Fatal(err)
	}
	assets, _ := artifactPaths(lock)
	bundle, err := corpus.Load(root, assets)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Records) != 5728 || len(bundle.Categories) != 8 {
		t.Fatal("unreviewed coverage")
	}
	for _, category := range bundle.Categories {
		if strings.Join(category.Scripts, ",") != "Latin" {
			t.Fatal("non-Latin surname pack")
		}
	}
	for _, record := range bundle.Records {
		if len(record.Genders) != 0 {
			t.Fatal("generic surname classified as gendered")
		}
		for _, ref := range record.SourceRefs {
			if !strings.HasSuffix(ref.Path, "/last_name.ts") || ref.Bucket != "generic" {
				t.Fatal("wrong training provenance")
			}
		}
	}
}

func TestSurnameMaintenanceUsesSeparateArtifacts(t *testing.T) {
	cache := memoryCache{}
	lock, err := PinSurnames(context.Background(), func(_ context.Context, path string) ([]byte, error) {
		if path == "LICENSE" {
			return []byte("non-name test notice\n"), nil
		}
		if !strings.HasSuffix(path, "/last_name.ts") {
			t.Fatalf("unexpected source %s", path)
		}
		return []byte(`export default {generic: ['qzx','vrkt']};`), nil
	}, cache)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Build(context.Background(), lock, cache)
	if err != nil {
		t.Fatal(err)
	}
	root := artifactFS(first)
	if err := Verify(lock, root); err != nil {
		t.Fatal(err)
	}
	second, err := Build(context.Background(), lock, cache)
	if err != nil {
		t.Fatal(err)
	}
	if err := Compare(second, root); err != nil {
		t.Fatal(err)
	}
	if _, ok := first[AssetDirectory+"/names.jsonl"]; ok {
		t.Fatal("overwrote given-name pack")
	}
	root["internal/corpus/assets/surnames/licenses/FAKER-LICENSE"].Data = []byte("truncated")
	if err := Verify(lock, root); err == nil {
		t.Fatal("truncated license accepted")
	}
	cache[cacheKey(lock.Targets[0].Path)] = []byte("corrupt")
	if _, err := Build(context.Background(), lock, cache); err == nil {
		t.Fatal("corrupt source accepted")
	}
	lock.Targets[0].Path = "src/locales/en/person/last_name.ts"
	if err := lock.Validate(); err == nil {
		t.Fatal("implicit English replacement accepted")
	}
}
