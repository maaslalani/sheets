package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func newProgramModel(args []string) (model, error) {
	m := newModel()
	if len(args) == 0 {
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
	m := newModel()
	if err := m.loadCSVFile(path); err != nil {
		return "", err
	}

	cell, ok := parseCellRef(ref)
	if !ok {
		return "", fmt.Errorf("invalid cell: %q", ref)
	}

	if !isFormulaCell(m.cellValue(cell.row, cell.col)) {
		return m.cellValue(cell.row, cell.col), nil
	}

	return m.computedCellValue(cell.row, cell.col), nil
}

func parseCellAssignment(input string) (cellKey, string, bool, error) {
	index := strings.Index(input, "=")
	if index == -1 {
		return cellKey{}, "", false, nil
	}

	refText := strings.TrimSpace(input[:index])
	ref, ok := parseCellRef(refText)
	if !ok {
		return cellKey{}, "", true, fmt.Errorf("invalid cell: %q", refText)
	}

	return ref, input[index+1:], true, nil
}

func writeCellValue(path, input string) error {
	ref, value, ok, err := parseCellAssignment(input)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid cell assignment: %q", input)
	}

	m, err := newProgramModel([]string{path})
	if err != nil {
		return err
	}

	m.setCellValue(ref.row, ref.col, value)
	return m.writeCurrentFile()
}

func run(args []string, stdout io.Writer) error {
	if len(args) > 2 {
		return fmt.Errorf("usage: sheets [file.csv [cell|cell=value]]")
	}

	if len(args) == 2 {
		if _, _, ok, err := parseCellAssignment(args[1]); ok || err != nil {
			return writeCellValue(args[0], args[1])
		}

		value, err := queryCellValue(args[0], args[1])
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, value)
		return err
	}

	m, err := newProgramModel(args)
	if err != nil {
		return err
	}

	program := tea.NewProgram(m, tea.WithAltScreen())
	_, err = program.Run()
	return err
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
