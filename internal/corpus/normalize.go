package corpus

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// NormalizationVersion identifies the spelling normalization used by corpus v1.
const NormalizationVersion = "nfc-v1"

// NormalizeName trims surrounding Unicode whitespace and returns NFC.
func NormalizeName(name string) string {
	return norm.NFC.String(strings.TrimSpace(name))
}

// validName checks the normalized corpus representation without changing it.
func validName(name string) bool {
	if name == "" || !utf8.ValidString(name) || !norm.NFC.IsNormalString(name) || strings.TrimSpace(name) != name {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
