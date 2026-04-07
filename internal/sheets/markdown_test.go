package sheets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsMarkdownPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"data.md", true},
		{"data.markdown", true},
		{"DATA.MD", true},
		{"data.csv", false},
		{"readme.txt", false},
	}
	for _, tt := range tests {
		if got := isMarkdownPath(tt.path); got != tt.want {
			t.Errorf("isMarkdownPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsSeparatorRow(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"|---|---|", true},
		{"| --- | --- |", true},
		{"|:---:|---:|", true},
		{"| Name | Age |", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isSeparatorRow(tt.line); got != tt.want {
			t.Errorf("isSeparatorRow(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestParseMarkdownRow(t *testing.T) {
	got := parseMarkdownRow("| Alice | 30 | NYC |")
	want := []string{"Alice", "30", "NYC"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cell %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLoadMarkdownFile(t *testing.T) {
	content := strings.Join([]string{
		"| Name  | Age |",
		"| ----- | --- |",
		"| Alice | 30  |",
		"| Bob   | 25  |",
	}, "\n")

	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := newModel()
	if err := m.loadCSVFile(path); err != nil {
		t.Fatal(err)
	}

	if v := m.cellValue(0, 0); v != "Name" {
		t.Errorf("cell(0,0) = %q, want %q", v, "Name")
	}
	if v := m.cellValue(1, 0); v != "Alice" {
		t.Errorf("cell(1,0) = %q, want %q", v, "Alice")
	}
	if v := m.cellValue(2, 1); v != "25" {
		t.Errorf("cell(2,1) = %q, want %q", v, "25")
	}
}

func TestWriteMarkdownFile(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "Name")
	m.setCellValue(0, 1, "Age")
	m.setCellValue(1, 0, "Alice")
	m.setCellValue(1, 1, "30")

	dir := t.TempDir()
	path := filepath.Join(dir, "out.md")

	if err := m.writeCSVFile(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	output := string(data)
	if !strings.Contains(output, "| Name") {
		t.Errorf("output missing header: %s", output)
	}
	if !strings.Contains(output, "---") {
		t.Errorf("output missing separator: %s", output)
	}
	if !strings.Contains(output, "| Alice") {
		t.Errorf("output missing data row: %s", output)
	}
}

func TestMarkdownRoundTrip(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "X")
	m.setCellValue(0, 1, "Y")
	m.setCellValue(1, 0, "1")
	m.setCellValue(1, 1, "2")

	dir := t.TempDir()
	path := filepath.Join(dir, "round.md")

	if err := m.writeCSVFile(path); err != nil {
		t.Fatal(err)
	}

	m2 := newModel()
	if err := m2.loadCSVFile(path); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		row, col int
		want     string
	}{
		{0, 0, "X"},
		{0, 1, "Y"},
		{1, 0, "1"},
		{1, 1, "2"},
	} {
		if got := m2.cellValue(tc.row, tc.col); got != tc.want {
			t.Errorf("cell(%d,%d) = %q, want %q", tc.row, tc.col, got, tc.want)
		}
	}
}
