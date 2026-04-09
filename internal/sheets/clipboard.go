package sheets

import tea "github.com/charmbracelet/bubbletea"

func cloneKeySequence(keys []tea.KeyMsg) []tea.KeyMsg {
	return append([]tea.KeyMsg(nil), keys...)
}

func cloneClipboard(clip clipboard) clipboard {
	cloned := clipboard{
		cells:     make([][]string, len(clip.cells)),
		sourceRow: clip.sourceRow,
		sourceCol: clip.sourceCol,
	}
	for i, row := range clip.cells {
		cloned.cells[i] = append([]string(nil), row...)
	}
	return cloned
}

func (m *model) saveLastChange(keys []tea.KeyMsg) {
	if m.replayingChange {
		return
	}
	m.lastChange = cloneKeySequence(keys)
}

func (m *model) repeatLastChange(count int) {
	if len(m.lastChange) == 0 {
		return
	}
	if count <= 0 {
		count = 1
	}
	m.replayingChange = true
	defer func() {
		m.replayingChange = false
	}()
	for repeat := 0; repeat < count; repeat++ {
		for _, key := range m.lastChange {
			updated, _ := m.Update(key)
			next, ok := updated.(model)
			if !ok {
				return
			}
			*m = next
		}
	}
}

func (m *model) setUnnamedClipboard(clip clipboard) {
	m.copyBuffer = cloneClipboard(clip)
	m.hasCopyBuffer = true
}

func (m *model) setRegisterClipboard(name rune, clip clipboard) {
	m.registers[name] = cloneClipboard(clip)
}

func (m *model) shiftDeleteRegisters(clip clipboard) {
	for register := '9'; register > '1'; register-- {
		previous, ok := m.registers[register-1]
		if ok {
			m.registers[register] = cloneClipboard(previous)
			continue
		}
		delete(m.registers, register)
	}
	m.registers['1'] = cloneClipboard(clip)
}

func (m *model) storeYankClipboard(clip clipboard) {
	if m.activeRegister == '_' {
		return
	}
	if m.activeRegister != 0 {
		m.setRegisterClipboard(m.activeRegister, clip)
	}
	m.setUnnamedClipboard(clip)
}

func (m *model) storeDeleteClipboard(clip clipboard) {
	if m.activeRegister == '_' {
		return
	}
	if m.activeRegister != 0 {
		m.setRegisterClipboard(m.activeRegister, clip)
	}
	m.shiftDeleteRegisters(clip)
	m.setUnnamedClipboard(clip)
}

func (m model) clipboardForPaste() (clipboard, bool) {
	if m.activeRegister != 0 {
		clip, ok := m.registers[m.activeRegister]
		if !ok {
			return clipboard{}, false
		}
		return cloneClipboard(clip), true
	}
	if !m.hasCopyBuffer {
		return clipboard{}, false
	}
	return cloneClipboard(m.copyBuffer), true
}

func (m model) selectionClipboard() clipboard {
	top, bottom, left, right := m.selectionBounds()
	rows := make([][]string, 0, bottom-top+1)
	for row := top; row <= bottom; row++ {
		values := make([]string, 0, right-left+1)
		for col := left; col <= right; col++ {
			values = append(values, m.cellValue(row, col))
		}
		rows = append(rows, values)
	}
	return clipboard{
		cells:     rows,
		sourceRow: top,
		sourceCol: left,
	}
}

func (m model) rowClipboard(startRow, count int) clipboard {
	if count <= 0 {
		count = 1
	}
	endRow := min(m.rowCount-1, startRow+count-1)
	width := 1
	for row := startRow; row <= endRow; row++ {
		width = max(width, m.rowWidth(row))
	}
	rows := make([][]string, 0, endRow-startRow+1)
	for row := startRow; row <= endRow; row++ {
		values := make([]string, width)
		for col := 0; col < width; col++ {
			values[col] = m.cellValue(row, col)
		}
		rows = append(rows, values)
	}
	return clipboard{
		cells:     rows,
		sourceRow: startRow,
		sourceCol: 0,
	}
}

func (m model) rowWidth(row int) int {
	width := 0
	for key := range m.cells {
		if key.row == row {
			width = max(width, key.col+1)
		}
	}
	if width == 0 {
		return 1
	}
	return width
}

func (m *model) yankRows(count int) {
	m.storeYankClipboard(m.rowClipboard(m.selectedRow, count))
}

func (m *model) deleteRows(startRow, count int) {
	if count <= 0 {
		return
	}
	if m.rowCount <= 1 {
		return
	}
	count = min(count, m.rowCount-1)
	clip := m.rowClipboard(startRow, count)
	m.pushUndoState()
	m.storeDeleteClipboard(clip)

	shifted := make(map[cellKey]string, len(m.cells))
	for key, value := range m.cells {
		switch {
		case key.row < startRow:
			shifted[key] = value
		case key.row >= startRow+count:
			shifted[cellKey{row: key.row - count, col: key.col}] = rewriteFormulaReferences(value, -count, 0)
		}
	}
	m.cells = shifted
	m.rowCount -= count
	m.syncRowLabelWidth()
	m.selectedRow = clamp(m.selectedRow, 0, m.rowCount-1)
	m.selectRow = clamp(m.selectRow, 0, m.rowCount-1)
}

func (m *model) copyCurrentCell(count int) {
	if count <= 0 {
		count = 1
	}
	values := make([]string, 0, count)
	for col := 0; col < count && m.selectedCol+col < totalCols; col++ {
		values = append(values, m.cellValue(m.selectedRow, m.selectedCol+col))
	}
	m.storeYankClipboard(clipboard{
		cells:     [][]string{values},
		sourceRow: m.selectedRow,
		sourceCol: m.selectedCol,
	})
}

func (m *model) cutCurrentCell(count int) bool {
	if count <= 0 {
		count = 1
	}
	clip := clipboard{
		cells:     [][]string{make([]string, 0, count)},
		sourceRow: m.selectedRow,
		sourceCol: m.selectedCol,
	}
	for col := 0; col < count && m.selectedCol+col < totalCols; col++ {
		clip.cells[0] = append(clip.cells[0], m.cellValue(m.selectedRow, m.selectedCol+col))
	}
	if len(clip.cells[0]) == 0 {
		return false
	}
	m.pushUndoState()
	m.storeDeleteClipboard(clip)
	for col := 0; col < len(clip.cells[0]); col++ {
		m.setCellValue(m.selectedRow, m.selectedCol+col, "")
	}
	return true
}

func (m *model) copySelection() {
	m.storeYankClipboard(m.selectionClipboard())
}

func (m *model) copySelectionReference() {
	m.storeYankClipboard(clipboard{
		cells:     [][]string{{m.selectionRef()}},
		sourceRow: m.selectedRow,
		sourceCol: m.selectedCol,
	})
}

func (m *model) cutSelection() {
	top, bottom, left, right := m.selectionBounds()
	m.pushUndoState()
	m.storeDeleteClipboard(m.selectionClipboard())
	for row := top; row <= bottom; row++ {
		for col := left; col <= right; col++ {
			m.setCellValue(row, col, "")
		}
	}
}

func (m *model) pasteIntoCurrentCell(count int) bool {
	clip, ok := m.clipboardForPaste()
	if !ok {
		return false
	}

	m.pushUndoState()
	if count <= 0 {
		count = 1
	}
	stepRows := max(1, len(clip.cells))
	for repeat := 0; repeat < count; repeat++ {
		targetBaseRow := m.selectedRow + repeat*stepRows
		deltaRow := targetBaseRow - clip.sourceRow
		deltaCol := m.selectedCol - clip.sourceCol
		for rowOffset, row := range clip.cells {
			for colOffset, value := range row {
				targetRow := targetBaseRow + rowOffset
				targetCol := m.selectedCol + colOffset
				if targetRow >= m.rowCount || targetCol >= totalCols {
					continue
				}
				m.setCellValue(targetRow, targetCol, rewriteFormulaReferences(value, deltaRow, deltaCol))
			}
		}
	}
	return true
}

func (m *model) insertColAt(insertCol int) {
	insertCol = clamp(insertCol, 0, totalCols-1)
	if insertCol >= totalCols-1 {
		return
	}
	m.pushUndoState()

	shifted := make(map[cellKey]string, len(m.cells))
	for key, value := range m.cells {
		newValue := rewriteFormulaForColInsert(value, insertCol)
		if key.col < insertCol {
			shifted[key] = newValue
			continue
		}
		newCol := key.col + 1
		if newCol >= totalCols {
			continue
		}
		shifted[cellKey{row: key.row, col: newCol}] = newValue
	}

	m.cells = shifted
}

func (m *model) insertRowAt(insertRow int) {
	if m.rowCount >= maxRows {
		return
	}
	insertRow = clamp(insertRow, 0, m.rowCount)
	m.pushUndoState()

	shifted := make(map[cellKey]string, len(m.cells))
	for key, value := range m.cells {
		if key.row < insertRow {
			shifted[key] = value
			continue
		}
		shifted[cellKey{row: key.row + 1, col: key.col}] = rewriteFormulaReferences(value, 1, 0)
	}

	m.cells = shifted
	m.rowCount++
	m.syncRowLabelWidth()
}

func (m *model) deleteRowAt(deleteRow int) {
	if m.rowCount <= 1 {
		return
	}
	deleteRow = clamp(deleteRow, 0, m.rowCount-1)
	m.pushUndoState()

	shifted := make(map[cellKey]string, len(m.cells))
	for key, value := range m.cells {
		if key.row < deleteRow {
			shifted[key] = value
			continue
		}
		if key.row == deleteRow {
			continue
		}

		shifted[cellKey{row: key.row - 1, col: key.col}] = rewriteFormulaReferences(value, -1, 0)
	}

	m.cells = shifted
	m.rowCount--
	m.syncRowLabelWidth()
	m.selectedRow = clamp(m.selectedRow, 0, m.rowCount-1)
}
