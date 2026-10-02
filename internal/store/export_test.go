package store

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteExportNoClobberAndAtomicFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "export.txt")
	ctx := context.Background()
	if err := WriteExport(ctx, path, []byte("first payload\n"), false); err != nil {
		t.Fatal(err)
	}
	if err := WriteExport(ctx, path, []byte("replacement\n"), false); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("clobber guard: %v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "first payload\n" {
		t.Fatal("existing destination changed")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := WriteExport(canceled, path, []byte("canceled\n"), true); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation ignored")
	}
	if err := WriteExport(ctx, path, []byte("replacement\n"), true); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "replacement\n" {
		t.Fatal("confirmed overwrite failed")
	}
	// Publishing over a directory must fail and clean its temporary sibling.
	if err := WriteExport(ctx, dir, []byte("unused\n"), true); err == nil {
		t.Fatal("published over directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary export leaked")
	}
}
