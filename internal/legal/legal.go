// Package legal supplies offline application, dependency and corpus notices.
package legal

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

// Only the explicitly derived redistributable notice directory is embedded.
//
//go:embed assets/PROJECT-LICENSE assets/DEPENDENCIES.txt
var assets embed.FS

const Summary = `Nameforge — Copyright (C) 2026 Nameforge contributors.
This program comes with ABSOLUTELY NO WARRANTY.
This is free software under GNU GPLv3; you may redistribute and modify it
under its conditions. Full terms, dependency notices and data attribution follow.
Source: https://github.com/DoNotResuscitate/nameforge`

func Text(bundle *corpus.Bundle) (string, error) {
	var output strings.Builder
	output.WriteString(Summary + "\n\n")
	for _, name := range []string{"PROJECT-LICENSE", "DEPENDENCIES.txt"} {
		data, err := assets.ReadFile("assets/" + name)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&output, "=== %s ===\n", name)
		output.Write(data)
		output.WriteByte('\n')
	}
	for _, license := range bundle.Manifest.Licenses {
		notice, err := fs.ReadFile(corpus.BuiltinFS(), path.Join("assets/builtin", license.Path))
		if err != nil {
			return "", fmt.Errorf("read embedded license %q: %w", license.Name, err)
		}
		fmt.Fprintf(&output, "=== %s ===\n", license.Name)
		output.Write(notice)
		output.WriteByte('\n')
	}
	return output.String(), nil
}
