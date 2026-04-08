package sheets

import "strings"

func (m *model) goToCell(row, col int) {
	m.selectedRow = clamp(row, 0, m.rowCount-1)
	m.selectedCol = clamp(col, 0, totalCols-1)

	m.ensureVisible()
}

func (m *model) goToBottom() {
	lastRow := m.rowCount - 1
	if m.selectedRow == lastRow {
		m.selectedCol = totalCols - 1
	} else {
		m.selectedRow = lastRow
	}

	m.ensureVisible()
}

func (m *model) moveSelection(deltaRow, deltaCol int) {
	m.selectedRow = clamp(m.selectedRow+deltaRow, 0, m.rowCount-1)
	m.selectedCol = clamp(m.selectedCol+deltaCol, 0, totalCols-1)
	m.ensureVisible()
}

func (m *model) moveHalfPage(direction int) {
	step := m.halfPageRows() * direction
	visibleRows := m.maxVisibleRows()
	maxRowOffset := max(0, m.rowCount-visibleRows)

	m.selectedRow = clamp(m.selectedRow+step, 0, m.rowCount-1)
	m.rowOffset = clamp(m.rowOffset+step, 0, maxRowOffset)
	m.ensureVisible()
}

func (m *model) moveToColumn(col int) {
	m.selectedCol = clamp(col, 0, totalCols-1)
	m.ensureVisible()
}

func (m *model) moveToWindowTop(count int) {
	visible := m.visibleRows()
	if visible <= 0 {
		return
	}
	m.selectedRow = clamp(m.rowOffset+max(0, count-1), 0, m.rowCount-1)
	m.ensureVisible()
}

func (m *model) moveToWindowMiddle() {
	visible := m.visibleRows()
	if visible <= 0 {
		return
	}
	m.selectedRow = clamp(m.rowOffset+(visible-1)/2, 0, m.rowCount-1)
	m.ensureVisible()
}

func (m *model) moveToWindowBottom(count int) {
	visible := m.visibleRows()
	if visible <= 0 {
		return
	}
	m.selectedRow = clamp(m.rowOffset+visible-count, 0, m.rowCount-1)
	m.ensureVisible()
}

func (m *model) alignSelectionTop() {
	maxOffset := max(0, m.rowCount-m.maxVisibleRows())
	m.rowOffset = clamp(m.selectedRow, 0, maxOffset)
	m.ensureVisible()
}

func (m *model) alignSelectionMiddle() {
	maxOffset := max(0, m.rowCount-m.maxVisibleRows())
	m.rowOffset = clamp(m.selectedRow-(m.visibleRows()-1)/2, 0, maxOffset)
	m.ensureVisible()
}

func (m *model) alignSelectionBottom() {
	maxOffset := max(0, m.rowCount-m.maxVisibleRows())
	m.rowOffset = clamp(m.selectedRow-m.visibleRows()+1, 0, maxOffset)
	m.ensureVisible()
}

func (m *model) ensureVisible() {
	maxRowOffset := max(0, m.rowCount-m.maxVisibleRows())
	m.rowOffset = clamp(m.rowOffset, 0, maxRowOffset)
	visibleRows := m.visibleRows()
	if m.selectedRow < m.rowOffset {
		m.rowOffset = m.selectedRow
	}
	if m.selectedRow >= m.rowOffset+visibleRows {
		m.rowOffset = m.selectedRow - visibleRows + 1
	}

	if m.selectedCol < m.colOffset {
		m.colOffset = m.selectedCol
	}
	for m.lastVisibleCol() < m.selectedCol && m.colOffset < m.selectedCol {
		m.colOffset++
	}
}

func (m model) visibleRows() int {
	rows := m.maxVisibleRows()
	return min(rows, m.rowCount-m.rowOffset)
}

func (m model) maxVisibleRows() int {
	available := m.height - 3
	if available < 3 {
		return 1
	}

	rows := (available - 1) / 2
	if rows < 1 {
		return 1
	}

	return min(rows, m.rowCount)
}

func (m model) halfPageRows() int {
	return max(1, m.maxVisibleRows()/2)
}

func (m model) visibleCols() int {
	available := m.availableColumnWidth()
	if available <= 0 || m.colOffset >= totalCols {
		return 1
	}

	used := 0
	visible := 0
	for col := m.colOffset; col < totalCols; col++ {
		width := m.columnWidth(col) + 1
		if visible > 0 && used+width > available {
			break
		}
		visible++
		used += width
		if used >= available {
			break
		}
	}
	if visible < 1 {
		return 1
	}

	return visible
}

func (m model) availableColumnWidth() int {
	return max(0, m.width-m.rowLabelWidth-2)
}

func (m model) columnWidth(col int) int {
	if width, ok := m.colWidths[col]; ok {
		return max(minCellWidth, width)
	}
	return max(minCellWidth, m.cellWidth)
}

func (m model) lastVisibleCol() int {
	return min(totalCols-1, m.colOffset+m.visibleCols()-1)
}

func (m model) firstNonBlankColumn(row int) int {
	for col := 0; col < totalCols; col++ {
		if strings.TrimSpace(m.cellValue(row, col)) != "" {
			return col
		}
	}
	return 0
}

func (m model) lastNonBlankColumn(row int) int {
	last := 0
	for col := 0; col < totalCols; col++ {
		if strings.TrimSpace(m.cellValue(row, col)) != "" {
			last = col
		}
	}
	return last
}

func (m *model) recordJumpFromCurrent() {
	m.recordJumpFrom(cellKey{row: m.selectedRow, col: m.selectedCol})
}

func (m *model) recordJumpFrom(current cellKey) {
	if len(m.jumpBack) > 0 && m.jumpBack[len(m.jumpBack)-1] == current {
		return
	}
	m.jumpBack = append(m.jumpBack, current)
	m.jumpForward = nil
}

func (m *model) navigateJumpList(direction, count int) {
	if count <= 0 {
		count = 1
	}
	for step := 0; step < count; step++ {
		current := cellKey{row: m.selectedRow, col: m.selectedCol}
		if direction < 0 {
			if len(m.jumpBack) == 0 {
				return
			}
			target := m.jumpBack[len(m.jumpBack)-1]
			m.jumpBack = m.jumpBack[:len(m.jumpBack)-1]
			m.jumpForward = append(m.jumpForward, current)
			m.goToCell(target.row, target.col)
			continue
		}
		if len(m.jumpForward) == 0 {
			return
		}
		target := m.jumpForward[len(m.jumpForward)-1]
		m.jumpForward = m.jumpForward[:len(m.jumpForward)-1]
		m.jumpBack = append(m.jumpBack, current)
		m.goToCell(target.row, target.col)
	}
}

func (m model) cellFromMouse(x, y int) (row, col int, ok bool) {
	// Column headers occupy line 0, top border is line 1.
	// Content lines are at y = 2, 4, 6, ... (every other line starting at 2).
	if y < 2 || (y-2)%2 != 0 {
		return 0, 0, false
	}
	visibleRowIndex := (y - 2) / 2
	if visibleRowIndex < 0 || visibleRowIndex >= m.visibleRows() {
		return 0, 0, false
	}

	// Cell content area starts after row label + space + left border.
	cellAreaStart := m.rowLabelWidth + 2
	if x < cellAreaStart {
		return 0, 0, false
	}

	remaining := x - cellAreaStart
	for visibleColIndex := 0; visibleColIndex < m.visibleCols(); visibleColIndex++ {
		col = m.colOffset + visibleColIndex
		width := m.columnWidth(col)
		if remaining < width {
			return m.rowOffset + visibleRowIndex, col, true
		}
		if remaining == width {
			return 0, 0, false
		}
		remaining -= width + 1
		if remaining < 0 {
			return 0, 0, false
		}
	}

	return 0, 0, false
}
