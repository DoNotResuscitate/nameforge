// Command release-build makes explicit, reproducible distribution archives.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

var targets = []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"}

type entry struct {
	name string
	data []byte
	mode int64
}

func main() {
	version := flag.String("version", "dev", "release tag (vX.Y.Z[-prerelease]) or dev")
	out := flag.String("out", "dist", "output directory")
	flag.Parse()
	if err := run(*version, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func command(args ...string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func run(version, out string) error {
	if !regexp.MustCompile(`^(dev|v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?)$`).MatchString(version) {
		return fmt.Errorf("invalid release version %q", version)
	}
	commitData, err := command("git", "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	commit := strings.TrimSpace(string(commitData))
	timestamp, err := command("git", "show", "-s", "--format=%ct", "HEAD")
	if err != nil {
		return err
	}
	epoch, err := strconv.ParseInt(strings.TrimSpace(string(timestamp)), 10, 64)
	if err != nil {
		return err
	}
	date := time.Unix(epoch, 0).UTC()
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		return err
	}
	bundleHash, err := corpus.BundleHash(bundle.Records, bundle.Categories)
	if err != nil {
		return err
	}
	// A private temporary staging area prevents stale files in dist from being
	// swept into an archive or its checksums. Only explicit entries are packed.
	stage, err := os.MkdirTemp("", "nameforge-release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	common, err := distributionFiles()
	if err != nil {
		return err
	}
	sourceName := "nameforge_" + version + "_source.tar.gz"
	sourceURL := "https://github.com/DoNotResuscitate/nameforge/releases/download/" + version + "/" + sourceName
	common = append(common, entry{"SOURCE.txt", []byte(fmt.Sprintf("Nameforge %s\nCommit: %s\nCorresponding source (including vendored dependencies and build instructions):\n%s\nRevision: https://github.com/DoNotResuscitate/nameforge/tree/%s\n\nDistributed under GNU GPLv3; see LICENSE. Corpus data keeps its separate\nMIT / CC BY-SA 4.0 / CC0 terms and exact attribution in licenses/.\n", version, commit, sourceURL, commit)), 0o644})
	var archives []entry
	for _, target := range targets {
		parts := strings.Split(target, "/")
		binary := filepath.Join(stage, "nameforge")
		ldflags := "-X github.com/DoNotResuscitate/nameforge/internal/cli.version=" + version + " -X github.com/DoNotResuscitate/nameforge/internal/cli.commit=" + commit
		cmd := exec.Command("go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", binary, "./cmd/nameforge")
		cmd.Env = append(os.Environ(), "GOOS="+parts[0], "GOARCH="+parts[1], "CGO_ENABLED=0", "GOTOOLCHAIN=local", "GOAMD64=v1", "GOARM64=v8.0")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("build %s: %w", target, err)
		}
		if err := auditBinary(binary, parts[0], parts[1]); err != nil {
			return err
		}
		data, err := os.ReadFile(binary)
		if err != nil {
			return err
		}
		metadata, err := json.MarshalIndent(struct {
			Version, Commit, Target, GoVersion, BundleHash string
			CGOEnabled                                     bool
		}{version, commit, target, "go1.27.1", bundleHash, false}, "", "  ")
		if err != nil {
			return err
		}
		files := append([]entry(nil), common...)
		files = append(files, entry{"nameforge", data, 0o755}, entry{"BUILD.json", append(metadata, '\n'), 0o644})
		name := "nameforge_" + version + "_" + parts[0] + "_" + parts[1]
		archive, err := pack(name, files, date)
		if err != nil {
			return err
		}
		archives = append(archives, entry{name + ".tar.gz", archive, 0o644})
		fmt.Println("built", target)
	}
	source, err := sourceFiles(stage)
	if err != nil {
		return err
	}
	sourceArchive, err := pack("nameforge_"+version+"_source", source, date)
	if err != nil {
		return err
	}
	archives = append(archives, entry{sourceName, sourceArchive, 0o644})
	sort.Slice(archives, func(i, j int) bool { return archives[i].name < archives[j].name })
	var checksums strings.Builder
	for _, archive := range archives {
		fmt.Fprintf(&checksums, "%x  %s\n", sha256.Sum256(archive.data), archive.name)
	}
	archives = append(archives, entry{"SHA256SUMS", []byte(checksums.String()), 0o644})
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for _, archive := range archives {
		if err := os.WriteFile(filepath.Join(out, archive.name), archive.data, fs.FileMode(archive.mode)); err != nil {
			return err
		}
	}
	return nil
}

func distributionFiles() ([]entry, error) {
	files := map[string]string{
		"LICENSE": "LICENSE", "README.md": "README.md", "AGENTS.md": "AGENTS.md",
		"DEPENDENCIES.txt":       "internal/legal/assets/DEPENDENCIES.txt",
		"data/sources.lock.json": "data/sources.lock.json", "data/faker.lock.json": "data/faker.lock.json",
		"data/quality.json":    "data/quality.json",
		"data/manifest.json":   "internal/corpus/assets/builtin/manifest.json",
		"data/categories.json": "internal/corpus/assets/builtin/categories.json",
	}
	for _, name := range []string{"CLI", "TUI", "COVERAGE", "DATA", "ROMANIZED", "ARCHITECTURE", "RELEASE", "PLAN", "DEVELOPING"} {
		files["docs/"+name+".md"] = "docs/" + name + ".md"
	}
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		return nil, err
	}
	for _, notice := range bundle.Manifest.Licenses {
		files[notice.Path] = "internal/corpus/assets/builtin/" + notice.Path
	}
	var result []entry
	for name, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		result = append(result, entry{name, data, 0o644})
	}
	return result, nil
}

func sourceFiles(stage string) ([]entry, error) {
	// Git's public file set plus an explicit path allowlist: ignored corpora,
	// fetched pages, exports, build outputs and home configuration cannot enter.
	data, err := command("git", "ls-files", "-z", "-co", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	var result []entry
	for _, path := range strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00") {
		if !publicSourcePath(path) {
			continue
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("source entry is not a regular file: %s", path)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		result = append(result, entry{path, content, 0o644})
	}
	vendor := filepath.Join(stage, "vendor")
	if _, err := command("go", "mod", "vendor", "-o", vendor); err != nil {
		return nil, err
	}
	err = filepath.WalkDir(vendor, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("non-regular vendor entry: %s", path)
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result = append(result, entry{filepath.ToSlash(rel), content, 0o644})
		return nil
	})
	return result, err
}

func publicSourcePath(path string) bool {
	for _, file := range []string{"LICENSE", "README.md", "AGENTS.md", "go.mod", "go.sum", "mise.toml", ".gitignore", "data/sources.lock.json", "data/faker.lock.json", "data/quality.json"} {
		if path == file {
			return true
		}
	}
	for _, prefix := range []string{"cmd/", "internal/", "docs/", ".github/workflows/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func auditBinary(path, goos, goarch string) error {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return err
	}
	settings := make(map[string]string)
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if info.GoVersion != "go1.27.1" || settings["GOOS"] != goos || settings["GOARCH"] != goarch || settings["CGO_ENABLED"] != "0" || settings["-trimpath"] != "true" {
		return fmt.Errorf("unexpected toolchain or build settings for %s/%s: %+v", goos, goarch, info)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if bytes.Contains(data, []byte(root+string(filepath.Separator))) {
		return fmt.Errorf("binary contains private checkout path")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "/" && bytes.Contains(data, []byte(home+string(filepath.Separator))) {
		return fmt.Errorf("binary contains private home/toolchain path")
	}
	return nil
}

func pack(root string, files []entry, date time.Time) ([]byte, error) {
	files = append([]entry(nil), files...)
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	var output bytes.Buffer
	gz := gzip.NewWriter(&output)
	tarball := tar.NewWriter(gz)
	for i, file := range files {
		if !fs.ValidPath(file.name) || strings.Contains(file.name, "\\") || i > 0 && files[i-1].name == file.name {
			return nil, fmt.Errorf("invalid or duplicate archive path: %q", file.name)
		}
		if err := tarball.WriteHeader(&tar.Header{Name: root + "/" + file.name, Mode: file.mode, Size: int64(len(file.data)), ModTime: date, Typeflag: tar.TypeReg, Format: tar.FormatPAX}); err != nil {
			return nil, err
		}
		if _, err := tarball.Write(file.data); err != nil {
			return nil, err
		}
	}
	if err := tarball.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
