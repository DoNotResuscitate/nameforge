package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestTUIRoutingWithoutTerminal(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		text string
	}{
		{nil, 2, "generate"},
		{[]string{"tui"}, 2, "Requires terminal stdin and stdout"},
		{[]string{"tui", "--help"}, 0, "--no-color"},
		{[]string{"tui", "--no-color", "--data-dir", "/absent"}, 2, "Requires terminal"},
		{[]string{"tui", "--unknown"}, 2, "flag provided but not defined"},
		{[]string{"tui", "unexpected"}, 2, "unexpected argument"},
	} {
		var stdout, stderr bytes.Buffer
		code := RunWithInput(context.Background(), tc.args, strings.NewReader(""), &stdout, &stderr)
		if code != tc.code || !strings.Contains(stdout.String()+stderr.String(), tc.text) || strings.Contains(stdout.String()+stderr.String(), "\x1b") {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", tc.args, code, stdout.String(), stderr.String())
		}
	}
}
