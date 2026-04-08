package sheets

import (
	"errors"
	"fmt"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
	"strconv"
	"strings"
	"unicode"
)

func (m *model) startCommandPrompt() tea.Cmd {
	m.clearNormalPrefixes()
	m.mode = commandMode
	m.promptKind = commandPrompt
	m.commandPending = true
	m.commandBuffer = ""
	m.commandCursor = 0
	return tea.Batch(
		m.editCursor.Focus(),
		m.editCursor.SetMode(cursor.CursorBlink),
	)
}

func (m *model) clearCommandPrompt() {
	m.mode = normalMode
	m.promptKind = noPrompt
	m.commandPending = false
	m.commandBuffer = ""
	m.commandCursor = 0
	m.editCursor.Blur()
}

func (m *model) handlePendingCommand(msg tea.KeyMsg) (tea.Cmd, bool) {
	if isEscapeKey(msg) {
		m.clearCommandPrompt()
		return nil, true
	}

	switch msg.Type {
	case tea.KeyEnter:
		return m.executePrompt(), true
	case tea.KeyLeft, tea.KeyCtrlB:
		moveTextInputCursor(m.commandBuffer, &m.commandCursor, -1)
		return m.restartCursorBlink(), true
	case tea.KeyRight, tea.KeyCtrlF:
		moveTextInputCursor(m.commandBuffer, &m.commandCursor, 1)
		return m.restartCursorBlink(), true
	case tea.KeyHome, tea.KeyCtrlA:
		m.commandCursor = 0
		return m.restartCursorBlink(), true
	case tea.KeyEnd, tea.KeyCtrlE:
		m.commandCursor = len(normalizedTextInputValue(m.commandBuffer))
		return m.restartCursorBlink(), true
	case tea.KeyBackspace:
		if m.commandBuffer == "" {
			m.clearCommandPrompt()
			return nil, true
		}
		deleteTextInputBeforeCursor(&m.commandBuffer, &m.commandCursor)
		return m.restartCursorBlink(), true
	case tea.KeyDelete, tea.KeyCtrlD:
		deleteTextInputAtCursor(&m.commandBuffer, &m.commandCursor)
		return m.restartCursorBlink(), true
	case tea.KeySpace:
		insertRunesAtTextInputCursor(&m.commandBuffer, &m.commandCursor, []rune{' '})
		return m.restartCursorBlink(), true
	case tea.KeyCtrlK:
		deleteTextInputToEndOfCursor(&m.commandBuffer, &m.commandCursor)
		return m.restartCursorBlink(), true
	case tea.KeyCtrlU:
		deleteTextInputToStartOfCursor(&m.commandBuffer, &m.commandCursor)
		return m.restartCursorBlink(), true
	case tea.KeyCtrlW:
		deleteTextInputWordBeforeCursor(&m.commandBuffer, &m.commandCursor)
		return m.restartCursorBlink(), true
	case tea.KeyRunes:
		if len(msg.Runes) == 0 {
			return m.restartCursorBlink(), true
		}
		insertRunesAtTextInputCursor(&m.commandBuffer, &m.commandCursor, msg.Runes)
		return m.restartCursorBlink(), true
	}

	switch msg.String() {
	case "ctrl+h":
		if m.commandCursor == 0 && m.commandBuffer == "" {
			m.clearCommandPrompt()
			return nil, true
		}
		deleteTextInputBeforeCursor(&m.commandBuffer, &m.commandCursor)
		return m.restartCursorBlink(), true
	default:
		return nil, true
	}
}

func (m *model) executePrompt() tea.Cmd {
	command := strings.TrimSpace(m.commandBuffer)
	promptKind := m.promptKind
	m.clearCommandPrompt()
	if promptKind == searchForwardPrompt || promptKind == searchBackwardPrompt {
		return m.executeSearchPrompt(command, promptKind)
	}
	if command == "" {
		return nil
	}

	name, arg := splitCommandArgument(command)
	switch {
	case strings.EqualFold(command, "q"),
		strings.EqualFold(command, "q!"),
		strings.EqualFold(command, "quit"),
		strings.EqualFold(command, "quit!"),
		strings.EqualFold(command, "exit"),
		strings.EqualFold(command, "exit!"):
		return tea.Quit
	case strings.EqualFold(command, "wq"),
		strings.EqualFold(command, "x"):
		if err := m.writeCurrentFile(); err != nil {
			m.commandMessage = err.Error()
			m.commandError = true
			return nil
		}
		return tea.Quit
	case strings.EqualFold(command, "help"),
		strings.EqualFold(command, "?"):
		m.commandMessage = "Commands: q, w, wq, x, goto <cell>, <cell>, resize <n|auto|content|width>, e[dit] <path>, w[rite] [path]"
		m.commandError = false
		return nil
	}

	if ref, ok := parseCellRef(command); ok {
		m.recordJumpFromCurrent()
		m.goToCell(ref.row, ref.col)
		return nil
	}

	if strings.EqualFold(name, "goto") || strings.EqualFold(name, "go") {
		ref, ok := parseCellRef(arg)
		if !ok {
			m.commandMessage = fmt.Sprintf("invalid cell: '%s'", arg)
			m.commandError = true
			return nil
		}
		m.recordJumpFromCurrent()
		m.goToCell(ref.row, ref.col)
		return nil
	}

	if strings.EqualFold(name, "write") || strings.EqualFold(name, "w") {
		if arg == "" {
			if err := m.writeCurrentFile(); err != nil {
				m.commandMessage = err.Error()
				m.commandError = true
				return nil
			}
			m.commandError = false
			return nil
		}
		if err := m.writeCSVFile(arg); err != nil {
			m.commandMessage = fmt.Sprintf("write %q: %v", arg, err)
			m.commandError = true
			return nil
		}
		m.currentFilePath = arg
		m.commandMessage = fmt.Sprintf("wrote %s", arg)
		m.commandError = false
		return nil
	}

	if strings.EqualFold(name, "edit") || strings.EqualFold(name, "e") {
		if arg == "" {
			if m.currentFilePath == "" {
				m.commandMessage = "edit requires a path"
				m.commandError = true
				return nil
			}
			arg = m.currentFilePath
		}
		if err := m.loadCSVFile(arg); err != nil {
			m.commandMessage = fmt.Sprintf("edit %q: %v", arg, err)
			m.commandError = true
			return nil
		}
		m.commandMessage = fmt.Sprintf("loaded %s", arg)
		m.commandError = false
		return nil
	}

	if strings.EqualFold(name, "resize") {
		switch {
		case strings.EqualFold(arg, "auto"), strings.EqualFold(arg, "content"):
			m.pushUndoState()
			m.fitColumnToContent(m.selectedCol)
			m.commandMessage = fmt.Sprintf("resized column %s to content", columnLabel(m.selectedCol))
			m.commandError = false
			m.ensureVisible()
			return nil
		case strings.EqualFold(arg, "width"):
			m.pushUndoState()
			m.fitVisibleColumnsToScreen()
			m.commandMessage = "resized visible columns to screen width"
			m.commandError = false
			m.ensureVisible()
			return nil
		case arg == "":
			m.commandMessage = "resize requires a width or mode"
			m.commandError = true
			return nil
		default:
			width, err := strconv.Atoi(arg)
			if err != nil || width < minCellWidth {
				m.commandMessage = fmt.Sprintf("invalid width: '%s'", arg)
				m.commandError = true
				return nil
			}
			m.pushUndoState()
			m.setColumnWidth(m.selectedCol, width)
			m.commandMessage = fmt.Sprintf("resized column %s to %d", columnLabel(m.selectedCol), width)
			m.commandError = false
			m.ensureVisible()
			return nil
		}
	}

	m.commandMessage = fmt.Sprintf("no such command: '%s'", command)
	m.commandError = true
	return nil
}

func (m *model) fitColumnToContent(col int) {
	col = clamp(col, 0, totalCols-1)
	width := runewidth.StringWidth(columnLabel(col))
	for row := 0; row < m.rowCount; row++ {
		width = max(width, runewidth.StringWidth(m.displayValue(row, col)))
	}
	m.setColumnWidth(col, width)
}

func (m *model) fitVisibleColumnsToScreen() {
	visibleCols := m.visibleCols()
	if visibleCols <= 0 {
		return
	}

	usable := m.availableColumnWidth() - visibleCols
	if usable < visibleCols*minCellWidth {
		usable = visibleCols * minCellWidth
	}
	baseWidth := max(minCellWidth, usable/visibleCols)
	extra := max(0, usable-baseWidth*visibleCols)
	for i := 0; i < visibleCols; i++ {
		width := baseWidth
		if i < extra {
			width++
		}
		m.setColumnWidth(m.colOffset+i, width)
	}
}

func (m *model) setColumnWidth(col, width int) {
	col = clamp(col, 0, totalCols-1)
	width = max(minCellWidth, width)
	if width == m.cellWidth {
		delete(m.colWidths, col)
		return
	}
	m.colWidths[col] = width
}

func splitCommandArgument(input string) (name, arg string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", ""
	}

	index := strings.IndexFunc(input, unicode.IsSpace)
	if index == -1 {
		return input, ""
	}

	return input[:index], strings.TrimSpace(input[index:])
}

func (m *model) writeCurrentFile() error {
	if m.currentFilePath == "" {
		return errors.New("write requires a path")
	}
	if err := m.writeCSVFile(m.currentFilePath); err != nil {
		return fmt.Errorf("write %q: %w", m.currentFilePath, err)
	}
	m.commandMessage = fmt.Sprintf("wrote %s", m.currentFilePath)
	m.commandError = false
	return nil
}
