package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"

	"github.com/DoNotResuscitate/nameforge/internal/source/faker"
)

const surnameLockPath = "data/surnames.lock.json"

func surnames(ctx context.Context, command string, cache diskCache, rebuild bool) error {
	remote := faker.HTTPRemote(&http.Client{})
	if command == "pin-surnames" {
		if _, err := os.Stat(surnameLockPath); !os.IsNotExist(err) {
			return fmt.Errorf("surname lock already exists; refresh explicitly")
		}
		lock, err := faker.PinSurnames(ctx, remote, cache)
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(lock, "", "  ")
		if err != nil {
			return err
		}
		return write(surnameLockPath, append(data, '\n'))
	}
	data, err := os.ReadFile(surnameLockPath)
	if err != nil {
		return err
	}
	lock, err := faker.DecodeLock(data)
	if err != nil {
		return err
	}
	if lock.NameType != "surname" {
		return fmt.Errorf("surname lock must identify surname data")
	}
	switch command {
	case "fetch":
		return faker.Fetch(ctx, lock, remote, cache)
	case "verify":
		if err := faker.Verify(lock, os.DirFS(".")); err != nil {
			return err
		}
		if !rebuild {
			return nil
		}
	}
	artifacts, err := faker.Build(ctx, lock, cache)
	if err != nil {
		return err
	}
	if command == "verify" {
		return faker.Compare(artifacts, os.DirFS("."))
	}
	var paths []string
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
}
