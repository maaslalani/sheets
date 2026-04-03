package sheets

import (
	"github.com/charmbracelet/lipgloss"
	"strconv"
	"strings"
)

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading spreadsheet..."
	}

	commandLine := m.renderCommandLine()
	bottomBar := m.renderStatusBar()
	if m.mode == commandMode && m.commandPending {
		bottomBar = m.renderCommandPromptLine(m.width)
	}
	commandLineHeight := 0
	if commandLine != "" {
		commandLineHeight = lipgloss.Height(commandLine)
	}
	columnHeaders := m.renderColumnHeaders()
	grid := m.renderGrid()
	spacer := m.renderStatusSpacer(
		lipgloss.Height(columnHeaders) +
			lipgloss.Height(grid) +
			commandLineHeight +
			lipgloss.Height(bottomBar),
	)

	parts := []string{columnHeaders, grid}
	if spacer != "" {
		parts = append(parts, spacer)
	}
	if commandLine != "" {
		parts = append(parts, commandLine)
	}
	parts = append(parts, bottomBar)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) renderStatusSpacer(contentHeight int) string {
	spacerHeight := max(0, m.height-contentHeight)
	blankLine := strings.Repeat(" ", m.width)
	lines := make([]string, spacerHeight)
	for i := range lines {
		lines[i] = blankLine
	}

	return strings.Join(lines, "\n")
}

func (m model) renderStatusBar() string {
	modeBox := m.renderStatusMode()
	position := m.renderStatusPosition()
	titleWidth := max(0, m.width-lipgloss.Width(modeBox)-lipgloss.Width(position))
	title := fit(" "+m.statusTitle(), titleWidth)
	return modeBox + m.statusTextStyle.Render(title) + position
}

func (m model) renderStatusMode() string {
	modeLabel := m.statusModeLabel()
	label := fit(modeLabel, max(6, len(modeLabel)))
	if m.mode == commandMode {
		return m.statusTextStyle.Render(label)
	}
	if m.mode == insertMode {
		return m.statusInsertStyle.Render(label)
	}
	if m.mode == selectMode {
		return m.statusSelectStyle.Render(label)
	}

	return m.statusNormalStyle.Render(label)
}

func (m model) statusModeLabel() string {
	if m.mode == commandMode {
		return "COMMAND"
	}
	if m.mode == selectMode {
		return "VISUAL"
	}

	return string(m.mode)
}

func (m model) renderCommandLine() string {
	width := m.width
	if width <= 0 {
		return ""
	}

	if m.commandMessage != "" {
		style := m.commandLineStyle
		if m.commandError {
			style = m.commandErrorStyle
		}
		return style.Render(fit(m.commandMessage, width))
	}

	return ""
}

func (m model) renderCommandPromptLine(width int) string {
	if width <= 0 {
		return ""
	}

	cursorModel := m.editCursor
	cursorModel.Style = m.commandLineStyle
	cursorModel.TextStyle = m.commandLineStyle
	prefix := ":"
	if m.promptKind != noPrompt {
		prefix = string(rune(m.promptKind))
	}
	return renderTextInput(prefix+m.commandBuffer, m.commandCursor+1, width, cursorModel, m.commandLineStyle)
}

func (m model) statusTitle() string {
	if prefix := m.pendingStatusPrefix(); prefix != "" {
		return prefix
	}
	if m.gotoPending {
		return "g" + m.gotoBuffer
	}
	if m.deletePending {
		return "d"
	}

	value := strings.TrimSpace(m.activeValue())
	return value
}

func (m model) renderStatusPosition() string {
	position := " " + m.activeRef() + " "
	return m.statusTextStyle.Render(position)
}

func (m model) renderColumnHeaders() string {
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", m.rowLabelWidth+2))

	for i := 0; i < m.visibleCols(); i++ {
		col := m.colOffset + i
		label := alignCenter(columnLabel(col), m.cellWidth)
		if m.mode == selectMode && m.selectionContains(m.selectedRow, col) {
			b.WriteString(m.activeHeaderStyle.Render(label))
		} else if col == m.selectedCol {
			b.WriteString(m.activeHeaderStyle.Render(label))
		} else {
			b.WriteString(m.headerStyle.Render(label))
		}

		if i < m.visibleCols()-1 {
			b.WriteString(" ")
		}
	}

	return b.String()
}

func (m model) renderGrid() string {
	visibleRows := m.visibleRows()
	visibleCols := m.visibleCols()

	lines := make([]string, 0, 1+visibleRows*2)
	lines = append(lines, m.renderBorderLine(m.rowOffset, "┌", "┬", "┐", visibleCols))

	for i := range visibleRows {
		row := m.rowOffset + i
		lines = append(lines, m.renderContentLine(row, visibleCols))

		left, middle, right := "├", "┼", "┤"
		if i == visibleRows-1 {
			left, middle, right = "└", "┴", "┘"
		}

		lines = append(lines, m.renderBorderLine(row+1, left, middle, right, visibleCols))
	}

	return strings.Join(lines, "\n")
}

func (m model) renderBorderLine(borderRow int, left, middle, right string, visibleCols int) string {
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", m.rowLabelWidth))
	b.WriteString(" ")
	b.WriteString(m.renderBorderJunction(borderRow, m.colOffset, left))

	segment := strings.Repeat("─", m.cellWidth)
	for i := range visibleCols {
		col := m.colOffset + i
		b.WriteString(m.renderBorderSegment(borderRow, col, segment))
		if i == visibleCols-1 {
			b.WriteString(m.renderBorderJunction(borderRow, col+1, right))
			continue
		}

		b.WriteString(m.renderBorderJunction(borderRow, col+1, middle))
	}

	return b.String()
}

func (m model) renderContentLine(row, visibleCols int) string {
	var b strings.Builder
	label := fitLeft(strconv.Itoa(row+1), m.rowLabelWidth)
	if m.mode == selectMode && m.selectionContains(row, m.selectedCol) {
		b.WriteString(m.activeRowStyle.Render(label))
	} else if row == m.selectedRow {
		b.WriteString(m.activeRowStyle.Render(label))
	} else {
		b.WriteString(m.rowLabelStyle.Render(label))
	}

	b.WriteString(" ")
	b.WriteString(m.renderVerticalBorder(row, m.colOffset))

	for i := range visibleCols {
		col := m.colOffset + i
		cell := fit(m.displayValue(row, col), m.cellWidth)
		formula := m.isFormulaDisplayCell(row, col)
		formulaError := formula && m.isFormulaErrorDisplayCell(row, col)
		raw := m.cellValue(row, col)
		_, fmtBold, fmtUnderline, fmtItalic := parseCellFormatting(raw)
		hasFormatting := fmtBold || fmtUnderline || fmtItalic
		if row == m.selectedRow && col == m.selectedCol && m.mode == insertMode {
			b.WriteString(m.renderEditingCell())
		} else {
			style, styled := m.cellBaseStyle(row, col, formula, formulaError)
			if hasFormatting {
				style = applyTextFormatting(style, fmtBold, fmtUnderline, fmtItalic)
				b.WriteString(style.Render(cell))
			} else if styled {
				b.WriteString(style.Render(cell))
			} else {
				b.WriteString(cell)
			}
		}

		b.WriteString(m.renderVerticalBorder(row, col+1))
	}

	return b.String()
}

func (m model) cellBaseStyle(row, col int, formula, formulaError bool) (lipgloss.Style, bool) {
	switch {
	case row == m.selectedRow && col == m.selectedCol && m.mode == selectMode:
		if formulaError {
			return m.selectActiveFormulaErrorStyle, true
		}
		if formula {
			return m.selectActiveFormulaStyle, true
		}
		return m.selectActiveCellStyle, true
	case m.mode == selectMode && m.selectionContains(row, col):
		if formulaError {
			return m.selectFormulaErrorStyle, true
		}
		if formula {
			return m.selectFormulaStyle, true
		}
		return m.selectCellStyle, true
	case row == m.selectedRow && col == m.selectedCol:
		if formulaError {
			return m.activeFormulaErrorStyle, true
		}
		if formula {
			return m.activeFormulaStyle, true
		}
		return m.activeCellStyle, true
	case formulaError:
		return m.formulaErrorStyle, true
	case formula:
		return m.formulaCellStyle, true
	default:
		return lipgloss.NewStyle(), false
	}
}

func applyTextFormatting(style lipgloss.Style, bold, underline, italic bool) lipgloss.Style {
	if bold {
		style = style.Bold(true)
	}
	if underline {
		style = style.Underline(true)
	}
	if italic {
		style = style.Italic(true)
	}
	return style
}

func parseCellFormatting(value string) (stripped string, bold, underline, italic bool) {
	changed := true
	for changed {
		changed = false
		if len(value) >= 2 && value[0] == '*' && value[len(value)-1] == '*' {
			value = value[1 : len(value)-1]
			bold = true
			changed = true
		}
		if len(value) >= 2 && value[0] == '_' && value[len(value)-1] == '_' {
			value = value[1 : len(value)-1]
			underline = true
			changed = true
		}
		if len(value) >= 2 && value[0] == '/' && value[len(value)-1] == '/' {
			value = value[1 : len(value)-1]
			italic = true
			changed = true
		}
	}
	return value, bold, underline, italic
}

func (m *model) toggleCellFormatting(marker byte) {
	raw := m.cellValue(m.selectedRow, m.selectedCol)
	if raw == "" {
		return
	}
	m.pushUndoState()
	s := string(marker)
	if len(raw) >= 2 && raw[0] == marker && raw[len(raw)-1] == marker {
		m.setCellValue(m.selectedRow, m.selectedCol, raw[1:len(raw)-1])
	} else {
		m.setCellValue(m.selectedRow, m.selectedCol, s+raw+s)
	}
}

func (m *model) toggleSelectionFormatting(marker byte) {
	top, bottom, left, right := m.selectionBounds()
	m.pushUndoState()
	s := string(marker)
	for row := top; row <= bottom; row++ {
		for col := left; col <= right; col++ {
			raw := m.cellValue(row, col)
			if raw == "" {
				continue
			}
			if len(raw) >= 2 && raw[0] == marker && raw[len(raw)-1] == marker {
				m.setCellValue(row, col, raw[1:len(raw)-1])
			} else {
				m.setCellValue(row, col, s+raw+s)
			}
		}
	}
}

func (m model) renderVerticalBorder(row, borderCol int) string {
	if m.selectionVerticalBorderHighlighted(row, borderCol) {
		return m.selectBorderStyle.Render("│")
	}

	return m.gridStyle.Render("│")
}

func (m model) renderBorderSegment(borderRow, col int, segment string) string {
	if m.selectionHorizontalBorderHighlighted(borderRow, col) {
		return m.selectBorderStyle.Render(segment)
	}

	return m.gridStyle.Render(segment)
}

func (m model) renderBorderJunction(borderRow, borderCol int, fallback string) string {
	if glyph, ok := m.selectionBorderJunction(borderRow, borderCol); ok {
		return m.selectBorderStyle.Render(glyph)
	}

	return m.gridStyle.Render(fallback)
}

func (m model) selectionBorderJunction(borderRow, borderCol int) (string, bool) {
	left := m.selectionHorizontalBorderHighlighted(borderRow, borderCol-1)
	right := m.selectionHorizontalBorderHighlighted(borderRow, borderCol)
	up := m.selectionVerticalBorderHighlighted(borderRow-1, borderCol)
	down := m.selectionVerticalBorderHighlighted(borderRow, borderCol)

	switch {
	case left && right && up && down:
		return "┼", true
	case left && right && down:
		return "┬", true
	case left && right && up:
		return "┴", true
	case up && down && right:
		return "├", true
	case up && down && left:
		return "┤", true
	case down && right:
		return "┌", true
	case down && left:
		return "┐", true
	case up && right:
		return "└", true
	case up && left:
		return "┘", true
	case left && right:
		return "─", true
	case up && down:
		return "│", true
	case left:
		return "─", true
	case right:
		return "─", true
	case up:
		return "│", true
	case down:
		return "│", true
	default:
		return "", false
	}
}

func (m model) selectionHorizontalBorderHighlighted(borderRow, col int) bool {
	if m.mode != selectMode {
		return false
	}

	return m.selectionContains(borderRow-1, col) || m.selectionContains(borderRow, col)
}

func (m model) selectionVerticalBorderHighlighted(row, borderCol int) bool {
	if m.mode != selectMode {
		return false
	}

	return m.selectionContains(row, borderCol-1) || m.selectionContains(row, borderCol)
}

func (m model) activeRef() string {
	if m.mode == selectMode {
		return m.selectionRef()
	}

	return cellRef(m.selectedRow, m.selectedCol)
}

func (m model) activeValue() string {
	if m.mode == insertMode {
		return m.editingValue
	}

	return m.cellValue(m.selectedRow, m.selectedCol)
}

func (m model) displayValue(row, col int) string {
	if row == m.selectedRow && col == m.selectedCol && m.mode == insertMode {
		return m.editingValue
	}

	raw := m.cellValue(row, col)
	if !isFormulaCell(raw) {
		stripped, _, _, _ := parseCellFormatting(raw)
		return stripped
	}

	value := m.computedCellValue(row, col)
	if shouldPrefixDisplayedFormula(raw) {
		return "=" + value
	}

	return value
}

func alignCenter(value string, width int) string {
	value = truncate(value, width)
	if len(value) >= width {
		return value
	}

	padding := width - len(value)
	left := padding / 2
	right := padding - left
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", right)
}

func fit(value string, width int) string {
	value = truncate(value, width)
	if len(value) >= width {
		return value
	}

	return value + strings.Repeat(" ", width-len(value))
}

func fitLeft(value string, width int) string {
	value = truncate(value, width)
	if len(value) >= width {
		return value
	}

	return strings.Repeat(" ", width-len(value)) + value
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}

	runes := []rune(strings.ReplaceAll(value, "\n", " "))
	if len(runes) <= width {
		return string(runes)
	}
	if width == 1 {
		return string(runes[:1])
	}

	return string(runes[:width-1]) + "…"
}
