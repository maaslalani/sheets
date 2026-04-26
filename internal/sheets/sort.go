package sheets

import (
	"fmt"
	"sort"
	"strconv"
)

func (m *model) sortByColumn(col int, ascending bool) {
	if m.rowCount <= 1 {
		return
	}

	const startRow = 1
	numDataRows := m.rowCount - startRow
	if numDataRows <= 0 {
		return
	}

	// Single pass: mark non-empty rows and grab sort-column values.
	nonEmptyRow := make([]bool, numDataRows)
	for k := range m.cells {
		if k.row >= startRow {
			nonEmptyRow[k.row-startRow] = true
		}
	}

	type sortKey struct {
		f       float64
		s       string
		isFloat bool
	}
	keys := make([]sortKey, numDataRows)
	for i := range keys {
		v := m.cells[cellKey{row: startRow + i, col: col}]
		f, err := strconv.ParseFloat(v, 64)
		keys[i] = sortKey{f: f, s: v, isFloat: err == nil}
	}

	perm := make([]int, numDataRows)
	for i := range perm {
		perm[i] = i
	}
	sort.SliceStable(perm, func(a, b int) bool {
		ia, ib := perm[a], perm[b]
		if nonEmptyRow[ia] != nonEmptyRow[ib] {
			return nonEmptyRow[ia]
		}
		var less bool
		if keys[ia].isFloat && keys[ib].isFloat {
			less = keys[ia].f < keys[ib].f
		} else {
			less = keys[ia].s < keys[ib].s
		}
		if ascending {
			return less
		}
		return !less
	})

	// Inverse perm: inv[srcIdx] = destIdx — lets us remap in one map pass.
	inv := make([]int, numDataRows)
	for destIdx, srcIdx := range perm {
		inv[srcIdx] = destIdx
	}

	// Swapping m.cells (not mutating) means oldCells is safe to store in undo
	// without cloning — subsequent writes go to newCells, not oldCells.
	newCells := make(map[cellKey]string, len(m.cells))
	for k, v := range m.cells {
		if k.row < startRow {
			newCells[k] = v
			continue
		}
		newCells[cellKey{row: startRow + inv[k.row-startRow], col: k.col}] = v
	}

	m.undoStack = append(m.undoStack, undoState{
		cells:       m.cells,
		rowCount:    m.rowCount,
		selectedRow: m.selectedRow,
		selectedCol: m.selectedCol,
		selectRow:   m.selectRow,
		selectCol:   m.selectCol,
		selectRows:  m.selectRows,
		rowOffset:   m.rowOffset,
		colOffset:   m.colOffset,
	})
	m.redoStack = nil
	m.cells = newCells

	m.dirtyFile = true
	direction := "ASC"
	if !ascending {
		direction = "DESC"
	}
	m.commandMessage = fmt.Sprintf("Sorted by column %s (%s)", columnLabel(col), direction)
}
