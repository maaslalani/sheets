package sheets

import (
	"errors"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	defaultRows = 999
	maxRows     = 50000
	totalCols   = 52
)

type mode string

const (
	normalMode  mode = "NORMAL"
	insertMode  mode = "INSERT"
	selectMode  mode = "SELECT"
	commandMode mode = ":"
)

type cellKey struct {
	row int
	col int
}

type clipboard struct {
	cells       [][]string
	sourceRow   int
	sourceCol   int
	isReference bool
}

type promptKind rune

const (
	noPrompt             promptKind = 0
	commandPrompt        promptKind = ':'
	searchForwardPrompt  promptKind = '/'
	searchBackwardPrompt promptKind = '?'
)

type undoState struct {
	cells       map[cellKey]string
	rowCount    int
	selectedRow int
	selectedCol int
	selectRow   int
	selectCol   int
	selectRows  bool
	rowOffset   int
	colOffset   int
}

const (
	formulaErrorDisplay = "#ERR"
	formulaCycleDisplay = "#CYCLE"
)

var errCircularReference = errors.New("circular reference")

type formulaValueKind int

const (
	formulaBlankValue formulaValueKind = iota
	formulaNumberValue
	formulaTextValue
)

type formulaValue struct {
	kind   formulaValueKind
	number float64
	text   string
}

type aggregateFunction int

const (
	aggregateFunctionSum aggregateFunction = iota
	aggregateFunctionAvg
	aggregateFunctionMin
	aggregateFunctionMax
	aggregateFunctionCount
)

type aggregateState struct {
	sum   float64
	count int
	min   float64
	max   float64
}

type formulaEvalContext struct {
	visiting map[cellKey]bool
}

type formulaParser struct {
	input   string
	pos     int
	model   model
	ctx     *formulaEvalContext
	current cellKey
}

type model struct {
	width    int
	height   int
	rowCount int

	mode mode

	promptKind      promptKind
	gotoPending     bool
	gotoBuffer      string
	commandPending  bool
	commandBuffer   string
	commandCursor   int
	commandMessage  string
	deletePending   bool
	yankPending     bool
	yankCount       int
	zPending        bool
	registerPending bool
	activeRegister  rune
	countBuffer     string
	currentFilePath string
	searchQuery     string
	searchDirection int
	markPending     bool
	markJumpPending bool
	markJumpExact   bool
	commandError    bool
	dirtyFile       bool

	selectedRow int
	selectedCol int
	selectRow   int
	selectCol   int
	selectRows  bool
	rowOffset   int
	colOffset   int

	cellWidth     int
	rowLabelWidth int

	cells           map[cellKey]string
	copyBuffer      clipboard
	hasCopyBuffer   bool
	registers       map[rune]clipboard
	marks           map[rune]cellKey
	jumpBack        []cellKey
	jumpForward     []cellKey
	undoStack       []undoState
	redoStack       []undoState
	editingValue    string
	editingCursor   int
	editCursor      cursor.Model
	insertKeys      []tea.KeyMsg
	recordingInsert bool
	lastChange      []tea.KeyMsg
	replayingChange bool

	headerStyle                   lipgloss.Style
	activeHeaderStyle             lipgloss.Style
	rowLabelStyle                 lipgloss.Style
	activeRowStyle                lipgloss.Style
	gridStyle                     lipgloss.Style
	formulaCellStyle              lipgloss.Style
	formulaErrorStyle             lipgloss.Style
	activeCellStyle               lipgloss.Style
	activeFormulaStyle            lipgloss.Style
	activeFormulaErrorStyle       lipgloss.Style
	selectCellStyle               lipgloss.Style
	selectFormulaStyle            lipgloss.Style
	selectFormulaErrorStyle       lipgloss.Style
	selectActiveCellStyle         lipgloss.Style
	selectActiveFormulaStyle      lipgloss.Style
	selectActiveFormulaErrorStyle lipgloss.Style
	selectHeaderStyle             lipgloss.Style
	selectActiveHeaderStyle       lipgloss.Style
	selectRowStyle                lipgloss.Style
	selectBorderStyle             lipgloss.Style
	statusBarStyle                lipgloss.Style
	statusTextStyle               lipgloss.Style
	statusAccentStyle             lipgloss.Style
	statusNormalStyle             lipgloss.Style
	statusInsertStyle             lipgloss.Style
	statusSelectStyle             lipgloss.Style
	commandLineStyle              lipgloss.Style
	commandErrorStyle             lipgloss.Style
}
