// corpus-build is maintenance-only. The runtime does not import source/faker.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"

	"github.com/DoNotResuscitate/nameforge/internal/source/faker"
)

type diskCache string

func (cache diskCache) Read(key string) ([]byte, error) {
	return os.ReadFile(filepath.Join(string(cache), filepath.FromSlash(key)))
}
func (cache diskCache) Write(key string, data []byte) error {
	return write(filepath.Join(string(cache), filepath.FromSlash(key)), data)
}
func write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".corpus-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0644); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: corpus-build fetch|build|verify|pin [--rebuild] [--cache path]")
	}
	command := args[0]
	f := flag.NewFlagSet("corpus-build", flag.ContinueOnError)
	cachePath := f.String("cache", ".local/faker", "locked raw source cache")
	lockPath := f.String("lock", "data/sources.lock.json", "reviewed source lock")
	rebuild := f.Bool("rebuild", false, "verify identical rebuild from locked cache")
	if err := f.Parse(args[1:]); err != nil {
		return err
	}
	if f.NArg() != 0 || (*rebuild && command != "verify") {
		return fmt.Errorf("unexpected arguments")
	}
	cache := diskCache(*cachePath)
	remote := faker.HTTPRemote(&http.Client{})
	if command == "pin" {
		if _, err := os.Stat(*lockPath); !os.IsNotExist(err) {
			return fmt.Errorf("pin requires an absent lock; existing source locks must be refreshed explicitly")
		}
		lock, err := faker.Pin(ctx, remote, cache)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(lock, "", "  ")
		if err != nil {
			return err
		}
		return write(*lockPath, append(data, '\n'))
	}
	data, err := os.ReadFile(*lockPath)
	if err != nil {
		return err
	}
	lock, err := faker.DecodeLock(data)
	if err != nil {
		return err
	}
	switch command {
	case "fetch":
		return faker.Fetch(ctx, lock, remote, cache)
	case "build":
		artifacts, err := faker.Build(ctx, lock, cache)
		if err != nil {
			return err
		}
		paths := make([]string, 0, len(artifacts))
		for path := range artifacts {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			if err := write(path, artifacts[path]); err != nil {
				return err
			}
		}
		return nil
	case "verify":
		if err := faker.Verify(lock, os.DirFS(".")); err != nil {
			return err
		}
		if *rebuild {
			artifacts, err := faker.Build(ctx, lock, cache)
			if err != nil {
				return err
			}
			return faker.Compare(artifacts, os.DirFS("."))
		}
		return nil
	default:
		return fmt.Errorf("unknown maintenance command %q", command)
	}
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
