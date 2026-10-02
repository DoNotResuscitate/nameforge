// Command notices derives complete legal texts from checksum-pinned modules.
// Run through mise; normal builds use only the committed output.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type module struct {
	Path, Version, Dir, Sum string
	Replace                 *module
}

func main() {
	verify := flag.Bool("verify", false, "compare derived notices with committed bytes")
	flag.Parse()
	if err := run(*verify); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(verify bool) error {
	modules := make(map[string]module)
	// Include every runtime dependency for every release target, plus the PTY
	// test dependency, whose source is included in the corresponding source.
	for _, target := range []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"} {
		parts := strings.Split(target, "/")
		cmd := exec.Command("go", "list", "-mod=readonly", "-deps", "-test", "-json", "./cmd/nameforge")
		cmd.Env = append(os.Environ(), "GOOS="+parts[0], "GOARCH="+parts[1], "CGO_ENABLED=0", "GOTOOLCHAIN=local")
		data, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("list %s dependencies: %w", target, err)
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		for {
			var pkg struct{ Module *module }
			if err := decoder.Decode(&pkg); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return err
			}
			if pkg.Module != nil && pkg.Module.Version != "" {
				if pkg.Module.Replace != nil {
					return fmt.Errorf("replacement module is not permitted: %s", pkg.Module.Path)
				}
				modules[pkg.Module.Path] = *pkg.Module
			}
		}
	}
	var paths []string
	for path := range modules {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var output bytes.Buffer
	output.WriteString("Nameforge dependency notices\n\nDerived from the pinned Go toolchain and the union of macOS/Linux runtime\nand binary-test modules. Full original texts follow; module identities and\nchecksums match go.mod/go.sum. No local paths are included.\n\n")
	goRoot, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return err
	}
	goVersion, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		return err
	}
	if err := appendNotices(&output, "Go "+strings.TrimSpace(string(goVersion)), strings.TrimSpace(string(goRoot))); err != nil {
		return err
	}
	for _, path := range paths {
		m := modules[path]
		if m.Sum == "" || m.Dir == "" {
			return fmt.Errorf("missing checksum or module cache: %s", path)
		}
		if err := appendNotices(&output, m.Path+" "+m.Version+"\nModule checksum: "+m.Sum, m.Dir); err != nil {
			return err
		}
	}
	// Remove only our final inter-notice separator, retaining upstream bytes.
	output.Truncate(output.Len() - 2)
	project, err := os.ReadFile("LICENSE")
	if err != nil {
		return err
	}
	for path, data := range map[string][]byte{
		"internal/legal/assets/PROJECT-LICENSE":  project,
		"internal/legal/assets/DEPENDENCIES.txt": output.Bytes(),
	} {
		if verify {
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, data) {
				return fmt.Errorf("%s is stale; run mise run notices:build", path)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

func appendNotices(output *bytes.Buffer, label, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	foundLicense := false
	for _, entry := range entries {
		upper := strings.ToUpper(entry.Name())
		if entry.IsDir() || !(strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "COPYING") || upper == "NOTICE" || upper == "PATENTS") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return err
		}
		fmt.Fprintf(output, "=== %s / %s ===\n", label, entry.Name())
		output.Write(data)
		output.WriteString("\n\n")
		foundLicense = foundLicense || strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "COPYING")
	}
	if !foundLicense {
		return fmt.Errorf("no full license text found for %s", label)
	}
	return nil
}
