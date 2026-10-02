package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"testing"
	"time"
)

func TestArchiveReproducibilityAndSafety(t *testing.T) {
	date := time.Unix(1234567890, 0).UTC()
	files := []entry{{"nameforge", []byte("non-binary-test-token"), 0o755}, {"LICENSE", []byte("notice-test-token"), 0o644}}
	one, err := pack("nameforge_dev_linux_amd64", files, date)
	if err != nil {
		t.Fatal(err)
	}
	two, err := pack("nameforge_dev_linux_amd64", []entry{files[1], files[0]}, date)
	if err != nil || !bytes.Equal(one, two) {
		t.Fatalf("archives depend on entry order: %v", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(one))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for _, expected := range []entry{files[1], files[0]} {
		header, err := reader.Next()
		if err != nil || header.Name != "nameforge_dev_linux_amd64/"+expected.name || header.Mode != expected.mode || !header.ModTime.Equal(date) || header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" {
			t.Fatalf("unsafe or incorrect header: %+v, %v", header, err)
		}
		data, err := io.ReadAll(reader)
		if err != nil || !bytes.Equal(data, expected.data) {
			t.Fatal("archive content changed")
		}
	}
	if _, err := reader.Next(); err != io.EOF {
		t.Fatal("unexpected extra archive content")
	}
	for _, bad := range []string{"../private", "/private", "data/../private", `data\private`, ""} {
		if _, err := pack("root", []entry{{bad, nil, 0o644}}, date); err == nil {
			t.Fatalf("accepted unsafe archive path %q", bad)
		}
	}
	if _, err := pack("root", []entry{files[0], files[0]}, date); err == nil {
		t.Fatal("accepted duplicate path")
	}
}

func TestPublicSourceBoundary(t *testing.T) {
	for _, path := range []string{".local/faker/page.html", "data/local/personal.jsonl", "exports/names.json", "dist/nameforge", "bin/nameforge", "mise.local.toml", ".git/config", "data/research.json"} {
		if publicSourcePath(path) {
			t.Errorf("private or generated path allowed: %s", path)
		}
	}
	for _, path := range []string{"go.mod", "go.sum", "mise.toml", "LICENSE", "cmd/nameforge/main.go", "internal/corpus/assets/builtin/names.jsonl", "internal/legal/assets/DEPENDENCIES.txt", "data/sources.lock.json", ".github/workflows/release.yml"} {
		if !publicSourcePath(path) {
			t.Errorf("missing corresponding source: %s", path)
		}
	}
}
