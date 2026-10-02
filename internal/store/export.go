// Package store owns explicit local file writes. Built-in data never uses it.
package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// WriteExport publishes a complete file from a temporary sibling. Without
// overwrite, linking instead of renaming prevents even a racing writer from
// clobbering an existing destination. Parents must already exist.
func WriteExport(ctx context.Context, path string, data []byte, overwrite bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".nameforge-export-*")
	if err != nil {
		return fmt.Errorf("create export: %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return fmt.Errorf("write export: %w", err)
	}
	if err = f.Sync(); err != nil {
		return fmt.Errorf("sync export: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("close export: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if overwrite {
		err = os.Rename(f.Name(), path)
	} else {
		err = os.Link(f.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("publish export: %w", err)
	}
	return nil
}
