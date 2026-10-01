// Package generator defines requests and reproducibility metadata shared by
// command-line, TUI, and generation implementations.
package generator

// Mode determines whether each output is trained from one selected category
// or from the union of all selected categories.
type Mode string

const (
	ModeCategory Mode = "category"
	ModeBlend    Mode = "blend"
)

// GenderFilter selects source-supported gender labels. Any includes entries
// with or without a specified gender; Unisex requires both source labels.
type GenderFilter string

const (
	GenderAny       GenderFilter = "any"
	GenderMasculine GenderFilter = "masculine"
	GenderFeminine  GenderFilter = "feminine"
	GenderUnisex    GenderFilter = "unisex"
)

// Request contains deterministic generation inputs. A nil Seed requests a
// cryptographically sourced seed at generation time.
type Request struct {
	CategoryIDs   []string     `json:"category_ids"`
	AllCategories bool         `json:"all_categories,omitempty"`
	Mode          Mode         `json:"mode"`
	Gender        GenderFilter `json:"gender"`
	Count         int          `json:"count"`
	Order         int          `json:"order"`
	MinLength     int          `json:"min_length,omitempty"`
	MaxLength     int          `json:"max_length,omitempty"`
	Seed          *uint64      `json:"seed,omitempty"`
	AllowExisting bool         `json:"allow_existing,omitempty"`
}

// LengthBounds are the effective rune-count limits for a category/model.
type LengthBounds struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// GeneratedName is a sampled candidate and its category attribution. Category
// mode attributes one category; blend mode attributes all contributing IDs.
type GeneratedName struct {
	Name        string   `json:"name"`
	CategoryIDs []string `json:"category_ids"`
}

// Rejections counts candidates discarded by the bounded generation service.
type Rejections struct {
	Length     int `json:"length"`
	Script     int `json:"script"`
	Separators int `json:"separators"`
	Duplicates int `json:"duplicates"`
	Existing   int `json:"existing"`
	Exhausted  int `json:"exhausted"`
}

// Result records enough metadata to replay a generation request and to report
// bounded partial completion.
type Result struct {
	Seed             uint64                  `json:"seed"`
	AlgorithmVersion string                  `json:"algorithm_version"`
	BundleHash       string                  `json:"bundle_hash"`
	CategoryIDs      []string                `json:"category_ids"`
	Mode             Mode                    `json:"mode"`
	Bounds           map[string]LengthBounds `json:"bounds"`
	Options          Request                 `json:"options"`
	Names            []GeneratedName         `json:"names"`
	Attempts         int                     `json:"attempts"`
	Rejections       Rejections              `json:"rejections"`
	Complete         bool                    `json:"complete"`
}

// GenerationError reports invalid or unsatisfied generation requests. Partial
// contains accepted candidates only when bounded generation is incomplete.
type GenerationError struct {
	Kind       ErrorKind `json:"kind"`
	Message    string    `json:"message"`
	CategoryID string    `json:"category_id,omitempty"`
	Partial    *Result   `json:"partial,omitempty"`
}

// ErrorKind identifies a stable class of generation failure.
type ErrorKind string

const (
	ErrorInvalidRequest    ErrorKind = "invalid_request"
	ErrorEmptySelection    ErrorKind = "empty_selection"
	ErrorIncompatibleBlend ErrorKind = "incompatible_blend"
	ErrorUnsupportedScript ErrorKind = "unsupported_script"
	ErrorAttemptsExhausted ErrorKind = "attempts_exhausted"
	ErrorCanceled          ErrorKind = "canceled"
)

func (e *GenerationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return string(e.Kind)
}
