//go:build darwin || linux

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/DoNotResuscitate/nameforge/internal/export"
	"github.com/creack/pty"
	"golang.org/x/term"
)

type terminalSession struct {
	t      *testing.T
	cmd    *exec.Cmd
	master *os.File
	slave  *os.File
	before *term.State
	mu     sync.Mutex
	output strings.Builder
	mark   int
}

func startTerminal(t *testing.T, cmd *exec.Cmd) *terminalSession {
	t.Helper()
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	if err := pty.Setsize(master, &pty.Winsize{Rows: 24, Cols: 100}); err != nil {
		t.Fatal(err)
	}
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	s := &terminalSession{t: t, cmd: cmd, master: master, slave: slave, before: before}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = slave.Close()
		_ = master.Close()
	})
	go func() {
		buffer := make([]byte, 8192)
		for {
			n, err := master.Read(buffer)
			if n > 0 {
				s.mu.Lock()
				s.output.Write(buffer[:n])
				s.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	s.wait("CATEGORIES")
	return s
}

func (s *terminalSession) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.output.String()
}

func (s *terminalSession) send(keys string) {
	s.t.Helper()
	s.mark = len(s.text())
	if _, err := s.master.WriteString(keys); err != nil {
		s.t.Fatal(err)
	}
}

func (s *terminalSession) wait(text string) {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(s.text()[s.mark:], text) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.t.Fatalf("terminal did not show %q; tail since action:\n%s", text, s.text()[s.mark:])
}

func (s *terminalSession) selectCategory(id string) {
	s.send("/\x15" + id)
	s.wait("Search:")
	// Esc is sent separately because Esc followed immediately by Space may be
	// parsed as an Alt-Space key by terminal input decoders.
	s.send("\x1b")
	time.Sleep(40 * time.Millisecond)
	s.send(" ")
	s.wait("[x]")
}

func (s *terminalSession) exportBatch(path string, favorites bool) {
	s.send("e")
	s.wait("EXPORT")
	if favorites {
		s.send(" ")
		s.wait("session favorites")
	}
	s.send("\t\t\x15" + path + "\r")
	s.wait("RESULTS")
}

func (s *terminalSession) exportText(path string) {
	s.send("e")
	s.wait("EXPORT")
	s.send("\t ")
	s.wait("Format: text")
	s.wait("names.txt")
	s.send("\t\x15" + path + "\r")
	s.wait("RESULTS")
}

func (s *terminalSession) finish(keys string, code int) {
	s.t.Helper()
	s.send(keys)
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()
	select {
	case err := <-done:
		actual := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				s.t.Fatal(err)
			}
			actual = exit.ExitCode()
		}
		if actual != code {
			s.t.Fatalf("terminal exit %d, want %d", actual, code)
		}
	case <-time.After(10 * time.Second):
		s.t.Fatal("terminal did not quit")
	}
	after, err := term.GetState(int(s.master.Fd()))
	if err != nil || !reflect.DeepEqual(after, s.before) {
		s.t.Fatalf("terminal modes not restored: %v", err)
	}
	if !strings.Contains(s.text(), "\x1b[?1049l") {
		s.t.Fatal("alternate screen not restored")
	}
	if regexp.MustCompile(`\x1b\[[0-9;]*m`).MatchString(s.text()) {
		s.t.Fatal("no-color emitted SGR styling")
	}
}

// Real offline first-launch walkthrough: keyboard selection/settings, generation,
// exports, CLI replay, resize, and cleanup. macOS enforces network/home-write
// denial; Linux exercises the same workflow without claiming an OS sandbox.
func TestBinaryTUI(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "nameforge")
	build := exec.Command("mise", "exec", "--", "go", "build", "-trimpath", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o555); err != nil {
		t.Fatal(err)
	}
	command := func(args ...string) *exec.Cmd {
		cmd := exec.Command(binary, args...)
		if runtime.GOOS == "darwin" {
			profile := fmt.Sprintf(`(version 1)(allow default)(deny network*)(deny file-write* (subpath %q))`, home)
			cmd = exec.Command("/usr/bin/sandbox-exec", append([]string{"-p", profile, binary}, args...)...)
		}
		cmd.Dir = root
		cmd.Env = []string{"HOME=" + home, "PATH=" + filepath.Join(root, "no-tools"), "TERM=xterm-256color", "NO_COLOR=1"}
		return cmd
	}
	for _, mode := range []string{"category", "blend"} {
		for _, categories := range [][]string{{"french", "italian"}, {"greek", "arabic"}, {"all"}} {
			t.Run(mode+"/"+strings.Join(categories, "+"), func(t *testing.T) {
				// Exercise both default startup and the explicit tui command.
				var args []string
				if mode == "blend" {
					args = []string{"tui", "--no-color", "--data-dir", filepath.Join(root, "absent-state")}
				}
				s := startTerminal(t, command(args...))
				for _, id := range categories {
					if id == "all" {
						s.send("a")
						s.wait("Selected 10")
					} else {
						s.selectCategory(id)
					}
				}
				s.send("\t\t")
				s.wait("SETTINGS")
				if mode == "blend" {
					s.send(" ")
					s.wait("Mode: blend")
				}
				s.send(strings.Repeat("\x1b[B", 6) + "42\r")
				s.wait("Complete: 20 names")
				if !strings.Contains(s.text(), "Seed: 42") {
					t.Fatal("replay seed not displayed")
				}
				if mode == "category" && categories[0] == "greek" {
					if !strings.Contains(s.text(), "Wikipedia") || !strings.Contains(s.text(), "Wikidata") || !strings.Contains(s.text(), "Latin") {
						t.Fatal("romanized source/script labels absent")
					}
					s.send("\x1b[Z" + strings.Repeat("\x1b[A", 5) + " \r")
					s.wait("Generation failed") // Greek has no masculine evidence.
					for _, gender := range []string{"feminine", "unisex", "any"} {
						s.send(" ")
						s.wait("Gender: " + gender)
					}
					s.send("\r") // Restore any and replay.
					s.wait("Complete: 20 names")
				}
				path := filepath.Join(root, mode+"-"+strings.Join(categories, "-")+".json")
				s.exportBatch(path, false)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var result export.Result
				if err := json.Unmarshal(data, &result); err != nil || !result.Complete || result.Seed != 42 || len(result.Names) != 20 {
					t.Fatalf("TUI result: %v", err)
				}
				cliArgs := []string{"generate", "--mode", mode, "--seed", "42", "--format", "json"}
				for _, id := range result.CategoryIDs {
					cliArgs = append(cliArgs, "--category", id)
				}
				replay, err := command(cliArgs...).Output()
				if err != nil || string(replay) != string(data) {
					t.Fatalf("displayed seed CLI replay differs: %v", err)
				}
				textPath := strings.TrimSuffix(path, ".json") + ".txt"
				s.exportText(textPath)
				text, err := os.ReadFile(textPath)
				var expected strings.Builder
				for _, name := range result.Names {
					expected.WriteString(name.Name + "\n")
				}
				if err != nil || string(text) != expected.String() {
					t.Fatalf("actual text export: %v", err)
				}
				if mode == "category" && categories[0] == "french" {
					s.send(" ")
					s.wait("favorites 1")
					s.send("r")
					s.wait("Complete: 20 names")
					s.send(" ")
					s.wait("favorites 2")
					favoritesPath := filepath.Join(root, "favorites.json")
					s.exportBatch(favoritesPath, true)
					favorites, err := os.ReadFile(favoritesPath)
					if err != nil {
						t.Fatal(err)
					}
					var batches struct {
						Batches []export.FavoriteBatch `json:"batches"`
					}
					if err := json.Unmarshal(favorites, &batches); err != nil || len(batches.Batches) != 2 || batches.Batches[0].Result.Seed != 42 || batches.Batches[1].Result.Seed == 42 {
						t.Fatalf("multi-batch favorites: %v", err)
					}
					if err := pty.Setsize(s.master, &pty.Winsize{Rows: 12, Cols: 40}); err != nil {
						t.Fatal(err)
					}
					if err := s.cmd.Process.Signal(syscall.SIGWINCH); err != nil {
						t.Fatal(err)
					}
					s.send("?")
					s.wait("HELP")
					s.send("\x1b")
					time.Sleep(40 * time.Millisecond)
				}
				s.finish("q", 0)
			})
		}
	}
	for _, interrupt := range []string{"key", "signal"} {
		t.Run("active-interrupt-"+interrupt, func(t *testing.T) {
			s := startTerminal(t, command("tui"))
			s.selectCategory("french")
			s.send("\t\t" + strings.Repeat("\x1b[B", 2) + "\x154\x1b[B\x151000\x1b[B64\x1b[B64\r")
			s.wait("Generating")
			if interrupt == "signal" {
				if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
					t.Fatal(err)
				}
				s.finish("", 130)
			} else {
				s.finish("\x03", 130)
			}
		})
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("TUI wrote implicit local state: %v", err)
	}
}
