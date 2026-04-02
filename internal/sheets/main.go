package sheets

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var openTTY = func() (io.ReadCloser, error) {
	return os.Open("/dev/tty")
}

var readBuildInfo = debug.ReadBuildInfo

const helpText = `Spreadsheets in your terminal.

USAGE
  sheets [file.csv]
      Launch sheets interactively.

  sheets [file.csv] [cell|range|cell=value|range=value]...
      Query or modify a cell through the command-line interface.

  sheets < input.csv
      Launch sheets interactively and load CSV through stdin.

OPTIONS
  -h, --help
      Show this help message.

  -v, --version
      Show the current version.

EXAMPLES
  sheets budget.csv
  sheets budget.csv B9
  sheets budget.csv B1:B3
  sheets budget.csv B7=10 B8=20
  cat budget.csv | sheets
`

func newProgramModel(args []string) (model, error) {
	return newProgramModelWithInput(args, nil)
}

func newProgramModelWithInput(args []string, stdin io.Reader) (model, error) {
	m := newModel()
	if len(args) == 0 {
		if stdin != nil {
			if err := m.loadCSVReader(stdin); err != nil {
				return model{}, err
			}
		}
		return m, nil
	}

	if err := m.loadCSVFile(args[0]); err != nil {
		if os.IsNotExist(err) {
			m.currentFilePath = args[0]
			return m, nil
		}
		return model{}, err
	}

	return m, nil
}

func queryCellValue(path, ref string) (string, error) {
	records, err := queryCellValues(path, ref)
	if err != nil {
		return "", err
	}
	return records[0][0], nil
}

func queryCellValues(path, ref string) ([][]string, error) {
	m := newModel()
	if err := m.loadCSVFile(path); err != nil {
		return nil, err
	}

	return m.queryCellValues(ref)
}

func (m model) queryCellValues(ref string) ([][]string, error) {
	target, ok := parseCellRangeRef(ref)
	if !ok {
		return nil, fmt.Errorf("invalid cell or range: %q", ref)
	}

	top, bottom, left, right := target.bounds()
	records := make([][]string, 0, bottom-top+1)
	for row := top; row <= bottom; row++ {
		record := make([]string, 0, right-left+1)
		for col := left; col <= right; col++ {
			value := m.cellValue(row, col)
			if isFormulaCell(value) {
				value = m.computedCellValue(row, col)
			}
			record = append(record, value)
		}
		records = append(records, record)
	}

	return records, nil
}

func (m model) queryOperationValues(input string) ([][]string, error) {
	parts := strings.Split(input, ",")
	if len(parts) == 1 {
		return m.queryCellValues(input)
	}

	blocks := make([][][]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("invalid cell or range: %q", input)
		}

		block, err := m.queryCellValues(part)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return combineQueryBlocks(blocks, input)
}

func combineQueryBlocks(blocks [][][]string, input string) ([][]string, error) {
	if len(blocks) == 0 {
		return nil, fmt.Errorf("invalid cell or range: %q", input)
	}

	height := len(blocks[0])
	combined := make([][]string, height)
	for row := range height {
		combined[row] = append([]string(nil), blocks[0][row]...)
	}

	for _, block := range blocks[1:] {
		if len(block) != height {
			return nil, fmt.Errorf("query shape mismatch: %q", input)
		}
		for row := range height {
			combined[row] = append(combined[row], block[row]...)
		}
	}

	return combined, nil
}

func parseCellAssignment(input string) (cellRange, string, bool, error) {
	index := strings.Index(input, "=")
	if index == -1 {
		return cellRange{}, "", false, nil
	}

	refText := strings.TrimSpace(input[:index])
	ref, ok := parseCellRangeRef(refText)
	if !ok {
		return cellRange{}, "", true, fmt.Errorf("invalid cell or range: %q", refText)
	}

	return ref, input[index+1:], true, nil
}

func writeCellValue(path, input string) error {
	m, err := newProgramModel([]string{path})
	if err != nil {
		return err
	}

	if err := m.applyCellAssignment(input); err != nil {
		return err
	}

	return m.writeCurrentFile()
}

func (m *model) applyCellAssignment(input string) error {
	ref, value, ok, err := parseCellAssignment(input)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid cell assignment: %q", input)
	}

	if ref.isSingleCell() {
		m.setCellValue(ref.start.row, ref.start.col, value)
		return nil
	}

	values, err := parseAssignmentValues(value)
	if err != nil {
		return fmt.Errorf("invalid cell assignment %q: %w", input, err)
	}

	rows, err := assignmentMatrixForRange(ref, values)
	if err != nil {
		return fmt.Errorf("invalid cell assignment %q: %w", input, err)
	}

	top, _, left, _ := ref.bounds()
	for rowOffset, row := range rows {
		for colOffset, cellValue := range row {
			if len(values) == 1 && len(values[0]) == 1 && isFormulaCell(cellValue) {
				cellValue = rewriteFormulaReferences(cellValue, rowOffset, colOffset)
			}
			m.setCellValue(top+rowOffset, left+colOffset, cellValue)
		}
	}

	return nil
}

func parseAssignmentValues(value string) ([][]string, error) {
	if value == "" {
		return [][]string{{""}}, nil
	}

	reader := csv.NewReader(strings.NewReader(value))
	reader.FieldsPerRecord = -1

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return [][]string{{""}}, nil
	}

	width := 1
	for _, row := range records {
		if len(row) > width {
			width = len(row)
		}
	}

	normalized := make([][]string, len(records))
	for i, row := range records {
		normalized[i] = make([]string, width)
		copy(normalized[i], row)
	}

	return normalized, nil
}

func assignmentMatrixForRange(target cellRange, values [][]string) ([][]string, error) {
	height := target.height()
	width := target.width()
	sourceHeight := len(values)
	sourceWidth := 0
	if sourceHeight > 0 {
		sourceWidth = len(values[0])
	}

	if sourceHeight == 1 && sourceWidth == 1 {
		filled := make([][]string, height)
		for row := range height {
			filled[row] = make([]string, width)
			for col := range width {
				filled[row][col] = values[0][0]
			}
		}
		return filled, nil
	}

	if sourceHeight == height && sourceWidth == width {
		return values, nil
	}

	if width == 1 && sourceHeight == 1 && sourceWidth == height {
		column := make([][]string, height)
		for row := range height {
			column[row] = []string{values[0][row]}
		}
		return column, nil
	}

	if height == 1 && sourceWidth == 1 && sourceHeight == width {
		row := make([]string, width)
		for col := range width {
			row[col] = values[col][0]
		}
		return [][]string{row}, nil
	}

	return nil, fmt.Errorf(
		"shape mismatch: target %dx%d, values %dx%d",
		height, width, sourceHeight, sourceWidth,
	)
}

func writeQueryRecords(stdout io.Writer, path string, records [][][]string) error {
	for _, block := range records {
		writer := newDelimitedWriter(path, stdout)
		if err := writer.WriteAll(block); err != nil {
			return err
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
	}

	return nil
}

func maybeHandleTopLevelOption(args []string, stdout io.Writer) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "-h", "--help":
		_, err := io.WriteString(stdout, helpText)
		return true, err
	case "-v", "--version":
		_, err := fmt.Fprintf(stdout, "sheets %s\n", buildVersion())
		return true, err
	default:
		return false, nil
	}
}

func buildVersion() string {
	info, ok := readBuildInfo()
	if !ok {
		return "dev"
	}

	version := strings.TrimSpace(info.Main.Version)
	if version == "" || version == "(devel)" {
		return "dev"
	}

	return version
}

func run(args []string, stdout io.Writer) error {
	return runWithIO(args, nil, nil, stdout)
}

func runWithIO(args []string, stdin io.Reader, input io.Reader, stdout io.Writer) error {
	if handled, err := maybeHandleTopLevelOption(args, stdout); handled || err != nil {
		return err
	}

	if len(args) > 1 {
		return runCLI(args, stdout)
	}

	m, err := newProgramModelWithInput(args, stdin)
	if err != nil {
		return err
	}

	options := []tea.ProgramOption{tea.WithAltScreen()}
	if input != nil {
		options = append(options, tea.WithInput(input))
	}

	program := tea.NewProgram(m, options...)
	_, err = program.Run()
	return err
}

func runCLI(args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: sheets [file.csv [cell|range|cell=value|range=value]...]")
	}

	path := args[0]
	operations := args[1:]
	hasWrites := false
	for _, operation := range operations {
		if _, _, ok, err := parseCellAssignment(operation); ok || err != nil {
			hasWrites = true
			break
		}
	}

	var m model
	var err error
	if hasWrites {
		m, err = newProgramModel([]string{path})
	} else {
		m = newModel()
		err = m.loadCSVFile(path)
	}
	if err != nil {
		return err
	}

	var queryResults [][][]string
	for _, operation := range operations {
		if _, _, ok, err := parseCellAssignment(operation); ok || err != nil {
			if err != nil {
				return err
			}
			if err := m.applyCellAssignment(operation); err != nil {
				return err
			}
			continue
		}

		records, err := m.queryOperationValues(operation)
		if err != nil {
			return err
		}
		queryResults = append(queryResults, records)
	}

	if hasWrites {
		if err := m.writeCurrentFile(); err != nil {
			return err
		}
	}

	return writeQueryRecords(stdout, path, queryResults)
}

func Main(args []string, stdin *os.File, stdout, stderr io.Writer) int {
	startupInput, programInput, cleanup, err := resolveInputStreams(args, stdin)
	if err == nil && cleanup != nil {
		defer cleanup.Close()
	}
	if err == nil {
		err = runWithIO(args, startupInput, programInput, stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	return 0
}

func resolveInputStreams(args []string, stdin *os.File) (io.Reader, io.Reader, io.Closer, error) {
	if len(args) != 0 || stdin == nil {
		return nil, nil, nil, nil
	}

	info, err := stdin.Stat()
	if err != nil {
		return nil, nil, nil, err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, nil, nil, nil
	}

	tty, err := openTTY()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("interactive mode requires a tty when reading CSV from stdin: %w", err)
	}

	return stdin, tty, tty, nil
}
