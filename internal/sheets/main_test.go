package sheets

import (
	"bytes"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestUpdateQuitsOnCtrlC(t *testing.T) {
	testCases := []struct {
		name string
		mode mode
		msg  tea.KeyMsg
	}{
		{
			name: "normal mode ctrl+c type",
			mode: normalMode,
			msg:  tea.KeyMsg{Type: tea.KeyCtrlC},
		},
		{
			name: "insert mode ctrl+c type",
			mode: insertMode,
			msg:  tea.KeyMsg{Type: tea.KeyCtrlC},
		},
		{
			name: "ctrl+c string form",
			mode: insertMode,
			msg:  tea.KeyMsg{Type: tea.KeyCtrlC},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel()
			m.mode = tc.mode

			_, cmd := m.Update(tc.msg)
			if cmd == nil {
				t.Fatal("expected quit command")
			}
		})
	}
}

func TestLoadCSVPopulatesSheetAndPreservesFormulas(t *testing.T) {
	m := newModel()
	reader := csv.NewReader(strings.NewReader("\"hello,world\",1,=B1+1\nplain,2,\n"))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("expected CSV read to succeed, got %v", err)
	}

	if err := m.loadCSV(records); err != nil {
		t.Fatalf("expected CSV load to succeed, got %v", err)
	}

	assertCellValue(t, m, 0, 0, "hello,world")
	assertCellValue(t, m, 0, 2, "=B1+1")
	assertDisplayValue(t, m, 0, 2, "=2")
	assertSelection(t, m, 0, 0)
}

func TestLoadCSVExpandsRowCountForLargeFiles(t *testing.T) {
	m := newModel()
	records := make([][]string, defaultRows+1)
	records[0] = []string{"top"}
	records[defaultRows] = []string{"bottom"}

	if err := m.loadCSV(records); err != nil {
		t.Fatalf("expected CSV load to succeed, got %v", err)
	}

	if got, want := m.rowCount, defaultRows+1; got != want {
		t.Fatalf("expected row count %d, got %d", want, got)
	}
	assertCellValue(t, m, 0, 0, "top")
	assertCellValue(t, m, defaultRows, 0, "bottom")
}

func TestLoadCSVExpandsRowLabelWidthForFiveDigitRows(t *testing.T) {
	m := newModel()
	records := make([][]string, 10000)

	if err := m.loadCSV(records); err != nil {
		t.Fatalf("expected CSV load to succeed, got %v", err)
	}

	if got, want := m.rowLabelWidth, 5; got != want {
		t.Fatalf("expected row label width %d, got %d", want, got)
	}
	if line := m.renderContentLine(9999, 1); !strings.Contains(line, "10000") {
		t.Fatalf("expected rendered row label to include 10000, got %q", line)
	}
}

func TestLoadCSVRejectsFilesAboveMaxRows(t *testing.T) {
	m := newModel()

	err := m.loadCSV(make([][]string, maxRows+1))
	if err == nil {
		t.Fatal("expected oversized CSV load to fail")
	}
	if want := "maximum supported is 50000"; !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %q", want, err)
	}
	if got, want := m.rowCount, defaultRows; got != want {
		t.Fatalf("expected failed load to preserve row count %d, got %d", want, got)
	}
}

func TestNewProgramModelLoadsCSVFromFirstArg(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "name,value\napples,3\n")

	m, err := newProgramModel([]string{path})
	if err != nil {
		t.Fatalf("expected startup CSV load to succeed, got %v", err)
	}
	assertCellValue(t, m, 0, 0, "name")
	assertCellValue(t, m, 1, 1, "3")
}

func TestNewProgramModelLoadsCSVFromStdin(t *testing.T) {
	m, err := newProgramModelWithInput(nil, strings.NewReader("name,value\nmaas,26\n"))
	if err != nil {
		t.Fatalf("expected startup CSV load from stdin to succeed, got %v", err)
	}

	assertCellValue(t, m, 0, 0, "name")
	assertCellValue(t, m, 1, 0, "maas")
	assertCellValue(t, m, 1, 1, "26")
}

func TestNewProgramModelLoadsTSVFile(t *testing.T) {
	path := writeTempTSV(t, "data.tsv", "name\tvalue\napples\t3\n")

	m, err := newProgramModel([]string{path})
	if err != nil {
		t.Fatalf("expected startup TSV load to succeed, got %v", err)
	}
	assertCellValue(t, m, 0, 0, "name")
	assertCellValue(t, m, 0, 1, "value")
	assertCellValue(t, m, 1, 0, "apples")
	assertCellValue(t, m, 1, 1, "3")
}

func TestRunQueriesTSVFile(t *testing.T) {
	path := writeTempTSV(t, "data.tsv", "name\tvalue\napples\t3\n")
	var stdout bytes.Buffer

	if err := run([]string{path, "A2"}, &stdout); err != nil {
		t.Fatalf("expected TSV cell query to succeed, got %v", err)
	}

	if got, want := stdout.String(), "apples\n"; got != want {
		t.Fatalf("expected TSV query output %q, got %q", want, got)
	}
}

func TestRunQueriesCellRangeFromTSV(t *testing.T) {
	path := writeTempTSV(t, "data.tsv", "1\t2\t=A1+B1\n4\t5\t=A2+B2\n")
	var stdout bytes.Buffer

	if err := run([]string{path, "A1:C2"}, &stdout); err != nil {
		t.Fatalf("expected TSV range query to succeed, got %v", err)
	}

	if got, want := stdout.String(), "1\t2\t3\n4\t5\t9\n"; got != want {
		t.Fatalf("expected TSV range query output %q, got %q", want, got)
	}
}

func TestRunAssignsCellValueInTSV(t *testing.T) {
	path := writeTempTSV(t, "data.tsv", "name\tvalue\napples\t3\n")

	if err := run([]string{path, "B2=20"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("expected TSV cell assignment to succeed, got %v", err)
	}

	loaded := newModel()
	if err := loaded.loadCSVFile(path); err != nil {
		t.Fatalf("expected written TSV to load, got %v", err)
	}

	assertCellValue(t, loaded, 1, 1, "20")
}

func TestResolveInputStreamsUsesStdinOnlyWhenPipedAndNoArgs(t *testing.T) {
	originalOpenTTY := openTTY
	openTTY = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("")), nil
	}
	t.Cleanup(func() {
		openTTY = originalOpenTTY
	})

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	t.Cleanup(func() {
		readPipe.Close()
		writePipe.Close()
	})

	startupInput, programInput, cleanup, err := resolveInputStreams(nil, readPipe)
	if err != nil {
		t.Fatalf("expected piped stdin to resolve, got %v", err)
	}
	if startupInput != readPipe {
		t.Fatal("expected startup input to use piped stdin")
	}
	if programInput == nil {
		t.Fatal("expected interactive input to be configured")
	}
	if cleanup == nil {
		t.Fatal("expected tty cleanup to be returned")
	}
	cleanup.Close()
}

func TestResolveInputStreamsIgnoresStdinWhenArgsPresent(t *testing.T) {
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	t.Cleanup(func() {
		readPipe.Close()
		writePipe.Close()
	})

	startupInput, programInput, cleanup, err := resolveInputStreams([]string{"data.csv"}, readPipe)
	if err != nil {
		t.Fatalf("expected args to bypass stdin resolution, got %v", err)
	}
	if startupInput != nil || programInput != nil || cleanup != nil {
		t.Fatal("expected stdin resolution to be skipped when args are present")
	}
}

func TestRunQueriesPlainCellValueFromCSV(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "name,value\napples,3\n")
	var stdout bytes.Buffer

	if err := run([]string{path, "A2"}, &stdout); err != nil {
		t.Fatalf("expected cell query to succeed, got %v", err)
	}

	if got, want := stdout.String(), "apples\n"; got != want {
		t.Fatalf("expected query output %q, got %q", want, got)
	}
}

func TestRunQueriesEvaluatedFormulaFromCSV(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "1,2,=A1+B1\n")
	var stdout bytes.Buffer

	if err := run([]string{path, "C1"}, &stdout); err != nil {
		t.Fatalf("expected formula query to succeed, got %v", err)
	}

	if got, want := stdout.String(), "3\n"; got != want {
		t.Fatalf("expected query output %q, got %q", want, got)
	}
}

func TestRunQueriesCellRangeFromCSV(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "1,2,=A1+B1\n4,5,=A2+B2\n")
	var stdout bytes.Buffer

	if err := run([]string{path, "A1:C2"}, &stdout); err != nil {
		t.Fatalf("expected range query to succeed, got %v", err)
	}

	if got, want := stdout.String(), "1,2,3\n4,5,9\n"; got != want {
		t.Fatalf("expected range query output %q, got %q", want, got)
	}
}

func TestRunQueriesCommaSeparatedCellsOnSameLine(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "Rent,2000\n")
	var stdout bytes.Buffer

	if err := run([]string{path, "A1,B1"}, &stdout); err != nil {
		t.Fatalf("expected comma-separated query to succeed, got %v", err)
	}

	if got, want := stdout.String(), "Rent,2000\n"; got != want {
		t.Fatalf("expected comma-separated query output %q, got %q", want, got)
	}
}

func TestRunQueriesCommaSeparatedRangesByRow(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "1,4\n2,5\n3,6\n")
	var stdout bytes.Buffer

	if err := run([]string{path, "A1:A3,B1:B3"}, &stdout); err != nil {
		t.Fatalf("expected comma-separated range query to succeed, got %v", err)
	}

	if got, want := stdout.String(), "1,4\n2,5\n3,6\n"; got != want {
		t.Fatalf("expected comma-separated range query output %q, got %q", want, got)
	}
}

func TestRunAssignsCellValueInCSV(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "name,value\napples,3\n")

	if err := run([]string{path, "B2=20"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("expected cell assignment to succeed, got %v", err)
	}

	loaded := newModel()
	if err := loaded.loadCSVFile(path); err != nil {
		t.Fatalf("expected written CSV to load, got %v", err)
	}

	assertCellValue(t, loaded, 1, 1, "20")
}

func TestRunAssignsCellRangeInCSV(t *testing.T) {
	path := writeTempCSV(t, "data.csv", ",,\n,,\n,,\n")

	if err := run([]string{path, "A1:A3=1,2,3"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("expected range assignment to succeed, got %v", err)
	}

	loaded := newModel()
	if err := loaded.loadCSVFile(path); err != nil {
		t.Fatalf("expected written CSV to load, got %v", err)
	}

	assertCellValue(t, loaded, 0, 0, "1")
	assertCellValue(t, loaded, 1, 0, "2")
	assertCellValue(t, loaded, 2, 0, "3")
}

func TestRunAssignsFormulaAcrossRangeWithRelativeReferences(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "1,\n2,\n3,\n")

	if err := run([]string{path, "B1:B3==A1*2"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("expected formula range assignment to succeed, got %v", err)
	}

	loaded := newModel()
	if err := loaded.loadCSVFile(path); err != nil {
		t.Fatalf("expected written CSV to load, got %v", err)
	}

	assertCellValue(t, loaded, 0, 1, "=A1*2")
	assertCellValue(t, loaded, 1, 1, "=A2*2")
	assertCellValue(t, loaded, 2, 1, "=A3*2")
	assertDisplayValue(t, loaded, 0, 1, "=2")
	assertDisplayValue(t, loaded, 1, 1, "=4")
	assertDisplayValue(t, loaded, 2, 1, "=6")
}

func TestRunAssignsFormulaCellInCSV(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "1,2,\n")

	if err := run([]string{path, "C1==A1+B1"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("expected formula assignment to succeed, got %v", err)
	}

	loaded := newModel()
	if err := loaded.loadCSVFile(path); err != nil {
		t.Fatalf("expected written CSV to load, got %v", err)
	}

	assertCellValue(t, loaded, 0, 2, "=A1+B1")
	if records, err := loaded.queryCellValues("C1"); err != nil {
		t.Fatalf("expected assigned formula cell to be queryable, got %v", err)
	} else if got, want := records[0][0], "3"; got != want {
		t.Fatalf("expected queried formula value %q, got %q", want, got)
	}
}

func TestRunProcessesMultipleCLIQueriesAndWritesInOrder(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "1,2\n3,4\n")
	var stdout bytes.Buffer

	if err := run([]string{path, "A1:B2", "B1:B2=9,8", "B1:B2"}, &stdout); err != nil {
		t.Fatalf("expected multiple CLI operations to succeed, got %v", err)
	}

	if got, want := stdout.String(), "1,2\n3,4\n9\n8\n"; got != want {
		t.Fatalf("expected combined query output %q, got %q", want, got)
	}

	loaded := newModel()
	if err := loaded.loadCSVFile(path); err != nil {
		t.Fatalf("expected written CSV to load, got %v", err)
	}

	assertCellValue(t, loaded, 0, 1, "9")
	assertCellValue(t, loaded, 1, 1, "8")
}

func TestRunPrintsMultipleSingleCellQueriesWithoutBlankLines(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "1,2,3\n4,5,6\n7,8,9\n")
	var stdout bytes.Buffer

	if err := run([]string{path, "B1", "B2", "B3"}, &stdout); err != nil {
		t.Fatalf("expected multiple single-cell queries to succeed, got %v", err)
	}

	if got, want := stdout.String(), "2\n5\n8\n"; got != want {
		t.Fatalf("expected combined query output %q, got %q", want, got)
	}
}

func TestRunPrintsHelp(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		t.Run(flag, func(t *testing.T) {
			var stdout bytes.Buffer

			if err := run([]string{flag}, &stdout); err != nil {
				t.Fatalf("expected help output to succeed, got %v", err)
			}

			if got, want := stdout.String(), helpText; got != want {
				t.Fatalf("expected help output %q, got %q", want, got)
			}
		})
	}
}

func TestRunPrintsVersion(t *testing.T) {
	originalReadBuildInfo := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v1.2.3"},
		}, true
	}
	t.Cleanup(func() {
		readBuildInfo = originalReadBuildInfo
	})

	for _, flag := range []string{"-v", "--version"} {
		t.Run(flag, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := run([]string{flag}, &stdout); err != nil {
				t.Fatalf("expected version output to succeed, got %v", err)
			}

			if got, want := stdout.String(), "sheets v1.2.3\n"; got != want {
				t.Fatalf("expected version output %q, got %q", want, got)
			}
		})
	}
}

func TestBuildVersionFallsBackToDev(t *testing.T) {
	testCases := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{
			name: "missing build info",
			want: "dev",
		},
		{
			name: "empty version",
			info: &debug.BuildInfo{},
			ok:   true,
			want: "dev",
		},
		{
			name: "devel version",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
			},
			ok:   true,
			want: "dev",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			originalReadBuildInfo := readBuildInfo
			readBuildInfo = func() (*debug.BuildInfo, bool) {
				return tc.info, tc.ok
			}
			t.Cleanup(func() {
				readBuildInfo = originalReadBuildInfo
			})

			if got := buildVersion(); got != tc.want {
				t.Fatalf("expected version %q, got %q", tc.want, got)
			}
		})
	}
}

func TestRunRejectsInvalidQueryCell(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "value\n")

	if err := run([]string{path, "not-a-cell"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected invalid cell query to fail")
	}
}

func TestRunRejectsInvalidAssignmentCell(t *testing.T) {
	path := writeTempCSV(t, "data.csv", "value\n")

	if err := run([]string{path, "not-a-cell=value"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected invalid cell assignment to fail")
	}
}

func TestRunAssignsCellInMissingCSVPath(t *testing.T) {
	path := tempCSVPath(t, "new-sheet.csv")

	if err := run([]string{path, "B8=20"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("expected assignment against a missing CSV path to succeed, got %v", err)
	}

	loaded := newModel()
	if err := loaded.loadCSVFile(path); err != nil {
		t.Fatalf("expected written CSV to load, got %v", err)
	}

	assertCellValue(t, loaded, 7, 1, "20")
}

func TestNewProgramModelTracksMissingStartupPathForWrite(t *testing.T) {
	path := tempCSVPath(t, "new-sheet.csv")

	m, err := newProgramModel([]string{path})
	if err != nil {
		t.Fatalf("expected missing startup path to initialize a blank sheet, got %v", err)
	}
	if m.currentFilePath != path {
		t.Fatalf("expected current file path %q, got %q", path, m.currentFilePath)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected startup not to create %q before saving", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("expected missing startup file error, got %v", err)
	}

	m.setCellValue(0, 0, "draft")
	pending := startCommand(t, m, "write")
	written := applyKey(t, pending, tea.KeyMsg{Type: tea.KeyEnter})
	if written.commandError {
		t.Fatalf("expected write to create missing startup file, got %q", written.commandMessage)
	}

	loaded := newModel()
	if err := loaded.loadCSVFile(path); err != nil {
		t.Fatalf("expected written startup file to load, got %v", err)
	}
	assertCellValue(t, loaded, 0, 0, "draft")
}

func TestNewModelStartsAtA1(t *testing.T) {
	m := newModel()

	assertSelection(t, m, 0, 0)
	assertSelectionAnchor(t, m, 0, 0)
	if m.rowCount != defaultRows {
		t.Fatalf("expected default row count %d, got %d", defaultRows, m.rowCount)
	}
}

func TestRenderStatusBarShowsSpreadsheetStatus(t *testing.T) {
	m := newModel()
	m.width = 80
	m.selectedRow = 5
	m.selectedCol = 2
	m.setCellValue(m.selectedRow, m.selectedCol, "sum(A1:B2)")

	normalBar := m.renderStatusBar()
	assertContainsAll(t, "status bar", normalBar, "NORMAL", "sum(A1:B2)", "C6")
	assertNotContainsAny(t, "status bar", normalBar, "1 cells", "1 sel", "6:3")

	m.mode = insertMode
	m.editingValue = "editing"

	insertBar := m.renderStatusBar()
	assertContainsAll(t, "insert status bar", insertBar, string(insertMode), "editing", "C6")
	assertNotContainsAny(t, "insert status bar", insertBar, "1 cells", "1 sel", "6:3")
}

func TestRenderCommandLineShowsPendingCommandPrompt(t *testing.T) {
	m := newPendingCommandModel("goto E9", len("goto E9"))
	m.width = 80

	line := m.renderCommandLine()
	if line != "" {
		t.Fatalf("expected no separate command line while prompt is active: %q", line)
	}
	prompt := m.renderCommandPromptLine(m.width)
	assertContainsAll(t, "command prompt", prompt, ":goto E9")
}

func TestRenderCommandLineShowsErrorMessage(t *testing.T) {
	m := newModel()
	m.width = 80
	m.commandMessage = "no such command: 'bogus'"
	m.commandError = true

	line := m.renderCommandLine()
	assertContainsAll(t, "command line", line, "no such command: 'bogus'")
}

func TestRenderStatusBarShowsSelectionRangeInSelectMode(t *testing.T) {
	m := newModel()
	m.width = 80
	m.selectedRow = 3
	m.selectedCol = 1

	selected := applyKeys(t, m, runeKey("v"), tea.KeyMsg{Type: tea.KeyLeft}, tea.KeyMsg{Type: tea.KeyUp})

	status := selected.renderStatusBar()
	assertContainsAll(t, "select status bar", status, "VISUAL", "A3:B4")
	if strings.Contains(status, "B4") && !strings.Contains(status, "A3:B4") {
		t.Fatalf("expected full selection range instead of active cell only: %q", status)
	}
}

func TestActiveRefUsesNormalizedSelectionBounds(t *testing.T) {
	m := newModel()
	m.mode = selectMode
	m.selectRow = 4
	m.selectCol = 3
	m.selectedRow = 1
	m.selectedCol = 1

	if got, want := m.activeRef(), "B2:D5"; got != want {
		t.Fatalf("expected active ref %q, got %q", want, got)
	}
}

func TestParseCellRef(t *testing.T) {
	testCases := []struct {
		name   string
		ref    string
		want   cellKey
		wantOK bool
	}{
		{name: "single letter column", ref: "A1", want: cellKey{row: 0, col: 0}, wantOK: true},
		{name: "double letter column", ref: "AA10", want: cellKey{row: 9, col: 26}, wantOK: true},
		{name: "lowercase ref", ref: "az1", want: cellKey{row: 0, col: 51}, wantOK: true},
		{name: "max row", ref: "A50000", want: cellKey{row: maxRows - 1, col: 0}, wantOK: true},
		{name: "out of range column", ref: "BA1", wantOK: false},
		{name: "row zero invalid", ref: "A0", wantOK: false},
		{name: "row above max", ref: "A50001", wantOK: false},
		{name: "missing row", ref: "A", wantOK: false},
		{name: "missing column", ref: "10", wantOK: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseCellRef(tc.ref)
			if ok != tc.wantOK {
				t.Fatalf("expected ok=%v, got %v", tc.wantOK, ok)
			}
			if ok && got != tc.want {
				t.Fatalf("expected ref %#v, got %#v", tc.want, got)
			}
		})
	}
}

func TestParseCellRangeRef(t *testing.T) {
	testCases := []struct {
		name   string
		ref    string
		want   cellRange
		wantOK bool
	}{
		{name: "single cell", ref: "A1", want: cellRange{start: cellKey{row: 0, col: 0}, end: cellKey{row: 0, col: 0}}, wantOK: true},
		{name: "range", ref: "A1:C3", want: cellRange{start: cellKey{row: 0, col: 0}, end: cellKey{row: 2, col: 2}}, wantOK: true},
		{name: "trimmed", ref: " b2 : a1 ", want: cellRange{start: cellKey{row: 1, col: 1}, end: cellKey{row: 0, col: 0}}, wantOK: true},
		{name: "missing end", ref: "A1:", wantOK: false},
		{name: "too many separators", ref: "A1:B2:C3", wantOK: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseCellRangeRef(tc.ref)
			if ok != tc.wantOK {
				t.Fatalf("expected ok=%v, got %v", tc.wantOK, ok)
			}
			if ok && got != tc.want {
				t.Fatalf("expected ref %#v, got %#v", tc.want, got)
			}
		})
	}
}

func TestDisplayValueEvaluatesSumFormula(t *testing.T) {
	m := newModel()
	m.width = 80
	m.selectedRow = 2
	m.selectedCol = 0
	m.setCellValue(0, 0, "1")
	m.setCellValue(1, 0, "2")
	m.setCellValue(2, 0, "=SUM(A1:A2)")

	assertDisplayValue(t, m, 2, 0, "3")
	if got, want := m.activeValue(), "=SUM(A1:A2)"; got != want {
		t.Fatalf("expected raw active value %q, got %q", want, got)
	}
	status := m.renderStatusBar()
	assertContainsAll(t, "status bar", status, "=SUM(A1:A2)")
}

func TestDisplayValueEvaluatesDirectRefsAndArithmetic(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "4")
	m.setCellValue(0, 1, "=A1*2+1")
	m.setCellValue(1, 0, "hello")
	m.setCellValue(1, 1, "=A2")

	assertDisplayValue(t, m, 0, 1, "=9")
	assertDisplayValue(t, m, 1, 1, "=hello")
}

func TestFormulaDisplayUpdatesWhenReferencedCellsChange(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "1")
	m.setCellValue(0, 1, "=A1+1")

	assertDisplayValue(t, m, 0, 1, "=2")

	m.setCellValue(0, 0, "4")
	assertDisplayValue(t, m, 0, 1, "=5")
}

func TestSumIgnoresBlankAndNonNumericCells(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "1")
	m.setCellValue(1, 0, "")
	m.setCellValue(2, 0, "hello")
	m.setCellValue(3, 0, "2.5")
	m.setCellValue(4, 0, "=SUM(A1:A4)")

	assertDisplayValue(t, m, 4, 0, "3.50")
}

func TestSumColumnShorthandUsesRowsBeforeCurrentCell(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "1")
	m.setCellValue(1, 0, "2")
	m.setCellValue(2, 0, "3")
	m.setCellValue(3, 0, "4")
	m.setCellValue(4, 1, "=SUM(A)")

	assertDisplayValue(t, m, 4, 1, "10")
	m.selectedRow = 4
	m.selectedCol = 1
	if got, want := m.activeValue(), "=SUM(A)"; got != want {
		t.Fatalf("expected raw shorthand formula %q, got %q", want, got)
	}
}

func TestSumColumnShorthandAtTopRowReturnsZero(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "5")
	m.setCellValue(0, 1, "=SUM(A)")

	assertDisplayValue(t, m, 0, 1, "0")
}

func TestSumRowRangeShorthandUsesCurrentColumn(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "100")
	m.setCellValue(1, 0, "200")
	m.setCellValue(2, 0, "300")
	m.setCellValue(0, 1, "1")
	m.setCellValue(1, 1, "2")
	m.setCellValue(2, 1, "3")
	m.setCellValue(4, 1, "=SUM(1:3)")

	assertDisplayValue(t, m, 4, 1, "6")
	m.selectedRow = 4
	m.selectedCol = 1
	if got, want := m.activeValue(), "=SUM(1:3)"; got != want {
		t.Fatalf("expected raw row shorthand formula %q, got %q", want, got)
	}
}

func TestAggregateFunctionsRowRangeShorthandUsesCurrentColumn(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 2, "1")
	m.setCellValue(1, 2, "")
	m.setCellValue(2, 2, "5")
	m.setCellValue(3, 2, "hello")
	m.setCellValue(4, 2, "=AVG(1:4)")
	m.setCellValue(5, 2, "=MIN(4:1)")
	m.setCellValue(6, 2, "=MAX(1:4)")
	m.setCellValue(7, 2, "=COUNT(1:4)")

	for _, tc := range []struct {
		name string
		row  int
		want string
	}{
		{name: "avg", row: 4, want: "=3"},
		{name: "min", row: 5, want: "=1"},
		{name: "max", row: 6, want: "=5"},
		{name: "count", row: 7, want: "=2"},
	} {
		if got := m.displayValue(tc.row, 2); got != tc.want {
			t.Fatalf("expected row shorthand %s display %q, got %q", tc.name, tc.want, got)
		}
	}
}

func TestAggregateFunctionsEvaluateRangesAndExpressions(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "1")
	m.setCellValue(1, 0, "")
	m.setCellValue(2, 0, "hello")
	m.setCellValue(3, 0, "3")
	m.setCellValue(0, 1, "=AVG(A1:A4)")
	m.setCellValue(1, 1, "=MIN(A1:A4, 5)")
	m.setCellValue(2, 1, "=MAX(A1:A4, 5)")
	m.setCellValue(3, 1, "=COUNT(A1:A4, 5, A1+1)")

	for _, tc := range []struct {
		name string
		row  int
		want string
	}{
		{name: "avg", row: 0, want: "=2"},
		{name: "min", row: 1, want: "=1"},
		{name: "max", row: 2, want: "=5"},
		{name: "count", row: 3, want: "=4"},
	} {
		if got := m.displayValue(tc.row, 1); got != tc.want {
			t.Fatalf("expected %s display %q, got %q", tc.name, tc.want, got)
		}
	}
}

func TestAggregateFunctionsColumnShorthandUseRowsBeforeCurrentCell(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "1")
	m.setCellValue(1, 0, "4")
	m.setCellValue(2, 0, "")
	m.setCellValue(3, 0, "hello")
	m.setCellValue(4, 1, "=AVG(A)")
	m.setCellValue(4, 2, "=MIN(A)")
	m.setCellValue(4, 3, "=MAX(A)")
	m.setCellValue(4, 4, "=COUNT(A)")

	for _, tc := range []struct {
		name string
		col  int
		want string
	}{
		{name: "avg", col: 1, want: "=2.50"},
		{name: "min", col: 2, want: "=1"},
		{name: "max", col: 3, want: "=4"},
		{name: "count", col: 4, want: "=2"},
	} {
		if got := m.displayValue(4, tc.col); got != tc.want {
			t.Fatalf("expected shorthand %s display %q, got %q", tc.name, tc.want, got)
		}
	}
}

func TestAggregateFunctionsHandleMissingNumericValues(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "")
	m.setCellValue(1, 0, "hello")
	m.setCellValue(0, 1, "=AVG(A1:A2)")
	m.setCellValue(1, 1, "=MIN(A1:A2)")
	m.setCellValue(2, 1, "=MAX(A1:A2)")
	m.setCellValue(3, 1, "=COUNT(A1:A2)")

	for _, tc := range []struct {
		name string
		row  int
		want string
	}{
		{name: "avg", row: 0, want: "=" + formulaErrorDisplay},
		{name: "min", row: 1, want: "=" + formulaErrorDisplay},
		{name: "max", row: 2, want: "=" + formulaErrorDisplay},
		{name: "count", row: 3, want: "=0"},
	} {
		if got := m.displayValue(tc.row, 1); got != tc.want {
			t.Fatalf("expected %s display %q, got %q", tc.name, tc.want, got)
		}
	}
}

func TestFormulaDisplayShowsErrors(t *testing.T) {
	t.Run("invalid formula", func(t *testing.T) {
		m := newModel()
		m.setCellValue(0, 0, "=SUM(A1:")

		assertDisplayValue(t, m, 0, 0, formulaErrorDisplay)
	})

	t.Run("circular reference", func(t *testing.T) {
		m := newModel()
		m.setCellValue(0, 0, "=B1")
		m.setCellValue(0, 1, "=A1")

		assertDisplayValue(t, m, 0, 0, "="+formulaCycleDisplay)
	})
}

func TestFormulaCellsUseGreenForeground(t *testing.T) {
	m := newModel()

	for _, tc := range []struct {
		name string
		got  lipgloss.TerminalColor
	}{
		{name: "formula cell", got: m.formulaCellStyle.GetForeground()},
		{name: "active formula cell", got: m.activeFormulaStyle.GetForeground()},
		{name: "selected formula cell", got: m.selectFormulaStyle.GetForeground()},
	} {
		if want := lipgloss.Color("2"); tc.got != want {
			t.Fatalf("expected %s foreground %v, got %v", tc.name, want, tc.got)
		}
	}
}

func TestFormulaErrorCellsUseRedForeground(t *testing.T) {
	m := newModel()

	for _, tc := range []struct {
		name string
		got  lipgloss.TerminalColor
	}{
		{name: "formula error cell", got: m.formulaErrorStyle.GetForeground()},
		{name: "active formula error cell", got: m.activeFormulaErrorStyle.GetForeground()},
		{name: "selected formula error cell", got: m.selectFormulaErrorStyle.GetForeground()},
	} {
		if want := lipgloss.Color("9"); tc.got != want {
			t.Fatalf("expected %s foreground %v, got %v", tc.name, want, tc.got)
		}
	}
}

func TestRenderContentLinePrefixesAndStylesFormulaCells(t *testing.T) {
	m := newModel()
	m.selectedRow = 1
	m.selectedCol = 1
	m.setCellValue(0, 1, "1")
	m.setCellValue(1, 1, "2")
	m.setCellValue(0, 0, "=SUM(B1:B2)")

	line := m.renderContentLine(0, 2)
	want := m.formulaCellStyle.Render(fit("3", m.cellWidth))
	if !strings.Contains(line, want) {
		t.Fatalf("expected rendered line to include styled formula cell:\nwant fragment=%q\ngot line=%q", want, line)
	}
}

func TestRenderContentLineStylesFormulaErrorsRed(t *testing.T) {
	m := newModel()
	m.selectedRow = 1
	m.selectedCol = 1
	m.setCellValue(0, 0, "=SUM(A1:")

	line := m.renderContentLine(0, 2)
	want := m.formulaErrorStyle.Render(fit(formulaErrorDisplay, m.cellWidth))
	if !strings.Contains(line, want) {
		t.Fatalf("expected rendered line to include styled formula error cell:\nwant fragment=%q\ngot line=%q", want, line)
	}
}

func TestShiftVEntersFullRowSelection(t *testing.T) {
	m := newModel()
	m.selectedRow = 3
	m.selectedCol = 4

	selected := applyKey(t, m, runeKey("V"))
	if selected.mode != selectMode {
		t.Fatalf("expected select mode, got %q", selected.mode)
	}
	if !selected.selectRows {
		t.Fatal("expected Shift+V to enable full-row selection")
	}
	if selected.selectRow != 3 || selected.selectCol != 4 {
		t.Fatalf("expected selection anchor (3,4), got (%d,%d)", selected.selectRow, selected.selectCol)
	}
	if got, want := selected.activeRef(), cellRef(3, 0)+":"+cellRef(3, totalCols-1); got != want {
		t.Fatalf("expected active ref %q, got %q", want, got)
	}
	if !selected.selectionContains(3, 0) || !selected.selectionContains(3, totalCols-1) {
		t.Fatal("expected full row to be selected")
	}
	if selected.selectionContains(2, 0) {
		t.Fatal("did not expect other rows to be selected")
	}

	expanded := applyKey(t, selected, tea.KeyMsg{Type: tea.KeyDown})
	if got, want := expanded.activeRef(), cellRef(3, 0)+":"+cellRef(4, totalCols-1); got != want {
		t.Fatalf("expected expanded active ref %q, got %q", want, got)
	}
}

func TestSelectModeShiftVExpandsSelectionToFullRows(t *testing.T) {
	m := newModel()
	m.selectedRow = 0
	m.selectedCol = 1

	selected := applyKey(t, m, runeKey("v"))
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyRight})
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyDown})

	got := applyKey(t, selected, runeKey("V"))
	if !got.selectRows {
		t.Fatal("expected Shift+V to convert selection to full rows")
	}
	if got.mode != selectMode {
		t.Fatalf("expected select mode, got %q", got.mode)
	}
	if got.selectRow != 0 || got.selectedRow != 1 {
		t.Fatalf("expected row range 0:1, got %d:%d", got.selectRow, got.selectedRow)
	}
	if got.selectCol != 1 || got.selectedCol != 2 {
		t.Fatalf("expected original column endpoints to be preserved, got %d:%d", got.selectCol, got.selectedCol)
	}
	if got.selectionContains(2, 0) {
		t.Fatal("did not expect rows outside the selection to be selected")
	}
	if !got.selectionContains(0, 0) || !got.selectionContains(1, totalCols-1) {
		t.Fatal("expected full-width row selection after Shift+V")
	}
	if got, want := got.activeRef(), cellRef(0, 0)+":"+cellRef(1, totalCols-1); got != want {
		t.Fatalf("expected active ref %q, got %q", want, got)
	}
}

func TestInsertModeEmacsBindings(t *testing.T) {
	testCases := []struct {
		name       string
		value      string
		cursor     int
		key        tea.KeyMsg
		wantValue  string
		wantCursor int
	}{
		{
			name:       "ctrl+a moves to beginning",
			value:      "hello",
			cursor:     5,
			key:        tea.KeyMsg{Type: tea.KeyCtrlA},
			wantValue:  "hello",
			wantCursor: 0,
		},
		{
			name:       "ctrl+e moves to end",
			value:      "hello",
			cursor:     1,
			key:        tea.KeyMsg{Type: tea.KeyCtrlE},
			wantValue:  "hello",
			wantCursor: 5,
		},
		{
			name:       "ctrl+b moves backward",
			value:      "hello",
			cursor:     3,
			key:        tea.KeyMsg{Type: tea.KeyCtrlB},
			wantValue:  "hello",
			wantCursor: 2,
		},
		{
			name:       "ctrl+f moves forward",
			value:      "hello",
			cursor:     2,
			key:        tea.KeyMsg{Type: tea.KeyCtrlF},
			wantValue:  "hello",
			wantCursor: 3,
		},
		{
			name:       "ctrl+d deletes at cursor",
			value:      "hello",
			cursor:     1,
			key:        tea.KeyMsg{Type: tea.KeyCtrlD},
			wantValue:  "hllo",
			wantCursor: 1,
		},
		{
			name:       "ctrl+k deletes to end",
			value:      "hello",
			cursor:     2,
			key:        tea.KeyMsg{Type: tea.KeyCtrlK},
			wantValue:  "he",
			wantCursor: 2,
		},
		{
			name:       "ctrl+u deletes to beginning",
			value:      "hello",
			cursor:     2,
			key:        tea.KeyMsg{Type: tea.KeyCtrlU},
			wantValue:  "llo",
			wantCursor: 0,
		},
		{
			name:       "ctrl+w deletes previous word",
			value:      "hello world",
			cursor:     len("hello world"),
			key:        tea.KeyMsg{Type: tea.KeyCtrlW},
			wantValue:  "hello ",
			wantCursor: len("hello "),
		},
		{
			name:       "ctrl+w deletes spaces and previous word",
			value:      "hello   world",
			cursor:     len("hello   "),
			key:        tea.KeyMsg{Type: tea.KeyCtrlW},
			wantValue:  "world",
			wantCursor: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newInsertEditingModel(tc.value, tc.cursor)

			got, _ := updateKey(t, m, tc.key)

			if got.editingValue != tc.wantValue {
				t.Fatalf("expected value %q, got %q", tc.wantValue, got.editingValue)
			}
			if got.editingCursor != tc.wantCursor {
				t.Fatalf("expected cursor %d, got %d", tc.wantCursor, got.editingCursor)
			}
		})
	}
}

func TestInsertModeAllowsSpaces(t *testing.T) {
	m := newInsertEditingModel("hello", len("hello"))

	got := applyKey(t, m, tea.KeyMsg{Type: tea.KeySpace})

	if got.editingValue != "hello " {
		t.Fatalf("expected space to be inserted, got %q", got.editingValue)
	}
	if got.editingCursor != len("hello ") {
		t.Fatalf("expected cursor at %d, got %d", len("hello "), got.editingCursor)
	}
}

func TestInsertModeInsertsAtCursorWithoutCorruptingTail(t *testing.T) {
	m := newInsertEditingModel("=(1:3)", 1)

	got := applyKey(t, m, runeKey("A"))

	if got.editingValue != "=A(1:3)" {
		t.Fatalf("expected insertion to preserve tail, got %q", got.editingValue)
	}
	if got.editingCursor != 2 {
		t.Fatalf("expected cursor at 2, got %d", got.editingCursor)
	}
}

func TestEditingSumFormulaCanBecomeAverage(t *testing.T) {
	m := newModel()
	m.selectedRow = 4
	m.selectedCol = 1
	m.setCellValue(4, 1, "=SUM(1:3)")

	got := applyKeys(
		t,
		m,
		runeKey("i"),
		tea.KeyMsg{Type: tea.KeyCtrlA},
		tea.KeyMsg{Type: tea.KeyCtrlF},
		tea.KeyMsg{Type: tea.KeyCtrlD},
		tea.KeyMsg{Type: tea.KeyCtrlD},
		tea.KeyMsg{Type: tea.KeyCtrlD},
		runeKey("A"),
		runeKey("V"),
		runeKey("G"),
	)

	if got.editingValue != "=AVG(1:3)" {
		t.Fatalf("expected in-progress edit %q, got %q", "=AVG(1:3)", got.editingValue)
	}

	got = applyKey(t, got, tea.KeyMsg{Type: tea.KeyEscape})

	assertCellValue(t, got, 4, 1, "=AVG(1:3)")
}

func TestInsertModeCtrlNAndCtrlPNavigateRows(t *testing.T) {
	testCases := []struct {
		name          string
		key           tea.KeyMsg
		startRow      int
		startCol      int
		startValue    string
		targetValue   string
		wantRow       int
		wantCol       int
		wantEditValue string
	}{
		{
			name:          "ctrl+n moves down and stays in insert mode",
			key:           tea.KeyMsg{Type: tea.KeyCtrlN},
			startRow:      1,
			startCol:      2,
			startValue:    "edit-me",
			targetValue:   "below",
			wantRow:       2,
			wantCol:       2,
			wantEditValue: "below",
		},
		{
			name:          "ctrl+p moves up and stays in insert mode",
			key:           tea.KeyMsg{Type: tea.KeyCtrlP},
			startRow:      2,
			startCol:      2,
			startValue:    "edit-me",
			targetValue:   "above",
			wantRow:       1,
			wantCol:       2,
			wantEditValue: "above",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newInsertEditingModel(tc.startValue, len(tc.startValue))
			m.selectedRow = tc.startRow
			m.selectedCol = tc.startCol
			m.setCellValue(tc.startRow+deltaForKey(tc.key), tc.startCol, tc.targetValue)

			got := applyKey(t, m, tc.key)

			if got.mode != insertMode {
				t.Fatalf("expected to stay in insert mode, got %q", got.mode)
			}
			assertSelection(t, got, tc.wantRow, tc.wantCol)
			assertCellValue(t, got, tc.startRow, tc.startCol, tc.startValue)
			if got.editingValue != tc.wantEditValue {
				t.Fatalf("expected loaded edit value %q, got %q", tc.wantEditValue, got.editingValue)
			}
		})
	}
}

func TestInsertModeTabNavigatesToNextColumn(t *testing.T) {
	m := newInsertEditingModel("edit-me", len("edit-me"))
	m.selectedRow = 1
	m.selectedCol = 2
	m.setCellValue(1, 3, "right")

	got := applyKey(t, m, tea.KeyMsg{Type: tea.KeyTab})

	if got.mode != insertMode {
		t.Fatalf("expected to stay in insert mode, got %q", got.mode)
	}
	assertSelection(t, got, 1, 3)
	assertCellValue(t, got, 1, 2, "edit-me")
	if got.editingValue != "right" {
		t.Fatalf("expected loaded edit value %q, got %q", "right", got.editingValue)
	}
	if got.editingCursor != len("right") {
		t.Fatalf("expected cursor at %d, got %d", len("right"), got.editingCursor)
	}
}

func TestInsertModeShiftTabNavigatesToPreviousColumn(t *testing.T) {
	m := newInsertEditingModel("edit-me", len("edit-me"))
	m.selectedRow = 1
	m.selectedCol = 2
	m.setCellValue(1, 1, "left")

	got := applyKey(t, m, tea.KeyMsg{Type: tea.KeyShiftTab})

	if got.mode != insertMode {
		t.Fatalf("expected to stay in insert mode, got %q", got.mode)
	}
	assertSelection(t, got, 1, 1)
	assertCellValue(t, got, 1, 2, "edit-me")
	if got.editingValue != "left" {
		t.Fatalf("expected loaded edit value %q, got %q", "left", got.editingValue)
	}
	if got.editingCursor != len("left") {
		t.Fatalf("expected cursor at %d, got %d", len("left"), got.editingCursor)
	}
}

func TestNormalModeLowercaseOInsertsRowBelowAndEntersInsertMode(t *testing.T) {
	m := newModel()
	m.selectedRow = 1
	m.selectedCol = 2
	startRows := m.rowCount
	m.setCellValue(1, 2, "current")
	m.setCellValue(2, 0, "below-left")
	m.setCellValue(2, 2, "below")
	m.setCellValue(3, 2, "=A3")

	got := applyKey(t, m, runeKey("o"))

	if got.mode != insertMode {
		t.Fatalf("expected insert mode, got %q", got.mode)
	}
	if got.selectedRow != 2 || got.selectedCol != 2 {
		t.Fatalf("expected selection (2,2), got (%d,%d)", got.selectedRow, got.selectedCol)
	}
	if got.editingValue != "" {
		t.Fatalf("expected empty inserted row editor, got %q", got.editingValue)
	}
	if got.editingCursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", got.editingCursor)
	}
	if got.rowCount != startRows+1 {
		t.Fatalf("expected row count %d after o, got %d", startRows+1, got.rowCount)
	}
	if got.cellValue(1, 2) != "current" {
		t.Fatalf("expected current row to stay in place, got %q", got.cellValue(1, 2))
	}
	if got.cellValue(2, 0) != "" || got.cellValue(2, 2) != "" {
		t.Fatalf("expected inserted row to be blank, got (%q,%q)", got.cellValue(2, 0), got.cellValue(2, 2))
	}
	if got.cellValue(3, 0) != "below-left" || got.cellValue(3, 2) != "below" {
		t.Fatalf("expected previous lower row to shift down, got (%q,%q)", got.cellValue(3, 0), got.cellValue(3, 2))
	}
	if got.cellValue(4, 2) != "=A4" {
		t.Fatalf("expected shifted formula to be rewritten, got %q", got.cellValue(4, 2))
	}
}

func TestNormalModeUppercaseIEntersInsertModeAtCellStart(t *testing.T) {
	m := newModel()
	m.selectedRow = 2
	m.selectedCol = 1
	m.setCellValue(2, 1, "current")

	got := applyKey(t, m, runeKey("I"))

	if got.mode != insertMode {
		t.Fatalf("expected insert mode, got %q", got.mode)
	}
	if got.selectedRow != 2 || got.selectedCol != 1 {
		t.Fatalf("expected selection (2,1), got (%d,%d)", got.selectedRow, got.selectedCol)
	}
	if got.editingValue != "current" {
		t.Fatalf("expected loaded cell value %q, got %q", "current", got.editingValue)
	}
	if got.editingCursor != 0 {
		t.Fatalf("expected cursor at cell start, got %d", got.editingCursor)
	}
}

func TestNormalModeCLowersIntoInsertAfterClearingCell(t *testing.T) {
	m := newModel()
	m.selectedRow = 2
	m.selectedCol = 1
	m.setCellValue(2, 1, "current")

	got := applyKey(t, m, runeKey("c"))

	if got.mode != insertMode {
		t.Fatalf("expected insert mode, got %q", got.mode)
	}
	if got.selectedRow != 2 || got.selectedCol != 1 {
		t.Fatalf("expected selection (2,1), got (%d,%d)", got.selectedRow, got.selectedCol)
	}
	if got.cellValue(2, 1) != "" {
		t.Fatalf("expected current cell to be cleared immediately, got %q", got.cellValue(2, 1))
	}
	if got.editingValue != "" {
		t.Fatalf("expected empty editing buffer after c, got %q", got.editingValue)
	}
	if got.editingCursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", got.editingCursor)
	}

	got = applyKey(t, got, runeKey("x"))
	got = applyKey(t, got, tea.KeyMsg{Type: tea.KeyEscape})
	if got.cellValue(2, 1) != "x" {
		t.Fatalf("expected edited replacement value %q, got %q", "x", got.cellValue(2, 1))
	}
}

func TestNormalModeUppercaseOInsertsRowAboveAndUndoRestoresRows(t *testing.T) {
	m := newModel()
	m.selectedRow = 2
	m.selectedCol = 1
	startRows := m.rowCount
	m.setCellValue(1, 1, "above")
	m.setCellValue(2, 1, "current")
	m.setCellValue(3, 1, "below")

	inserted := applyKey(t, m, runeKey("O"))

	if inserted.mode != insertMode {
		t.Fatalf("expected insert mode, got %q", inserted.mode)
	}
	if inserted.selectedRow != 2 || inserted.selectedCol != 1 {
		t.Fatalf("expected selection (2,1), got (%d,%d)", inserted.selectedRow, inserted.selectedCol)
	}
	if inserted.editingValue != "" {
		t.Fatalf("expected empty inserted row editor, got %q", inserted.editingValue)
	}
	if inserted.rowCount != startRows+1 {
		t.Fatalf("expected row count %d after O, got %d", startRows+1, inserted.rowCount)
	}
	if inserted.cellValue(1, 1) != "above" {
		t.Fatalf("expected row above to stay in place, got %q", inserted.cellValue(1, 1))
	}
	if inserted.cellValue(2, 1) != "" {
		t.Fatalf("expected inserted row to be blank, got %q", inserted.cellValue(2, 1))
	}
	if inserted.cellValue(3, 1) != "current" || inserted.cellValue(4, 1) != "below" {
		t.Fatalf("expected rows at and below selection to shift down, got (%q,%q)", inserted.cellValue(3, 1), inserted.cellValue(4, 1))
	}

	normal := applyKey(t, inserted, tea.KeyMsg{Type: tea.KeyEscape})
	undone := applyKey(t, normal, runeKey("u"))

	if undone.cellValue(1, 1) != "above" || undone.cellValue(2, 1) != "current" || undone.cellValue(3, 1) != "below" {
		t.Fatalf("expected undo to restore original rows, got (%q,%q,%q)", undone.cellValue(1, 1), undone.cellValue(2, 1), undone.cellValue(3, 1))
	}
	if undone.rowCount != startRows {
		t.Fatalf("expected undo to restore row count %d, got %d", startRows, undone.rowCount)
	}
}

func TestNormalModeDWaitsForDeleteRowTarget(t *testing.T) {
	m := newModel()
	m.selectedRow = 1
	m.selectedCol = 2
	m.setCellValue(1, 2, "current")

	got := applyKey(t, m, runeKey("d"))

	if !got.deletePending {
		t.Fatal("expected d to start a pending delete command")
	}
	if got.statusTitle() != "d" {
		t.Fatalf("expected status title %q, got %q", "d", got.statusTitle())
	}
	if got.cellValue(1, 2) != "current" {
		t.Fatalf("expected row contents to remain unchanged, got %q", got.cellValue(1, 2))
	}
}

func TestNormalModeDDDeletesRowAndUndoRestoresIt(t *testing.T) {
	m := newModel()
	m.selectedRow = 1
	m.selectedCol = 2
	startRows := m.rowCount
	m.setCellValue(0, 2, "above")
	m.setCellValue(1, 2, "current")
	m.setCellValue(2, 0, "below-left")
	m.setCellValue(2, 2, "below")
	m.setCellValue(3, 2, "=A3")

	pending := applyKey(t, m, runeKey("d"))
	deleted := applyKey(t, pending, runeKey("d"))

	if deleted.deletePending {
		t.Fatal("expected pending delete command to clear after dd")
	}
	if deleted.mode != normalMode {
		t.Fatalf("expected normal mode, got %q", deleted.mode)
	}
	if deleted.selectedRow != 1 || deleted.selectedCol != 2 {
		t.Fatalf("expected selection to stay at (1,2), got (%d,%d)", deleted.selectedRow, deleted.selectedCol)
	}
	if deleted.cellValue(0, 2) != "above" {
		t.Fatalf("expected row above to stay in place, got %q", deleted.cellValue(0, 2))
	}
	if deleted.cellValue(1, 0) != "below-left" || deleted.cellValue(1, 2) != "below" {
		t.Fatalf("expected lower row to shift up, got (%q,%q)", deleted.cellValue(1, 0), deleted.cellValue(1, 2))
	}
	if deleted.cellValue(2, 2) != "=A2" {
		t.Fatalf("expected shifted formula to be rewritten, got %q", deleted.cellValue(2, 2))
	}
	if deleted.cellValue(3, 2) != "" {
		t.Fatalf("expected last shifted row to be cleared, got %q", deleted.cellValue(3, 2))
	}
	if deleted.rowCount != startRows-1 {
		t.Fatalf("expected row count %d after dd, got %d", startRows-1, deleted.rowCount)
	}

	undone := applyKey(t, deleted, runeKey("u"))

	if undone.cellValue(0, 2) != "above" || undone.cellValue(1, 2) != "current" || undone.cellValue(2, 2) != "below" || undone.cellValue(3, 2) != "=A3" {
		t.Fatalf("expected undo to restore original rows, got (%q,%q,%q,%q)", undone.cellValue(0, 2), undone.cellValue(1, 2), undone.cellValue(2, 2), undone.cellValue(3, 2))
	}
	if undone.rowCount != startRows {
		t.Fatalf("expected undo to restore row count %d, got %d", startRows, undone.rowCount)
	}
}

func TestNormalModeGWaitsForGotoTarget(t *testing.T) {
	m := newModel()
	m.mode = normalMode
	m.selectedRow = 5
	m.selectedCol = 2

	got := applyKey(t, m, runeKey("g"))
	if !got.gotoPending {
		t.Fatal("expected g to start a pending goto command")
	}
	if got.gotoBuffer != "" {
		t.Fatalf("expected empty goto buffer, got %q", got.gotoBuffer)
	}
	if got.selectedRow != 5 || got.selectedCol != 2 {
		t.Fatalf("expected selection to stay at (5,2), got (%d,%d)", got.selectedRow, got.selectedCol)
	}
}

func TestNormalModeColonStartsCommandPrompt(t *testing.T) {
	m := newModel()
	m.mode = normalMode
	m.selectedRow = 5
	m.selectedCol = 2

	got := applyKey(t, m, runeKey(":"))

	if !got.commandPending {
		t.Fatal("expected : to start a pending command prompt")
	}
	if got.mode != commandMode {
		t.Fatalf("expected command mode, got %q", got.mode)
	}
	if got.commandBuffer != "" {
		t.Fatalf("expected empty command buffer, got %q", got.commandBuffer)
	}
	if got.commandCursor != 0 {
		t.Fatalf("expected empty command cursor, got %d", got.commandCursor)
	}
	if got.statusTitle() != "" {
		t.Fatalf("expected empty status title, got %q", got.statusTitle())
	}
	if got.selectedRow != 5 || got.selectedCol != 2 {
		t.Fatalf("expected selection to stay at (5,2), got (%d,%d)", got.selectedRow, got.selectedCol)
	}
}

func TestCommandPromptGotoMovesToTypedCell(t *testing.T) {
	m := newModel()
	m.mode = normalMode

	pending := startCommand(t, m, "goto E9")

	got, cmd := updateKey(t, pending, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("expected goto command not to quit, got command %v", cmd)
	}
	if got.commandPending {
		t.Fatal("expected command prompt to close after Enter")
	}
	if got.mode != normalMode {
		t.Fatalf("expected normal mode after command executes, got %q", got.mode)
	}
	assertSelection(t, got, 8, 4)
}

func TestCommandPromptQuitReturnsQuitCommand(t *testing.T) {
	m := newModel()
	m.mode = normalMode

	pending := startCommand(t, m, "quit")

	_, cmd := updateKey(t, pending, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestCommandPromptWritePersistsCSVToPath(t *testing.T) {
	path := tempCSVPath(t, "with space/sheet.csv")

	m := newModel()
	m.mode = normalMode
	m.setCellValue(0, 0, "hello,world")
	m.setCellValue(0, 2, "=B1+1")
	m.setCellValue(2, 1, "tail")

	pending := startCommand(t, m, "write "+path)
	got := applyKey(t, pending, tea.KeyMsg{Type: tea.KeyEnter})

	if got.commandPending {
		t.Fatal("expected command prompt to close after Enter")
	}
	if got.commandError {
		t.Fatalf("expected successful write command, got %q", got.commandMessage)
	}
	if got.commandMessage != "wrote "+path {
		t.Fatalf("expected write success message, got %q", got.commandMessage)
	}

	loaded := newModel()
	if err := loaded.loadCSVFile(path); err != nil {
		t.Fatalf("expected written CSV to load, got %v", err)
	}
	assertCellValue(t, loaded, 0, 0, "hello,world")
	assertCellValue(t, loaded, 0, 2, "=B1+1")
	assertCellValue(t, loaded, 2, 1, "tail")
}

func TestCommandPromptEditLoadsCSVFromPath(t *testing.T) {
	path := writeTempCSV(t, "with space/input.csv", "name,value,=B1+1\napples,3,\n")

	m := newModel()
	m.mode = normalMode
	m.setCellValue(4, 4, "stale")
	m.selectedRow = 4
	m.selectedCol = 4

	pending := startCommand(t, m, "edit "+path)
	got := applyKey(t, pending, tea.KeyMsg{Type: tea.KeyEnter})

	if got.commandPending {
		t.Fatal("expected command prompt to close after Enter")
	}
	if got.commandError {
		t.Fatalf("expected successful edit command, got %q", got.commandMessage)
	}
	if got.commandMessage != "loaded "+path {
		t.Fatalf("expected edit success message, got %q", got.commandMessage)
	}
	assertCellValue(t, got, 0, 0, "name")
	assertCellValue(t, got, 0, 2, "=B1+1")
	assertSelection(t, got, 0, 0)
	assertCellValue(t, got, 4, 4, "")
}

func TestCommandPromptUnknownCommandShowsMessage(t *testing.T) {
	m := newModel()
	m.mode = normalMode

	pending := startCommand(t, m, "bogus")
	got := applyKey(t, pending, tea.KeyMsg{Type: tea.KeyEnter})

	if got.commandPending {
		t.Fatal("expected command prompt to close after Enter")
	}
	if got.commandMessage != "no such command: 'bogus'" {
		t.Fatalf("expected unknown command message, got %q", got.commandMessage)
	}
	if !got.commandError {
		t.Fatal("expected unknown command to set command error flag")
	}
}

func TestCommandModeUsesPlainStatusStyle(t *testing.T) {
	m := newModel()
	m.mode = commandMode

	got := m.renderStatusMode()
	want := m.statusTextStyle.Render(fit("COMMAND", len("COMMAND")))
	if got != want {
		t.Fatalf("expected command mode to use plain status style:\nwant=%q\ngot=%q", want, got)
	}
}

func TestCommandModeEmacsBindings(t *testing.T) {
	testCases := []struct {
		name       string
		value      string
		cursor     int
		key        tea.KeyMsg
		wantValue  string
		wantCursor int
	}{
		{
			name:       "ctrl+a moves to beginning",
			value:      "goto E9",
			cursor:     len("goto E9"),
			key:        tea.KeyMsg{Type: tea.KeyCtrlA},
			wantValue:  "goto E9",
			wantCursor: 0,
		},
		{
			name:       "ctrl+e moves to end",
			value:      "goto E9",
			cursor:     1,
			key:        tea.KeyMsg{Type: tea.KeyCtrlE},
			wantValue:  "goto E9",
			wantCursor: len("goto E9"),
		},
		{
			name:       "ctrl+b moves backward",
			value:      "goto E9",
			cursor:     5,
			key:        tea.KeyMsg{Type: tea.KeyCtrlB},
			wantValue:  "goto E9",
			wantCursor: 4,
		},
		{
			name:       "ctrl+f moves forward",
			value:      "goto E9",
			cursor:     4,
			key:        tea.KeyMsg{Type: tea.KeyCtrlF},
			wantValue:  "goto E9",
			wantCursor: 5,
		},
		{
			name:       "ctrl+d deletes at cursor",
			value:      "goto E9",
			cursor:     5,
			key:        tea.KeyMsg{Type: tea.KeyCtrlD},
			wantValue:  "goto 9",
			wantCursor: 5,
		},
		{
			name:       "ctrl+k deletes to end",
			value:      "goto E9",
			cursor:     5,
			key:        tea.KeyMsg{Type: tea.KeyCtrlK},
			wantValue:  "goto ",
			wantCursor: 5,
		},
		{
			name:       "ctrl+u deletes to beginning",
			value:      "goto E9",
			cursor:     5,
			key:        tea.KeyMsg{Type: tea.KeyCtrlU},
			wantValue:  "E9",
			wantCursor: 0,
		},
		{
			name:       "ctrl+w deletes previous word",
			value:      "goto E9",
			cursor:     len("goto E9"),
			key:        tea.KeyMsg{Type: tea.KeyCtrlW},
			wantValue:  "goto ",
			wantCursor: len("goto "),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newPendingCommandModel(tc.value, tc.cursor)

			got, cmd := updateKey(t, m, tc.key)

			if cmd == nil {
				t.Fatal("expected cursor blink restart command")
			}
			if got.commandBuffer != tc.wantValue {
				t.Fatalf("expected value %q, got %q", tc.wantValue, got.commandBuffer)
			}
			if got.commandCursor != tc.wantCursor {
				t.Fatalf("expected cursor %d, got %d", tc.wantCursor, got.commandCursor)
			}
			if got.editCursor.Blink {
				t.Fatal("expected command cursor to restart in visible phase")
			}
		})
	}
}

func TestCommandModeInsertsAtCursor(t *testing.T) {
	m := newPendingCommandModel("goto 9", len("goto "))

	got, cmd := updateKey(t, m, runeKey("E"))
	if cmd == nil {
		t.Fatal("expected cursor blink restart command")
	}
	if got.commandBuffer != "goto E9" {
		t.Fatalf("expected mid-buffer insertion, got %q", got.commandBuffer)
	}
	if got.commandCursor != len("goto E") {
		t.Fatalf("expected cursor at %d, got %d", len("goto E"), got.commandCursor)
	}
}

func TestNormalModeGGoesToA1(t *testing.T) {
	m := newModel()
	m.mode = normalMode
	m.selectedRow = 5
	m.selectedCol = 2

	got := applyKeys(t, m, runeKey("g"), runeKey("g"))

	if got.gotoPending {
		t.Fatal("expected goto command to finish after gg")
	}
	assertSelection(t, got, 0, 0)
}

func TestNormalModeGoesToTypedCellReference(t *testing.T) {
	m := newModel()
	m.mode = normalMode

	got := applyKeys(t, m, runeKey("g"), runeKey("E"), runeKey("9"))

	if got.selectedRow != 8 || got.selectedCol != 4 {
		t.Fatalf("expected selection at E9, got (%d,%d)", got.selectedRow, got.selectedCol)
	}
}

func TestNormalModeGoesToBottomWithUppercaseG(t *testing.T) {
	testCases := []struct {
		name    string
		row     int
		col     int
		wantRow int
		wantCol int
	}{
		{
			name:    "from earlier row keeps column",
			row:     5,
			col:     2,
			wantRow: defaultRows - 1,
			wantCol: 2,
		},
		{
			name:    "from last row goes to last column",
			row:     defaultRows - 1,
			col:     2,
			wantRow: defaultRows - 1,
			wantCol: totalCols - 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel()
			m.mode = normalMode
			m.selectedRow = tc.row
			m.selectedCol = tc.col

			updated, _ := m.Update(runeKey("G"))
			got, ok := updated.(model)
			if !ok {
				t.Fatal("expected updated model")
			}

			if got.selectedRow != tc.wantRow || got.selectedCol != tc.wantCol {
				t.Fatalf("expected selection (%d,%d), got (%d,%d)", tc.wantRow, tc.wantCol, got.selectedRow, got.selectedCol)
			}
		})
	}
}

func TestNormalModeCtrlDAndCtrlUMoveHalfPage(t *testing.T) {
	testCases := []struct {
		name          string
		key           tea.KeyMsg
		startRow      int
		startOffset   int
		wantRow       int
		wantRowOffset int
	}{
		{
			name:          "ctrl+d moves down by half page",
			key:           tea.KeyMsg{Type: tea.KeyCtrlD},
			startRow:      10,
			startOffset:   8,
			wantRow:       13,
			wantRowOffset: 11,
		},
		{
			name:          "ctrl+u moves up by half page",
			key:           tea.KeyMsg{Type: tea.KeyCtrlU},
			startRow:      10,
			startOffset:   8,
			wantRow:       7,
			wantRowOffset: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel()
			m.mode = normalMode
			m.height = 16
			m.selectedRow = tc.startRow
			m.selectedCol = 2
			m.rowOffset = tc.startOffset

			got := applyKey(t, m, tc.key)

			if got.selectedRow != tc.wantRow || got.selectedCol != 2 {
				t.Fatalf("expected selection (%d,%d), got (%d,%d)", tc.wantRow, 2, got.selectedRow, got.selectedCol)
			}
			if got.rowOffset != tc.wantRowOffset {
				t.Fatalf("expected row offset %d, got %d", tc.wantRowOffset, got.rowOffset)
			}
		})
	}
}

func TestSelectModeEntryAndEscape(t *testing.T) {
	m := newModel()
	m.selectedRow = 3
	m.selectedCol = 2

	selected := applyKey(t, m, runeKey("v"))
	if selected.mode != selectMode {
		t.Fatalf("expected select mode, got %q", selected.mode)
	}
	if selected.selectRow != 3 || selected.selectCol != 2 {
		t.Fatalf("expected selection anchor (3,2), got (%d,%d)", selected.selectRow, selected.selectCol)
	}

	normal := applyKey(t, selected, tea.KeyMsg{Type: tea.KeyEscape})
	if normal.mode != normalMode {
		t.Fatalf("expected normal mode after escape, got %q", normal.mode)
	}
}

func TestSelectModeTracksRectangularSelection(t *testing.T) {
	m := newModel()
	m.selectedRow = 1
	m.selectedCol = 1

	selected := applyKey(t, m, runeKey("v"))
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyRight})
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyDown})

	for _, cell := range []cellKey{
		{row: 1, col: 1},
		{row: 1, col: 2},
		{row: 2, col: 1},
		{row: 2, col: 2},
	} {
		if !selected.selectionContains(cell.row, cell.col) {
			t.Fatalf("expected cell (%d,%d) to be selected", cell.row, cell.col)
		}
	}

	for _, cell := range []cellKey{
		{row: 0, col: 1},
		{row: 2, col: 3},
	} {
		if selected.selectionContains(cell.row, cell.col) {
			t.Fatalf("did not expect cell (%d,%d) to be selected", cell.row, cell.col)
		}
	}
}

func TestSelectModeCtrlDAndCtrlUMoveHalfPage(t *testing.T) {
	testCases := []struct {
		name          string
		key           tea.KeyMsg
		startRow      int
		startOffset   int
		wantRow       int
		wantRowOffset int
	}{
		{
			name:          "ctrl+d moves down by half page",
			key:           tea.KeyMsg{Type: tea.KeyCtrlD},
			startRow:      10,
			startOffset:   8,
			wantRow:       13,
			wantRowOffset: 11,
		},
		{
			name:          "ctrl+u moves up by half page",
			key:           tea.KeyMsg{Type: tea.KeyCtrlU},
			startRow:      10,
			startOffset:   8,
			wantRow:       7,
			wantRowOffset: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel()
			m.height = 16
			m.selectedRow = tc.startRow
			m.selectedCol = 2
			m.rowOffset = tc.startOffset

			selected := applyKey(t, m, runeKey("v"))
			got := applyKey(t, selected, tc.key)

			if got.mode != selectMode {
				t.Fatalf("expected select mode, got %q", got.mode)
			}
			if got.selectedRow != tc.wantRow || got.selectedCol != 2 {
				t.Fatalf("expected selection (%d,%d), got (%d,%d)", tc.wantRow, 2, got.selectedRow, got.selectedCol)
			}
			if got.rowOffset != tc.wantRowOffset {
				t.Fatalf("expected row offset %d, got %d", tc.wantRowOffset, got.rowOffset)
			}
			if got.selectRow != tc.startRow || got.selectCol != 2 {
				t.Fatalf("expected selection anchor (%d,%d), got (%d,%d)", tc.startRow, 2, got.selectRow, got.selectCol)
			}
		})
	}
}

func TestSelectModeHighlightsSelectionBordersWithoutTouchingRowLabel(t *testing.T) {
	m := newModel()
	m.selectedRow = 1
	m.selectedCol = 1

	selected := applyKey(t, m, runeKey("v"))
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyRight})
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyDown})

	selectedLine := selected.renderContentLine(1, 3)
	wantSelectedPrefix := selected.activeRowStyle.Render(fitLeft("2", selected.rowLabelWidth)) + " " + selected.gridStyle.Render("│")
	if !strings.HasPrefix(selectedLine, wantSelectedPrefix) {
		t.Fatalf("expected selected row label to remain unhighlighted:\nwant prefix=%q\ngot line=%q", wantSelectedPrefix, selectedLine)
	}

	if !strings.Contains(selectedLine, selected.selectBorderStyle.Render("│")) {
		t.Fatalf("expected selected row to render blue selection borders:\nline=%q", selectedLine)
	}
}

func TestSelectModeKeepsAllSelectedRowLabelsWhite(t *testing.T) {
	m := newModel()
	m.selectedRow = 1
	m.selectedCol = 1

	selected := applyKey(t, m, runeKey("v"))
	selected = applyKey(t, selected, runeKey("j"))

	rowOne := selected.renderContentLine(1, 3)
	rowTwo := selected.renderContentLine(2, 3)
	whiteRowOne := selected.activeRowStyle.Render(fitLeft("2", selected.rowLabelWidth))
	whiteRowTwo := selected.activeRowStyle.Render(fitLeft("3", selected.rowLabelWidth))
	if !strings.HasPrefix(rowOne, whiteRowOne) {
		t.Fatalf("expected first selected row label to be white:\nline=%q", rowOne)
	}
	if !strings.HasPrefix(rowTwo, whiteRowTwo) {
		t.Fatalf("expected second selected row label to be white:\nline=%q", rowTwo)
	}
}

func TestSelectModeHighlightsConnectedSelectionBorders(t *testing.T) {
	m := newModel()
	m.selectedRow = 1
	m.selectedCol = 1

	selected := applyKey(t, m, runeKey("v"))
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyRight})
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyDown})

	testCases := []struct {
		name      string
		borderRow int
		borderCol int
		wantGlyph string
		wantOK    bool
	}{
		{name: "top left corner", borderRow: 1, borderCol: 1, wantGlyph: "┌", wantOK: true},
		{name: "top middle junction", borderRow: 1, borderCol: 2, wantGlyph: "┬", wantOK: true},
		{name: "top right corner", borderRow: 1, borderCol: 3, wantGlyph: "┐", wantOK: true},
		{name: "center junction", borderRow: 2, borderCol: 2, wantGlyph: "┼", wantOK: true},
		{name: "left edge through middle border", borderRow: 2, borderCol: 1, wantGlyph: "├", wantOK: true},
		{name: "right edge through middle border", borderRow: 2, borderCol: 3, wantGlyph: "┤", wantOK: true},
		{name: "outside selection", borderRow: 1, borderCol: 0, wantGlyph: "", wantOK: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotGlyph, gotOK := selected.selectionBorderJunction(tc.borderRow, tc.borderCol)
			if gotOK != tc.wantOK || gotGlyph != tc.wantGlyph {
				t.Fatalf("expected (%q, %t), got (%q, %t)", tc.wantGlyph, tc.wantOK, gotGlyph, gotOK)
			}
		})
	}

	if !selected.selectionHorizontalBorderHighlighted(2, 1) {
		t.Fatal("expected interior horizontal border segment to be highlighted")
	}
	if !selected.selectionVerticalBorderHighlighted(1, 2) {
		t.Fatal("expected interior vertical border segment to be highlighted")
	}
}

func TestSelectModeBordersUseThemeAccentForeground(t *testing.T) {
	m := newModel()

	if got, want := m.selectBorderStyle.GetForeground(), lipgloss.Color("#2F66C7"); got != want {
		t.Fatalf("expected selected border foreground %v, got %v", want, got)
	}
	if got, want := m.selectBorderStyle.GetBackground(), m.selectCellStyle.GetBackground(); got != want {
		t.Fatalf("expected selected border background %v, got %v", want, got)
	}
}

func TestSelectModeUsesReverseVideoBackground(t *testing.T) {
	m := newModel()

	if got, want := m.selectCellStyle.GetBackground(), lipgloss.Color("#264F78"); got != want {
		t.Fatalf("expected selected cell background %v, got %v", want, got)
	}
	if got, want := m.selectRowStyle.GetBackground(), m.selectCellStyle.GetBackground(); got != want {
		t.Fatalf("expected selected row background %v to match selected cell background %v", got, want)
	}
	if got, want := m.selectBorderStyle.GetBackground(), m.selectCellStyle.GetBackground(); got != want {
		t.Fatalf("expected selected border background %v to match selected cell background %v", got, want)
	}
}

func TestSelectModeUsesBrightCursorCellInsideSelection(t *testing.T) {
	m := newModel()
	m.selectedRow = 1
	m.selectedCol = 1

	selected := applyKey(t, m, runeKey("v"))
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyRight})
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyDown})

	line := selected.renderContentLine(1, 3)
	active := selected.selectActiveCellStyle.Render(fit(selected.displayValue(1, 1), selected.cellWidth))
	other := selected.selectCellStyle.Render(fit(selected.displayValue(1, 2), selected.cellWidth))
	if !strings.Contains(line, active) {
		t.Fatalf("expected visual cursor cell to use active selection style:\nline=%q", line)
	}
	if !strings.Contains(line, other) {
		t.Fatalf("expected surrounding selected cell to keep selection style:\nline=%q", line)
	}
}

func TestSelectModeKeepsSelectedColumnLabelsWhite(t *testing.T) {
	m := newModel()
	m.width = 80
	m.selectedRow = 1
	m.selectedCol = 1

	selected := applyKey(t, m, runeKey("v"))
	selected = applyKey(t, selected, runeKey("l"))
	headers := selected.renderColumnHeaders()

	if !strings.Contains(headers, selected.activeHeaderStyle.Render(alignCenter("B", selected.cellWidth))) {
		t.Fatalf("expected B header to be white in visual selection:\nheaders=%q", headers)
	}
	if !strings.Contains(headers, selected.activeHeaderStyle.Render(alignCenter("C", selected.cellWidth))) {
		t.Fatalf("expected C header to be white in visual selection:\nheaders=%q", headers)
	}
}

func TestGridBordersUseSofterGray(t *testing.T) {
	m := newModel()

	if got, want := m.gridStyle.GetForeground(), lipgloss.Color("8"); got != want {
		t.Fatalf("expected grid border foreground %v, got %v", want, got)
	}
}

func TestSelectModeGoesToBottomWithUppercaseG(t *testing.T) {
	testCases := []struct {
		name    string
		row     int
		col     int
		wantRow int
		wantCol int
	}{
		{
			name:    "from earlier row keeps column",
			row:     5,
			col:     2,
			wantRow: defaultRows - 1,
			wantCol: 2,
		},
		{
			name:    "from last row goes to last column",
			row:     defaultRows - 1,
			col:     2,
			wantRow: defaultRows - 1,
			wantCol: totalCols - 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel()
			m.selectedRow = tc.row
			m.selectedCol = tc.col

			selected := applyKey(t, m, runeKey("v"))
			got := applyKey(t, selected, runeKey("G"))

			if got.mode != selectMode {
				t.Fatalf("expected select mode, got %q", got.mode)
			}
			if got.selectedRow != tc.wantRow || got.selectedCol != tc.wantCol {
				t.Fatalf("expected selection (%d,%d), got (%d,%d)", tc.wantRow, tc.wantCol, got.selectedRow, got.selectedCol)
			}
			if got.selectRow != tc.row || got.selectCol != tc.col {
				t.Fatalf("expected selection anchor (%d,%d), got (%d,%d)", tc.row, tc.col, got.selectRow, got.selectCol)
			}
		})
	}
}

func TestSelectModeGoesToTypedCellReference(t *testing.T) {
	m := newModel()
	m.selectedRow = 2
	m.selectedCol = 1

	selected := applyKey(t, m, runeKey("v"))
	got := applyKey(t, selected, runeKey("g"))
	got = applyKey(t, got, runeKey("E"))
	got = applyKey(t, got, runeKey("9"))

	if got.mode != selectMode {
		t.Fatalf("expected select mode, got %q", got.mode)
	}
	if got.selectedRow != 8 || got.selectedCol != 4 {
		t.Fatalf("expected selection at E9, got (%d,%d)", got.selectedRow, got.selectedCol)
	}
	if got.selectRow != 2 || got.selectCol != 1 {
		t.Fatalf("expected selection anchor to stay at (2,1), got (%d,%d)", got.selectRow, got.selectCol)
	}
}

func TestSelectModeYankAndPasteRectangularRange(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "A")
	m.setCellValue(0, 1, "B")
	m.setCellValue(1, 0, "C")
	m.setCellValue(1, 1, "D")
	m.selectedRow = 0
	m.selectedCol = 0

	selected := applyKey(t, m, runeKey("v"))
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyRight})
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyDown})
	normal := applyKey(t, selected, runeKey("y"))

	if normal.mode != normalMode {
		t.Fatalf("expected normal mode after yank, got %q", normal.mode)
	}
	if !normal.hasCopyBuffer {
		t.Fatal("expected copy buffer after yank")
	}
	if len(normal.copyBuffer.cells) != 2 || len(normal.copyBuffer.cells[0]) != 2 {
		t.Fatalf("expected 2x2 copy buffer, got %#v", normal.copyBuffer.cells)
	}

	normal.selectedRow = 3
	normal.selectedCol = 3
	pasted := applyKey(t, normal, runeKey("p"))

	if got := pasted.cellValue(3, 3); got != "A" {
		t.Fatalf("expected A at D4, got %q", got)
	}
	if got := pasted.cellValue(3, 4); got != "B" {
		t.Fatalf("expected B at E4, got %q", got)
	}
	if got := pasted.cellValue(4, 3); got != "C" {
		t.Fatalf("expected C at D5, got %q", got)
	}
	if got := pasted.cellValue(4, 4); got != "D" {
		t.Fatalf("expected D at E5, got %q", got)
	}
}

func TestSelectModeShiftYYanksSelectionReference(t *testing.T) {
	m := newModel()
	m.selectedRow = 0
	m.selectedCol = 0

	selected := applyKey(t, m, runeKey("v"))
	for range 5 {
		selected = applyKey(t, selected, runeKey("j"))
	}
	normal := applyKey(t, selected, runeKey("Y"))

	if normal.mode != normalMode {
		t.Fatalf("expected normal mode after Y, got %q", normal.mode)
	}
	if !normal.hasCopyBuffer {
		t.Fatal("expected copy buffer after Y")
	}
	if got, want := len(normal.copyBuffer.cells), 1; got != want {
		t.Fatalf("expected single-cell clipboard after range yank, got %d rows", got)
	}
	if got, want := normal.copyBuffer.cells[0][0], "A1:A6"; got != want {
		t.Fatalf("expected Y to yank textual range %q, got %q", want, got)
	}

	normal.selectedRow = 2
	normal.selectedCol = 2
	pasted := applyKey(t, normal, runeKey("p"))
	if got, want := pasted.cellValue(2, 2), "A1:A6"; got != want {
		t.Fatalf("expected range reference %q to paste into C3, got %q", want, got)
	}
}

func TestSelectModeEqualsStartsFormulaTemplateBelowSelection(t *testing.T) {
	m := newModel()
	m.selectedRow = 0
	m.selectedCol = 0

	selected := applyKey(t, m, runeKey("v"))
	for range 5 {
		selected = applyKey(t, selected, runeKey("j"))
	}
	inserting := applyKey(t, selected, runeKey("="))

	if inserting.mode != insertMode {
		t.Fatalf("expected insert mode after =, got %q", inserting.mode)
	}
	if inserting.selectedRow != 6 || inserting.selectedCol != 0 {
		t.Fatalf("expected formula template target at A7, got (%d,%d)", inserting.selectedRow, inserting.selectedCol)
	}
	if got, want := inserting.editingValue, "=(A1:A6)"; got != want {
		t.Fatalf("expected formula template %q, got %q", want, got)
	}
	if got, want := inserting.editingCursor, 1; got != want {
		t.Fatalf("expected cursor after = at %d, got %d", want, got)
	}

	inserting = applyKey(t, inserting, runeKey("SUM"))
	committed := applyKey(t, inserting, tea.KeyMsg{Type: tea.KeyEscape})
	if got, want := committed.cellValue(6, 0), "=SUM(A1:A6)"; got != want {
		t.Fatalf("expected committed formula %q, got %q", want, got)
	}
}

func TestPasteAdjustsCopiedFormulaReferencesAcrossColumns(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "1")
	m.setCellValue(0, 1, "=A1")
	m.selectedRow = 0
	m.selectedCol = 1

	copied := applyKey(t, m, runeKey("y"))
	copied.selectedCol = 2
	pasted := applyKey(t, copied, runeKey("p"))

	if got, want := pasted.cellValue(0, 2), "=B1"; got != want {
		t.Fatalf("expected pasted formula %q, got %q", want, got)
	}
	if got, want := pasted.displayValue(0, 2), "=1"; got != want {
		t.Fatalf("expected pasted display value %q, got %q", want, got)
	}
}

func TestPasteAdjustsCopiedFormulaReferencesAcrossRows(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "1")
	m.setCellValue(1, 0, "=A1")
	m.selectedRow = 1
	m.selectedCol = 0

	copied := applyKey(t, m, runeKey("y"))
	copied.selectedRow = 2
	pasted := applyKey(t, copied, runeKey("p"))

	if got, want := pasted.cellValue(2, 0), "=A2"; got != want {
		t.Fatalf("expected pasted formula %q, got %q", want, got)
	}
}

func TestPasteAdjustsFormulaReferencesForRectangularSelections(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "1")
	m.setCellValue(0, 1, "=A1")
	m.setCellValue(1, 0, "2")
	m.setCellValue(1, 1, "=A2")
	m.selectedRow = 0
	m.selectedCol = 0

	selected := applyKey(t, m, runeKey("v"))
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyRight})
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyDown})
	copied := applyKey(t, selected, runeKey("y"))
	copied.selectedRow = 0
	copied.selectedCol = 2
	pasted := applyKey(t, copied, runeKey("p"))

	if got, want := pasted.cellValue(0, 2), "1"; got != want {
		t.Fatalf("expected pasted value %q at C1, got %q", want, got)
	}
	if got, want := pasted.cellValue(0, 3), "=C1"; got != want {
		t.Fatalf("expected pasted formula %q at D1, got %q", want, got)
	}
	if got, want := pasted.cellValue(1, 2), "2"; got != want {
		t.Fatalf("expected pasted value %q at C2, got %q", want, got)
	}
	if got, want := pasted.cellValue(1, 3), "=C2"; got != want {
		t.Fatalf("expected pasted formula %q at D2, got %q", want, got)
	}
}

func TestPasteRewritesOutOfBoundsReferencesToRefError(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "1")
	m.setCellValue(0, 1, "=A1")
	m.selectedRow = 0
	m.selectedCol = 1

	copied := applyKey(t, m, runeKey("y"))
	copied.selectedCol = 0
	pasted := applyKey(t, copied, runeKey("p"))

	if got, want := pasted.cellValue(0, 0), "=#REF!"; got != want {
		t.Fatalf("expected pasted formula %q, got %q", want, got)
	}
	if got, want := pasted.displayValue(0, 0), "="+formulaErrorDisplay; got != want {
		t.Fatalf("expected pasted display error %q, got %q", want, got)
	}
}

func TestUndoRestoresLastCommittedEdit(t *testing.T) {
	m := newModel()
	m.selectedRow = 0
	m.selectedCol = 0
	m.setCellValue(0, 0, "before")
	m.mode = insertMode
	m.editingValue = "after"

	normal := applyKey(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	if got, want := normal.cellValue(0, 0), "after"; got != want {
		t.Fatalf("expected committed value %q, got %q", want, got)
	}

	undone := applyKey(t, normal, runeKey("u"))
	if got, want := undone.cellValue(0, 0), "before"; got != want {
		t.Fatalf("expected undone value %q, got %q", want, got)
	}
}

func TestUndoRestoresLastPaste(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "A")
	m.selectedRow = 0
	m.selectedCol = 0

	copied := applyKey(t, m, runeKey("y"))
	copied.selectedRow = 1
	copied.selectedCol = 1
	pasted := applyKey(t, copied, runeKey("p"))
	if got, want := pasted.cellValue(1, 1), "A"; got != want {
		t.Fatalf("expected pasted value %q, got %q", want, got)
	}

	undone := applyKey(t, pasted, runeKey("u"))
	if got := undone.cellValue(1, 1); got != "" {
		t.Fatalf("expected paste to be undone, got %q", got)
	}
}

func TestUndoRestoresLastCut(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "A")
	m.selectedRow = 0
	m.selectedCol = 0

	cut := applyKey(t, m, runeKey("x"))
	if got := cut.cellValue(0, 0); got != "" {
		t.Fatalf("expected cut cell to be empty, got %q", got)
	}

	undone := applyKey(t, cut, runeKey("u"))
	if got, want := undone.cellValue(0, 0), "A"; got != want {
		t.Fatalf("expected cut to be undone to %q, got %q", want, got)
	}
}

func TestUndoWithoutHistoryDoesNothing(t *testing.T) {
	m := newModel()
	m.selectedRow = 2
	m.selectedCol = 3

	undone := applyKey(t, m, runeKey("u"))
	if undone.selectedRow != 2 || undone.selectedCol != 3 {
		t.Fatalf("expected selection to stay at (2,3), got (%d,%d)", undone.selectedRow, undone.selectedCol)
	}
}

func TestCtrlRRedoesLastUndoneEdit(t *testing.T) {
	m := newModel()
	m.selectedRow = 0
	m.selectedCol = 0
	m.setCellValue(0, 0, "before")
	m.mode = insertMode
	m.editingValue = "after"

	normal := applyKey(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	undone := applyKey(t, normal, runeKey("u"))
	redone := applyKey(t, undone, tea.KeyMsg{Type: tea.KeyCtrlR})

	if got, want := redone.cellValue(0, 0), "after"; got != want {
		t.Fatalf("expected redone value %q, got %q", want, got)
	}
}

func TestUppercaseURedoesLastUndoneEdit(t *testing.T) {
	m := newModel()
	m.selectedRow = 0
	m.selectedCol = 0
	m.setCellValue(0, 0, "before")
	m.mode = insertMode
	m.editingValue = "after"

	normal := applyKey(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	undone := applyKey(t, normal, runeKey("u"))
	redone := applyKey(t, undone, runeKey("U"))

	if got, want := redone.cellValue(0, 0), "after"; got != want {
		t.Fatalf("expected redone value %q, got %q", want, got)
	}
}

func TestRedoWithoutHistoryDoesNothing(t *testing.T) {
	m := newModel()
	m.selectedRow = 1
	m.selectedCol = 1

	redone := applyKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlR})
	if redone.selectedRow != 1 || redone.selectedCol != 1 {
		t.Fatalf("expected selection to stay at (1,1), got (%d,%d)", redone.selectedRow, redone.selectedCol)
	}
}

func TestNewChangeClearsRedoHistory(t *testing.T) {
	m := newModel()
	m.selectedRow = 0
	m.selectedCol = 0
	m.setCellValue(0, 0, "first")
	m.mode = insertMode
	m.editingValue = "second"

	normal := applyKey(t, m, tea.KeyMsg{Type: tea.KeyEscape})
	undone := applyKey(t, normal, runeKey("u"))

	undone.mode = insertMode
	undone.editingValue = "third"
	replaced := applyKey(t, undone, tea.KeyMsg{Type: tea.KeyEscape})
	redone := applyKey(t, replaced, tea.KeyMsg{Type: tea.KeyCtrlR})

	if got, want := redone.cellValue(0, 0), "third"; got != want {
		t.Fatalf("expected redo history to be cleared and value to stay %q, got %q", want, got)
	}
}

func TestSelectModeCutClearsAndPastesRectangularRange(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "A")
	m.setCellValue(0, 1, "B")
	m.setCellValue(1, 0, "C")
	m.setCellValue(1, 1, "D")
	m.selectedRow = 0
	m.selectedCol = 0

	selected := applyKey(t, m, runeKey("v"))
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyRight})
	selected = applyKey(t, selected, tea.KeyMsg{Type: tea.KeyDown})
	normal := applyKey(t, selected, runeKey("x"))

	for _, cell := range []cellKey{
		{row: 0, col: 0},
		{row: 0, col: 1},
		{row: 1, col: 0},
		{row: 1, col: 1},
	} {
		if got := normal.cellValue(cell.row, cell.col); got != "" {
			t.Fatalf("expected source cell (%d,%d) to be cleared, got %q", cell.row, cell.col, got)
		}
	}

	normal.selectedRow = 2
	normal.selectedCol = 1
	pasted := applyKey(t, normal, runeKey("p"))

	if got := pasted.cellValue(2, 1); got != "A" {
		t.Fatalf("expected A at B3, got %q", got)
	}
	if got := pasted.cellValue(2, 2); got != "B" {
		t.Fatalf("expected B at C3, got %q", got)
	}
	if got := pasted.cellValue(3, 1); got != "C" {
		t.Fatalf("expected C at B4, got %q", got)
	}
	if got := pasted.cellValue(3, 2); got != "D" {
		t.Fatalf("expected D at C4, got %q", got)
	}
}

func TestEditCursorMatchesInsertModeColor(t *testing.T) {
	m := newModel()

	if got, want := m.editCursor.Style.GetForeground(), m.statusInsertStyle.GetBackground(); got != want {
		t.Fatalf("expected edit cursor color %v to match insert mode color %v", got, want)
	}
}

func TestStatusModeColorsUseLegacyInsertAndNormalColors(t *testing.T) {
	m := newModel()

	if got, want := m.statusInsertStyle.GetBackground(), lipgloss.Color("#D79921"); got != want {
		t.Fatalf("expected insert status color %v, got %v", want, got)
	}
	if got, want := m.statusNormalStyle.GetBackground(), lipgloss.Color("33"); got != want {
		t.Fatalf("expected normal status color %v, got %v", want, got)
	}
	if got, want := m.statusSelectStyle.GetBackground(), lipgloss.Color("13"); got != want {
		t.Fatalf("expected visual status color %v, got %v", want, got)
	}
}

func TestViewRendersStatusBarLast(t *testing.T) {
	m := newModel()
	m.width = 80
	m.height = 24

	view := m.View()
	statusBar := m.renderStatusBar()
	if !strings.HasSuffix(view, statusBar) {
		t.Fatalf("expected view to end with status bar:\nview=%q\nstatus=%q", view, statusBar)
	}
}

func TestViewLeavesBlankLineBeforeStatusBar(t *testing.T) {
	m := newModel()
	m.width = 80
	m.height = 24

	view := m.View()
	statusBar := m.renderStatusBar()
	lines := strings.Split(view, "\n")
	if len(lines) != m.height {
		t.Fatalf("expected view height %d, got %d", m.height, len(lines))
	}
	if lines[len(lines)-1] != statusBar || strings.TrimSpace(lines[len(lines)-2]) != "" {
		t.Fatalf("expected blank line before status bar:\nview=%q\nstatus=%q", view, statusBar)
	}
}

func TestViewUsesBottomBarForActiveCommandPrompt(t *testing.T) {
	m := newModel()
	m.width = 80
	m.height = 24
	m.mode = commandMode
	m.commandPending = true
	m.commandBuffer = "write"
	m.commandCursor = len("write")
	m.editCursor.Focus()

	view := m.View()
	lines := strings.Split(view, "\n")
	if len(lines) != m.height {
		t.Fatalf("expected view height %d, got %d", m.height, len(lines))
	}
	if !strings.Contains(lines[len(lines)-1], ":write") {
		t.Fatalf("expected active command prompt on bottom line, got %q", lines[len(lines)-1])
	}
	if strings.TrimSpace(lines[len(lines)-2]) != "" {
		t.Fatalf("expected no separate command line above active prompt, got %q", lines[len(lines)-2])
	}
}

func TestNormalModeCountedMotionsAndRowDelete(t *testing.T) {
	m := newModel()
	m.height = 16
	m.selectedRow = 1
	m.selectedCol = 1
	m.setCellValue(1, 1, "keep")
	m.setCellValue(2, 1, "drop-1")
	m.setCellValue(3, 1, "drop-2")
	m.setCellValue(4, 1, "keep-2")

	got := applyKey(t, m, runeKey("2"))
	got = applyKey(t, got, runeKey("j"))
	if got.selectedRow != 3 || got.selectedCol != 1 {
		t.Fatalf("expected 2j to land on row 4, got (%d,%d)", got.selectedRow, got.selectedCol)
	}

	got = applyKey(t, got, runeKey("2"))
	got = applyKey(t, got, runeKey("d"))
	got = applyKey(t, got, runeKey("d"))
	if got.cellValue(3, 1) != "" {
		t.Fatalf("expected rows to shift after 2dd, got %q in original row", got.cellValue(3, 1))
	}
	if got.cellValue(2, 1) != "drop-1" {
		t.Fatalf("expected unaffected rows above the deletion to stay put, got %q", got.cellValue(2, 1))
	}
}

func TestNormalModeDotRepeatsInsertAndPaste(t *testing.T) {
	m := newModel()
	m.selectedRow = 0
	m.selectedCol = 0

	inserted := applyKey(t, m, runeKey("i"))
	inserted = applyKey(t, inserted, runeKey("a"))
	inserted = applyKey(t, inserted, runeKey("b"))
	inserted = applyKey(t, inserted, runeKey("c"))
	inserted = applyKey(t, inserted, tea.KeyMsg{Type: tea.KeyEscape})
	inserted.selectedCol = 1

	repeatedInsert := applyKey(t, inserted, runeKey("."))
	if got, want := repeatedInsert.cellValue(0, 1), "abc"; got != want {
		t.Fatalf("expected dot to repeat insert into B1 as %q, got %q", want, got)
	}

	yanked := applyKey(t, repeatedInsert, runeKey("y"))
	yanked.selectedCol = 2
	pasted := applyKey(t, yanked, runeKey("p"))
	pasted.selectedCol = 3
	repeatedPaste := applyKey(t, pasted, runeKey("."))
	if got, want := repeatedPaste.cellValue(0, 2), "abc"; got != want {
		t.Fatalf("expected paste to write into C1 as %q, got %q", want, got)
	}
	if got, want := repeatedPaste.cellValue(0, 3), "abc"; got != want {
		t.Fatalf("expected dot to repeat paste into D1 as %q, got %q", want, got)
	}
}

func TestSearchPromptsAndRepeatNavigation(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "alpha")
	m.setCellValue(1, 0, "beta")
	m.setCellValue(2, 0, "alpha again")
	m.selectedRow = 0
	m.selectedCol = 0

	pending := applyKey(t, m, runeKey("/"))
	if pending.mode != commandMode || pending.promptKind != searchForwardPrompt {
		t.Fatalf("expected / to start search prompt, got mode=%q prompt=%q", pending.mode, pending.promptKind)
	}
	pending = applyKey(t, pending, runeKey("alpha"))
	found := applyKey(t, pending, tea.KeyMsg{Type: tea.KeyEnter})
	if found.selectedRow != 2 || found.selectedCol != 0 {
		t.Fatalf("expected /alpha to jump to A3, got (%d,%d)", found.selectedRow, found.selectedCol)
	}

	repeated := applyKey(t, found, runeKey("n"))
	if repeated.selectedRow != 0 || repeated.selectedCol != 0 {
		t.Fatalf("expected n to wrap to A1, got (%d,%d)", repeated.selectedRow, repeated.selectedCol)
	}

	backwardPrompt := applyKey(t, repeated, runeKey("?"))
	backwardPrompt = applyKey(t, backwardPrompt, runeKey("beta"))
	backward := applyKey(t, backwardPrompt, tea.KeyMsg{Type: tea.KeyEnter})
	if backward.selectedRow != 1 || backward.selectedCol != 0 {
		t.Fatalf("expected ?beta to jump backward to A2, got (%d,%d)", backward.selectedRow, backward.selectedCol)
	}
}

func TestCommandPromptAliasesTrackCurrentFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sheet.csv")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatalf("expected temp CSV write to succeed, got %v", err)
	}

	m := newModel()
	if err := m.loadCSVFile(path); err != nil {
		t.Fatalf("expected CSV load to succeed, got %v", err)
	}
	m.setCellValue(0, 0, "updated")

	pending := applyKey(t, m, runeKey(":"))
	written := applyKey(t, pending, runeKey("w"))
	written = applyKey(t, written, tea.KeyMsg{Type: tea.KeyEnter})
	if written.commandError {
		t.Fatalf("expected :w alias to succeed, got %q", written.commandMessage)
	}

	reloaded := applyKey(t, written, runeKey(":"))
	reloaded = applyKey(t, reloaded, runeKey("e"))
	reloaded = applyKey(t, reloaded, tea.KeyMsg{Type: tea.KeyEnter})
	if got, want := reloaded.cellValue(0, 0), "updated"; got != want {
		t.Fatalf("expected :e to reload current file value %q, got %q", want, got)
	}

	quitPending := applyKey(t, reloaded, runeKey(":"))
	quitPending = applyKey(t, quitPending, runeKey("wq"))
	if _, cmd := quitPending.Update(tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil {
		t.Fatal("expected :wq alias to return quit command")
	}
}

func TestMarksAndJumpListNavigation(t *testing.T) {
	m := newModel()
	m.selectedRow = 4
	m.selectedCol = 2

	marked := applyKey(t, m, runeKey("m"))
	marked = applyKey(t, marked, runeKey("a"))
	marked.selectedRow = 1
	marked.selectedCol = 1

	jumped := applyKey(t, marked, runeKey("'"))
	jumped = applyKey(t, jumped, runeKey("a"))
	if jumped.selectedRow != 4 || jumped.selectedCol != 2 {
		t.Fatalf("expected 'a to jump back to C5, got (%d,%d)", jumped.selectedRow, jumped.selectedCol)
	}

	back := applyKey(t, jumped, tea.KeyMsg{Type: tea.KeyCtrlO})
	if back.selectedRow != 1 || back.selectedCol != 1 {
		t.Fatalf("expected ctrl+o to jump back to B2, got (%d,%d)", back.selectedRow, back.selectedCol)
	}

	forward := applyKey(t, back, tea.KeyMsg{Type: tea.KeyCtrlI})
	if forward.selectedRow != 4 || forward.selectedCol != 2 {
		t.Fatalf("expected ctrl+i to jump forward to C5, got (%d,%d)", forward.selectedRow, forward.selectedCol)
	}
}

func TestRegistersBlackHoleAndNumberedDeletes(t *testing.T) {
	m := newModel()
	m.setCellValue(0, 0, "alpha")
	m.setCellValue(0, 1, "beta")
	m.selectedRow = 0
	m.selectedCol = 0

	named := applyKey(t, m, runeKey(`"`))
	named = applyKey(t, named, runeKey("a"))
	named = applyKey(t, named, runeKey("y"))
	named.selectedCol = 2
	named = applyKey(t, named, runeKey(`"`))
	named = applyKey(t, named, runeKey("a"))
	named = applyKey(t, named, runeKey("p"))
	if got, want := named.cellValue(0, 2), "alpha"; got != want {
		t.Fatalf("expected named register paste %q into C1, got %q", want, got)
	}

	named.selectedCol = 1
	blackHole := applyKey(t, named, runeKey(`"`))
	blackHole = applyKey(t, blackHole, runeKey("_"))
	blackHole = applyKey(t, blackHole, runeKey("x"))
	blackHole.selectedCol = 3
	blackHole = applyKey(t, blackHole, runeKey("p"))
	if got, want := blackHole.cellValue(0, 3), "alpha"; got != want {
		t.Fatalf("expected black-hole delete to preserve unnamed register %q, got %q", want, got)
	}

	blackHole.selectedCol = 2
	numbered := applyKey(t, blackHole, runeKey("x"))
	numbered.selectedCol = 4
	numbered = applyKey(t, numbered, runeKey(`"`))
	numbered = applyKey(t, numbered, runeKey("1"))
	numbered = applyKey(t, numbered, runeKey("p"))
	if got, want := numbered.cellValue(0, 4), "alpha"; got != want {
		t.Fatalf("expected numbered delete register to paste %q into E1, got %q", want, got)
	}
}

func TestExtraMotionsAndScrollCommands(t *testing.T) {
	m := newModel()
	m.height = 16
	m.selectedRow = 12
	m.selectedCol = 4
	m.rowOffset = 10
	m.setCellValue(12, 2, "first")
	m.setCellValue(12, 4, "last")

	got := applyKey(t, m, runeKey("^"))
	if got.selectedCol != 2 {
		t.Fatalf("expected ^ to move to first non-blank column, got %d", got.selectedCol)
	}
	got = applyKey(t, got, runeKey("$"))
	if got.selectedCol != 4 {
		t.Fatalf("expected $ to move to last non-blank column, got %d", got.selectedCol)
	}
	got = applyKey(t, got, runeKey("0"))
	if got.selectedCol != 0 {
		t.Fatalf("expected 0 to move to first column, got %d", got.selectedCol)
	}

	got = applyKey(t, got, runeKey("H"))
	if got.selectedRow != 10 {
		t.Fatalf("expected H to move to top visible row 10, got %d", got.selectedRow)
	}
	got.selectedRow = 12
	got = applyKey(t, got, runeKey("M"))
	if got.selectedRow != 12 {
		t.Fatalf("expected M to move to middle visible row 12, got %d", got.selectedRow)
	}
	got.selectedRow = 12
	got = applyKey(t, got, runeKey("L"))
	if got.selectedRow != 15 {
		t.Fatalf("expected L to move to bottom visible row 15, got %d", got.selectedRow)
	}

	got.selectedRow = 12
	got = applyKey(t, got, runeKey("z"))
	got = applyKey(t, got, runeKey("t"))
	if got.rowOffset != 12 {
		t.Fatalf("expected zt to align current row to top, got row offset %d", got.rowOffset)
	}
	got = applyKey(t, got, runeKey("z"))
	got = applyKey(t, got, runeKey("b"))
	if got.rowOffset != 7 {
		t.Fatalf("expected zb to align current row to bottom, got row offset %d", got.rowOffset)
	}
}

func TestFitWithAccentedCharacters(t *testing.T) {
	// "café" is 4 display columns but 5 bytes in UTF-8
	got := fit("café", 8)
	if got != "café    " {
		t.Fatalf("expected %q, got %q", "café    ", got)
	}
}

func TestAlignCenterWithAccentedCharacters(t *testing.T) {
	got := alignCenter("café", 8)
	if got != "  café  " {
		t.Fatalf("expected %q, got %q", "  café  ", got)
	}
}

func TestFitLeftWithAccentedCharacters(t *testing.T) {
	got := fitLeft("café", 8)
	if got != "    café" {
		t.Fatalf("expected %q, got %q", "    café", got)
	}
}

func TestTruncateWithAccentedCharacters(t *testing.T) {
	// "café latte" is 10 display columns; truncate to 6
	got := truncate("café latte", 6)
	if got != "café …" {
		t.Fatalf("expected %q, got %q", "café …", got)
	}
}

func TestRenderTextInputWithAccentedCharacters(t *testing.T) {
	cur := cursor.New()
	cur.Focus()
	style := lipgloss.NewStyle()

	// "café" cursor at end (pos=4): should pad to full width
	got := renderTextInput("café", 4, 8, cur, style)
	if w := lipgloss.Width(got); w != 8 {
		t.Fatalf("expected display width 8 at pos=4, got %d: %q", w, got)
	}

	// "café" cursor at start (pos=0): cursor on 'c', then "afé" + padding
	got = renderTextInput("café", 0, 8, cur, style)
	if w := lipgloss.Width(got); w != 8 {
		t.Fatalf("expected display width 8 at pos=0, got %d: %q", w, got)
	}
}

func TestMouseClickFocusesCell(t *testing.T) {
	m := newModel()
	m.width = 80
	m.height = 24
	m.setCellValue(0, 0, "A")
	m.setCellValue(1, 1, "B")

	// Click on row 1, col 1: y=4 (line 2 + 2*1), x = rowLabelWidth+2 + 1*(cellWidth+1)
	clickX := m.rowLabelWidth + 2 + 1*(m.cellWidth+1)
	clickY := 4
	clickMsg := tea.MouseMsg{X: clickX, Y: clickY, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	updated, _ := m.Update(clickMsg)
	got := updated.(model)
	assertSelection(t, got, 1, 1)
	if got.mode != normalMode {
		t.Fatalf("expected normal mode after click, got %s", got.mode)
	}
}

func TestMouseClickExitsInsertMode(t *testing.T) {
	m := newModel()
	m.width = 80
	m.height = 24
	m.mode = insertMode
	m.editingValue = "hello"
	m.editingCursor = 5

	// Click on row 0, col 0
	clickX := m.rowLabelWidth + 2
	clickY := 2
	clickMsg := tea.MouseMsg{X: clickX, Y: clickY, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	updated, _ := m.Update(clickMsg)
	got := updated.(model)
	if got.mode != normalMode {
		t.Fatalf("expected normal mode, got %s", got.mode)
	}
	assertSelection(t, got, 0, 0)
}

func TestMouseClickOnBorderIgnored(t *testing.T) {
	m := newModel()
	m.width = 80
	m.height = 24

	// Click on border line (y=1 is the top border)
	clickMsg := tea.MouseMsg{X: 10, Y: 1, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	updated, _ := m.Update(clickMsg)
	got := updated.(model)
	assertSelection(t, got, 0, 0) // should remain at default
}

func TestCellFromMouseMapping(t *testing.T) {
	m := newModel()
	m.width = 80
	m.height = 24

	// Row 0, Col 0
	row, col, ok := m.cellFromMouse(m.rowLabelWidth+2, 2)
	if !ok || row != 0 || col != 0 {
		t.Fatalf("expected (0,0,true), got (%d,%d,%v)", row, col, ok)
	}

	// Row 2, Col 3
	x := m.rowLabelWidth + 2 + 3*(m.cellWidth+1)
	y := 2 + 2*2
	row, col, ok = m.cellFromMouse(x, y)
	if !ok || row != 2 || col != 3 {
		t.Fatalf("expected (2,3,true), got (%d,%d,%v)", row, col, ok)
	}

	// Click on row label area
	_, _, ok = m.cellFromMouse(1, 2)
	if ok {
		t.Fatal("expected click on row label to return false")
	}

	// Click on border between columns
	borderX := m.rowLabelWidth + 2 + m.cellWidth
	_, _, ok = m.cellFromMouse(borderX, 2)
	if ok {
		t.Fatal("expected click on column border to return false")
	}
}
