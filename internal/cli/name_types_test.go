package cli

import (
	"strings"
	"testing"
)

func TestNameTypesCLIAndComponentSettings(t *testing.T) {
	for _, nameType := range []string{"surname", "full"} {
		for _, mode := range []string{"category", "blend"} {
			args := []string{"generate", "--name-type", nameType, "--category", "turkish", "--category", "italian", "--gender", "feminine", "--mode", mode, "--seed", "42"}
			if nameType == "full" {
				args = append(args, "--surname-order", "1", "--surname-min-length", "4", "--surname-max-length", "10", "--surname-allow-existing")
			}
			code, json, stderr := runCaptured(t, append(args, "--format", "json")...)
			if code != 0 || stderr != "" {
				t.Fatalf("%s: %d %s", nameType, code, stderr)
			}
			result := decodeResult(t, json)
			if string(result.NameType) != nameType || string(result.Options.NameType) != nameType {
				t.Fatal("name type omitted")
			}
			if nameType == "full" && (result.Options.Surname.Order != 1 || result.Options.Surname.MinLength != 4 || result.Options.Surname.MaxLength != 10 || !result.Options.Surname.AllowExisting || result.Names[0].Given == nil || result.Names[0].Surname == nil) {
				t.Fatal("component settings/metadata lost")
			}
			code, replay, stderr := runCaptured(t, append(args, "--format", "json")...)
			if code != 0 || stderr != "" || replay != json {
				t.Fatal("JSON replay changed")
			}
			code, text, metadata := runCaptured(t, args...)
			if code != 0 || !strings.Contains(text, result.Names[0].Name+"\n") || string(decodeResult(t, metadata).NameType) != nameType {
				t.Fatal("text metadata lost")
			}
		}
	}
	for _, args := range [][]string{
		{"--name-type", "unknown"}, {"--name-type", ""}, {"--surname-order", "1"},
		{"--name-type", "full", "--surname-order", "0"}, {"--name-type", "full", "--surname-order", "5"},
		{"--name-type", "full", "--surname-min-length", "10", "--surname-max-length", "5"},
	} {
		code, stdout, _ := runCaptured(t, append([]string{"generate", "--category", "french"}, args...)...)
		if code != 2 || stdout != "" {
			t.Fatalf("invalid options accepted: %v", args)
		}
	}
	for _, nameType := range []string{"surname", "full"} {
		code, stdout, stderr := runCaptured(t, "generate", "--name-type", nameType, "--category", "greek", "--seed", "42")
		if code != 1 || stdout != "" || !strings.Contains(stderr, "surname data unavailable") {
			t.Fatalf("missing category: %d %s", code, stderr)
		}
		code, stdout, stderr = runCaptured(t, "generate", "--name-type", nameType, "--category", "french", "--min-length", "64", "--max-length", "64", "--seed", "42")
		if code != 1 || stdout != "" || !strings.Contains(stderr, "attempts_exhausted") {
			t.Fatalf("partial batch leaked: %d %s", code, stderr)
		}
	}
	code, list, stderr := runCaptured(t, "data", "list", "--name-type", "surname")
	if code != 0 || stderr != "" || !strings.Contains(list, "turkish") || strings.Contains(list, "greek") {
		t.Fatal("surname catalog unavailable")
	}
	code, inspect, stderr := runCaptured(t, "data", "inspect", "--name-type", "surname", "--category", "french")
	if code != 0 || stderr != "" || !strings.Contains(inspect, "Records: 150") || !strings.Contains(inspect, "unspecified") {
		t.Fatal("wrong surname inspection")
	}
}
