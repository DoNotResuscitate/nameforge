// Command release-version calculates a version without writing tags or files.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type version struct {
	major, minor, patch uint64
}

var (
	stableTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	header    = regexp.MustCompile(`^([a-z]+)(\([^\r\n]+\))?(!)?: .+`)
	breaking  = regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE: .+`)
)

func main() {
	value, err := next()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(value)
}

func git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	return strings.TrimSpace(string(output)), err
}

func next() (string, error) {
	// Restrict automatic versions to stable release tags in this main history.
	// Prerelease tags and unrelated branch tags do not advance stable versions.
	tags, err := git("tag", "--merged", "HEAD", "--list", "v*")
	if err != nil {
		return "", err
	}
	var latest *version
	latestTag := ""
	for _, tag := range strings.Fields(tags) {
		parsed, err := parse(tag)
		if err != nil {
			return "", err
		}
		if parsed != nil && (latest == nil || parsed.after(*latest)) {
			latest, latestTag = parsed, tag
		}
	}
	if latest == nil {
		return "v0.1.0", nil
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	tagCommit, err := git("rev-parse", latestTag+"^{commit}")
	if err != nil {
		return "", err
	}
	if tagCommit == head {
		return latestTag, nil // Retry the existing release, never retag a revision.
	}
	log, err := git("log", "--format=%B%x00", latestTag+"..HEAD")
	if err != nil {
		return "", err
	}
	return bump(*latest, strings.Split(log, "\x00"))
}

func parse(tag string) (*version, error) {
	match := stableTag.FindStringSubmatch(tag)
	if match == nil {
		return nil, nil
	}
	var values [3]uint64
	for i := range values {
		value, err := strconv.ParseUint(match[i+1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid release tag %q: %w", tag, err)
		}
		values[i] = value
	}
	return &version{values[0], values[1], values[2]}, nil
}

func (v version) after(other version) bool {
	if v.major != other.major {
		return v.major > other.major
	}
	if v.minor != other.minor {
		return v.minor > other.minor
	}
	return v.patch > other.patch
}

func (v version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.major, v.minor, v.patch)
}

func bump(previous version, messages []string) (string, error) {
	level := 0 // Every main update releases, including docs/ci/chore-only changes.
	for _, message := range messages {
		message = strings.TrimSpace(message)
		first, _, _ := strings.Cut(message, "\n")
		match := header.FindStringSubmatch(first)
		if breaking.MatchString(message) || match != nil && match[3] == "!" {
			level = 2
		} else if match != nil && match[1] == "feat" && level < 1 {
			level = 1
		}
	}
	var component *uint64
	switch level {
	case 2:
		component = &previous.major
		previous.minor, previous.patch = 0, 0
	case 1:
		component = &previous.minor
		previous.patch = 0
	default:
		component = &previous.patch
	}
	if *component == ^uint64(0) {
		return "", fmt.Errorf("release version overflow")
	}
	*component++
	return previous.String(), nil
}
