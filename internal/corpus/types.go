// Package corpus defines the versioned built-in and local corpus contracts.
package corpus

// Gender is a source-supported gender classification. An empty classification
// means unspecified; it does not imply unisex.
type Gender string

const (
	GenderMasculine Gender = "masculine"
	GenderFeminine  Gender = "feminine"
)

// Record is one normalized spelling with all retained source occurrences.
type Record struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Categories []string    `json:"categories"`
	Genders    []Gender    `json:"genders"`
	SourceRefs []SourceRef `json:"source_refs"`
}

// SourceRef traces a spelling to its original static-array occurrence.
type SourceRef struct {
	Revision string `json:"revision"`
	Path     string `json:"path"`
	Bucket   string `json:"bucket"`
	Index    int    `json:"index"`
}

// Category describes a reviewed source-list category and observed coverage.
type Category struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Group            string   `json:"group"`
	SourceLocale     string   `json:"source_locale"`
	Scripts          []string `json:"scripts"`
	SupportedGenders []Gender `json:"supported_genders"`
	RecordCount      int      `json:"record_count"`
}

// SourceFile records a pinned upstream file used to build the corpus.
type SourceFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// LicenseRef points to a complete notice included with the corpus assets.
type LicenseRef struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Manifest identifies the corpus schema, source revision, build versions, and
// canonical content hashes. Timestamps are intentionally absent.
type Manifest struct {
	SchemaVersion        int          `json:"schema_version"`
	SourceRevision       string       `json:"source_revision"`
	Sources              []SourceFile `json:"sources"`
	Licenses             []LicenseRef `json:"licenses"`
	ExtractorVersion     string       `json:"extractor_version"`
	NormalizationVersion string       `json:"normalization_version"`
	RecordCount          int          `json:"record_count"`
	RecordsSHA256        string       `json:"records_sha256"`
	CategoriesSHA256     string       `json:"categories_sha256"`
}
