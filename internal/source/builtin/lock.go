// Package builtin composes reviewed, revision-pinned sources at maintenance time.
// The runtime never imports this package.
package builtin

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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/source/faker"
)

const Version = "multisource-static-v1"
const FakerLockPath = "data/faker.lock.json"
const GreekURL = "https://en.wikipedia.org/w/index.php?title=Greek_name&oldid=1377658475&action=raw"
const CC0URL = "https://creativecommons.org/publicdomain/zero/1.0/legalcode.txt"
const BYSAURL = "https://creativecommons.org/licenses/by-sa/4.0/legalcode.txt"

type Lock struct {
	ExtractorVersion string              `json:"extractor_version"`
	FakerLock        corpus.SourceFile   `json:"faker_lock"`
	Greek            corpus.SourceFile   `json:"greek"`
	Arabic           []corpus.SourceFile `json:"arabic"`
	Notices          []corpus.SourceFile `json:"notices"`
}

func Hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func Encode(value any) ([]byte, error) {
	b, err := json.MarshalIndent(value, "", "  ")
	return append(b, '\n'), err
}
func DecodeLock(data []byte) (Lock, error) {
	var lock Lock
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&lock); err != nil {
		return lock, err
	}
	b, _ := Encode(lock)
	if !bytes.Equal(data, b) {
		return lock, fmt.Errorf("source lock must be canonical JSON")
	}
	return lock, lock.Validate()
}

var entityPath = regexp.MustCompile(`^wikidata/(Q[1-9][0-9]*)/([1-9][0-9]*)\.json$`)

func entityURL(qid, revision string) string {
	return "https://www.wikidata.org/wiki/Special:EntityData/" + qid + ".json?revision=" + revision
}
func (lock Lock) Validate() error {
	if lock.ExtractorVersion != Version || lock.FakerLock.Path != FakerLockPath || lock.Greek.Path != "wikipedia/Greek_name.wiki" || lock.Greek.Revision != "1377658475" || lock.Greek.URL != GreekURL || len(lock.Arabic) == 0 || len(lock.Notices) != 2 {
		return fmt.Errorf("unsupported reviewed source lock")
	}
	if lock.Notices[0].Path != "CC0-1.0" || lock.Notices[0].URL != CC0URL || lock.Notices[1].Path != "CC-BY-SA-4.0" || lock.Notices[1].URL != BYSAURL {
		return fmt.Errorf("incorrect notices")
	}
	previous := ""
	for _, file := range lock.Arabic {
		m := entityPath.FindStringSubmatch(file.Path)
		if m == nil || file.Revision != m[2] || file.URL != entityURL(m[1], m[2]) || file.Path <= previous {
			return fmt.Errorf("invalid/unsorted entity source %s", file.Path)
		}
		previous = file.Path
	}
	for _, file := range append(lock.Files(), lock.FakerLock) {
		decoded, err := hex.DecodeString(file.SHA256)
		if err != nil || len(decoded) != 32 || file.SHA256 != strings.ToLower(file.SHA256) {
			return fmt.Errorf("invalid checksum for %s", file.Path)
		}
	}
	return nil
}
func (lock Lock) Files() []corpus.SourceFile {
	files := []corpus.SourceFile{lock.Greek}
	files = append(files, lock.Arabic...)
	return append(files, lock.Notices...)
}
func key(file corpus.SourceFile) string { return "supplemental/" + file.Path + "/" + file.Revision }
func read(ctx context.Context, cache faker.Cache, file corpus.SourceFile) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := cache.Read(key(file))
	if err != nil {
		return nil, fmt.Errorf("read %s (run data:fetch): %w", file.Path, err)
	}
	if Hash(b) != file.SHA256 {
		return nil, fmt.Errorf("checksum mismatch for %s", file.Path)
	}
	return b, nil
}

// Remote uses explicit locked HTTPS URLs and bounds response size, time and retries.
func Remote(client *http.Client) faker.Remote {
	return func(ctx context.Context, url string) ([]byte, error) {
		var last error
		for attempt := 0; attempt < 3; attempt++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			b, err := request(ctx, client, url)
			if err == nil {
				return b, nil
			}
			last = err
			if attempt < 2 {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(attempt+1) * time.Second):
				}
			}
		}
		return nil, last
	}
}
func request(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Nameforge corpus maintenance (https://github.com/DoNotResuscitate/nameforge)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 8<<20 {
		return nil, fmt.Errorf("source exceeds size limit")
	}
	return b, nil
}
func Fetch(ctx context.Context, lock Lock, remote faker.Remote, cache faker.Cache) error {
	if err := lock.Validate(); err != nil {
		return err
	}
	for _, file := range lock.Files() {
		_, err := read(ctx, cache, file)
		if err == nil {
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		b, err := remote(ctx, file.URL)
		if err != nil {
			return err
		}
		if Hash(b) != file.SHA256 {
			return fmt.Errorf("download checksum mismatch for %s", file.Path)
		}
		if err := cache.Write(key(file), b); err != nil {
			return err
		}
	}
	return nil
}

// Pin establishes a new lock from a research roster containing QIDs/revisions.
// It copies no spellings from that roster: only pinned upstream entity JSON is read.
func Pin(ctx context.Context, fakerLock []byte, roster []byte, remote faker.Remote, cache faker.Cache) (Lock, error) {
	var discovery struct {
		Items []struct {
			QID      string `json:"qid"`
			Revision int64  `json:"revision"`
		} `json:"items"`
	}
	if err := json.Unmarshal(roster, &discovery); err != nil {
		return Lock{}, err
	}
	lock := Lock{ExtractorVersion: Version, FakerLock: corpus.SourceFile{Path: FakerLockPath, SHA256: Hash(fakerLock)}, Greek: corpus.SourceFile{Path: "wikipedia/Greek_name.wiki", Revision: "1377658475", URL: GreekURL}, Notices: []corpus.SourceFile{{Path: "CC0-1.0", URL: CC0URL}, {Path: "CC-BY-SA-4.0", URL: BYSAURL}}}
	for _, item := range discovery.Items {
		if item.Revision < 1 || !regexp.MustCompile(`^Q[1-9][0-9]*$`).MatchString(item.QID) {
			return Lock{}, fmt.Errorf("invalid discovery entity")
		}
		rev := strconv.FormatInt(item.Revision, 10)
		lock.Arabic = append(lock.Arabic, corpus.SourceFile{Path: "wikidata/" + item.QID + "/" + rev + ".json", Revision: rev, URL: entityURL(item.QID, rev)})
	}
	sort.Slice(lock.Arabic, func(i, j int) bool { return lock.Arabic[i].Path < lock.Arabic[j].Path })
	files := lock.Files()
	for i := range files {
		if err := ctx.Err(); err != nil {
			return Lock{}, err
		}
		file := &files[i]
		b, err := cache.Read(key(*file))
		if errors.Is(err, fs.ErrNotExist) {
			b, err = remote(ctx, file.URL)
			if err == nil {
				err = cache.Write(key(*file), b)
			}
		}
		if err != nil {
			return Lock{}, err
		}
		file.SHA256 = Hash(b)
		if entityPath.MatchString(file.Path) {
			if _, err := decodeEntity(b, *file); err != nil {
				return Lock{}, err
			}
		}
		fmt.Printf("pinned %s\n", file.Path)
	}
	lock.Greek = files[0]
	copy(lock.Arabic, files[1:1+len(lock.Arabic)])
	copy(lock.Notices, files[1+len(lock.Arabic):])
	return lock, lock.Validate()
}
