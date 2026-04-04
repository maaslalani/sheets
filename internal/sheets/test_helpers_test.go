package sheets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newInsertEditingModel(value string, cursor int) model {
	m := newModel()
	m.mode = insertMode
	m.editingValue = value
	m.editingCursor = cursor
	return m
}

func newPendingCommandModel(buffer string, cursor int) model {
	m := newModel()
	m.mode = commandMode
	m.commandPending = true
	m.commandBuffer = buffer
	m.commandCursor = cursor
	m.editCursor.Focus()
	return m
}

func tempCSVPath(t *testing.T, relativePath string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("expected temp dir creation to succeed, got %v", err)
	}

	return path
}

func writeTempCSV(t *testing.T, relativePath, contents string) string {
	t.Helper()

	path := tempCSVPath(t, relativePath)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("expected temp CSV write to succeed, got %v", err)
	}

	return path
}

func writeTempTSV(t *testing.T, relativePath, contents string) string {
	t.Helper()

	path := tempCSVPath(t, relativePath)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("expected temp TSV write to succeed, got %v", err)
	}

	return path
}

func runeKey(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func updateKey(t *testing.T, m model, msg tea.KeyMsg) (model, tea.Cmd) {
	t.Helper()

	updated, cmd := m.Update(msg)
	got, ok := updated.(model)
	if !ok {
		t.Fatal("expected updated model")
	}

	return got, cmd
}

func applyKey(t *testing.T, m model, msg tea.KeyMsg) model {
	t.Helper()

	got, _ := updateKey(t, m, msg)
	return got
}

func applyKeys(t *testing.T, m model, msgs ...tea.KeyMsg) model {
	t.Helper()

	for _, msg := range msgs {
		m = applyKey(t, m, msg)
	}

	return m
}

func startCommand(t *testing.T, m model, command string) model {
	t.Helper()
	return applyKeys(t, m, runeKey(":"), runeKey(command))
}

func assertCellValue(t *testing.T, m model, row, col int, want string) {
	t.Helper()

	if got := m.cellValue(row, col); got != want {
		t.Fatalf("expected cell %s value %q, got %q", cellRef(row, col), want, got)
	}
}

func assertDisplayValue(t *testing.T, m model, row, col int, want string) {
	t.Helper()

	if got := m.displayValue(row, col); got != want {
		t.Fatalf("expected display at %s %q, got %q", cellRef(row, col), want, got)
	}
}

func assertSelection(t *testing.T, m model, wantRow, wantCol int) {
	t.Helper()

	if m.selectedRow != wantRow || m.selectedCol != wantCol {
		t.Fatalf("expected selection (%d,%d), got (%d,%d)", wantRow, wantCol, m.selectedRow, m.selectedCol)
	}
}

func assertSelectionAnchor(t *testing.T, m model, wantRow, wantCol int) {
	t.Helper()

	if m.selectRow != wantRow || m.selectCol != wantCol {
		t.Fatalf("expected selection anchor (%d,%d), got (%d,%d)", wantRow, wantCol, m.selectRow, m.selectCol)
	}
}

func assertContainsAll(t *testing.T, label, text string, parts ...string) {
	t.Helper()

	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Fatalf("expected %s to include %q: %q", label, part, text)
		}
	}
}

func assertNotContainsAny(t *testing.T, label, text string, parts ...string) {
	t.Helper()

	for _, part := range parts {
		if strings.Contains(text, part) {
			t.Fatalf("did not expect %s to include %q: %q", label, part, text)
		}
	}
}

func deltaForKey(msg tea.KeyMsg) int {
	switch msg.Type {
	case tea.KeyCtrlN:
		return 1
	case tea.KeyCtrlP:
		return -1
	default:
		return 0
	}
}
