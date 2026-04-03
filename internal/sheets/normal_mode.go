package sheets

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"strconv"
	"strings"
	"unicode"
)

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && isCountDigit(msg.Runes[0], m.countBuffer != "") {
		m.countBuffer += string(msg.Runes[0])
		return m, nil
	}

	hasCount := m.countBuffer != ""
	count := m.currentCount()
	switch msg.Type {
	case tea.KeyLeft:
		m.moveSelection(0, -count)
		m.clearCount()
		m.clearRegisterState()
	case tea.KeyDown:
		m.moveSelection(count, 0)
		m.clearCount()
		m.clearRegisterState()
	case tea.KeyUp:
		m.moveSelection(-count, 0)
		m.clearCount()
		m.clearRegisterState()
	case tea.KeyRight:
		m.moveSelection(0, count)
		m.clearCount()
		m.clearRegisterState()
	case tea.KeyCtrlD:
		m.moveHalfPage(count)
		m.clearCount()
		m.clearRegisterState()
	case tea.KeyCtrlI:
		m.navigateJumpList(1, count)
		m.clearCount()
		m.clearRegisterState()
	case tea.KeyCtrlO:
		m.navigateJumpList(-1, count)
		m.clearCount()
		m.clearRegisterState()
	case tea.KeyCtrlR:
		m.redoLastOperation()
	case tea.KeyCtrlB:
		m.toggleCellFormatting('*')
		m.clearCount()
		m.clearRegisterState()
	case tea.KeyCtrlU:
		m.moveHalfPage(-count)
		m.clearCount()
		m.clearRegisterState()
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "q":
			return m, tea.Quit
		case ":":
			m.clearNormalPrefixes()
			return m, m.startCommandPrompt()
		case "/":
			m.clearNormalPrefixes()
			return m, m.startSearchPrompt(1)
		case "?":
			m.clearNormalPrefixes()
			return m, m.startSearchPrompt(-1)
		case "\"":
			m.registerPending = true
			return m, nil
		case "g":
			m.startGotoCellCommand()
		case "G":
			if hasCount {
				m.recordJumpFromCurrent()
				m.goToCell(count-1, m.selectedCol)
			} else {
				m.recordJumpFromCurrent()
				m.goToBottom()
			}
			m.clearCount()
			m.clearRegisterState()
		case "h":
			m.moveSelection(0, -count)
			m.clearCount()
			m.clearRegisterState()
		case "j":
			m.moveSelection(count, 0)
			m.clearCount()
			m.clearRegisterState()
		case "k":
			m.moveSelection(-count, 0)
			m.clearCount()
			m.clearRegisterState()
		case "l":
			m.moveSelection(0, count)
			m.clearCount()
			m.clearRegisterState()
		case "0":
			m.moveToColumn(0)
			m.clearCount()
			m.clearRegisterState()
		case "^":
			m.moveToColumn(m.firstNonBlankColumn(m.selectedRow))
			m.clearCount()
			m.clearRegisterState()
		case "$":
			m.moveToColumn(m.lastNonBlankColumn(m.selectedRow))
			m.clearCount()
			m.clearRegisterState()
		case "H":
			m.moveToWindowTop(count)
			m.clearCount()
			m.clearRegisterState()
		case "M":
			m.moveToWindowMiddle()
			m.clearCount()
			m.clearRegisterState()
		case "L":
			m.moveToWindowBottom(count)
			m.clearCount()
			m.clearRegisterState()
		case "z":
			m.zPending = true
		case "i":
			return m, m.enterInsertModeWithKeys(append(m.commandPrefixKeys(), msg))
		case "I":
			return m, m.enterInsertModeAtStartWithKeys(append(m.commandPrefixKeys(), msg))
		case "c":
			return m.changeCurrentCell(append(m.commandPrefixKeys(), msg))
		case "d":
			m.startDeleteRowCommand()
		case "a":
			return m, m.openColAfterWithKeys(append(m.commandPrefixKeys(), msg))
		case "A":
			return m, m.openColBeforeWithKeys(append(m.commandPrefixKeys(), msg))
		case "o":
			return m, m.openRowBelowWithKeys(append(m.commandPrefixKeys(), msg))
		case "O":
			return m, m.openRowAboveWithKeys(append(m.commandPrefixKeys(), msg))
		case "v":
			m.clearNormalPrefixes()
			m.enterSelectMode()
		case "V":
			m.clearNormalPrefixes()
			m.enterRowSelectMode()
		case "u":
			m.undoLastOperation()
		case "U": // helix-like shift+u redo
			m.redoLastOperation()
		case "y":
			m.copyCurrentCell(count)
			m.yankCount = count
			m.clearCount()
			m.yankPending = true
		case "x":
			if m.cutCurrentCell(count) {
				m.saveLastChange(append(m.commandPrefixKeys(), msg))
			}
			m.clearCount()
			m.clearRegisterState()
		case "p":
			if m.pasteIntoCurrentCell(count) {
				m.saveLastChange(append(m.commandPrefixKeys(), msg))
			}
			m.clearCount()
			m.clearRegisterState()
		case "n":
			m.repeatSearch(count, false)
			m.clearCount()
			m.clearRegisterState()
		case "N":
			m.repeatSearch(count, true)
			m.clearCount()
			m.clearRegisterState()
		case ".":
			m.repeatLastChange(count)
			m.clearCount()
			m.clearRegisterState()
		case "m":
			m.markPending = true
		case "'":
			m.markJumpPending = true
			m.markJumpExact = false
		case "`":
			m.markJumpPending = true
			m.markJumpExact = true
		}
	}

	return m, nil
}

func (m *model) startDeleteRowCommand() {
	m.deletePending = true
}

func (m *model) clearDeleteRowCommand() {
	m.deletePending = false
}

func (m *model) clearYankCommand() {
	m.yankPending = false
	m.yankCount = 0
}

func (m *model) handlePendingDelete(msg tea.KeyMsg) bool {
	if isEscapeKey(msg) {
		m.clearRegisterState()
		m.clearDeleteRowCommand()
		return true
	}

	switch msg.Type {
	case tea.KeyBackspace:
		m.clearRegisterState()
		m.clearDeleteRowCommand()
		return true
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			m.clearRegisterState()
			m.clearDeleteRowCommand()
			return false
		}
		if msg.Runes[0] == 'd' {
			keys := append(m.commandPrefixKeys(), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}, msg)
			count := m.consumeCount()
			m.clearDeleteRowCommand()
			if m.deleteCurrentRow(count) {
				m.saveLastChange(keys)
			}
			m.clearRegisterState()
			return true
		}
	}

	m.clearRegisterState()
	m.clearDeleteRowCommand()
	return false
}

func (m *model) startGotoCellCommand() {
	m.gotoPending = true
	m.gotoBuffer = ""
}

func (m *model) clearGotoCellCommand() {
	m.gotoPending = false
	m.gotoBuffer = ""
}

func (m *model) handlePendingGoto(msg tea.KeyMsg) bool {
	if isEscapeKey(msg) {
		m.clearGotoCellCommand()
		m.clearRegisterState()
		return true
	}

	switch msg.Type {
	case tea.KeyBackspace:
		if m.gotoBuffer == "" {
			m.clearGotoCellCommand()
			m.clearRegisterState()
			return true
		}
		m.gotoBuffer = m.gotoBuffer[:len(m.gotoBuffer)-1]
		if ref, ok := parseCellRef(m.gotoBuffer); ok {
			m.goToCell(ref.row, ref.col)
		}
		return true
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			m.clearGotoCellCommand()
			return false
		}

		ch := byte(msg.Runes[0])
		if ch == 'g' && m.gotoBuffer == "" {
			if m.countBuffer != "" {
				m.recordJumpFromCurrent()
				m.goToCell(m.consumeCount()-1, 0)
			} else {
				m.recordJumpFromCurrent()
				m.goToCell(0, 0)
			}
			m.clearGotoCellCommand()
			m.clearRegisterState()
			return true
		}
		if m.gotoBuffer == "" {
			m.clearCount()
		}
		if !isLetter(ch) && !isDigit(ch) {
			m.clearGotoCellCommand()
			m.clearRegisterState()
			return false
		}

		next := strings.ToUpper(m.gotoBuffer + string(ch))
		if !isCellRefCommandPrefix(next) {
			m.clearGotoCellCommand()
			m.clearRegisterState()
			return false
		}

		m.gotoBuffer = next
		if ref, ok := parseCellRef(m.gotoBuffer); ok {
			m.goToCell(ref.row, ref.col)
		}
		return true
	default:
		m.clearGotoCellCommand()
		m.clearRegisterState()
		return false
	}
}

func (m *model) handlePendingYank(msg tea.KeyMsg) bool {
	if isEscapeKey(msg) {
		m.clearYankCommand()
		m.clearRegisterState()
		return true
	}

	switch msg.Type {
	case tea.KeyBackspace:
		m.clearYankCommand()
		m.clearRegisterState()
		return true
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			m.clearYankCommand()
			m.clearRegisterState()
			return false
		}
		if msg.Runes[0] == 'y' {
			count := m.yankCount
			if count <= 0 {
				count = 1
			}
			m.yankRows(count)
			m.clearYankCommand()
			m.clearRegisterState()
			return true
		}
	}

	m.clearYankCommand()
	m.clearRegisterState()
	return false
}

func (m *model) handlePendingRegister(msg tea.KeyMsg) bool {
	if isEscapeKey(msg) {
		m.registerPending = false
		m.activeRegister = 0
		return true
	}
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		m.registerPending = false
		m.activeRegister = 0
		return false
	}
	if register, ok := normalizeRegister(msg.Runes[0]); ok {
		m.activeRegister = register
	}
	m.registerPending = false
	return true
}

func (m *model) handlePendingZ(msg tea.KeyMsg) bool {
	if isEscapeKey(msg) {
		m.zPending = false
		m.clearRegisterState()
		return true
	}
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		m.zPending = false
		m.clearRegisterState()
		return false
	}
	switch msg.Runes[0] {
	case 't':
		m.alignSelectionTop()
	case 'z':
		m.alignSelectionMiddle()
	case 'b':
		m.alignSelectionBottom()
	default:
		m.zPending = false
		m.clearRegisterState()
		return false
	}
	m.zPending = false
	m.clearRegisterState()
	return true
}

func (m *model) handlePendingMark(msg tea.KeyMsg) bool {
	if isEscapeKey(msg) {
		m.markPending = false
		m.markJumpPending = false
		m.markJumpExact = false
		m.clearRegisterState()
		return true
	}
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		m.markPending = false
		m.markJumpPending = false
		m.markJumpExact = false
		m.clearRegisterState()
		return false
	}
	name, ok := normalizeMark(msg.Runes[0])
	if !ok {
		m.markPending = false
		m.markJumpPending = false
		m.markJumpExact = false
		m.clearRegisterState()
		return false
	}
	if m.markPending {
		m.marks[name] = cellKey{row: m.selectedRow, col: m.selectedCol}
		m.markPending = false
		m.clearRegisterState()
		return true
	}
	if ref, exists := m.marks[name]; exists {
		m.recordJumpFromCurrent()
		m.goToCell(ref.row, ref.col)
	} else {
		m.commandMessage = fmt.Sprintf("mark not set: %c", name)
		m.commandError = true
	}
	m.markJumpPending = false
	m.markJumpExact = false
	m.clearRegisterState()
	return true
}

func (m *model) deleteCurrentRow(count int) bool {
	if m.rowCount <= 1 {
		return false
	}
	if count <= 0 {
		count = 1
	}
	if count > m.rowCount-m.selectedRow {
		count = m.rowCount - m.selectedRow
	}
	if count <= 0 {
		return false
	}
	m.deleteRows(m.selectedRow, count)
	m.ensureVisible()
	return true
}

func isCountDigit(r rune, hasPrefix bool) bool {
	if r >= '1' && r <= '9' {
		return true
	}
	return hasPrefix && r == '0'
}

func (m model) commandPrefixKeys() []tea.KeyMsg {
	keys := make([]tea.KeyMsg, 0, len(m.countBuffer)+2)
	for _, r := range m.countBuffer {
		keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if m.activeRegister != 0 {
		keys = append(keys,
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'"'}},
			tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{m.activeRegister}},
		)
	}
	return keys
}

func (m *model) consumeCount() int {
	if m.countBuffer == "" {
		return 1
	}
	count, err := strconv.Atoi(m.countBuffer)
	m.countBuffer = ""
	if err != nil || count < 1 {
		return 1
	}
	return count
}

func (m model) currentCount() int {
	if m.countBuffer == "" {
		return 1
	}
	count, err := strconv.Atoi(m.countBuffer)
	if err != nil || count < 1 {
		return 1
	}
	return count
}

func (m *model) clearCount() {
	m.countBuffer = ""
}

func (m *model) clearRegisterState() {
	m.activeRegister = 0
	m.registerPending = false
}

func (m *model) clearNormalPrefixes() {
	m.clearCount()
	m.clearRegisterState()
	m.deletePending = false
	m.yankPending = false
	m.yankCount = 0
	m.zPending = false
	m.gotoPending = false
	m.gotoBuffer = ""
	m.markPending = false
	m.markJumpPending = false
	m.markJumpExact = false
}

func (m model) pendingStatusPrefix() string {
	var b strings.Builder
	if m.countBuffer != "" {
		b.WriteString(m.countBuffer)
	}
	if m.registerPending {
		b.WriteByte('"')
		return b.String()
	}
	if m.activeRegister != 0 {
		b.WriteByte('"')
		b.WriteRune(m.activeRegister)
	}
	switch {
	case m.gotoPending:
		b.WriteString("g")
		b.WriteString(m.gotoBuffer)
	case m.deletePending:
		b.WriteString("d")
	case m.yankPending:
		b.WriteString("y")
	case m.zPending:
		b.WriteString("z")
	case m.markPending:
		b.WriteString("m")
	case m.markJumpPending && m.markJumpExact:
		b.WriteString("`")
	case m.markJumpPending:
		b.WriteString("'")
	}
	return b.String()
}

func normalizeRegister(r rune) (rune, bool) {
	switch {
	case r == '_':
		return r, true
	case r >= '1' && r <= '9':
		return r, true
	case unicode.IsLetter(r):
		return unicode.ToLower(r), true
	default:
		return 0, false
	}
}

func normalizeMark(r rune) (rune, bool) {
	if !unicode.IsLetter(r) {
		return 0, false
	}
	return unicode.ToLower(r), true
}
