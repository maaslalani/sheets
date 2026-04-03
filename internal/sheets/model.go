package sheets

import (
	"encoding/csv"
	"io"
	"maps"
	"os"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newModel() model {
	insertAccent := lipgloss.Color("#D79921")
	selectAccent := lipgloss.Color("#2F66C7")
	statusSelectAccent := lipgloss.Color("13")
	formulaGreen := lipgloss.Color("2")
	errorRed := lipgloss.Color("9")
	gridGray := lipgloss.Color("8")
	selectBackground := lipgloss.Color("#264F78")
	white := lipgloss.Color("15")

	editCursor := cursor.New()
	editCursor.Style = lipgloss.NewStyle().Foreground(insertAccent)
	editCursor.TextStyle = lipgloss.NewStyle()
	editCursor.Blur()

	headerGray := lipgloss.Color("8")
	activeHeaderGray := white
	statusGray := lipgloss.Color("0")
	statusText := lipgloss.Color("7")
	statusAccent := insertAccent

	return model{
		mode:          normalMode,
		rowCount:      defaultRows,
		selectedRow:   0,
		selectedCol:   0,
		selectRow:     0,
		selectCol:     0,
		cellWidth:     12,
		rowLabelWidth: 4,
		cells:         make(map[cellKey]string),
		registers:     make(map[rune]clipboard),
		marks:         make(map[rune]cellKey),
		editCursor:    editCursor,
		headerStyle: lipgloss.NewStyle().
			Foreground(headerGray),
		activeHeaderStyle: lipgloss.NewStyle().
			Foreground(activeHeaderGray).
			Bold(true),
		rowLabelStyle: lipgloss.NewStyle().
			Foreground(headerGray),
		activeRowStyle: lipgloss.NewStyle().
			Foreground(activeHeaderGray).
			Bold(true),
		gridStyle: lipgloss.NewStyle().
			Foreground(gridGray),
		formulaCellStyle: lipgloss.NewStyle().
			Foreground(formulaGreen),
		formulaErrorStyle: lipgloss.NewStyle().
			Foreground(errorRed),
		activeCellStyle: lipgloss.NewStyle().
			Reverse(true),
		activeFormulaStyle: lipgloss.NewStyle().
			Reverse(true).
			Foreground(formulaGreen),
		activeFormulaErrorStyle: lipgloss.NewStyle().
			Reverse(true).
			Foreground(errorRed),
		selectCellStyle: lipgloss.NewStyle().
			Background(selectBackground).
			Foreground(white).
			Bold(true),
		selectFormulaStyle: lipgloss.NewStyle().
			Background(selectBackground).
			Foreground(formulaGreen).
			Bold(true),
		selectFormulaErrorStyle: lipgloss.NewStyle().
			Background(selectBackground).
			Foreground(errorRed).
			Bold(true),
		selectActiveCellStyle: lipgloss.NewStyle().
			Background(selectAccent).
			Foreground(white).
			Bold(true).
			Underline(true),
		selectActiveFormulaStyle: lipgloss.NewStyle().
			Background(selectAccent).
			Foreground(formulaGreen).
			Bold(true).
			Underline(true),
		selectActiveFormulaErrorStyle: lipgloss.NewStyle().
			Background(selectAccent).
			Foreground(errorRed).
			Bold(true).
			Underline(true),
		selectHeaderStyle: lipgloss.NewStyle().
			Background(selectBackground).
			Foreground(white).
			Bold(true),
		selectActiveHeaderStyle: lipgloss.NewStyle().
			Background(selectAccent).
			Foreground(white).
			Bold(true),
		selectRowStyle: lipgloss.NewStyle().
			Background(selectBackground).
			Foreground(white).
			Bold(true),
		selectBorderStyle: lipgloss.NewStyle().
			Background(selectBackground).
			Foreground(selectAccent),
		statusBarStyle: lipgloss.NewStyle().
			Background(statusGray).
			Foreground(statusText),
		statusTextStyle: lipgloss.NewStyle().
			Background(statusGray).
			Foreground(statusText),
		statusAccentStyle: lipgloss.NewStyle().
			Background(statusGray).
			Foreground(statusAccent),
		statusNormalStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("33")).
			Foreground(white).
			Padding(0, 1),
		statusInsertStyle: lipgloss.NewStyle().
			Background(insertAccent).
			Foreground(white).
			Padding(0, 1),
		statusSelectStyle: lipgloss.NewStyle().
			Background(statusSelectAccent).
			Foreground(white).
			Padding(0, 1),
		commandLineStyle: lipgloss.NewStyle().
			Foreground(statusText),
		commandErrorStyle: lipgloss.NewStyle().
			Foreground(errorRed),
	}
}

func (m *model) loadCSVFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := m.loadCSVReader(file); err != nil {
		return err
	}
	m.currentFilePath = path
	return nil
}

func (m *model) loadCSVReader(reader io.Reader) error {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	if err := m.loadCSV(csvReader); err != nil {
		return err
	}
	m.currentFilePath = ""
	return nil
}

func (m *model) loadCSV(reader *csv.Reader) error {
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	m.cells = make(map[cellKey]string)
	m.rowCount = defaultRows
	m.undoStack = nil
	m.redoStack = nil
	m.promptKind = noPrompt
	m.editingValue = ""
	m.editingCursor = 0
	m.deletePending = false
	m.yankPending = false
	m.yankCount = 0
	m.zPending = false
	m.gotoPending = false
	m.gotoBuffer = ""
	m.commandPending = false
	m.commandBuffer = ""
	m.commandCursor = 0
	m.commandMessage = ""
	m.commandError = false
	m.countBuffer = ""
	m.registerPending = false
	m.activeRegister = 0
	m.searchDirection = 0
	m.markPending = false
	m.markJumpPending = false
	m.markJumpExact = false
	m.selectRows = false
	m.hasCopyBuffer = false
	m.selectedRow = 0
	m.selectedCol = 0
	m.selectRow = 0
	m.selectCol = 0
	m.rowOffset = 0
	m.colOffset = 0
	m.jumpBack = nil
	m.jumpForward = nil
	m.lastChange = nil
	m.insertKeys = nil
	m.recordingInsert = false
	m.replayingChange = false

	for row := 0; row < len(records) && row < maxRows; row++ {
		record := records[row]
		for col := 0; col < len(record) && col < totalCols; col++ {
			m.setCellValue(row, col, record[col])
		}
	}
	if len(records) > m.rowCount {
		m.rowCount = min(len(records), maxRows)
	}

	return nil
}

func (m model) csvRecords() [][]string {
	maxRow := -1
	rowWidths := make(map[int]int)
	for key := range m.cells {
		if key.row > maxRow {
			maxRow = key.row
		}
		if width := key.col + 1; width > rowWidths[key.row] {
			rowWidths[key.row] = width
		}
	}

	if maxRow < 0 {
		return nil
	}

	records := make([][]string, maxRow+1)
	for row := 0; row <= maxRow; row++ {
		width := rowWidths[row]
		if width == 0 && row < maxRow {
			width = 2
		}
		record := make([]string, width)
		for col := 0; col < width; col++ {
			record[col] = m.cellValue(row, col)
		}
		records[row] = record
	}

	return records
}

func (m model) writeCSV(writer *csv.Writer) error {
	return writer.WriteAll(m.csvRecords())
}

func (m model) writeCSVFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	return m.writeCSV(writer)
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureVisible()
		return m, nil

	case tea.KeyMsg:
		if isQuitKey(msg) {
			return m, tea.Quit
		}
		if !m.commandPending {
			m.commandMessage = ""
			m.commandError = false
		}
		if m.mode != insertMode && m.commandPending {
			if cmd, handled := m.handlePendingCommand(msg); handled {
				return m, cmd
			}
		}
		if m.mode != insertMode && m.registerPending {
			if m.handlePendingRegister(msg) {
				return m, nil
			}
		}
		if m.mode != insertMode && m.deletePending {
			if m.handlePendingDelete(msg) {
				return m, nil
			}
		}
		if m.mode != insertMode && m.yankPending {
			if m.handlePendingYank(msg) {
				return m, nil
			}
		}
		if m.mode != insertMode && m.zPending {
			if m.handlePendingZ(msg) {
				return m, nil
			}
		}
		if m.mode != insertMode && m.gotoPending {
			if m.handlePendingGoto(msg) {
				return m, nil
			}
		}
		if m.mode != insertMode && (m.markPending || m.markJumpPending) {
			if m.handlePendingMark(msg) {
				return m, nil
			}
		}

		if m.mode == insertMode && isEscapeKey(msg) {
			if m.recordingInsert && !m.replayingChange {
				m.insertKeys = append(m.insertKeys, msg)
			}
			return m.exitInsertMode()
		}
		if m.mode == selectMode && isEscapeKey(msg) {
			m.clearNormalPrefixes()
			return m.exitSelectMode(), nil
		}

		switch m.mode {
		case normalMode:
			return m.updateNormal(msg)
		case insertMode:
			return m.updateInsert(msg)
		case selectMode:
			return m.updateSelect(msg)
		case commandMode:
			return m, nil
		}
	}

	if m.mode == insertMode || m.mode == commandMode {
		var cmd tea.Cmd
		m.editCursor, cmd = m.editCursor.Update(msg)
		return m, cmd
	}

	return m, nil
}

func isQuitKey(msg tea.KeyMsg) bool {
	if msg.Type == tea.KeyCtrlC {
		return true
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == rune(3) {
		return true
	}

	return msg.String() == "ctrl+c"
}

func isEscapeKey(msg tea.KeyMsg) bool {
	if msg.Type == tea.KeyEscape {
		return true
	}

	if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == rune(27) {
		return true
	}

	switch msg.String() {
	case "esc", "ctrl+[", "\x1b":
		return true
	}

	return false
}

func (m *model) pushUndoState() {
	m.undoStack = append(m.undoStack, m.snapshotUndoState())
	m.redoStack = nil
}

func (m *model) undoLastOperation() {
	if len(m.undoStack) == 0 {
		return
	}

	m.redoStack = append(m.redoStack, m.snapshotUndoState())
	last := m.undoStack[len(m.undoStack)-1]
	m.undoStack = m.undoStack[:len(m.undoStack)-1]
	m.restoreUndoState(last)
}

func (m *model) redoLastOperation() {
	if len(m.redoStack) == 0 {
		return
	}

	m.undoStack = append(m.undoStack, m.snapshotUndoState())
	last := m.redoStack[len(m.redoStack)-1]
	m.redoStack = m.redoStack[:len(m.redoStack)-1]
	m.restoreUndoState(last)
}

func (m model) snapshotUndoState() undoState {
	return undoState{
		cells:       cloneCells(m.cells),
		rowCount:    m.rowCount,
		selectedRow: m.selectedRow,
		selectedCol: m.selectedCol,
		selectRow:   m.selectRow,
		selectCol:   m.selectCol,
		selectRows:  m.selectRows,
		rowOffset:   m.rowOffset,
		colOffset:   m.colOffset,
	}
}

func (m *model) restoreUndoState(state undoState) {
	m.cells = cloneCells(state.cells)
	m.rowCount = max(1, state.rowCount)
	m.selectedRow = state.selectedRow
	m.selectedCol = state.selectedCol
	m.selectRow = state.selectRow
	m.selectCol = state.selectCol
	m.selectRows = state.selectRows
	m.rowOffset = state.rowOffset
	m.colOffset = state.colOffset
	m.ensureVisible()
}

func (m model) cellValue(row, col int) string {
	return m.cells[cellKey{row: row, col: col}]
}

func (m *model) setCellValue(row, col int, value string) {
	key := cellKey{row: row, col: col}
	if value == "" {
		delete(m.cells, key)
		return
	}

	m.cells[key] = value
}

func cloneCells(cells map[cellKey]string) map[cellKey]string {
	cloned := make(map[cellKey]string, len(cells))
	maps.Copy(cloned, cells)
	return cloned
}
