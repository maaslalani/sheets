package sheets

import (
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"strings"
	"unicode"
)

func defaultInsertKeys() Keymap {
	return Keymap{
		"ret":       ActionCommitDown,
		"tab":       ActionCommitRight,
		"S-tab":     ActionCommitLeft,
		"C-n":       ActionCommitDown,
		"C-p":       ActionCommitUp,
		"left":      ActionCursorLeft,
		"C-b":       ActionCursorLeft,
		"right":     ActionCursorRight,
		"C-f":       ActionCursorRight,
		"home":      ActionCursorHome,
		"C-a":       ActionCursorHome,
		"end":       ActionCursorEnd,
		"C-e":       ActionCursorEnd,
		"del":       ActionDeleteAtCursor,
		"C-d":       ActionDeleteAtCursor,
		"backspace": ActionDeleteBeforeCursor, // Ctrl+H == Backspace in terminals
		"C-u":       ActionDeleteToStart,
		"C-k":       ActionDeleteToEnd,
		"C-w":       ActionDeleteWordBefore,
		"space":     ActionInsertSpace,
	}
}

func (m model) updateInsert(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.recordingInsert && !m.replayingChange {
		m.insertKeys = append(m.insertKeys, msg)
	}

	action, ok := m.keymap.Insert[keyToString(msg)]
	if ok {
		switch action {
		case ActionCommitDown:
			return m.moveInsertSelection(1, 0)
		case ActionCommitRight:
			return m.moveInsertSelection(0, 1)
		case ActionCommitLeft:
			return m.moveInsertSelection(0, -1)
		case ActionCommitUp:
			return m.moveInsertSelection(-1, 0)
		case ActionCursorLeft:
			m.moveEditingCursor(-1)
			return m, m.restartCursorBlink()
		case ActionCursorRight:
			m.moveEditingCursor(1)
			return m, m.restartCursorBlink()
		case ActionCursorHome:
			m.editingCursor = 0
			return m, m.restartCursorBlink()
		case ActionCursorEnd:
			m.editingCursor = len([]rune(strings.ReplaceAll(m.editingValue, "\n", " ")))
			return m, m.restartCursorBlink()
		case ActionDeleteAtCursor:
			m.deleteAtEditingCursor()
			return m, m.restartCursorBlink()
		case ActionDeleteBeforeCursor:
			m.deleteBeforeEditingCursor()
			return m, m.restartCursorBlink()
		case ActionDeleteToStart:
			m.deleteToStartOfEditingCursor()
			return m, m.restartCursorBlink()
		case ActionDeleteToEnd:
			m.deleteToEndOfEditingCursor()
			return m, m.restartCursorBlink()
		case ActionDeleteWordBefore:
			m.deleteWordBeforeEditingCursor()
			return m, m.restartCursorBlink()
		case ActionInsertSpace:
			m.insertRunesAtEditingCursor([]rune{' '})
			return m, m.restartCursorBlink()
		}
	}

	// Fallback: insert typed runes (not configurable — raw text input).
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
		m.insertRunesAtEditingCursor(msg.Runes)
		return m, m.restartCursorBlink()
	}

	return m, nil
}

func (m model) moveInsertSelection(deltaRow, deltaCol int) (tea.Model, tea.Cmd) {
	m.commitCurrentInput()
	m.moveSelection(deltaRow, deltaCol)
	m.loadCurrentCellIntoEditor()
	return m, m.restartCursorBlink()
}

func (m *model) openRowBelowWithKeys(keys []tea.KeyMsg) tea.Cmd {
	insertRow := min(m.selectedRow+1, m.rowCount)
	m.insertRowAt(insertRow)
	m.selectedRow = clamp(insertRow, 0, m.rowCount-1)
	m.ensureVisible()
	return m.enterInsertModeWithKeys(keys)
}

func (m *model) openRowAboveWithKeys(keys []tea.KeyMsg) tea.Cmd {
	m.insertRowAt(m.selectedRow)
	m.ensureVisible()
	return m.enterInsertModeWithKeys(keys)
}

func (m *model) openColAfterWithKeys(keys []tea.KeyMsg) tea.Cmd {
	insertCol := min(m.selectedCol+1, totalCols-1)
	m.insertColAt(insertCol)
	m.selectedCol = clamp(insertCol, 0, totalCols-1)
	m.ensureVisible()
	return m.enterInsertModeWithKeys(keys)
}

func (m *model) openColBeforeWithKeys(keys []tea.KeyMsg) tea.Cmd {
	m.insertColAt(m.selectedCol)
	m.ensureVisible()
	return m.enterInsertModeWithKeys(keys)
}

func (m *model) enterInsertModeWithKeys(keys []tea.KeyMsg) tea.Cmd {
	m.mode = insertMode
	m.clearCount()
	m.recordingInsert = !m.replayingChange
	m.insertKeys = cloneKeySequence(keys)
	m.loadCurrentCellIntoEditor()
	return tea.Batch(
		m.editCursor.Focus(),
		m.editCursor.SetMode(cursor.CursorBlink),
	)
}

func (m *model) enterInsertModeAtStartWithKeys(keys []tea.KeyMsg) tea.Cmd {
	cmd := m.enterInsertModeWithKeys(keys)
	m.editingCursor = 0
	return cmd
}

func (m *model) changeCurrentCell(keys []tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.pushUndoState()
	m.setCellValue(m.selectedRow, m.selectedCol, "")
	cmd := m.enterInsertModeWithKeys(keys)
	m.editingValue = ""
	m.editingCursor = 0
	return *m, cmd
}

func (m *model) startFormulaFromSelection() (tea.Model, tea.Cmd) {
	ref := m.selectionRef()
	target := m.selectionInsertTarget()
	m.selectedRow = target.row
	m.selectedCol = target.col
	m.selectRow = target.row
	m.selectCol = target.col
	m.selectRows = false

	cmd := m.enterInsertModeWithKeys(nil)
	m.editingValue = "=(" + ref + ")"
	m.editingCursor = 1
	return *m, cmd
}

func (m *model) loadCurrentCellIntoEditor() {
	m.editingValue = strings.ReplaceAll(m.cellValue(m.selectedRow, m.selectedCol), "\n", " ")
	m.editingCursor = len([]rune(m.editingValue))
}

func (m *model) commitCurrentInput() {
	if m.cellValue(m.selectedRow, m.selectedCol) != m.editingValue {
		m.pushUndoState()
	}
	m.setCellValue(m.selectedRow, m.selectedCol, m.editingValue)
}

func (m model) exitInsertMode() (tea.Model, tea.Cmd) {
	changed := m.cellValue(m.selectedRow, m.selectedCol) != m.editingValue
	m.commitCurrentInput()
	m.mode = normalMode
	if changed && m.recordingInsert {
		m.saveLastChange(m.insertKeys)
	}
	m.insertKeys = nil
	m.recordingInsert = false
	m.editCursor.Blur()
	return m, nil
}

func (m model) renderEditingCell() string {
	cursorModel := m.editCursor
	cursorModel.TextStyle = lipgloss.NewStyle()
	return renderTextInput(m.editingValue, m.editingCursor, m.cellWidth, cursorModel, lipgloss.NewStyle())
}

func (m *model) moveEditingCursor(delta int) {
	moveTextInputCursor(m.editingValue, &m.editingCursor, delta)
}

func (m *model) insertRunesAtEditingCursor(runes []rune) {
	insertRunesAtTextInputCursor(&m.editingValue, &m.editingCursor, runes)
}

func (m *model) deleteBeforeEditingCursor() {
	deleteTextInputBeforeCursor(&m.editingValue, &m.editingCursor)
}

func (m *model) deleteAtEditingCursor() {
	deleteTextInputAtCursor(&m.editingValue, &m.editingCursor)
}

func (m *model) deleteToStartOfEditingCursor() {
	deleteTextInputToStartOfCursor(&m.editingValue, &m.editingCursor)
}

func (m *model) deleteWordBeforeEditingCursor() {
	deleteTextInputWordBeforeCursor(&m.editingValue, &m.editingCursor)
}

func (m *model) deleteToEndOfEditingCursor() {
	deleteTextInputToEndOfCursor(&m.editingValue, &m.editingCursor)
}

func renderTextInput(value string, cursorPos, width int, cursorModel cursor.Model, textStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}

	runes := normalizedTextInputValue(value)
	pos := clamp(cursorPos, 0, len(runes))
	cursorModel.TextStyle = textStyle

	cursorChar := " "
	cursorW := 1
	if pos < len(runes) {
		cursorChar = string(runes[pos])
		cursorW = runewidth.RuneWidth(runes[pos])
	}

	// Walk backward from cursor to find visible start index.
	start := pos
	usedWidth := cursorW
	for start > 0 {
		rw := runewidth.RuneWidth(runes[start-1])
		if usedWidth+rw > width {
			break
		}
		start--
		usedWidth += rw
	}

	cursorModel.SetChar(cursorChar)

	if pos < len(runes) {
		// Walk forward from cursor to find visible end index.
		end := pos + 1
		for end < len(runes) {
			rw := runewidth.RuneWidth(runes[end])
			if usedWidth+rw > width {
				break
			}
			end++
			usedWidth += rw
		}

		before := string(runes[start:pos])
		after := string(runes[pos+1 : end])
		renderedWidth := runewidth.StringWidth(before) + cursorW + runewidth.StringWidth(after)
		return textStyle.Render(before) + cursorModel.View() + textStyle.Render(after) + textStyle.Render(strings.Repeat(" ", max(0, width-renderedWidth)))
	}

	before := string(runes[start:pos])
	renderedWidth := runewidth.StringWidth(before) + 1
	return textStyle.Render(before) + cursorModel.View() + textStyle.Render(strings.Repeat(" ", max(0, width-renderedWidth)))
}

func normalizedTextInputValue(value string) []rune {
	return []rune(strings.ReplaceAll(value, "\n", " "))
}

func moveTextInputCursor(value string, cursor *int, delta int) {
	*cursor = clamp(*cursor+delta, 0, len(normalizedTextInputValue(value)))
}

func insertRunesAtTextInputCursor(value *string, cursor *int, runes []rune) {
	current := normalizedTextInputValue(*value)
	pos := clamp(*cursor, 0, len(current))
	updated := make([]rune, 0, len(current)+len(runes))
	updated = append(updated, current[:pos]...)
	updated = append(updated, runes...)
	updated = append(updated, current[pos:]...)
	*value = string(updated)
	*cursor = pos + len(runes)
}

func deleteTextInputBeforeCursor(value *string, cursor *int) {
	current := normalizedTextInputValue(*value)
	pos := clamp(*cursor, 0, len(current))
	if pos == 0 {
		return
	}

	*value = string(append(current[:pos-1], current[pos:]...))
	*cursor = pos - 1
}

func deleteTextInputAtCursor(value *string, cursor *int) {
	current := normalizedTextInputValue(*value)
	pos := clamp(*cursor, 0, len(current))
	if pos >= len(current) {
		return
	}

	*value = string(append(current[:pos], current[pos+1:]...))
}

func deleteTextInputToStartOfCursor(value *string, cursor *int) {
	current := normalizedTextInputValue(*value)
	pos := clamp(*cursor, 0, len(current))
	*value = string(current[pos:])
	*cursor = 0
}

func deleteTextInputWordBeforeCursor(value *string, cursor *int) {
	current := normalizedTextInputValue(*value)
	pos := clamp(*cursor, 0, len(current))
	start := pos
	for start > 0 && unicode.IsSpace(current[start-1]) {
		start--
	}
	for start > 0 && !unicode.IsSpace(current[start-1]) {
		start--
	}
	if start == pos {
		return
	}

	*value = string(append(current[:start], current[pos:]...))
	*cursor = start
}

func deleteTextInputToEndOfCursor(value *string, cursor *int) {
	current := normalizedTextInputValue(*value)
	pos := clamp(*cursor, 0, len(current))
	*value = string(current[:pos])
}

func (m *model) restartCursorBlink() tea.Cmd {
	m.editCursor.Blink = false
	return m.editCursor.BlinkCmd()
}
