package faker

import (
	"reflect"
	"testing"
)

func TestParseLiteralSubset(t *testing.T) {
	// Deliberately non-name tokens, not an authored name corpus.
	data := []byte(`/* header */ export // wrapper
default { 'female': ['qzx', "qr\"x",], male: ['q\'vx', 'q\\rx'], generic: ['\x71\u0072\u{78}', '\uD83D\uDE00', 'q\
zx'] }; // end`)
	got, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"female": {"qzx", `qr"x`}, "male": {"q'vx", `q\rx`}, "generic": {"qrx", "😀", "qzx"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Parse = %#v, want %#v", got, want)
	}
	got, err = Parse([]byte(`export default ['qzx', 'vrk'];`))
	if err != nil || !reflect.DeepEqual(got, map[string][]string{"generic": {"qzx", "vrk"}}) {
		t.Fatalf("bare array = %#v, %v", got, err)
	}
}

func TestParseFailsClosed(t *testing.T) {
	for _, source := range []string{
		`import data from './x'; export default data;`,
		`export default {male: [...data]};`,
		`export default {male: ['qzx' + 'vrk']};`,
		`export default {male: [makeToken()]};`,
		"export default {male: [`qzx`]};",
		`export default {male: [12]};`,
		`export default {male: ['qzx'], male: ['vrk']};`,
		`export default {other: ['qzx']};`,
		`export default {male: ['qzx' 'vrk']};`,
		`export default {male: ['qzx'], generic: null};`,
		`export default {male: ['qzx']}; alert('x');`,
		`export default {male: ['qzx']`,
		`export default {male: ['qzx]};`,
		`export default {male: ['\uD800']};`,
		`export default {male: ['\uDC00']};`,
		`export default {male: ['\u{110000}']};`,
		`export default {male: ['\x+1']};`,
		`export default {male: ['\u12zz']};`,
		`export default {male: ['\01']};`,
		`export default {male: ['\z']};`,
		"export default {male: ['q\nzx']};",
		`export default {};`, `export default {male: []};`,
		`/* unterminated`, string([]byte{0xff}),
	} {
		t.Run(source, func(t *testing.T) {
			if got, err := Parse([]byte(source)); err == nil {
				t.Fatalf("accepted unsupported source %q: %#v", source, got)
			}
		})
	}
}
