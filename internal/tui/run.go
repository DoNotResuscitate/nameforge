package tui

import (
	"context"
	"errors"
	"io"

	"github.com/DoNotResuscitate/nameforge/internal/corpus"
	tea "github.com/charmbracelet/bubbletea"
)

var ErrInterrupted = errors.New("interactive session interrupted")

// Run delegates raw-mode/alternate-screen lifecycle to Bubble Tea. A child
// context also cancels outstanding work on normal quit or a program failure.
func Run(ctx context.Context, bundle *corpus.Bundle, stdin io.Reader, stdout io.Writer, noColor bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m := New(ctx, bundle, noColor)
	defer m.stopWork()
	_, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(stdin), tea.WithOutput(stdout),
		tea.WithAltScreen(), tea.WithoutSignalHandler()).Run()
	if m.interrupted {
		return ErrInterrupted
	}
	return err
}
