package tui

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/export"
	"github.com/DoNotResuscitate/nameforge/internal/generator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func testModel(t *testing.T) *Model {
	t.Helper()
	bundle, err := corpus.LoadBuiltin()
	if err != nil {
		t.Fatal(err)
	}
	m := New(context.Background(), bundle, true)
	t.Cleanup(m.stopWork)
	return m
}

func key(m *Model, value string) tea.Cmd {
	var msg tea.KeyMsg
	switch value {
	case "tab":
		msg.Type = tea.KeyTab
	case "shift+tab":
		msg.Type = tea.KeyShiftTab
	case "enter":
		msg.Type = tea.KeyEnter
	case "esc":
		msg.Type = tea.KeyEsc
	case "ctrl+c":
		msg.Type = tea.KeyCtrlC
	case "up":
		msg.Type = tea.KeyUp
	case "down":
		msg.Type = tea.KeyDown
	default:
		msg.Type, msg.Runes = tea.KeyRunes, []rune(value)
	}
	_, cmd := m.Update(msg)
	return cmd
}

func complete(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatalf("expected asynchronous command; status: %s", m.status)
	}
	m.Update(cmd())
}

func setInput(m *Model, field int, value string) {
	i := m.inputs[field]
	i.SetValue(value)
	m.inputs[field] = i
}

func TestPickerFocusAndTextEntry(t *testing.T) {
	m := testModel(t)
	if len(m.selected) != 0 {
		t.Fatal("implicit default category")
	}
	key(m, "/")
	key(m, "french")
	if m.search.Value() != "french" || len(m.filtered()) != 1 {
		t.Fatal("search failed")
	}
	key(m, "esc")
	key(m, " ")
	if !m.selected["french"] {
		t.Fatal("toggle filtered selection failed")
	}
	key(m, "/")
	m.search.SetValue("italian")
	key(m, "esc")
	key(m, " ")
	if !m.selected["french"] || !m.selected["italian"] {
		t.Fatal("search lost selection")
	}
	key(m, "/")
	m.search.SetValue("zzz")
	key(m, "esc")
	key(m, " ") // Empty search results must not panic or lose selections.
	key(m, "a")
	r, err := m.request(false)
	if err != nil || len(r.CategoryIDs) != 10 {
		t.Fatalf("all selection: %v", err)
	}
	key(m, "c")
	if cmd := key(m, "enter"); cmd != nil || !strings.Contains(m.status, "category") {
		t.Fatal("empty selection not rejected before command")
	}
	key(m, "tab")
	key(m, "qjr?e")
	if m.focus != searchFocus || !strings.Contains(m.search.Value(), "qjr?e") || m.help || m.dialog != nil {
		t.Fatal("global shortcuts consumed text entry")
	}
	key(m, "tab")
	if m.focus != settingsFocus {
		t.Fatal("tab focus")
	}
	key(m, " ")
	if m.mode != generator.ModeBlend {
		t.Fatal("mode control")
	}
	key(m, "down")
	key(m, " ")
	if m.gender != generator.GenderMasculine {
		t.Fatal("gender control")
	}
	key(m, "down")
	if !m.inputs[orderField].Focused() {
		t.Fatal("numeric field not focused")
	}
	key(m, "q")
	if m.inputs[orderField].Value() != "2q" {
		t.Fatal("quit consumed numeric editing")
	}
	key(m, "shift+tab")
	if m.focus != searchFocus || m.inputs[orderField].Focused() {
		t.Fatal("reverse focus or blur")
	}
}

func TestSharedValidation(t *testing.T) {
	for _, tc := range []struct {
		field int
		value string
	}{
		{orderField, "0"}, {orderField, "5"}, {countField, "1001"},
		{countField, ""}, {minField, "0"}, {maxField, "65"},
		{seedField, "-1"}, {seedField, "18446744073709551616"},
	} {
		m := testModel(t)
		m.selected["french"] = true
		setInput(m, tc.field, tc.value)
		if cmd := key(m, "enter"); cmd != nil || m.phase != ready {
			t.Fatalf("accepted invalid %d=%q", tc.field, tc.value)
		}
	}
	m := testModel(t)
	m.selected["french"] = true
	setInput(m, minField, "10")
	setInput(m, maxField, "5")
	if cmd := key(m, "enter"); cmd != nil {
		t.Fatal("contradictory bounds accepted")
	}
	setInput(m, minField, "")
	setInput(m, maxField, "")
	for _, seed := range []string{"0", "18446744073709551615"} {
		setInput(m, seedField, seed)
		r, err := m.request(false)
		if err != nil || r.Seed == nil || r.MinLength != 0 || r.MaxLength != 0 {
			t.Fatalf("valid seed/defaults: %v", err)
		}
	}
}

func TestGenerationReplayFreshSeedsAndStaleMessages(t *testing.T) {
	m := testModel(t)
	m.selected["french"], m.selected["italian"] = true, true
	setInput(m, seedField, "42")
	old := key(m, "enter")
	if m.phase != generating || m.result != nil {
		t.Fatal("work did not run asynchronously")
	}
	newer := key(m, "enter")
	m.Update(old())
	if m.phase != generating || m.result != nil {
		t.Fatal("stale completion replaced newer request")
	}
	complete(t, m, newer)
	if m.result == nil || !m.result.Complete || m.result.Seed != 42 || m.focus != resultsFocus {
		t.Fatalf("completion: %s", m.status)
	}
	want, err := generator.Generate(context.Background(), m.bundle, m.result.Options)
	if err != nil || !reflect.DeepEqual(want, *m.result) {
		t.Fatal("TUI does not share engine output")
	}
	key(m, " ")
	previous := m.result
	canceled := key(m, "r")
	key(m, "esc")
	m.Update(canceled())
	if m.result != previous || len(m.favorites) != 1 || m.phase != ready {
		t.Fatal("cancel lost session or accepted stale partial")
	}
	complete(t, m, key(m, "r"))
	if m.result.Seed == 42 || m.inputs[seedField].Value() != "42" {
		t.Fatal("fresh regeneration did not retain explicit replay setting")
	}
	complete(t, m, key(m, "enter"))
	if !reflect.DeepEqual(*m.result, want) {
		t.Fatal("explicit seed did not replay")
	}
}

func TestErrorRecoveryAndPartialResults(t *testing.T) {
	m := testModel(t)
	m.selected["greek"] = true
	m.gender = generator.GenderFeminine
	complete(t, m, key(m, "enter"))
	if m.result != nil || m.phase != ready || !strings.Contains(m.status, "gender any") {
		t.Fatalf("missing gender: %s", m.status)
	}
	m.gender = generator.GenderAny
	setInput(m, seedField, "42")
	complete(t, m, key(m, "enter"))
	if m.result == nil || !m.result.Complete {
		t.Fatal("cannot recover")
	}
	// A provenance-bearing subset of the bundled data supplies a deliberately
	// tiny pool; no invented names are used for bounded exhaustion.
	pool, err := corpus.Select(m.bundle.Records, []string{"french"}, corpus.GenderFilter("any"))
	if err != nil {
		t.Fatal(err)
	}
	category, _ := m.bundle.Category("french")
	tiny := *m.bundle
	tiny.Records = pool[0].Records[:3]
	tiny.Categories = []corpus.Category{category}
	m.bundle = &tiny
	clear(m.selected)
	m.selected["french"] = true
	m.allow = true
	setInput(m, orderField, "4")
	setInput(m, countField, "1000")
	complete(t, m, key(m, "enter"))
	if m.result == nil || m.result.Complete || len(m.result.Names) == 0 || !strings.Contains(m.status, "fewer names") || !strings.Contains(m.View(), "INCOMPLETE") {
		t.Fatalf("bounded partial state not shown: %s", m.status)
	}
}

func TestFavoritesExportMetadataAndOverwrite(t *testing.T) {
	m := testModel(t)
	m.selected["french"] = true
	setInput(m, seedField, "42")
	complete(t, m, key(m, "enter"))
	first := *m.result
	key(m, " ")
	key(m, " ")
	if len(m.favorites) != 0 {
		t.Fatal("favorite toggle off")
	}
	key(m, " ")
	m.mode = generator.ModeBlend
	m.selected["italian"] = true
	setInput(m, seedField, "43")
	complete(t, m, key(m, "enter"))
	second := *m.result
	key(m, " ")
	key(m, "e")
	key(m, " ") // session favorites
	path := filepath.Join(t.TempDir(), "favorites.json")
	m.dialog.path.SetValue(path)
	complete(t, m, key(m, "enter"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		SchemaVersion int                    `json:"schema_version"`
		Batches       []export.FavoriteBatch `json:"batches"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if wire.SchemaVersion != 1 || len(wire.Batches) != 2 || !reflect.DeepEqual(wire.Batches[0].Result.Result, first) || !reflect.DeepEqual(wire.Batches[1].Result.Result, second) || len(wire.Batches[0].Names) != 1 || len(wire.Batches[1].Names) != 1 {
		t.Fatal("favorites lost original per-batch metadata or selection")
	}
	key(m, "e")
	m.dialog.path.SetValue(path)
	complete(t, m, key(m, "enter"))
	if m.dialog == nil || !m.dialog.confirm {
		t.Fatal("no overwrite confirmation")
	}
	before := *m.result
	key(m, "n")
	unchanged, _ := os.ReadFile(path)
	if !reflect.DeepEqual(unchanged, data) || !reflect.DeepEqual(*m.result, before) {
		t.Fatal("decline changed destination or result")
	}
	complete(t, m, key(m, "enter"))
	complete(t, m, key(m, "y"))
	data, _ = os.ReadFile(path)
	var current export.Result
	if err := json.Unmarshal(data, &current); err != nil || !reflect.DeepEqual(current.Result, second) {
		t.Fatal("overwrite did not export current batch")
	}
	key(m, "e")
	m.dialog.path.SetValue(filepath.Join(t.TempDir(), "missing-parent", "names.json"))
	complete(t, m, key(m, "enter"))
	if m.dialog == nil || !strings.Contains(m.status, "Export failed") || m.result == nil || len(m.favorites) != 2 {
		t.Fatal("write error lost session")
	}
	key(m, "esc")
}

func TestTextExportCancelAndGlobalQuit(t *testing.T) {
	m := testModel(t)
	m.selected["arabic"] = true
	complete(t, m, key(m, "enter"))
	key(m, "e")
	key(m, "tab")
	key(m, " ") // text
	if m.dialog.path.Value() != "names.txt" {
		t.Fatal("text format did not update the default filename")
	}
	path := filepath.Join(t.TempDir(), "names.txt")
	m.dialog.path.SetValue(path)
	cmd := key(m, "enter")
	key(m, "esc")
	m.Update(cmd())
	if _, err := os.Stat(path); !os.IsNotExist(err) || m.dialog != nil || m.phase != ready {
		t.Fatal("canceled export published or reopened dialog")
	}
	key(m, "e")
	key(m, "tab")
	key(m, " ")
	m.dialog.path.SetValue(path)
	complete(t, m, key(m, "enter"))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var expected strings.Builder
	for _, name := range m.result.Names {
		expected.WriteString(name.Name + "\n")
	}
	if string(data) != expected.String() {
		t.Fatal("text export does not match batch")
	}
	key(m, "e")
	key(m, "tab")
	key(m, "tab")
	key(m, "q") // q is path text, not quit.
	if !strings.Contains(m.dialog.path.Value(), "q") {
		t.Fatal("export text shortcut")
	}
	quit := key(m, "ctrl+c")
	if quit == nil || !m.interrupted {
		t.Fatal("Ctrl-C not global")
	}
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatal("Ctrl-C did not quit")
	}
}

func TestExportFormatUpdatesCustomFilename(t *testing.T) {
	m := testModel(t)
	m.selected["french"] = true
	complete(t, m, key(m, "enter"))
	key(m, "e")
	key(m, "tab") // format
	for _, tc := range []struct{ path, text, json string }{
		{"exports.v1/custom.json", "exports.v1/custom.txt", "exports.v1/custom.json"},
		{"custom.batch.json", "custom.batch.txt", "custom.batch.json"},
		{"custom", "custom.txt", "custom.json"},
		{".session", ".session.txt", ".session.json"},
		{"", "", ""},
	} {
		m.dialog.path.SetValue(tc.path)
		key(m, " ")
		if m.dialog.json || m.dialog.path.Value() != tc.text {
			t.Fatalf("text format for %q: path=%q", tc.path, m.dialog.path.Value())
		}
		key(m, " ")
		if !m.dialog.json || m.dialog.path.Value() != tc.json {
			t.Fatalf("JSON format for %q: path=%q", tc.path, m.dialog.path.Value())
		}
	}
}

func TestResizeNoColorAndSourceLabels(t *testing.T) {
	m := testModel(t)
	for _, size := range []tea.WindowSizeMsg{{Width: 100, Height: 30}, {Width: 40, Height: 12}, {Width: 20, Height: 8}, {Width: 8, Height: 2}} {
		m.Update(size)
		for _, f := range []focus{pickerFocus, searchFocus, settingsFocus, resultsFocus} {
			m.setFocus(f)
			view := m.View()
			if strings.Contains(view, "\x1b") {
				t.Fatal("no-color view emitted ANSI styling")
			}
			lines := strings.Split(view, "\n")
			if len(lines) > size.Height {
				t.Fatalf("view overflow height: %d > %d", len(lines), size.Height)
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > size.Width {
					t.Fatalf("view overflow width: %q", line)
				}
			}
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	greek, _ := m.bundle.Category("greek")
	arabic, _ := m.bundle.Category("arabic")
	if !strings.Contains(m.source(greek), "Wikipedia") || !strings.Contains(m.source(arabic), "Wikidata") || !strings.Contains(m.source(greek), "unspecified") {
		t.Fatal("inaccurate source labels")
	}
	m.setFocus(pickerFocus)
	key(m, "?")
	for range 500 {
		key(m, "down")
	}
	if !strings.Contains(m.View(), "North African") {
		t.Fatal("help scroll cannot reach source scope")
	}
	bottom := m.View()
	key(m, "up")
	if m.View() == bottom {
		t.Fatal("help overscroll prevents immediate upward navigation")
	}
	key(m, "esc")
	if m.help {
		t.Fatal("help not dismissed")
	}
}

func TestCompactLayoutAndLongPathEditing(t *testing.T) {
	m := testModel(t)
	m.selected["french"] = true
	complete(t, m, key(m, "enter"))
	key(m, "e")
	key(m, "tab")
	key(m, "tab")
	m.dialog.path.SetValue(strings.Repeat("directory/", 20) + "visible-tail.json")
	m.dialog.path.CursorEnd()
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	before := m.dialog.path.Value()
	if !strings.Contains(m.View(), "visible-tail.json") || m.dialog.path.Value() != before {
		t.Fatal("long path cursor not visible or View changed input")
	}
	m.Update(tea.WindowSizeMsg{Width: 20, Height: 8})
	m.dialog.confirm = true
	if !strings.Contains(m.View(), "OVERWRITE") {
		t.Fatal("compact confirmation hidden")
	}
	key(m, "?")
	key(m, "esc")
	if m.help || m.dialog == nil {
		t.Fatal("dismissing help lost export form")
	}
	key(m, "esc")
	m.setFocus(pickerFocus)
	if !strings.Contains(m.View(), "[ ]") {
		t.Fatal("compact picker has no category rows")
	}
}

func TestInteractiveLegalNoticesAndHelpRecovery(t *testing.T) {
	m := testModel(t)
	if !strings.Contains(m.View(), "NO WARRANTY") || !strings.Contains(m.View(), "GPLv3") {
		t.Fatal("startup legal summary missing")
	}
	key(m, "l")
	if !m.help || !m.legal || !strings.Contains(m.View(), "LICENSES") || !strings.Contains(strings.Join(m.helpLines(), "\n"), "GNU GENERAL PUBLIC LICENSE") {
		t.Fatal("interactive full GPL unavailable")
	}
	for range 10 {
		key(m, "down")
	}
	if m.helpOffset == 0 {
		t.Fatal("legal notices cannot scroll")
	}
	key(m, "l")
	if m.legal || m.helpOffset != 0 || !strings.Contains(m.View(), "HELP") {
		t.Fatal("legal-to-help switch failed")
	}
	key(m, "l")
	key(m, "esc")
	m.selected["french"] = true
	complete(t, m, key(m, "enter"))
	key(m, "e")
	key(m, "?")
	if m.legal || !m.help || m.dialog == nil {
		t.Fatal("export help reused stale legal screen")
	}
	key(m, "esc")
	key(m, "esc")
	m.setFocus(searchFocus)
	key(m, "l")
	if m.help || m.search.Value() != "l" {
		t.Fatal("legal shortcut intercepted text entry")
	}
}
