package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func testBinary(t *testing.T, root string) string {
	t.Helper()
	if supplied := os.Getenv("NAMEFORGE_TEST_BINARY"); supplied != "" {
		binary, err := filepath.Abs(supplied)
		if err != nil {
			t.Fatal(err)
		}
		if info, err := os.Stat(binary); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("invalid release executable %q: %v", binary, err)
		}
		return binary
	}
	binary := filepath.Join(root, "nameforge")
	build := exec.Command("mise", "exec", "--", "go", "build", "-trimpath", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build executable: %v\n%s", err, output)
	}
	return binary
}

// Linux CI opts into an OS-enforced empty network namespace. sudo is available
// on hosted runners; local developers need no elevated permissions by default.
func linuxSandbox(binary string, env []string, args ...string) *exec.Cmd {
	wrapped := []string{"-n", "/usr/bin/unshare", "--net", "--", "/usr/bin/env", "-i"}
	wrapped = append(wrapped, env...)
	wrapped = append(wrapped, binary)
	wrapped = append(wrapped, args...)
	return exec.Command("/usr/bin/sudo", wrapped...)
}
