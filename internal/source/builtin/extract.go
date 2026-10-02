package builtin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	"github.com/DoNotResuscitate/nameforge/internal/source/faker"
)

type occurrence struct {
	Name   string
	Ref    corpus.SourceRef
	Reason string
}

func latinName(raw string) bool {
	if !utf8.ValidString(raw) {
		return false
	}
	separator, letter := true, false
	for _, r := range raw {
		if strings.ContainsRune(" '-’ʼ", r) {
			if separator {
				return false
			}
			separator = true
			continue
		}
		if unicode.IsLetter(r) && unicode.Is(unicode.Latin, r) {
			separator = false
			letter = true
			continue
		}
		if unicode.IsMark(r) && !separator && (unicode.Is(unicode.Latin, r) || r >= 0x0300 && r <= 0x036f || r >= 0x1ab0 && r <= 0x1aff || r >= 0x1dc0 && r <= 0x1dff || r >= 0xfe20 && r <= 0xfe2f) {
			continue
		}
		return false
	}
	return letter && !separator
}

// Only these two static numbered lists are imported. Index is the zero-based
// numbered-row index within the original section; native associations are retained.
func greekOccurrences(data []byte, file corpus.SourceFile) ([]occurrence, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("Greek source is not UTF-8")
	}
	section := ""
	indices := map[string]int{}
	result := []occurrence{}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "===") {
			section = strings.TrimSpace(strings.Trim(line, "="))
		}
		if section != "Ancient names" && section != "Biblical and Christian names" || !strings.HasPrefix(line, "#") {
			continue
		}
		index := indices[section]
		indices[section]++
		// Native pair is the final parenthesized field, not parentheses in links.
		start := strings.LastIndex(line, " (")
		end := strings.LastIndex(line, ")")
		if start < 0 || end < start {
			return nil, fmt.Errorf("missing Greek pair in %s[%d]", section, index)
		}
		native := line[start+2 : end]
		label, err := wikiLabel(strings.TrimSpace(strings.TrimPrefix(line[:start], "#")))
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", section, index, err)
		}
		label = strings.ReplaceAll(label, " or ", "/")
		for variant, name := range strings.Split(label, "/") {
			name = strings.TrimSpace(name)
			ref := corpus.SourceRef{Revision: file.Revision, Path: file.Path, Bucket: "generic", Index: index, NativeName: native, StatementID: section + "/" + strconv.Itoa(index) + "/" + strconv.Itoa(variant)}
			reason := greekHoldout(section, index)
			if reason == "" && !latinName(name) {
				reason = "non-Latin or malformed source spelling"
			}
			result = append(result, occurrence{Name: name, Ref: ref, Reason: reason})
		}
	}
	if indices["Ancient names"] != 280 || indices["Biblical and Christian names"] != 242 {
		return nil, fmt.Errorf("unexpected pinned Greek section counts: %v", indices)
	}
	return result, nil
}

// wikiLabel supports only literal links (including supplied display text) and
// the one literal interlanguage-link row in the reviewed lists. No templates run.
func wikiLabel(s string) (string, error) {
	var out strings.Builder
	for len(s) > 0 {
		if strings.HasPrefix(s, "[[") {
			end := strings.Index(s, "]]")
			if end < 0 {
				return "", fmt.Errorf("unterminated link")
			}
			parts := strings.Split(s[2:end], "|")
			if len(parts) > 2 {
				return "", fmt.Errorf("unsupported link")
			}
			label := parts[len(parts)-1]
			if len(parts) == 1 {
				for _, suffix := range []string{" (disambiguation)", " (given name)", " (name)"} {
					label = strings.TrimSuffix(label, suffix)
				}
			}
			out.WriteString(label)
			s = s[end+2:]
			continue
		}
		if strings.HasPrefix(s, "{{ill|") {
			end := strings.Index(s, "}}")
			if end < 0 {
				return "", fmt.Errorf("unterminated ill")
			}
			parts := strings.Split(s[2:end], "|")
			if len(parts) != 4 || parts[0] != "ill" || parts[2] != "el" {
				return "", fmt.Errorf("unsupported template")
			}
			out.WriteString(parts[1])
			s = s[end+2:]
			continue
		}
		if strings.ContainsRune("{}[]<>", rune(s[0])) {
			return "", fmt.Errorf("unsupported wikitext")
		}
		out.WriteByte(s[0])
		s = s[1:]
	}
	return out.String(), nil
}
func greekHoldout(section string, index int) string {
	if section == "Ancient names" {
		switch index {
		case 1, 8, 9, 20:
			return "reviewed malformed Latin/Greek pairing"
		case 108, 129:
			return "reviewed English equivalent rather than Greek spelling"
		case 139:
			return "reviewed mismatched Greek association"
		case 152:
			return "epic title rather than given name"
		}
	}
	if section == "Biblical and Christian names" {
		switch index {
		case 2, 15, 22, 28, 77, 97, 126, 127, 128, 144, 145, 146, 150, 178, 225, 239:
			return "reviewed foreign equivalent rather than Greek spelling"
		case 49, 147:
			return "reviewed mismatched Greek association"
		case 207:
			return "linked source identifies a surname, not a given name"
		}
	}
	return ""
}

type snak struct {
	SnakType  string `json:"snaktype"`
	DataValue struct {
		Value json.RawMessage `json:"value"`
	} `json:"datavalue"`
}
type statement struct {
	ID         string            `json:"id"`
	Rank       string            `json:"rank"`
	Main       snak              `json:"mainsnak"`
	References []json.RawMessage `json:"references"`
}
type entity struct {
	ID       string                 `json:"id"`
	Revision int64                  `json:"lastrevid"`
	Claims   map[string][]statement `json:"claims"`
}

func decodeEntity(data []byte, file corpus.SourceFile) (entity, error) {
	var raw struct {
		Entities map[string]entity `json:"entities"`
	}
	if !utf8.Valid(data) {
		return entity{}, fmt.Errorf("invalid entity UTF-8")
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return entity{}, err
	}
	m := entityPath.FindStringSubmatch(file.Path)
	if m == nil {
		return entity{}, fmt.Errorf("invalid entity path")
	}
	e, ok := raw.Entities[m[1]]
	if !ok || len(raw.Entities) != 1 || e.ID != m[1] || strconv.FormatInt(e.Revision, 10) != file.Revision {
		return entity{}, fmt.Errorf("entity identity/revision mismatch in %s", file.Path)
	}
	return e, nil
}
func itemID(s statement) string {
	if s.Main.SnakType != "value" || s.Rank == "deprecated" {
		return ""
	}
	var value struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(s.Main.DataValue.Value, &value)
	return value.ID
}
func textValue(s statement) (string, string) {
	if s.Main.SnakType != "value" || s.Rank == "deprecated" {
		return "", ""
	}
	var value struct {
		Text     string `json:"text"`
		Language string `json:"language"`
	}
	_ = json.Unmarshal(s.Main.DataValue.Value, &value)
	return value.Text, value.Language
}
func arabicOccurrences(data []byte, file corpus.SourceFile) ([]occurrence, error) {
	e, err := decodeEntity(data, file)
	if err != nil {
		return nil, err
	}
	genders := map[string][]string{}
	evidence := []string{}
	for _, s := range e.Claims["P31"] {
		bucket := ""
		switch itemID(s) {
		case "Q12308941":
			bucket = "male"
		case "Q11879590":
			bucket = "female"
		case "Q202444":
			bucket = "generic"
		}
		if bucket != "" {
			genders[bucket] = append(genders[bucket], s.ID)
		}
	}
	for _, s := range e.Claims["P407"] {
		if itemID(s) == "Q13955" {
			evidence = append(evidence, s.ID)
		}
	}
	if len(genders) == 0 || len(evidence) == 0 {
		return nil, fmt.Errorf("missing Arabic language/given-name classification in %s", file.Path)
	}
	natives := []string{}
	for _, s := range e.Claims["P1705"] {
		text, lang := textValue(s)
		if lang == "ar" && strings.IndexFunc(text, func(r rune) bool { return unicode.Is(unicode.Arabic, r) && unicode.IsLetter(r) }) >= 0 {
			natives = append(natives, text)
			evidence = append(evidence, s.ID)
		}
	}
	result := []occurrence{}
	for index, s := range e.Claims["P1705"] {
		text, lang := textValue(s)
		if lang != "mul" {
			continue
		}
		for _, bucket := range []string{"female", "generic", "male"} {
			if len(genders[bucket]) == 0 {
				continue
			}
			refs := append(append([]string{}, evidence...), genders[bucket]...)
			sort.Strings(refs)
			ref := corpus.SourceRef{Revision: file.Revision, Path: file.Path, Bucket: bucket, Index: index, StatementID: s.ID, NativeName: strings.Join(natives, " / "), Evidence: refs}
			reason := arabicHoldout(e.ID)
			if reason == "" {
				reason = arabicStatementHoldout(s.ID)
			}
			if reason == "" && len(natives) == 0 {
				reason = "no Arabic-script native name association"
			}
			if reason == "" && !latinName(text) {
				reason = "non-Latin or malformed source spelling"
			}
			result = append(result, occurrence{Name: text, Ref: ref, Reason: reason})
		}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no supplied mul spellings in %s", file.Path)
	}
	return result, nil
}
func arabicHoldout(id string) string {
	switch id {
	case "Q639748", "Q21946841", "Q63919383", "Q113517732", "Q479503":
		return "reviewed multilingual spelling ambiguity; whole entity held out without invented corrections"
	case "Q21069195", "Q28790469", "Q139882721":
		return "reviewed Persian-script association rather than verified Arabic spelling"
	case "Q59051920":
		return "reviewed Persian/Urdu reading without verified Arabic romanization"
	}
	return ""
}

func arabicStatementHoldout(id string) string {
	switch id {
	case "Q7380459$1D5B62A5-803F-4554-A096-58854DB9F567", "Q107260989$21408579-6109-4DC0-AA90-8C038B77541A", "Q21081645$13FA3431-7F19-4925-BB86-BF1B15E5C563":
		return "reviewed Persian/Uzbek-specific rendering rather than Arabic romanization"
	case "Q3194364$98B01DB4-318D-44D2-B0CD-4BB1B4C13C97", "Q3607213$E570AE9E-5467-4155-8244-4A12E489AEE2":
		return "reviewed South/Southeast Asian rendering rather than Arabic romanization"
	case "Q4164677$53742042-75E4-48DD-A6AA-B4B6DC4457AC", "Q4164677$FE34CE0C-A233-49B3-BA49-CBF7426FE879":
		return "reviewed Turkish rendering; retain only supplied Arabic q-spelling"
	}
	return ""
}
func merge(category string, occurrences []occurrence) ([]corpus.Record, []faker.Rejection) {
	records := map[string]*corpus.Record{}
	rejected := []faker.Rejection{}
	for _, o := range occurrences {
		if o.Reason != "" {
			rejected = append(rejected, faker.Rejection{Ref: o.Ref, Reason: o.Reason})
			continue
		}
		name := corpus.NormalizeName(o.Name)
		k := strings.ToLower(name)
		r := records[k]
		if r == nil {
			r = &corpus.Record{ID: recordPrefix(category) + Hash([]byte(k)), Name: name, Categories: []string{category}, Genders: []corpus.Gender{}, SourceRefs: []corpus.SourceRef{}}
			records[k] = r
		}
		r.SourceRefs = append(r.SourceRefs, o.Ref)
		gender := corpus.Gender("")
		if o.Ref.Bucket == "male" {
			gender = corpus.GenderMasculine
		}
		if o.Ref.Bucket == "female" {
			gender = corpus.GenderFeminine
		}
		if gender != "" {
			found := false
			for _, g := range r.Genders {
				found = found || g == gender
			}
			if !found {
				r.Genders = append(r.Genders, gender)
			}
		}
	}
	keys := []string{}
	for k := range records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := []corpus.Record{}
	for _, k := range keys {
		r := records[k]
		sort.Slice(r.Genders, func(i, j int) bool { return r.Genders[i] < r.Genders[j] })
		result = append(result, *r)
	}
	return result, rejected
}

func recordPrefix(category string) string {
	if category == "greek" {
		return "wikipedia:el:"
	}
	if category == "arabic" {
		return "wikidata:ar:"
	}
	return "sourced:" + category + ":"
}

func sameJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
