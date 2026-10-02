package corpus

import (
	"embed"
	"fmt"
	"io/fs"
)

// Only the explicit redistributable bundle files are embedded.
//
//go:embed assets/builtin/manifest.json assets/builtin/categories.json assets/builtin/names.jsonl assets/builtin/licenses/FAKER-LICENSE assets/builtin/licenses/WIKIMEDIA-NOTICE assets/builtin/licenses/CC0-1.0 assets/builtin/licenses/CC-BY-SA-4.0
var builtinAssets embed.FS

// LoadBuiltin validates and returns the bundled corpus without filesystem writes.
func LoadBuiltin() (*Bundle, error) {
	bundle, err := Load(builtinAssets, "assets/builtin")
	if err != nil {
		return nil, fmt.Errorf("load built-in corpus: %w", err)
	}
	return bundle, nil
}

// BuiltinFS exposes the embedded files as a read-only filesystem for inspection.
func BuiltinFS() fs.FS {
	return builtinAssets
}
