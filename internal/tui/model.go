// Package tui presents the offline generator through a Bubble Tea state machine.
package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/export"
	"github.com/DoNotResuscitate/nameforge/internal/generator"
	"github.com/DoNotResuscitate/nameforge/internal/store"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type focus int

const (
	pickerFocus focus = iota
	searchFocus
	settingsFocus
	resultsFocus
)

type phase int

const (
	ready phase = iota
	generating
	exporting
)

const (
	modeField = iota
	genderField
	orderField
	countField
	minField
	maxField
	seedField
	noveltyField
)

type generationMsg struct {
	id     uint64
	result generator.Result
	err    error
}

type exportMsg struct {
	id  uint64
	err error
}

type favorite struct {
	batch  uint64
	result generator.Result
	name   generator.GeneratedName
}

type exportDialog struct {
	path        textinput.Model
	favorites   bool
	json        bool
	focus       int
	confirm     bool
	data        []byte
	destination string
}

// Model owns session state. Commands capture immutable requests/results and
// return IDs so canceled or superseded work can never replace current state.
type Model struct {
	ctx         context.Context
	bundle      *corpus.Bundle
	categories  []corpus.Category
	selected    map[string]bool
	search      textinput.Model
	inputs      map[int]textinput.Model
	focus       focus
	field       int
	picker      int
	cursor      int
	mode        generator.Mode
	gender      generator.GenderFilter
	allow       bool
	phase       phase
	cancel      context.CancelFunc
	requestID   uint64
	batchID     uint64
	result      *generator.Result
	favorites   []favorite
	dialog      *exportDialog
	help        bool
	helpOffset  int
	status      string
	width       int
	height      int
	noColor     bool
	interrupted bool
}

func input(value, placeholder string) textinput.Model {
	t := textinput.New()
	t.SetValue(value)
	t.Placeholder = placeholder
	t.CharLimit = 4096
	t.KeyMap.Paste.SetEnabled(false)
	return t
}

func New(ctx context.Context, bundle *corpus.Bundle, noColor bool) *Model {
	categories := append([]corpus.Category(nil), bundle.Categories...)
	sort.Slice(categories, func(i, j int) bool { return categories[i].ID < categories[j].ID })
	return &Model{
		ctx: ctx, bundle: bundle, categories: categories, selected: make(map[string]bool),
		search: input("", "filter categories"),
		inputs: map[int]textinput.Model{
			orderField: input("2", "1..4"), countField: input("20", "1..1000"),
			minField: input("", "auto"), maxField: input("", "auto"),
			seedField: input("", "random"),
		},
		mode: generator.ModeCategory, gender: generator.GenderAny,
		width: 80, height: 24, noColor: noColor,
		status: "Choose one or more categories; no category is selected by default.",
	}
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) filtered() []corpus.Category {
	query := strings.ToLower(m.search.Value())
	var result []corpus.Category
	for _, c := range m.categories {
		if strings.Contains(strings.ToLower(c.ID+" "+c.Label), query) {
			result = append(result, c)
		}
	}
	return result
}

func (m *Model) request(fresh bool) (generator.Request, error) {
	r := generator.Request{Mode: m.mode, Gender: m.gender, AllowExisting: m.allow}
	for _, c := range m.categories {
		if m.selected[c.ID] {
			r.CategoryIDs = append(r.CategoryIDs, c.ID)
		}
	}
	for _, field := range []int{orderField, countField, minField, maxField} {
		value := strings.TrimSpace(m.inputs[field].Value())
		if value == "" && (field == minField || field == maxField) {
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return r, fmt.Errorf("%s must be a positive integer (blank lengths mean auto)", fieldLabels[field])
		}
		switch field {
		case orderField:
			r.Order = n
		case countField:
			r.Count = n
		case minField:
			r.MinLength = n
		case maxField:
			r.MaxLength = n
		}
	}
	if seed := strings.TrimSpace(m.inputs[seedField].Value()); !fresh && seed != "" {
		n, err := strconv.ParseUint(seed, 10, 64)
		if err != nil {
			return r, fmt.Errorf("seed must be an unsigned 64-bit decimal integer")
		}
		r.Seed = &n
	}
	return generator.NormalizeRequest(r)
}

func (m *Model) stopWork() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.requestID++
	m.phase = ready
}

func (m *Model) generate(fresh bool) tea.Cmd {
	r, err := m.request(fresh)
	if err != nil {
		m.status = "Invalid settings: " + err.Error()
		return nil
	}
	m.stopWork()
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.phase = generating
	m.status = "Generating… Esc cancels; previous results and favorites are retained."
	id, bundle := m.requestID, m.bundle
	return func() tea.Msg {
		result, err := generator.Generate(ctx, bundle, r)
		return generationMsg{id: id, result: result, err: err}
	}
}

func (m *Model) setFocus(f focus) tea.Cmd {
	m.search.Blur()
	for id, value := range m.inputs {
		value.Blur()
		m.inputs[id] = value
	}
	m.focus = f
	if f == searchFocus {
		return m.search.Focus()
	}
	if f == settingsFocus {
		if value, ok := m.inputs[m.field]; ok {
			cmd := value.Focus()
			m.inputs[m.field] = value
			return cmd
		}
	}
	return nil
}

func (m *Model) editing() bool {
	_, numeric := m.inputs[m.field]
	return m.focus == searchFocus || (m.focus == settingsFocus && numeric)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
	case generationMsg:
		if msg.id != m.requestID || m.phase != generating {
			return m, nil
		}
		m.phase = ready
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		if msg.err == nil || len(msg.result.Names) > 0 {
			m.result = &msg.result
			m.batchID = msg.id
			m.cursor = 0
			m.setFocus(resultsFocus)
		}
		if msg.err != nil {
			m.status = "Generation failed: " + msg.err.Error() + ". Adjust filters/settings and press Enter."
			var typed *generator.GenerationError
			if errors.As(msg.err, &typed) {
				switch typed.Kind {
				case generator.ErrorEmptySelection:
					m.status += " Use gender any; unspecified is not unisex."
				case generator.ErrorAttemptsExhausted:
					m.status += " Try fewer names, broader lengths, lower order, or allow existing."
				}
			}
		} else {
			m.status = fmt.Sprintf("Complete: %d names. Space favorites; r fresh seed; e export.", len(msg.result.Names))
		}
	case exportMsg:
		if msg.id != m.requestID || m.phase != exporting {
			return m, nil
		}
		m.phase = ready
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		if errors.Is(msg.err, fs.ErrExist) {
			m.dialog.confirm = true
			m.status = "Destination exists. Confirm overwrite with y; n or Esc keeps it."
		} else if msg.err != nil {
			m.status = "Export failed: " + msg.err.Error() + ". Edit the path and retry."
		} else {
			m.status = "Exported to " + m.dialog.destination
			m.dialog = nil
		}
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" {
			m.interrupted = true
			m.stopWork()
			return m, tea.Quit
		}
		if key == "esc" {
			if m.help {
				m.help = false
				return m, nil
			}
			if m.phase != ready {
				m.stopWork()
				m.status = "Canceled. Results and favorites retained; an already-published export may exist."
			}
			m.dialog = nil
			m.help = false
			if m.editing() {
				return m, m.setFocus(pickerFocus)
			}
			return m, nil
		}
		if m.help {
			limit := max(0, len(m.helpLines())-m.helpPageSize())
			m.helpOffset = min(m.helpOffset, limit)
			switch key {
			case "?":
				m.help = false
			case "down", "j":
				m.helpOffset = min(limit, m.helpOffset+1)
			case "up", "k":
				m.helpOffset = max(0, m.helpOffset-1)
			case "q":
				m.stopWork()
				return m, tea.Quit
			}
			return m, nil
		}
		if m.dialog != nil {
			return m, m.updateDialog(msg)
		}
		if key == "tab" || key == "shift+tab" {
			delta := 1
			if key == "shift+tab" {
				delta = 3
			}
			return m, m.setFocus(focus((int(m.focus) + delta) % 4))
		}
		if key == "enter" {
			return m, m.generate(false)
		}
		if !m.editing() {
			switch key {
			case "q":
				m.stopWork()
				return m, tea.Quit
			case "?":
				m.help, m.helpOffset = true, 0
				return m, nil
			case "r":
				return m, m.generate(true)
			case "e":
				m.openExport()
				return m, nil
			case "/":
				return m, m.setFocus(searchFocus)
			}
		}
		switch m.focus {
		case pickerFocus:
			visible := m.filtered()
			switch key {
			case "up", "k":
				m.picker = max(0, m.picker-1)
			case "down", "j":
				m.picker = min(max(0, len(visible)-1), m.picker+1)
			case " ":
				if len(visible) > 0 {
					m.selected[visible[m.picker].ID] = !m.selected[visible[m.picker].ID]
				}
			case "a":
				for _, c := range m.categories {
					m.selected[c.ID] = true
				}
			case "c":
				clear(m.selected)
			}
		case searchFocus:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.picker = 0
			return m, cmd
		case settingsFocus:
			if key == "down" || key == "up" || (!m.editing() && (key == "j" || key == "k")) {
				delta := 1
				if key == "up" || key == "k" {
					delta = 7
				}
				m.field = (m.field + delta) % 8
				return m, m.setFocus(settingsFocus)
			}
			if key == " " || key == "left" || key == "right" {
				switch m.field {
				case modeField:
					if m.mode == generator.ModeCategory {
						m.mode = generator.ModeBlend
					} else {
						m.mode = generator.ModeCategory
					}
				case genderField:
					genders := []generator.GenderFilter{generator.GenderAny, generator.GenderMasculine, generator.GenderFeminine, generator.GenderUnisex}
					for i, g := range genders {
						if g == m.gender {
							m.gender = genders[(i+1)%len(genders)]
							break
						}
					}
				case noveltyField:
					m.allow = !m.allow
				}
			}
			if value, ok := m.inputs[m.field]; ok {
				value, cmd := value.Update(msg)
				m.inputs[m.field] = value
				return m, cmd
			}
		case resultsFocus:
			if m.result != nil {
				switch key {
				case "up", "k":
					m.cursor = max(0, m.cursor-1)
				case "down", "j":
					m.cursor = min(max(0, len(m.result.Names)-1), m.cursor+1)
				case " ":
					m.toggleFavorite()
				}
			}
		}
	default:
		// Cursor blink messages go only to the active text input.
		if m.dialog != nil && m.dialog.focus == 2 {
			var cmd tea.Cmd
			m.dialog.path, cmd = m.dialog.path.Update(msg)
			return m, cmd
		}
		if m.focus == searchFocus {
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			return m, cmd
		}
		if m.focus == settingsFocus {
			if value, ok := m.inputs[m.field]; ok {
				value, cmd := value.Update(msg)
				m.inputs[m.field] = value
				return m, cmd
			}
		}
	}
	return m, nil
}

func (m *Model) isFavorite(name generator.GeneratedName) bool {
	for _, f := range m.favorites {
		if f.batch == m.batchID && f.name.Name == name.Name {
			return true
		}
	}
	return false
}

func (m *Model) toggleFavorite() {
	if m.result == nil || len(m.result.Names) == 0 {
		return
	}
	name := m.result.Names[m.cursor]
	for i, f := range m.favorites {
		if f.batch == m.batchID && f.name.Name == name.Name {
			m.favorites = append(m.favorites[:i], m.favorites[i+1:]...)
			return
		}
	}
	m.favorites = append(m.favorites, favorite{batch: m.batchID, result: *m.result, name: name})
}

func (m *Model) favoriteBatches() []export.FavoriteBatch {
	var batches []export.FavoriteBatch
	indices := make(map[uint64]int)
	for _, f := range m.favorites {
		i, ok := indices[f.batch]
		if !ok {
			i = len(batches)
			indices[f.batch] = i
			batches = append(batches, export.FavoriteBatch{Result: export.Result{SchemaVersion: export.SchemaVersion, Result: f.result}})
		}
		batches[i].Names = append(batches[i].Names, f.name)
	}
	return batches
}

func (m *Model) openExport() {
	if m.phase != ready {
		m.status = "Cancel or wait for the active operation before exporting."
		return
	}
	if m.result == nil && len(m.favorites) == 0 {
		m.status = "Generate a batch before exporting."
		return
	}
	m.dialog = &exportDialog{path: input("names.json", "destination path"), json: true, favorites: m.result == nil}
}

func (m *Model) updateDialog(msg tea.KeyMsg) tea.Cmd {
	d := m.dialog
	key := msg.String()
	if d.focus != 2 || d.confirm {
		if key == "q" {
			m.stopWork()
			return tea.Quit
		}
		if key == "?" {
			m.help, m.helpOffset = true, 0
			return nil
		}
	}
	if m.phase == exporting {
		return nil
	}
	if d.confirm {
		if key == "y" {
			return m.writeExport(true)
		}
		if key == "n" {
			d.confirm = false
			m.status = "Overwrite declined; choose another path."
		}
		return nil
	}
	if key == "tab" || key == "shift+tab" {
		delta := 1
		if key == "shift+tab" {
			delta = 2
		}
		d.focus = (d.focus + delta) % 3
		d.path.Blur()
		if d.focus == 2 {
			return d.path.Focus()
		}
		return nil
	}
	if key == "enter" {
		if strings.TrimSpace(d.path.Value()) == "" {
			m.status = "Enter a destination path."
			return nil
		}
		var buffer bytes.Buffer
		var err error
		if d.favorites {
			if len(m.favorites) == 0 {
				m.status = "No session favorites selected."
				return nil
			}
			batches := m.favoriteBatches()
			if d.json {
				err = export.WriteFavoritesJSON(&buffer, batches)
			} else {
				for _, batch := range batches {
					if err == nil {
						err = export.WriteText(&buffer, generator.Result{Names: batch.Names})
					}
				}
			}
		} else {
			if m.result == nil {
				m.status = "No current batch to export."
				return nil
			}
			if d.json {
				err = export.WriteJSON(&buffer, *m.result)
			} else {
				err = export.WriteText(&buffer, *m.result)
			}
		}
		if err != nil {
			m.status = "Encode export: " + err.Error()
			return nil
		}
		d.data, d.destination = buffer.Bytes(), d.path.Value()
		return m.writeExport(false)
	}
	if d.focus < 2 && (key == " " || key == "left" || key == "right") {
		if d.focus == 0 {
			d.favorites = !d.favorites
		} else {
			d.json = !d.json
			if path := d.path.Value(); path != "" {
				ext := filepath.Ext(path)
				if ext == filepath.Base(path) {
					// A dotfile without another suffix is a basename, not an extension.
					ext = ""
				}
				suffix := ".txt"
				if d.json {
					suffix = ".json"
				}
				d.path.SetValue(strings.TrimSuffix(path, ext) + suffix)
				d.path.CursorEnd()
			}
		}
		return nil
	}
	if d.focus == 2 {
		var cmd tea.Cmd
		d.path, cmd = d.path.Update(msg)
		return cmd
	}
	return nil
}

func (m *Model) writeExport(overwrite bool) tea.Cmd {
	m.stopWork()
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel, m.phase = cancel, exporting
	m.status = "Exporting… Esc cancels."
	id, path, data := m.requestID, m.dialog.destination, m.dialog.data
	return func() tea.Msg {
		return exportMsg{id: id, err: store.WriteExport(ctx, path, data, overwrite)}
	}
}
