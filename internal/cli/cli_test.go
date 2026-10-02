package cli

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type limitedWriter struct {
	remaining int
	written   int
}

func (w *limitedWriter) Write(data []byte) (int, error) {
	n := min(len(data), w.remaining)
	w.remaining -= n
	w.written += n
	if n < len(data) {
		return n, errors.New("test output full")
	}
	return n, nil
}

func TestDataOutputFailures(t *testing.T) {
	// The list's tab-separated rows stay buffered until Flush, so these cases
	// cover delayed flush failures as well as inspect's immediate writes.
	for _, command := range []struct {
		name string
		args []string
	}{
		{"list", []string{"data", "list"}},
		{"inspect", []string{"data", "inspect", "--category", "french"}},
	} {
		for _, capacity := range []int{0, 17} {
			t.Run(command.name+"/"+fmt.Sprint(capacity), func(t *testing.T) {
				stdout := &limitedWriter{remaining: capacity}
				var stderr bytes.Buffer
				if code := Run(command.args, stdout, &stderr); code != 1 {
					t.Fatalf("exit code = %d, want 1", code)
				}
				if stdout.written != capacity {
					t.Fatalf("written = %d, want %d", stdout.written, capacity)
				}
				if !strings.Contains(stderr.String(), "write data "+command.name+": test output full") {
					t.Fatalf("missing output diagnostic: %q", stderr.String())
				}
			})
		}
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "help",
			args:       []string{"--help"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:       "version",
			args:       []string{"version"},
			wantCode:   0,
			wantStdout: "nameforge version dev",
		},
		{
			name:       "data list",
			args:       []string{"data", "list"},
			wantCode:   0,
			wantStdout: "French",
		},
		{
			name:       "data inspect",
			args:       []string{"data", "inspect", "--category", "french"},
			wantCode:   0,
			wantStdout: "Source locale: fr",
		},
		{
			name:       "data inspect unknown category",
			args:       []string{"data", "inspect", "--category", "missing"},
			wantCode:   2,
			wantStderr: `unknown category "missing"`,
		},
		{
			name:       "headless default gives usage error",
			wantCode:   2,
			wantStderr: "Usage:",
		},
		{
			name:       "unknown command",
			args:       []string{"unknown"},
			wantCode:   2,
			wantStderr: `unknown command "unknown"`,
		},
		{
			name:       "command help",
			args:       []string{"version", "--help"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(tt.args, &stdout, &stderr); code != tt.wantCode {
				t.Fatalf("Run() exit code = %d, want %d", code, tt.wantCode)
			}
			if tt.wantStdout != "" && !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout %q does not contain %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
