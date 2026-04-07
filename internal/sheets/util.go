package sheets

import (
	"github.com/mattn/go-runewidth"
	"strconv"
	"strings"
)

type cellRange struct {
	start cellKey
	end   cellKey
}

func (r cellRange) bounds() (top, bottom, left, right int) {
	return normalizeCellRange(r.start, r.end)
}

func (r cellRange) width() int {
	_, _, left, right := r.bounds()
	return right - left + 1
}

func (r cellRange) height() int {
	top, bottom, _, _ := r.bounds()
	return bottom - top + 1
}

func (r cellRange) isSingleCell() bool {
	return r.start == r.end
}

func parseCellRef(ref string) (cellKey, bool) {
	if ref == "" {
		return cellKey{}, false
	}

	ref = strings.ToUpper(strings.TrimSpace(ref))
	split := 0
	for split < len(ref) && isLetter(ref[split]) {
		split++
	}
	if split == 0 || split == len(ref) {
		return cellKey{}, false
	}

	columnPart := ref[:split]
	rowPart := ref[split:]
	for i := range rowPart {
		if !isDigit(rowPart[i]) {
			return cellKey{}, false
		}
	}

	row, err := strconv.Atoi(rowPart)
	if err != nil || row < 1 || row > maxRows {
		return cellKey{}, false
	}

	col := 0
	for i := range columnPart {
		col = col*26 + int(columnPart[i]-'A'+1)
	}
	col--
	if col < 0 || col >= totalCols {
		return cellKey{}, false
	}

	return cellKey{row: row - 1, col: col}, true
}

func parseCellRangeRef(ref string) (cellRange, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return cellRange{}, false
	}

	parts := strings.Split(ref, ":")
	if len(parts) == 1 {
		cell, ok := parseCellRef(parts[0])
		if !ok {
			return cellRange{}, false
		}
		return cellRange{start: cell, end: cell}, true
	}
	if len(parts) != 2 {
		return cellRange{}, false
	}

	start, ok := parseCellRef(parts[0])
	if !ok {
		return cellRange{}, false
	}
	end, ok := parseCellRef(parts[1])
	if !ok {
		return cellRange{}, false
	}

	return cellRange{start: start, end: end}, true
}

func parseColumnRef(ref string) (int, bool) {
	cell, ok := parseCellRef(strings.TrimSpace(ref) + "1")
	if !ok {
		return 0, false
	}

	return cell.col, true
}

func isCellRefCommandPrefix(input string) bool {
	if input == "" {
		return true
	}

	input = strings.ToUpper(strings.TrimSpace(input))
	split := 0
	for split < len(input) && isLetter(input[split]) {
		split++
	}
	if split == 0 || split > len(columnLabel(totalCols-1)) {
		return false
	}
	if !isColumnLabelPrefix(input[:split]) {
		return false
	}

	for i := split; i < len(input); i++ {
		if !isDigit(input[i]) {
			return false
		}
	}

	return len(input)-split <= len(strconv.Itoa(maxRows))
}

func isColumnLabelPrefix(prefix string) bool {
	for col := 0; col < totalCols; col++ {
		if strings.HasPrefix(columnLabel(col), prefix) {
			return true
		}
	}

	return false
}

func rowLabelWidthForCount(rowCount int) int {
	return max(4, len(strconv.Itoa(max(1, rowCount))))
}

func rewriteFormulaReferences(value string, deltaRow, deltaCol int) string {
	if !isFormulaCell(value) || (deltaRow == 0 && deltaCol == 0) {
		return value
	}

	body := value[1:]
	var b strings.Builder
	b.WriteByte('=')
	for i := 0; i < len(body); {
		if !isLetter(body[i]) {
			b.WriteByte(body[i])
			i++
			continue
		}

		start := i
		for i < len(body) && isLetter(body[i]) {
			i++
		}
		digitStart := i
		for i < len(body) && isDigit(body[i]) {
			i++
		}

		token := body[start:i]
		if digitStart > start && i > digitStart {
			if rewritten, ok := rewriteCellRefToken(token, deltaRow, deltaCol); ok {
				b.WriteString(rewritten)
				continue
			}
		}

		b.WriteString(token)
	}

	return b.String()
}

func rewriteCellRefToken(token string, deltaRow, deltaCol int) (string, bool) {
	ref, ok := parseCellRef(token)
	if !ok {
		return "", false
	}

	row := ref.row + deltaRow
	col := ref.col + deltaCol
	if row < 0 || row >= maxRows || col < 0 || col >= totalCols {
		return "#REF!", true
	}

	return cellRef(row, col), true
}

func rewriteFormulaForColInsert(value string, insertCol int) string {
	if !isFormulaCell(value) {
		return value
	}

	body := value[1:]
	var b strings.Builder
	b.WriteByte('=')
	for i := 0; i < len(body); {
		if !isLetter(body[i]) {
			b.WriteByte(body[i])
			i++
			continue
		}

		start := i
		for i < len(body) && isLetter(body[i]) {
			i++
		}
		digitStart := i
		for i < len(body) && isDigit(body[i]) {
			i++
		}

		token := body[start:i]
		if digitStart > start && i > digitStart {
			if rewritten, ok := rewriteCellRefForColInsert(token, insertCol); ok {
				b.WriteString(rewritten)
				continue
			}
		}

		b.WriteString(token)
	}

	return b.String()
}

func rewriteCellRefForColInsert(token string, insertCol int) (string, bool) {
	ref, ok := parseCellRef(token)
	if !ok {
		return "", false
	}

	col := ref.col
	if col >= insertCol {
		col++
	}
	if col < 0 || col >= totalCols {
		return "#REF!", true
	}

	return cellRef(ref.row, col), true
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func columnLabel(col int) string {
	if col < 0 {
		return ""
	}

	var label []byte
	for col >= 0 {
		label = append([]byte{byte('A' + (col % 26))}, label...)
		col = (col / 26) - 1
	}

	return string(label)
}

func cellRef(row, col int) string {
	return columnLabel(col) + strconv.Itoa(row+1)
}

func (m model) colWidth(col int) int {
	if w, ok := m.colWidths[col]; ok {
		return w
	}
	return m.cellWidth
}

func (m model) maxContentWidthForCol(col int) int {
	maxW := 4
	for key, val := range m.cells {
		if key.col != col {
			continue
		}
		stripped, _, _, _ := parseCellFormatting(val)
		w := runewidth.StringWidth(stripped)
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
