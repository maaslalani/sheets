package sheets

import (
	"fmt"
	"net/url"
	"regexp"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"strconv"
	"strings"
)

var urlEncodedPattern = regexp.MustCompile(`%[0-9A-Fa-f]{2}`)

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
	view := lipgloss.JoinVertical(lipgloss.Left, parts...)
	if m.viewerOpen {
		return m.renderViewer(view)
	}
	return view
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
	label := fit(modeLabel, max(6, runewidth.StringWidth(modeLabel)))
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
	if m.mode == commandMode {
		return ""
	}
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

	visibleCols := m.visibleColumns()
	for i, column := range visibleCols {
		label := alignCenter(columnLabel(column.col), column.width)
		if m.mode == selectMode && m.selectionContains(m.selectedRow, column.col) {
			b.WriteString(m.activeHeaderStyle.Render(label))
		} else if column.col == m.selectedCol {
			b.WriteString(m.activeHeaderStyle.Render(label))
		} else {
			b.WriteString(m.headerStyle.Render(label))
		}

		if i < len(visibleCols)-1 {
			b.WriteString(" ")
		}
	}

	return b.String()
}

func (m model) renderGrid() string {
	visibleRows := m.visibleRows()
	visibleCols := m.visibleColumns()

	lines := make([]string, 0, 1+visibleRows*2)
	lines = append(lines, m.renderBorderLine(m.rowOffset, "┌", "┬", "┐", visibleCols))

	for i := range visibleRows {
		row := m.rowOffset + i
		lines = append(lines, m.renderContentLineVisible(row, visibleCols))

		left, middle, right := "├", "┼", "┤"
		if i == visibleRows-1 {
			left, middle, right = "└", "┴", "┘"
		}

		lines = append(lines, m.renderBorderLine(row+1, left, middle, right, visibleCols))
	}

	return strings.Join(lines, "\n")
}

func (m model) renderBorderLine(borderRow int, left, middle, right string, visibleCols []visibleColumn) string {
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", m.rowLabelWidth))
	b.WriteString(" ")
	b.WriteString(m.renderBorderJunction(borderRow, m.colOffset, left))

	for i, column := range visibleCols {
		segment := strings.Repeat("─", column.width)
		b.WriteString(m.renderBorderSegment(borderRow, column.col, segment))
		if i == len(visibleCols)-1 {
			b.WriteString(m.renderBorderJunction(borderRow, column.col+1, right))
			continue
		}

		b.WriteString(m.renderBorderJunction(borderRow, column.col+1, middle))
	}

	return b.String()
}

func (m model) renderContentLine(row, visibleCols int) string {
	return m.renderContentLineVisible(row, m.renderColumns(visibleCols))
}

func (m model) renderContentLineVisible(row int, visibleCols []visibleColumn) string {
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

	for _, column := range visibleCols {
		cell := fit(m.displayValue(row, column.col), column.width)
		formula := m.isFormulaDisplayCell(row, column.col)
		formulaError := formula && m.isFormulaErrorDisplayCell(row, column.col)
		raw := m.cellValue(row, column.col)
		_, fmtBold, fmtUnderline, fmtItalic := parseCellFormatting(raw)
		hasFormatting := fmtBold || fmtUnderline || fmtItalic
		if row == m.selectedRow && column.col == m.selectedCol && m.mode == insertMode {
			b.WriteString(m.renderEditingCell())
		} else {
			style, styled := m.cellBaseStyle(row, column.col, formula, formulaError)
			if hasFormatting {
				style = applyTextFormatting(style, fmtBold, fmtUnderline, fmtItalic)
				b.WriteString(style.Render(cell))
			} else if styled {
				b.WriteString(style.Render(cell))
			} else {
				b.WriteString(cell)
			}
		}

		b.WriteString(m.renderVerticalBorder(row, column.col+1))
	}

	return b.String()
}

func (m model) renderViewer(base string) string {
	modalWidth, modalHeight := m.viewerDimensions()
	innerWidth := max(1, modalWidth-4)
	stackContext := m.viewerStacksContext(modalWidth)
	contextWidth := m.viewerContextWidth(innerWidth)
	contentWidth := max(1, m.viewerContentWidth(modalWidth))
	title := m.viewerTitleStyle.Render(m.viewerTitle)
	help := m.commandLineStyle.Render("Esc/q/Enter close  h/j/k/l move  PgUp/PgDn scroll")
	if m.mode == insertMode {
		help = m.commandLineStyle.Render("Esc preview  type to edit")
	}
	header := title
	if help != "" {
		if lipgloss.Width(title)+lipgloss.Width(help)+1 <= modalWidth-4 {
			header = lipgloss.JoinHorizontal(lipgloss.Top, title, strings.Repeat(" ", max(0, modalWidth-lipgloss.Width(title)-lipgloss.Width(help)-4)), help)
		} else {
			header = lipgloss.JoinVertical(lipgloss.Left, title, help)
		}
	}
	contentView := m.viewer.View()
	if m.mode == insertMode {
		contentView = renderViewerTextInput(m.editingValue, m.editingCursor, m.viewer.Width, m.editCursor, lipgloss.NewStyle())
	}
	contextView := m.renderViewerContext(contextWidth)
	body := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(contentWidth).Render(contentView), lipgloss.NewStyle().Width(contextWidth).Render(contextView))
	if stackContext {
		body = lipgloss.JoinVertical(lipgloss.Left, lipgloss.NewStyle().Width(contentWidth).Render(contentView), lipgloss.NewStyle().Width(contextWidth).Render(contextView))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, header, body)
	modal := m.viewerStyle.Width(modalWidth).Height(modalHeight).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

func (m model) viewerDimensions() (width, height int) {
	width = clamp(m.width*3/4, 40, max(40, m.width-4))
	height = clamp(m.height*3/4, 8, max(8, m.height-4))
	if m.width < 44 {
		width = max(10, m.width-2)
	}
	if m.height < 12 {
		height = max(4, m.height-2)
	}
	return width, height
}

func (m model) viewerContentWidth(modalWidth int) int {
	innerWidth := max(1, modalWidth-4)
	if m.viewerStacksContext(modalWidth) {
		return innerWidth
	}
	return max(1, innerWidth-m.viewerContextWidth(innerWidth)-1)
}

func (m model) viewerContextWidth(innerWidth int) int {
	if innerWidth < 24 {
		return innerWidth
	}
	if innerWidth < 48 {
		return min(18, max(12, innerWidth/3))
	}
	return min(26, max(18, innerWidth/4))
}

func (m model) viewerStacksContext(modalWidth int) bool {
	return modalWidth < 56
}

func (m model) renderViewerContext(width int) string {
	if width <= 0 {
		return ""
	}
	parts := []string{
		m.renderViewerPositionSummary(width),
		m.renderViewerNeighborGrid(width),
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) renderViewerPositionSummary(width int) string {
	rowSummary := fit(fmt.Sprintf("Row %d/%d", m.selectedRow+1, m.rowCount), width)
	colSummary := fit(fmt.Sprintf("Col %s/%s", columnLabel(m.selectedCol), columnLabel(totalCols-1)), width)
	windowSummary := fit(fmt.Sprintf("View %s-%s", columnLabel(m.colOffset), columnLabel(m.visibleColumns()[len(m.visibleColumns())-1].col)), width)
	return lipgloss.JoinVertical(lipgloss.Left, rowSummary, colSummary, windowSummary, fit("Nearby", width))
}

func (m model) renderViewerNeighborGrid(width int) string {
	cols := viewerContextWindow(m.selectedCol, totalCols)
	rows := viewerContextWindow(m.selectedRow, m.rowCount)
	cellWidth := max(4, (width-4)/3)
	var lines []string
	header := strings.Repeat(" ", 3)
	for _, col := range cols {
		header += " " + fit(columnLabel(col), cellWidth)
	}
	lines = append(lines, header)
	for _, row := range rows {
		line := fitLeft(strconv.Itoa(row+1), 3)
		for _, col := range cols {
			cell := fit(m.displayValue(row, col), cellWidth)
			if row == m.selectedRow && col == m.selectedCol {
				cell = m.activeCellStyle.Render(cell)
			}
			line += " " + cell
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func viewerContextWindow(center, limit int) []int {
	if limit <= 0 {
		return nil
	}
	if limit <= 3 {
		window := make([]int, limit)
		for i := 0; i < limit; i++ {
			window[i] = i
		}
		return window
	}
	start := clamp(center-1, 0, limit-3)
	return []int{start, start + 1, start + 2}
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

func (m model) visibleColumns() []visibleColumn {
	available := m.availableColumnWidth()
	if available <= 0 {
		return []visibleColumn{{col: m.colOffset, width: 1}}
	}

	visible := make([]visibleColumn, 0, totalCols-m.colOffset)
	used := 0
	for col := m.colOffset; col < totalCols; col++ {
		width := m.columnWidth(col)
		needed := width + 1
		if used+needed > available && len(visible) > 0 {
			break
		}
		visible = append(visible, visibleColumn{col: col, width: width})
		used += needed
	}

	if len(visible) == 0 {
		visible = append(visible, visibleColumn{col: m.colOffset, width: 1})
	}

	return visible
}

func (m model) renderColumns(count int) []visibleColumn {
	if count <= 0 {
		return nil
	}

	columns := make([]visibleColumn, 0, count)
	for i := 0; i < count && m.colOffset+i < totalCols; i++ {
		col := m.colOffset + i
		columns = append(columns, visibleColumn{col: col, width: m.columnWidth(col)})
	}
	return columns
}

func (m model) visibleCols() int {
	return len(m.visibleColumns())
}

func (m model) availableColumnWidth() int {
	return max(0, m.width-m.rowLabelWidth-2)
}

func (m model) maxColumnWidth() int {
	if m.width <= 0 {
		return 1024
	}
	return max(1, m.availableColumnWidth()-1)
}

func (m model) minColumnWidth(col int) int {
	return max(m.cellWidth, runewidth.StringWidth(columnLabel(col)))
}

func (m model) columnWidth(col int) int {
	minWidth := m.minColumnWidth(col)
	maxWidth := max(minWidth, m.maxColumnWidth())
	if width, ok := m.manualColumnWidths[col]; ok {
		return clamp(width, minWidth, maxWidth)
	}

	width := minWidth
	for key := range m.cells {
		if key.col != col {
			continue
		}
		width = max(width, runewidth.StringWidth(m.displayValue(key.row, col)))
		if width >= maxWidth {
			return maxWidth
		}
	}
	if col == m.selectedCol && m.mode == insertMode {
		width = max(width, runewidth.StringWidth(m.editingValue))
	}

	return min(width, maxWidth)
}

func wrapText(value string, width int) []string {
	if width <= 0 {
		return nil
	}

	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	rawLines := strings.Split(value, "\n")
	wrapped := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.ReplaceAll(line, "\t", "    ")
		if line == "" {
			wrapped = append(wrapped, "")
			continue
		}
		for runewidth.StringWidth(line) > width {
			segment := runewidth.Truncate(line, width, "")
			if segment == "" {
				break
			}
			wrapped = append(wrapped, segment)
			line = strings.TrimPrefix(line, segment)
		}
		wrapped = append(wrapped, line)
	}

	return wrapped
}

func renderCellViewerContent(value string, width int, markdown bool) string {
	if width <= 0 {
		return ""
	}
	value = viewerPreviewValue(value)
	if markdown {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width),
		)
		if err == nil {
			rendered, err := renderer.Render(strings.TrimSpace(value))
			if err == nil {
				return strings.TrimRight(rendered, "\n")
			}
		}
	}

	return strings.Join(wrapText(value, width), "\n")
}

func viewerPreviewValue(value string) string {
	if !urlEncodedPattern.MatchString(value) {
		return value
	}
	decoded, err := url.QueryUnescape(value)
	if err != nil || decoded == "" {
		return value
	}
	return decoded
}

func looksLikeMarkdown(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	lines := strings.Split(trimmed, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "#"),
			strings.HasPrefix(line, "- "),
			strings.HasPrefix(line, "* "),
			strings.HasPrefix(line, "> "),
			strings.HasPrefix(line, "```"),
			strings.HasPrefix(line, "1. "),
			strings.Contains(line, "**"),
			strings.Contains(line, "*"),
			strings.Contains(line, "[") && strings.Contains(line, "]("):
			return true
		}
	}
	return false
}

func longCellContent(value string, width int) bool {
	if strings.Contains(value, "\n") {
		return true
	}
	return runewidth.StringWidth(strings.ReplaceAll(value, "\n", " ")) > width
}

func viewerTitleForCell(row, col int) string {
	return fmt.Sprintf("Cell %s", cellRef(row, col))
}

func renderViewerTextInput(value string, cursorPos, width int, cursorModel cursor.Model, textStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	runes := rawTextInputValue(value)
	pos := clamp(cursorPos, 0, len(runes))
	cursorModel.TextStyle = textStyle

	var b strings.Builder
	lineWidth := 0
	writeCursor := func(char string) {
		cursorModel.SetChar(char)
		b.WriteString(cursorModel.View())
		lineWidth += max(1, runewidth.StringWidth(char))
	}
	for i := 0; i <= len(runes); i++ {
		if i == pos {
			if i < len(runes) && runes[i] != '\n' {
				writeCursor(string(runes[i]))
				i++
			} else {
				writeCursor(" ")
			}
		}
		if i >= len(runes) {
			break
		}
		if runes[i] == '\n' {
			b.WriteByte('\n')
			lineWidth = 0
			continue
		}
		rw := runewidth.RuneWidth(runes[i])
		if lineWidth+rw > width {
			b.WriteByte('\n')
			lineWidth = 0
		}
		b.WriteString(textStyle.Render(string(runes[i])))
		lineWidth += rw
	}
	return b.String()
}

func alignCenter(value string, width int) string {
	value = truncate(value, width)
	w := runewidth.StringWidth(value)
	if w >= width {
		return value
	}

	padding := width - w
	left := padding / 2
	right := padding - left
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", right)
}

func fit(value string, width int) string {
	value = truncate(value, width)
	w := runewidth.StringWidth(value)
	if w >= width {
		return value
	}

	return value + strings.Repeat(" ", width-w)
}

func fitLeft(value string, width int) string {
	value = truncate(value, width)
	w := runewidth.StringWidth(value)
	if w >= width {
		return value
	}

	return strings.Repeat(" ", width-w) + value
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}

	value = strings.ReplaceAll(value, "\n", " ")
	if runewidth.StringWidth(value) <= width {
		return value
	}
	if width == 1 {
		return string([]rune(value)[:1])
	}

	return runewidth.Truncate(value, width-1, "") + "…"
}
