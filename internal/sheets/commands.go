package sheets

import (
	"errors"
	"fmt"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
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
		strings.EqualFold(command, "quit"):
		if m.dirtyFile {
			m.commandMessage = "No write since last change (add ! to override)"
			m.commandError = true
			return nil
		}
		return tea.Quit
	case strings.EqualFold(command, "q!"),
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
		m.commandMessage = "Commands: q, w, wq, x, goto <cell>, <cell>, e[dit] <path>, w[rite] [path], sort, sort!"
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
		m.dirtyFile = false
		return nil
	}

	if strings.EqualFold(command, "sort") || command == "sort!" {
		ascending := command != "sort!"
		m.sortByColumn(m.selectedCol, ascending)
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

	m.commandMessage = fmt.Sprintf("no such command: '%s'", command)
	m.commandError = true
	return nil
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
