package sheets

import tea "github.com/charmbracelet/bubbletea"

func (m model) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && isCountDigit(msg.Runes[0], m.countBuffer != "") {
		m.countBuffer += string(msg.Runes[0])
		return m, nil
	}

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
		m.toggleSelectionFormatting('*')
		m.clearCount()
		m.clearRegisterState()
	case tea.KeyCtrlU:
		m.moveHalfPage(-count)
		m.clearCount()
		m.clearRegisterState()
	case tea.KeyRunes:
		switch string(msg.Runes) {
		case "g":
			m.startGotoCellCommand()
		case "G":
			m.recordJumpFromCurrent()
			m.goToBottom()
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
		case "V":
			m.selectRows = true
		case "U":
			m.undoLastOperation()
		case "y":
			m.copySelection()
			m.clearCount()
			m.clearRegisterState()
			return m.exitSelectMode(), nil
		case "Y":
			m.copySelectionReference()
			m.clearCount()
			m.clearRegisterState()
			return m.exitSelectMode(), nil
		case "=":
			m.clearCount()
			m.clearRegisterState()
			return m.startFormulaFromSelection()
		case "x":
			m.cutSelection()
			m.clearCount()
			m.clearRegisterState()
			return m.exitSelectMode(), nil
		case "/":
			m.clearNormalPrefixes()
			return m, m.startSearchPrompt(1)
		case "?":
			m.clearNormalPrefixes()
			return m, m.startSearchPrompt(-1)
		case "n":
			m.repeatSearch(count, false)
			m.clearCount()
			m.clearRegisterState()
		case "N":
			m.repeatSearch(count, true)
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
		case "\"":
			m.registerPending = true
			return m, nil
		}
	}

	return m, nil
}

func (m *model) enterSelectMode() {
	m.mode = selectMode
	m.selectRow = m.selectedRow
	m.selectCol = m.selectedCol
	m.selectRows = false
}

func (m *model) enterRowSelectMode() {
	m.enterSelectMode()
	m.selectRows = true
}

func (m model) exitSelectMode() model {
	m.mode = normalMode
	m.selectRow = m.selectedRow
	m.selectCol = m.selectedCol
	m.selectRows = false
	return m
}

func (m model) selectionRef() string {
	top, bottom, left, right := m.selectionBounds()
	start := cellRef(top, left)
	end := cellRef(bottom, right)
	if start == end {
		return start
	}
	return start + ":" + end
}

func (m model) selectionInsertTarget() cellKey {
	_, bottom, left, _ := m.selectionBounds()
	return cellKey{
		row: clamp(bottom+1, 0, m.rowCount-1),
		col: left,
	}
}

func (m model) selectionBounds() (top, bottom, left, right int) {
	top = min(m.selectRow, m.selectedRow)
	bottom = max(m.selectRow, m.selectedRow)
	if m.selectRows {
		left = 0
		right = totalCols - 1
		return top, bottom, left, right
	}

	left = min(m.selectCol, m.selectedCol)
	right = max(m.selectCol, m.selectedCol)
	return top, bottom, left, right
}

func (m model) selectionContains(row, col int) bool {
	if m.mode != selectMode {
		return false
	}

	top, bottom, left, right := m.selectionBounds()
	return row >= top && row <= bottom && col >= left && col <= right
}

func normalizeCellRange(start, end cellKey) (top, bottom, left, right int) {
	top = min(start.row, end.row)
	bottom = max(start.row, end.row)
	left = min(start.col, end.col)
	right = max(start.col, end.col)
	return top, bottom, left, right
}
