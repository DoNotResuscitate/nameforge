package faker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

const (
	Repository       = "https://github.com/faker-js/faker"
	Revision         = "2cb04231a6ace91a59ebe577c653f4ec66478ca3"
	ExtractorVersion = "faker-static-v1"
	maxSourceBytes   = 8 << 20
)

type Target struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Locale string `json:"locale"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Lock struct {
	NameType             string            `json:"name_type,omitempty"`
	Repository           string            `json:"repository"`
	Revision             string            `json:"revision"`
	Release              string            `json:"release"`
	SchemaVersion        int               `json:"schema_version"`
	ExtractorVersion     string            `json:"extractor_version"`
	NormalizationVersion string            `json:"normalization_version"`
	Targets              []Target          `json:"targets"`
	License              corpus.SourceFile `json:"license"`
}

// Targets is reviewed metadata, never inferred from spelling or fallback locales.
func Targets() []Target {
	values := []Target{
		{ID: "arabic", Label: "Arabic (broad source list)", Locale: "ar"},
		{ID: "dutch", Label: "Dutch", Locale: "nl"},
		{ID: "english", Label: "English", Locale: "en"},
		{ID: "french", Label: "French", Locale: "fr"},
		{ID: "german", Label: "German", Locale: "de"},
		{ID: "greek", Label: "Greek", Locale: "el"},
		{ID: "italian", Label: "Italian", Locale: "it"},
		{ID: "portuguese-pt", Label: "Portuguese (Portugal)", Locale: "pt_PT"},
		{ID: "spanish", Label: "Spanish", Locale: "es"},
		{ID: "turkish", Label: "Turkish", Locale: "tr"},
	}
	for i := range values {
		values[i].Path = "src/locales/" + values[i].Locale + "/person/first_name.ts"
	}
	return values
}

// SurnameTargets excludes native-script Greek/Arabic arrays: no romanization
// or fallback is permitted. Labels describe Faker locales, not ancestry.
func SurnameTargets() []Target {
	var targets []Target
	for _, target := range Targets() {
		if target.ID == "greek" || target.ID == "arabic" {
			continue
		}
		target.Path = "src/locales/" + target.Locale + "/person/last_name.ts"
		targets = append(targets, target)
	}
	return targets
}

func DecodeLock(data []byte) (Lock, error) {
	var lock Lock
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&lock); err != nil {
		return lock, err
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return lock, fmt.Errorf("trailing lock data")
	}
	canonical, err := encode(lock)
	if err != nil {
		return lock, err
	}
	if !bytes.Equal(data, canonical) {
		return lock, fmt.Errorf("source lock must be canonical JSON with no duplicate fields")
	}
	return lock, lock.Validate()
}
func (lock Lock) Validate() error {
	if lock.Repository != Repository || lock.Revision != Revision || lock.Release != "v10.6.0" || lock.SchemaVersion != corpus.SchemaVersion || lock.ExtractorVersion != ExtractorVersion || lock.NormalizationVersion != corpus.NormalizationVersion {
		return errors.New("unsupported source lock identity/version")
	}
	if lock.License.Path != "LICENSE" || !validHash(lock.License.SHA256) {
		return errors.New("invalid locked license")
	}
	expected := Targets()
	if lock.NameType == "surname" {
		expected = SurnameTargets()
	} else if lock.NameType != "" {
		return errors.New("unsupported locked name type")
	}
	if len(lock.Targets) != len(expected) {
		return errors.New("source lock must cover all reviewed targets")
	}
	for i, target := range lock.Targets {
		want := expected[i]
		if target.ID != want.ID || target.Label != want.Label || target.Locale != want.Locale || target.Path != want.Path || !validHash(target.SHA256) {
			return fmt.Errorf("invalid locked target %q (expected %q)", target.ID, want.ID)
		}
	}
	return nil
}
func validHash(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32 && len(s) == 64 && s == strings.ToLower(s)
}
func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func (lock Lock) Files() []corpus.SourceFile {
	files := []corpus.SourceFile{lock.License}
	for _, target := range lock.Targets {
		files = append(files, corpus.SourceFile{Path: target.Path, SHA256: target.SHA256})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

// Cache and Remote are injected maintenance-only boundaries. The cache key is
// revision/path, so refreshes cannot accidentally reuse another release's files.
type Cache interface {
	Read(key string) ([]byte, error)
	Write(key string, data []byte) error
}
type Remote func(context.Context, string) ([]byte, error)

func cacheKey(path string) string { return Revision + "/" + path }

// HTTPRemote bounds every request and response. Fetch supplies bounded retries.
func HTTPRemote(client *http.Client) Remote {
	return func(ctx context.Context, path string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://raw.githubusercontent.com/faker-js/faker/"+Revision+"/"+path, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch %s: HTTP %d", path, resp.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxSourceBytes+1))
		if err != nil {
			return nil, err
		}
		if len(data) > maxSourceBytes {
			return nil, fmt.Errorf("source %s exceeds size limit", path)
		}
		return data, nil
	}
}

// Pin is an explicit maintainer operation for establishing checksums at the
// reviewed immutable revision; it never runs during ordinary fetch/build/verify.
func Pin(ctx context.Context, remote Remote, cache Cache) (Lock, error) {
	lock := Lock{Repository: Repository, Revision: Revision, Release: "v10.6.0", SchemaVersion: corpus.SchemaVersion, ExtractorVersion: ExtractorVersion, NormalizationVersion: corpus.NormalizationVersion, Targets: Targets(), License: corpus.SourceFile{Path: "LICENSE"}}
	return pin(ctx, remote, cache, lock)
}

func PinSurnames(ctx context.Context, remote Remote, cache Cache) (Lock, error) {
	lock := Lock{NameType: "surname", Repository: Repository, Revision: Revision, Release: "v10.6.0", SchemaVersion: corpus.SchemaVersion, ExtractorVersion: ExtractorVersion, NormalizationVersion: corpus.NormalizationVersion, Targets: SurnameTargets(), License: corpus.SourceFile{Path: "LICENSE"}}
	return pin(ctx, remote, cache, lock)
}

func pin(ctx context.Context, remote Remote, cache Cache, lock Lock) (Lock, error) {
	for i := range lock.Targets {
		data, err := remote(ctx, lock.Targets[i].Path)
		if err != nil {
			return Lock{}, err
		}
		if _, err := Parse(data); err != nil {
			return Lock{}, fmt.Errorf("%s: %w", lock.Targets[i].Path, err)
		}
		lock.Targets[i].SHA256 = hash(data)
		if err := cache.Write(cacheKey(lock.Targets[i].Path), data); err != nil {
			return Lock{}, err
		}
	}
	data, err := remote(ctx, lock.License.Path)
	if err != nil {
		return Lock{}, err
	}
	lock.License.SHA256 = hash(data)
	if err := cache.Write(cacheKey(lock.License.Path), data); err != nil {
		return Lock{}, err
	}
	return lock, lock.Validate()
}

func Fetch(ctx context.Context, lock Lock, remote Remote, cache Cache) error {
	if err := lock.Validate(); err != nil {
		return err
	}
	for _, file := range lock.Files() {
		if err := ctx.Err(); err != nil {
			return err
		}
		data, err := cache.Read(cacheKey(file.Path))
		if err == nil {
			if hash(data) != file.SHA256 {
				return fmt.Errorf("cached checksum mismatch for %s; remove the corrupt cache entry explicitly", file.Path)
			}
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("read cache %s: %w", file.Path, err)
		}
		for attempt := 0; attempt < 3; attempt++ {
			data, err = remote(ctx, file.Path)
			if err == nil {
				break
			}
			if attempt < 2 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(attempt+1) * 100 * time.Millisecond):
				}
			}
		}
		if err != nil {
			return err
		}
		if hash(data) != file.SHA256 {
			return fmt.Errorf("download checksum mismatch for %s", file.Path)
		}
		if err := cache.Write(cacheKey(file.Path), data); err != nil {
			return err
		}
	}
	return nil
}

func readLocked(ctx context.Context, cache Cache, file corpus.SourceFile) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := cache.Read(cacheKey(file.Path))
	if err != nil {
		return nil, fmt.Errorf("read locked cache %s (run data:fetch): %w", file.Path, err)
	}
	if hash(data) != file.SHA256 {
		return nil, fmt.Errorf("cached checksum mismatch for %s", file.Path)
	}
	return data, nil
}
