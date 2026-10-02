package tui

import (
	"fmt"
	"strings"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/legal"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var fieldLabels = []string{"Mode", "Gender", "Order", "Count", "Minimum length", "Maximum length", "Replay seed", "Allow existing"}

func marker(active bool) string {
	if active {
		return "> "
	}
	return "  "
}

// window keeps the focused row visible even in a narrow/short terminal.
func window(lines []string, cursor, size int) []string {
	if len(lines) == 0 || size <= 0 {
		return nil
	}
	start := min(max(0, cursor-size/2), max(0, len(lines)-size))
	return lines[start:min(len(lines), start+size)]
}

func (m *Model) inputView(field int) string {
	return renderInput(m.inputs[field], m.width-22)
}

func renderInput(t textinput.Model, width int) string {
	t.Width = max(1, width)
	// Recompute the viewport on a copy after resizing, keeping View read-only.
	t.SetCursor(t.Position())
	return t.View()
}

func (m *Model) source(c corpus.Category) string {
	sources := make(map[string]bool)
	for _, r := range m.bundle.Records {
		for _, id := range r.Categories {
			if id == c.ID {
				sources[strings.SplitN(r.ID, ":", 2)[0]] = true
			}
		}
	}
	var labels []string
	if sources["faker"] {
		labels = append(labels, "Faker v10.6.0 / MIT")
	}
	if sources["wikipedia"] {
		labels = append(labels, "Wikipedia / CC BY-SA 4.0; gender unspecified")
	}
	if sources["wikidata"] {
		labels = append(labels, "Wikidata / CC0; broad Arabic")
	}
	return strings.Join(labels, "; ")
}

func (m *Model) settingsLines() []string {
	var lines []string
	for i, label := range fieldLabels {
		value := ""
		switch i {
		case modeField:
			value = string(m.mode)
		case genderField:
			value = string(m.gender)
		case noveltyField:
			value = fmt.Sprint(m.allow)
		default:
			value = m.inputView(i)
		}
		lines = append(lines, marker(m.field == i)+label+": "+value)
	}
	return lines
}

func (m *Model) helpLines() []string {
	if m.legal {
		return strings.Split(ansi.Hardwrap(m.legalText, max(1, m.width), true), "\n")
	}
	text := legal.Summary + "\nl: read complete legal notices here (up/down scroll; Esc dismisses)\nStatus: " + m.status + "\n"
	if m.result != nil {
		text += fmt.Sprintf("Batch seed: %d\nBatch mode: %s; complete: %t\nCorpus: %s\nAlgorithm: %s\n", m.result.Seed, m.result.Mode, m.result.Complete, m.result.BundleHash, m.result.AlgorithmVersion)
		for _, id := range m.result.CategoryIDs {
			bounds := m.result.Bounds[id]
			text += fmt.Sprintf("%s: effective rune lengths %d..%d\n", id, bounds.Min, bounds.Max)
		}
	}
	text += `Controls
Tab / Shift-Tab: categories -> search -> settings -> results
Arrows or j/k: navigate (j/k remain text while editing)
Space: toggle category, setting, or favorite
a: select ALL categories; c: clear selection; /: search
Enter: generate using settings and entered seed (blank = random)
r: regenerate using settings and a fresh seed; entered seed is retained
e: export dialog; Tab moves target / format / path; Space toggles
?: help; l: full legal notices; Esc: dismiss / cancel; q: quit outside text entry
Ctrl-C: quit globally, including text entry and active operations
Settings: up/down chooses a field; blank lengths use observed bounds
Text editing: Ctrl-A/E start/end; Ctrl-U clears before cursor
Favorites last only for this session. JSON preserves each original batch
and selected_names; text exports names only, without replay metadata.
Existing export paths require explicit y confirmation; n declines.
Generation is bounded: partial batches are labeled INCOMPLETE.
Category mode chooses categories equally; blend learns from the union,
where larger lists influence more transitions. Blends are hybrid styles.
Output is Latin-only, NFC, rune-length checked, with simple display casing.
Unspecified source gender is not unisex. No English fallback.
Built-ins need no writable home, network, or data directory.
Sources (revision-pinned; full notices: nameforge licenses)`
	for _, c := range m.categories {
		text += "\n" + c.Label + ": " + m.source(c)
	}
	text += "\nGreek mixes ancient, mythological, Christian and modern material.\nArabic has no North African regional claim.\nSource labels describe training lists, not generated-name origins."
	return strings.Split(ansi.Hardwrap(text, max(1, m.width), true), "\n")
}

func (m *Model) headerLines() []string {
	var selected []string
	for _, c := range m.categories {
		if m.selected[c.ID] {
			selected = append(selected, c.ID)
		}
	}
	selection := strings.Join(selected, ", ")
	if selection == "" {
		selection = "none"
	}
	title := "Nameforge © 2026 contributors — GPLv3; NO WARRANTY; l: licenses"
	if !m.noColor {
		title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Render(title)
	}
	lines := []string{title, fmt.Sprintf("Selected %d: %s", len(selected), selection),
		fmt.Sprintf("Settings: %s | %s | order %s | count %s | favorites %d", m.mode, m.gender, m.inputs[orderField].Value(), m.inputs[countField].Value(), len(m.favorites))}
	if m.result != nil {
		state := "complete"
		if !m.result.Complete {
			state = "INCOMPLETE"
		}
		lines = append(lines, fmt.Sprintf("Seed: %d | %s | %s | %d/%d names", m.result.Seed, state, m.result.Mode, len(m.result.Names), m.result.Options.Count))
	} else {
		lines = append(lines, "Batch: none | seed: not generated")
	}
	// Both essentials fit at the minimum supported width. Narrow layouts wrap
	// the legal heading into two fixed lines instead of truncating its terms.
	if m.width < 60 {
		lines = append([]string{"©2026 Nameforge team", "GPLv3 NO WARRANTY; l"}, lines[1:]...)
	}
	if m.height < 12 {
		seed := "none"
		if m.result != nil {
			seed = fmt.Sprint(m.result.Seed)
		}
		lines = []string{"©2026 Nameforge team", "GPLv3 NO WARRANTY; l", fmt.Sprintf("Selected %d | seed %s", len(selected), seed)}
	}
	return lines
}

func (m *Model) helpPageSize() int {
	maxStatus := 2
	if m.height < 12 {
		maxStatus = 1
	}
	status := min(maxStatus, len(strings.Split(ansi.Hardwrap(m.status, max(1, m.width), true), "\n")))
	return max(1, m.height-len(m.headerLines())-status-2) // help heading and footer
}

func (m *Model) View() string {
	if m.width < 20 || m.height < 8 {
		return ansi.Truncate("Resize terminal (20x8 minimum); Ctrl-C quits.", m.width, "")
	}
	lines := m.headerLines()
	// Reserve two status lines and a key-hint line, always visible.
	statusLines := strings.Split(ansi.Hardwrap(m.status, m.width, true), "\n")
	statusLines = statusLines[:min(2, len(statusLines))]
	if m.height < 12 {
		statusLines = statusLines[:1]
	}
	space := m.height - len(lines) - len(statusLines) - 1
	var body []string
	if m.help {
		heading := "HELP — up/down scroll; l legal notices; Esc dismisses"
		if m.legal {
			heading = "LICENSES — up/down scroll; l help; Esc dismisses"
		}
		body = append(body, heading)
		help := m.helpLines()
		page := m.helpPageSize()
		mOffset := min(m.helpOffset, max(0, len(help)-page))
		body = append(body, help[mOffset:min(len(help), mOffset+page)]...)
	} else if m.dialog != nil {
		d := m.dialog
		target, format := "current batch", "text"
		if d.favorites {
			target = "session favorites"
		}
		if d.json {
			format = "JSON"
		}
		body = []string{"EXPORT — Tab moves; Space toggles; Enter writes; Esc dismisses",
			marker(d.focus == 0) + "Target: " + target, marker(d.focus == 1) + "Format: " + format,
			marker(d.focus == 2) + "Path: " + renderInput(d.path, m.width-10)}
		if d.confirm {
			body = append(body, "OVERWRITE existing destination? y = yes; n = no", d.destination)
		}
		if len(body) > space {
			cursor := d.focus + 1
			if d.confirm {
				cursor = len(body) - 2
			}
			body = window(body, cursor, space)
		}
	} else {
		switch m.focus {
		case pickerFocus, searchFocus:
			body = append(body, "CATEGORIES — Space toggle; a all; c clear; / search", "Search: "+renderInput(m.search, m.width-10))
			var rows []string
			visible := m.filtered()
			for i, c := range visible {
				check := "[ ]"
				if m.selected[c.ID] {
					check = "[x]"
				}
				genders := "unspecified"
				if len(c.SupportedGenders) > 0 {
					genders = fmt.Sprint(c.SupportedGenders)
				}
				rows = append(rows, fmt.Sprintf("%s%s %s | %d | %s | %s", marker(i == m.picker), check, c.Label, c.RecordCount, strings.Join(c.Scripts, ","), genders))
			}
			if len(rows) == 0 {
				rows = append(rows, "No matching categories. Edit search; existing selections remain.")
			}
			body = append(body, window(rows, m.picker, max(1, space-3))...)
			if len(visible) > 0 {
				body = append(body, "Source: "+m.source(visible[m.picker]))
			}
		case settingsFocus:
			body = append(body, "SETTINGS — up/down field; Space/left/right cycles")
			body = append(body, window(m.settingsLines(), m.field, max(1, space-1))...)
		case resultsFocus:
			body = append(body, "RESULTS — up/down selects; Space favorites; r fresh seed; e export")
			var rows []string
			if m.result == nil || len(m.result.Names) == 0 {
				rows = append(rows, "No names yet. Tab to choose categories, then Enter.")
			} else {
				for i, name := range m.result.Names {
					star := "[ ]"
					if m.isFavorite(name) {
						star = "[*]"
					}
					attribution := strings.Join(name.CategoryIDs, ",")
					if m.result.Mode == "blend" {
						attribution = "blended: " + attribution
					}
					rows = append(rows, fmt.Sprintf("%s%s %d. %s (%s)", marker(i == m.cursor), star, i+1, name.Name, attribution))
				}
			}
			body = append(body, window(rows, m.cursor, max(1, space-2))...)
			if m.result != nil {
				body = append(body, "Corpus: "+m.result.BundleHash+" | "+m.result.AlgorithmVersion)
			}
		}
	}
	body = body[:min(len(body), max(0, space))]
	lines = append(lines, body...)
	for len(lines) < m.height-len(statusLines)-1 {
		lines = append(lines, "")
	}
	lines = append(lines, statusLines...)
	lines = append(lines, "Tab focus | Enter generate | r fresh | e export | ? help | Esc cancel | q quit")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, m.width, "")
	}
	view := strings.Join(lines, "\n")
	if m.noColor {
		view = ansi.Strip(view)
	}
	return view
}
