package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/DoNotResuscitate/nameforge/internal/export"
)

// Exercise the actual executable outside the checkout, with no Go on PATH and
// an empty home. On macOS sandbox-exec also denies network and filesystem writes.
func TestBinaryHeadless(t *testing.T) {
	root := t.TempDir()
	binary := testBinary(t, root)
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o555); err != nil {
		t.Fatal(err)
	}
	command := func(args ...string) *exec.Cmd {
		cmd := exec.Command(binary, args...)
		if runtime.GOOS == "darwin" {
			sandbox, err := exec.LookPath("sandbox-exec")
			if err != nil {
				t.Fatal(err)
			}
			args = append([]string{"-p", "(version 1)(allow default)(deny network*)(deny file-write*)", binary}, args...)
			cmd = exec.Command(sandbox, args...)
		}
		cmd.Dir = root
		cmd.Env = []string{"HOME=" + home, "PATH=" + filepath.Join(root, "absent-tools")}
		if runtime.GOOS == "linux" && os.Getenv("NAMEFORGE_TEST_LINUX_SANDBOX") == "1" {
			cmd = linuxSandbox(binary, cmd.Env, args...)
			cmd.Dir = root
		}
		if runtime.GOOS == "windows" {
			cmd.Env = append(cmd.Env, "USERPROFILE="+home, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"))
		}
		return cmd
	}
	run := func(args ...string) (int, string, string) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		cmd := command(args...)
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		code := 0
		if err := cmd.Run(); err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			code = exit.ExitCode()
		}
		return code, stdout.String(), stderr.String()
	}
	for _, mode := range []string{"category", "blend"} {
		for _, selection := range []struct {
			Label      string
			Args       []string
			Categories int
		}{
			{"french-italian", []string{"--category", "french", "--category", "italian"}, 2},
			{"greek", []string{"--category", "greek"}, 1},
			{"arabic", []string{"--category", "arabic"}, 1},
			{"all-categories", []string{"--all-categories"}, 10},
			{"surnames", []string{"--name-type", "surname", "--all-categories"}, 8},
			{"full-names", []string{"--name-type", "full", "--category", "french", "--category", "italian", "--gender", "feminine", "--surname-order", "1"}, 2},
		} {
			t.Run(mode+"/"+selection.Label, func(t *testing.T) {
				args := append([]string{"generate", "--mode", mode, "--seed", "42"}, selection.Args...)
				code, stdout, stderr := run(append(args, "--format", "json")...)
				if code != 0 || stderr != "" {
					t.Fatalf("first-run JSON: %d %s", code, stderr)
				}
				var result export.Result
				if err := json.Unmarshal([]byte(stdout), &result); err != nil || !result.Complete || len(result.Names) != 20 {
					t.Fatalf("bad JSON result: %v", err)
				}
				if len(result.CategoryIDs) != selection.Categories || len(result.Bounds) != selection.Categories || len(result.BundleHash) != 64 || result.AlgorithmVersion == "" || result.Seed != 42 {
					t.Fatal("incomplete reproduction metadata")
				}
				if selection.Label == "surnames" && result.NameType != "surname" {
					t.Fatal("surname replay type omitted")
				}
				if selection.Label == "full-names" && (result.NameType != "full" || len(result.SurnameBundleHash) != 64 || len(result.SurnameBounds) != 2 || result.Names[0].Given == nil || result.Names[0].Surname == nil) {
					t.Fatal("full-name component metadata omitted")
				}
				code, replay, stderr := run(append(args, "--format", "json")...)
				if code != 0 || stderr != "" || replay != stdout {
					t.Fatal("binary seeded output changed across runs")
				}
				code, text, metadata := run(args...)
				var expected strings.Builder
				for _, name := range result.Names {
					expected.WriteString(name.Name + "\n")
					for _, r := range name.Name {
						if unicode.IsLetter(r) && !unicode.Is(unicode.Latin, r) {
							t.Fatal("non-Latin output")
						}
					}
					if mode == "category" && len(name.CategoryIDs) != 1 || mode == "blend" && !reflect.DeepEqual(name.CategoryIDs, result.CategoryIDs) {
						t.Fatal("incorrect attribution")
					}
				}
				if code != 0 || text != expected.String() || !json.Valid([]byte(metadata)) {
					t.Fatalf("first-run text: %d %q", code, metadata)
				}
				code, textReplay, metadataReplay := run(args...)
				if code != 0 || textReplay != text || metadataReplay != metadata {
					t.Fatal("text replay changed")
				}
			})
		}
	}
	for _, args := range [][]string{{"--help"}, {"version"}, {"licenses"}} {
		code, stdout, stderr := run(args...)
		if code != 0 || stdout == "" || stderr != "" {
			t.Fatalf("offline %v: %d %q", args, code, stderr)
		}
		if args[0] == "licenses" && (!strings.Contains(stdout, "GNU GENERAL PUBLIC LICENSE") || !strings.Contains(stdout, "Nameforge dependency notices") || !strings.Contains(stdout, "ABSOLUTELY NO WARRANTY")) {
			t.Fatal("packaged binary omitted application/dependency legal notices")
		}
		if args[0] == "version" && os.Getenv("NAMEFORGE_TEST_VERSION") != "" && !strings.Contains(stdout, "version "+os.Getenv("NAMEFORGE_TEST_VERSION")+" (commit "+os.Getenv("NAMEFORGE_TEST_COMMIT")+")") {
			t.Fatalf("incorrect release metadata: %s", stdout)
		}
	}
	code, stdout, stderr := run("generate", "--category", "french", "--count", "0")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "--count") {
		t.Fatalf("invalid flags: %d %q %q", code, stdout, stderr)
	}
	code, stdout, stderr = run()
	if code != 2 || stdout != "" || !strings.Contains(stderr, "generate") {
		t.Fatalf("headless no-argument invocation: %d %q %q", code, stdout, stderr)
	}
	if runtime.GOOS != "windows" {
		for _, nameType := range []string{"given", "surname", "full"} {
			t.Run("interrupt/"+nameType, func(t *testing.T) {
				cmd := command("generate", "--name-type", nameType, "--category", "french", "--count", "1000", "--min-length", "64", "--max-length", "64", "--order", "4", "--seed", "42")
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = cmd.Process.Kill() })
				time.Sleep(100 * time.Millisecond)
				if err := cmd.Process.Signal(os.Interrupt); err != nil {
					t.Fatal(err)
				}
				var exit *exec.ExitError
				if err := cmd.Wait(); !errors.As(err, &exit) || exit.ExitCode() != 130 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "canceled") {
					t.Fatalf("interrupt: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
				}
			})
		}
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("runtime wrote local state: %v", err)
	}
}
