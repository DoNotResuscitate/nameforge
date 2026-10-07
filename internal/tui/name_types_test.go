package tui

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/DoNotResuscitate/nameforge/internal/export"
	"github.com/DoNotResuscitate/nameforge/internal/generator"
)

func TestNameTypeControlsAndFavorites(t *testing.T) {
	m := testModel(t)
	m.selected["french"], m.selected["italian"] = true, true
	setInput(m, seedField, "42")
	m.field = nameTypeField
	m.setFocus(settingsFocus)
	key(m, " ")
	if m.nameType != generator.NameSurname || !strings.Contains(m.View(), "Name type: surname") {
		t.Fatal("surname control missing")
	}
	complete(t, m, key(m, "enter"))
	key(m, " ")
	if m.result.NameType != generator.NameSurname {
		t.Fatal("wrong generated type")
	}
	m.field = nameTypeField
	m.setFocus(settingsFocus)
	key(m, " ")
	setInput(m, surnameOrderField, "1")
	setInput(m, surnameMinField, "4")
	setInput(m, surnameMaxField, "10")
	complete(t, m, key(m, "enter"))
	key(m, " ")
	if m.result.NameType != generator.NameFull || m.result.Options.Surname.Order != 1 || len(m.favorites) != 2 {
		t.Fatal("full generation/settings/favorites missing")
	}
	var buffer bytes.Buffer
	if err := export.WriteFavoritesJSON(&buffer, m.favoriteBatches()); err != nil {
		t.Fatal(err)
	}
	var favorites struct {
		Batches []export.FavoriteBatch `json:"batches"`
	}
	if err := json.Unmarshal(buffer.Bytes(), &favorites); err != nil {
		t.Fatal(err)
	}
	if len(favorites.Batches) != 2 || favorites.Batches[0].Result.NameType != generator.NameSurname || favorites.Batches[1].Result.NameType != generator.NameFull || favorites.Batches[1].Result.SurnameBundleHash == "" || !reflect.DeepEqual(favorites.Batches[1].Names[0], m.result.Names[0]) {
		t.Fatal("favorite metadata lost")
	}
	buffer.Reset()
	if err := export.WriteText(&buffer, *m.result); err != nil || !strings.HasPrefix(buffer.String(), m.result.Names[0].Name+"\n") {
		t.Fatal("full text export lost composition")
	}
	m.selected["greek"] = true
	m.setFocus(pickerFocus)
	if !strings.Contains(m.View(), "UNAVAILABLE") {
		t.Fatal("surname gap not visible")
	}
	previous := m.result
	complete(t, m, key(m, "enter"))
	if m.result != previous || !strings.Contains(m.status, "surname data unavailable") {
		t.Fatal("missing component not recoverable")
	}
	delete(m.selected, "greek")
	setInput(m, surnameOrderField, "0")
	if cmd := key(m, "enter"); cmd != nil {
		t.Fatal("invalid component settings reached generation")
	}
}
