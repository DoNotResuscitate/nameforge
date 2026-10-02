package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestVersionBumps(t *testing.T) {
	for _, test := range []struct {
		messages []string
		want     string
	}{
		{[]string{"fix(cli): correct option handling"}, "v1.2.4"},
		{[]string{"docs: explain controls", "ci: verify releases", "chore: update tooling"}, "v1.2.4"},
		{[]string{"Merge pull request #12 from branch", "feat(tui): add controls"}, "v1.3.0"},
		{[]string{"feat(tui)!: change export schema"}, "v2.0.0"},
		{[]string{"fix!: remove obsolete behavior"}, "v2.0.0"},
		{[]string{"feat: add controls\n\nBREAKING CHANGE: previous settings are incompatible"}, "v2.0.0"},
		{[]string{"chore: update dependencies\n\nBREAKING-CHANGE: minimum platform changed", "feat: add controls"}, "v2.0.0"},
		{[]string{"docs: mention feat: in prose"}, "v1.2.4"},
	} {
		got, err := bump(version{1, 2, 3}, test.messages)
		if err != nil || got != test.want {
			t.Errorf("bump(%q) = %q, %v; want %s", test.messages, got, err, test.want)
		}
	}
	if _, err := bump(version{patch: ^uint64(0)}, []string{"fix: test"}); err == nil {
		t.Fatal("version overflow accepted")
	}
}

func TestStableTags(t *testing.T) {
	for _, tag := range []string{"v1.2.3-rc.1", "v01.2.3", "v1.2", "unrelated", "v1.2.3+build"} {
		got, err := parse(tag)
		if err != nil || got != nil {
			t.Errorf("non-stable tag %q influenced release: %v, %v", tag, got, err)
		}
	}
	got, err := parse("v10.2.3")
	if err != nil || got == nil || !got.after(version{9, 20, 30}) {
		t.Fatalf("numeric version ordering failed: %v, %v", got, err)
	}
}

func TestHistoryAndRetry(t *testing.T) {
	// Isolated Git history exercises real tag ancestry and annotated-tag peeling.
	// No repository config changes, remote access or source-name fixtures.
	root := t.TempDir()
	t.Chdir(root)
	gitTest := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.invalid", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	assert := func(want string) {
		t.Helper()
		got, err := next()
		if err != nil || got != want {
			t.Fatalf("next = %q, %v; want %s", got, err, want)
		}
	}
	gitTest("init", "-b", "main")
	gitTest("commit", "--allow-empty", "-m", "feat: bootstrap")
	assert("v0.1.0")
	gitTest("tag", "-a", "v0.1.0", "-m", "release")
	assert("v0.1.0")
	gitTest("commit", "--allow-empty", "-m", "docs: clarify usage")
	assert("v0.1.1")
	gitTest("tag", "v0.1.1")
	gitTest("commit", "--allow-empty", "-m", "feat: extend controls")
	gitTest("tag", "v9.0.0-rc.1")
	assert("v0.2.0")
	gitTest("switch", "-c", "other")
	gitTest("commit", "--allow-empty", "-m", "feat!: unrelated branch")
	gitTest("tag", "v10.0.0")
	gitTest("switch", "main")
	assert("v0.2.0")
}
