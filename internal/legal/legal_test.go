package legal

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
)

func TestFullOfflineNotices(t *testing.T) {
	license, err := os.ReadFile("../../LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := assets.ReadFile("assets/PROJECT-LICENSE")
	if err != nil || !bytes.Equal(license, embedded) {
		t.Fatalf("embedded GPL differs from repository LICENSE: %v", err)
	}
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	text, err := Text(bundle)
	if err != nil || !strings.Contains(text, string(license)) {
		t.Fatalf("full GPL omitted: %v", err)
	}
	for _, fragment := range []string{"ABSOLUTELY NO WARRANTY", "github.com/charmbracelet/bubbletea v1.3.10", "github.com/atotto/clipboard v0.1.4", "Go go1.27.1", "Creative Commons", "Faker"} {
		if !strings.Contains(text, fragment) {
			t.Errorf("missing notice %q", fragment)
		}
	}
	if strings.Contains(text, "/Users/") || strings.Contains(text, "/home/") {
		t.Fatal("private path in embedded legal notices")
	}
}
