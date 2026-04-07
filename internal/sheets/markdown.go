package sheets

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func isMarkdownPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown")
}

func (m *model) loadMarkdownFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	var records [][]string
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineNum++
		if !strings.HasPrefix(line, "|") {
			continue
		}
		if isSeparatorRow(line) {
			continue
		}
		row := parseMarkdownRow(line)
		records = append(records, row)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	if len(records) == 0 {
		m.currentFilePath = path
		return nil
	}

	if err := m.loadCSV(records); err != nil {
		return err
	}
	m.currentFilePath = path
	return nil
}

func isSeparatorRow(line string) bool {
	trimmed := strings.Trim(line, "| ")
	if trimmed == "" {
		return false
	}
	for _, ch := range trimmed {
		if ch != '-' && ch != ':' && ch != '|' && ch != ' ' {
			return false
		}
	}
	return true
}

func parseMarkdownRow(line string) []string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "|") {
		line = line[1:]
	}
	if strings.HasSuffix(line, "|") {
		line = line[:len(line)-1]
	}

	parts := strings.Split(line, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

func (m model) writeMarkdownFile(path string) error {
	records := m.csvRecords()
	if len(records) == 0 {
		return os.WriteFile(path, nil, 0644)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	cols := 0
	for _, row := range records {
		if len(row) > cols {
			cols = len(row)
		}
	}

	widths := make([]int, cols)
	for i := range widths {
		widths[i] = 3
	}
	for _, row := range records {
		for col, val := range row {
			if len(val)+2 > widths[col] {
				widths[col] = len(val) + 2
			}
		}
	}

	writeRow := func(row []string) {
		fmt.Fprint(w, "|")
		for col := 0; col < cols; col++ {
			val := ""
			if col < len(row) {
				val = row[col]
			}
			pad := widths[col] - len(val)
			left := 1
			right := pad - 1
			if right < 1 {
				right = 1
			}
			fmt.Fprintf(w, "%s%s%s|", strings.Repeat(" ", left), val, strings.Repeat(" ", right))
		}
		fmt.Fprintln(w)
	}

	// header
	writeRow(records[0])

	// separator
	fmt.Fprint(w, "|")
	for col := 0; col < cols; col++ {
		fmt.Fprintf(w, " %s |", strings.Repeat("-", widths[col]-2))
	}
	fmt.Fprintln(w)

	// body
	for _, row := range records[1:] {
		writeRow(row)
	}

	return w.Flush()
}
