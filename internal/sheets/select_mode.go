package sheets

import tea "github.com/charmbracelet/bubbletea"

func defaultSelectKeys() Keymap {
	return Keymap{
		"h": ActionMoveLeft, "left": ActionMoveLeft,
		"j": ActionMoveDown, "down": ActionMoveDown,
		"k": ActionMoveUp, "up": ActionMoveUp,
		"l": ActionMoveRight, "right": ActionMoveRight,
		"0":    ActionMoveToFirstCol,
		"^":    ActionMoveToFirstNonempty,
		"$":    ActionMoveToLastNonempty,
		"H":    ActionMoveToWindowTop,
		"M":    ActionMoveToWindowMiddle,
		"L":    ActionMoveToWindowBottom,
		"C-d":  ActionScrollHalfDown,
		"C-u":  ActionScrollHalfUp,
		"g":    ActionGotoPending,
		"G":    ActionGotoBottom,
		"C-o":  ActionJumpBackward,
		"tab":   ActionJumpForward, // Ctrl+I == Tab in terminals
		"y":    ActionCopySelection,
		"Y":    ActionCopySelectionRef,
		"x":    ActionCutSelection,
		"D":    ActionDeleteSelection,
		"=":    ActionFormulaFromSelection,
		"V":    ActionToggleRowSelect,
		"u":    ActionUndo,
		"U":    ActionRedo,
		"C-r":  ActionRedo,
		"/":    ActionSearchForward,
		"?":    ActionSearchBackward,
		"n":    ActionSearchNext,
		"N":    ActionSearchPrev,
		"m":    ActionMarkSet,
		"'":    ActionMarkJump,
		"`":    ActionMarkJumpExact,
		"\"":   ActionRegisterPending,
		"C-b":  ActionToggleSelectionBold,
		"z":    ActionAlignPending,
	}
}

func (m model) updateSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && isCountDigit(msg.Runes[0], m.countBuffer != "") {
		m.countBuffer += string(msg.Runes[0])
		return m, nil
	}

	action, ok := m.keymap.Select[keyToString(msg)]
	if !ok {
		return m, nil
	}

	count := m.currentCount()

	switch action {
	case ActionMoveLeft:
		m.moveSelection(0, -count)
		m.clearCount()
		m.clearRegisterState()
	case ActionMoveDown:
		m.moveSelection(count, 0)
		m.clearCount()
		m.clearRegisterState()
	case ActionMoveUp:
		m.moveSelection(-count, 0)
		m.clearCount()
		m.clearRegisterState()
	case ActionMoveRight:
		m.moveSelection(0, count)
		m.clearCount()
		m.clearRegisterState()
	case ActionMoveToFirstCol:
		m.moveToColumn(0)
		m.clearCount()
		m.clearRegisterState()
	case ActionMoveToFirstNonempty:
		m.moveToColumn(m.firstNonBlankColumn(m.selectedRow))
		m.clearCount()
		m.clearRegisterState()
	case ActionMoveToLastNonempty:
		m.moveToColumn(m.lastNonBlankColumn(m.selectedRow))
		m.clearCount()
		m.clearRegisterState()
	case ActionMoveToWindowTop:
		m.moveToWindowTop(count)
		m.clearCount()
		m.clearRegisterState()
	case ActionMoveToWindowMiddle:
		m.moveToWindowMiddle()
		m.clearCount()
		m.clearRegisterState()
	case ActionMoveToWindowBottom:
		m.moveToWindowBottom(count)
		m.clearCount()
		m.clearRegisterState()
	case ActionScrollHalfDown:
		m.moveHalfPage(count)
		m.clearCount()
		m.clearRegisterState()
	case ActionScrollHalfUp:
		m.moveHalfPage(-count)
		m.clearCount()
		m.clearRegisterState()
	case ActionGotoPending:
		m.startGotoCellCommand()
	case ActionGotoBottom:
		m.recordJumpFromCurrent()
		m.goToBottom()
		m.clearCount()
		m.clearRegisterState()
	case ActionJumpBackward:
		m.navigateJumpList(-1, count)
		m.clearCount()
		m.clearRegisterState()
	case ActionJumpForward:
		m.navigateJumpList(1, count)
		m.clearCount()
		m.clearRegisterState()
	case ActionCopySelection:
		m.copySelection()
		m.clearCount()
		m.clearRegisterState()
		return m.exitSelectMode(), nil
	case ActionCopySelectionRef:
		m.copySelectionReference()
		m.clearCount()
		m.clearRegisterState()
		return m.exitSelectMode(), nil
	case ActionCutSelection:
		m.cutSelection()
		m.clearCount()
		m.clearRegisterState()
		return m.exitSelectMode(), nil
	case ActionDeleteSelection:
		m.deleteSelection()
		m.clearCount()
		m.clearRegisterState()
		return m.exitSelectMode(), nil
	case ActionFormulaFromSelection:
		m.clearCount()
		m.clearRegisterState()
		return m.startFormulaFromSelection()
	case ActionToggleRowSelect:
		m.selectRows = true
	case ActionUndo:
		m.undoLastOperation()
	case ActionRedo:
		m.redoLastOperation()
	case ActionSearchForward:
		m.clearNormalPrefixes()
		return m, m.startSearchPrompt(1)
	case ActionSearchBackward:
		m.clearNormalPrefixes()
		return m, m.startSearchPrompt(-1)
	case ActionSearchNext:
		m.repeatSearch(count, false)
		m.clearCount()
		m.clearRegisterState()
	case ActionSearchPrev:
		m.repeatSearch(count, true)
		m.clearCount()
		m.clearRegisterState()
	case ActionMarkSet:
		m.markPending = true
	case ActionMarkJump:
		m.markJumpPending = true
		m.markJumpExact = false
	case ActionMarkJumpExact:
		m.markJumpPending = true
		m.markJumpExact = true
	case ActionRegisterPending:
		m.registerPending = true
		return m, nil
	case ActionToggleSelectionBold:
		m.toggleSelectionFormatting('*')
		m.clearCount()
		m.clearRegisterState()
	case ActionAlignPending:
		m.zPending = true
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
