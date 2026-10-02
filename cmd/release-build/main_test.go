package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDistributionDeveloperGuide(t *testing.T) {
	t.Chdir("../..")
	files, err := distributionFiles()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("docs/DEVELOPING.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.name == "docs/DEVELOPING.md" {
			if !bytes.Equal(file.data, want) {
				t.Fatal("packaged developer guide differs from the source")
			}
			return
		}
	}
	t.Fatal("platform archive omits the developer guide linked from the README")
}

func TestTrackedSourceFilesBoundary(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	write := func(path, content string) {
		t.Helper()
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "--quiet")
	write(".gitignore", "/.local/\n/internal/ignored-private/\n")
	tracked := []string{"go.mod", "cmd/nameforge/main.go", "internal/corpus/assets/builtin/names.jsonl", "internal/legal/assets/DEPENDENCIES.txt", "docs/DATA.md", "data/sources.lock.json", ".github/workflows/release.yml"}
	for _, path := range tracked {
		write(path, "public-source-token")
	}
	git(append([]string{"add", ".gitignore"}, tracked...)...)
	// Neither ignored data nor unrelated untracked files enter the archive.
	for _, path := range []string{".local/page.html", "data/local/personal.jsonl", "exports/batch.json", "bin/nameforge", "dist/archive.tar.gz"} {
		write(path, "private-canary-token")
	}
	files, err := trackedSourceFiles(root)
	if err != nil || len(files) != len(tracked)+1 {
		t.Fatalf("tracked source files: %d entries, %v", len(files), err)
	}
	archive, err := pack("source", files, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for range files {
		header, err := reader.Next()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		expected := "public-source-token"
		if header.Name == "source/.gitignore" {
			expected = "/.local/\n/internal/ignored-private/\n"
		}
		if err != nil || string(data) != expected {
			t.Fatalf("unexpected archive content %q: %v", data, err)
		}
	}
	if _, err := reader.Next(); err != io.EOF {
		t.Fatalf("unexpected extra archive entry: %v", err)
	}
	// Git's local/global excludes must not hide source files from validation.
	write(".git/info/exclude", "/cmd/nameforge/local.go\n")
	globalExclude := filepath.Join(t.TempDir(), "global-ignore")
	if err := os.WriteFile(globalExclude, []byte("/internal/global.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("config", "core.excludesFile", globalExclude)
	for _, path := range []string{"cmd/private.txt", "internal/personal/corpus.jsonl", "internal/new.go", "docs/fetched.html", ".github/workflows/correspondence.txt", "internal/ignored-private/notes.txt", "cmd/nameforge/local.go", "internal/global.go"} {
		t.Run(path, func(t *testing.T) {
			write(path, "private-canary-token")
			if path == "internal/ignored-private/notes.txt" || path == "cmd/nameforge/local.go" || path == "internal/global.go" {
				git("check-ignore", "--quiet", path)
			}
			if _, err := trackedSourceFiles(root); err == nil || !strings.Contains(err.Error(), "untracked file in source archive boundary: "+path) {
				t.Fatalf("expected refusal for %s, got %v", path, err)
			}
			if err := os.Remove(filepath.Join(root, path)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

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
