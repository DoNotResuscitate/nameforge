// Package faker extracts literal upstream data without executing TypeScript.
package faker

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Parse accepts only export default { bucket: [quoted strings] };, or a bare
// exported array (the upstream generic-only form). Comments and trailing commas
// are supported. Everything else is an error, including extra statements.
func Parse(data []byte) (map[string][]string, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("source is not valid UTF-8")
	}
	p := parser{s: string(data)}
	for _, word := range []string{"export", "default"} {
		if err := p.expect(word); err != nil {
			return nil, err
		}
	}
	buckets := map[string][]string{}
	t, err := p.next()
	if err != nil {
		return nil, err
	}
	switch t {
	case "[":
		values, err := p.array()
		if err != nil {
			return nil, err
		}
		buckets["generic"] = values
	case "{":
		for {
			key, err := p.next()
			if err != nil {
				return nil, err
			}
			if key == "}" {
				break
			}
			if strings.HasPrefix(key, "\"") {
				key, _ = strconv.Unquote(key)
			}
			if key != "male" && key != "female" && key != "generic" {
				return nil, p.fail("unsupported bucket " + key)
			}
			if _, ok := buckets[key]; ok {
				return nil, p.fail("duplicate bucket " + key)
			}
			if err := p.expect(":"); err != nil {
				return nil, err
			}
			if err := p.expect("["); err != nil {
				return nil, err
			}
			values, err := p.array()
			if err != nil {
				return nil, err
			}
			buckets[key] = values
			t, err := p.next()
			if err != nil {
				return nil, err
			}
			if t == "}" {
				break
			}
			if t != "," {
				return nil, p.fail("expected comma or object end")
			}
		}
	default:
		return nil, p.fail("expected literal object or array")
	}
	t, err = p.next()
	if err != nil {
		return nil, err
	}
	if t == ";" {
		t, err = p.next()
	}
	if err != nil {
		return nil, err
	}
	if t != "" {
		return nil, p.fail("unexpected trailing syntax")
	}
	count := 0
	for _, values := range buckets {
		count += len(values)
	}
	if count == 0 {
		return nil, p.fail("missing or empty name arrays")
	}
	return buckets, nil
}

type parser struct {
	s string
	i int
}

func (p *parser) fail(message string) error {
	return fmt.Errorf("TypeScript byte %d: %s", p.i, message)
}
func (p *parser) expect(want string) error {
	got, err := p.next()
	if err != nil {
		return err
	}
	if got != want {
		return p.fail(fmt.Sprintf("expected %q, got %q", want, got))
	}
	return nil
}
func (p *parser) array() ([]string, error) {
	values := []string{}
	for {
		t, err := p.next()
		if err != nil {
			return nil, err
		}
		if t == "]" {
			return values, nil
		}
		if !strings.HasPrefix(t, "\"") {
			return nil, p.fail("array entries must be quoted strings")
		}
		value, err := strconv.Unquote(t)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
		t, err = p.next()
		if err != nil {
			return nil, err
		}
		if t == "]" {
			return values, nil
		}
		if t != "," {
			return nil, p.fail("expected comma or array end")
		}
	}
}
func (p *parser) next() (string, error) {
	for p.i < len(p.s) {
		r, n := utf8.DecodeRuneInString(p.s[p.i:])
		if unicode.IsSpace(r) {
			p.i += n
			continue
		}
		if strings.HasPrefix(p.s[p.i:], "//") {
			for p.i < len(p.s) && p.s[p.i] != '\n' {
				p.i++
			}
			continue
		}
		if strings.HasPrefix(p.s[p.i:], "/*") {
			end := strings.Index(p.s[p.i+2:], "*/")
			if end < 0 {
				return "", p.fail("unterminated comment")
			}
			p.i += end + 4
			continue
		}
		break
	}
	if p.i == len(p.s) {
		return "", nil
	}
	c := p.s[p.i]
	p.i++
	if c == '\'' || c == '"' {
		var b strings.Builder
		for p.i < len(p.s) {
			r, n := utf8.DecodeRuneInString(p.s[p.i:])
			p.i += n
			if r == rune(c) {
				return strconv.Quote(b.String()), nil
			}
			if r == '\n' || r == '\r' || r == '\u2028' || r == '\u2029' {
				return "", p.fail("unescaped newline in string")
			}
			if r != '\\' {
				b.WriteRune(r)
				continue
			}
			if p.i == len(p.s) {
				return "", p.fail("unterminated escape")
			}
			e := p.s[p.i]
			p.i++
			switch e {
			case '\'', '"', '\\':
				b.WriteByte(e)
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'v':
				b.WriteByte('\v')
			case '0':
				if p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
					return "", p.fail("octal escape is unsupported")
				}
				b.WriteByte(0)
			case '\n': // explicit JS line continuation
			case '\r':
				if p.i < len(p.s) && p.s[p.i] == '\n' {
					p.i++
				}
			case 'x', 'u':
				width := 2
				if e == 'u' {
					width = 4
				}
				braced := e == 'u' && p.i < len(p.s) && p.s[p.i] == '{'
				if braced {
					p.i++
					width = strings.IndexByte(p.s[p.i:], '}')
					if width < 1 || width > 6 {
						return "", p.fail("invalid Unicode escape")
					}
				}
				v, err := p.hex(width)
				if err != nil {
					return "", err
				}
				if braced {
					p.i++
				}
				if v >= 0xd800 && v <= 0xdbff && !braced {
					if !strings.HasPrefix(p.s[p.i:], "\\u") {
						return "", p.fail("unpaired Unicode surrogate")
					}
					p.i += 2
					low, err := p.hex(4)
					if err != nil {
						return "", err
					}
					if low < 0xdc00 || low > 0xdfff {
						return "", p.fail("unpaired Unicode surrogate")
					}
					v = utf16.DecodeRune(v, low)
				}
				if !utf8.ValidRune(v) {
					return "", p.fail("invalid Unicode code point")
				}
				b.WriteRune(v)
			default:
				return "", p.fail("unsupported string escape")
			}
		}
		return "", p.fail("unterminated string")
	}
	if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' {
		start := p.i - 1
		for p.i < len(p.s) {
			c := p.s[p.i]
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
				break
			}
			p.i++
		}
		return p.s[start:p.i], nil
	}
	if strings.ContainsRune("{}[],:;", rune(c)) {
		return string(c), nil
	}
	return "", p.fail(fmt.Sprintf("unsupported character %q", c))
}
func (p *parser) hex(width int) (rune, error) {
	if p.i+width > len(p.s) {
		return 0, p.fail("short hex escape")
	}
	for _, c := range p.s[p.i : p.i+width] {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return 0, p.fail("invalid hex escape")
		}
	}
	v, err := strconv.ParseUint(p.s[p.i:p.i+width], 16, 32)
	if err != nil {
		return 0, p.fail("invalid hex escape")
	}
	p.i += width
	return rune(v), nil
}
