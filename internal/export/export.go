// Package export encodes generation results without terminal formatting.
package export

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/DoNotResuscitate/nameforge/internal/generator"
)

const SchemaVersion = 1

// Result is the versioned JSON wire format, shared by CLI and future TUI exports.
type Result struct {
	SchemaVersion int `json:"schema_version"`
	generator.Result
}

func WriteJSON(w io.Writer, result generator.Result) error {
	return json.NewEncoder(w).Encode(Result{SchemaVersion: SchemaVersion, Result: result})
}

// WriteText writes exactly one accepted name per line.
func WriteText(w io.Writer, result generator.Result) error {
	for _, name := range result.Names {
		if _, err := fmt.Fprintln(w, name.Name); err != nil {
			return err
		}
	}
	return nil
}

// WriteMetadata emits the same reproduction fields as JSON, omitting names.
func WriteMetadata(w io.Writer, result generator.Result) error {
	metadata := struct {
		Result
		Names []generator.GeneratedName `json:"names,omitempty"`
	}{Result: Result{SchemaVersion: SchemaVersion, Result: result}}
	return json.NewEncoder(w).Encode(metadata)
}
