package sheets

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func isFormulaCell(value string) bool {
	return strings.HasPrefix(value, "=")
}

func shouldPrefixDisplayedFormula(value string) bool {
	if !isFormulaCell(value) {
		return false
	}

	formula := strings.TrimSpace(value[1:])
	return !strings.HasPrefix(strings.ToUpper(formula), "SUM(")
}

func blankFormulaValue() formulaValue {
	return formulaValue{kind: formulaBlankValue}
}

func numberFormulaValue(number float64) formulaValue {
	return formulaValue{kind: formulaNumberValue, number: number}
}

func textFormulaValue(text string) formulaValue {
	return formulaValue{kind: formulaTextValue, text: text}
}

func (v formulaValue) String() string {
	switch v.kind {
	case formulaBlankValue:
		return ""
	case formulaNumberValue:
		if v.number == 0 {
			return "0"
		}
		if v.number == math.Trunc(v.number) {
			return strconv.FormatFloat(v.number, 'f', -1, 64)
		}
		return strconv.FormatFloat(v.number, 'f', 2, 64)
	default:
		return v.text
	}
}

func parseScalarCellValue(raw string) formulaValue {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return blankFormulaValue()
	}

	stripped, _, _, _ := parseCellFormatting(trimmed)
	numeric := strings.TrimPrefix(stripped, "$")
	if number, err := strconv.ParseFloat(numeric, 64); err == nil {
		return numberFormulaValue(number)
	}

	return textFormulaValue(raw)
}

func formatFormulaError(err error) string {
	if errors.Is(err, errCircularReference) {
		return formulaCycleDisplay
	}

	return formulaErrorDisplay
}

func (m model) computedCellValue(row, col int) string {
	value, err := m.evaluateCell(row, col, &formulaEvalContext{
		visiting: make(map[cellKey]bool),
	})
	if err != nil {
		return formatFormulaError(err)
	}

	return value.String()
}

func (m model) isFormulaDisplayCell(row, col int) bool {
	return isFormulaCell(m.cellValue(row, col))
}

func (m model) isFormulaErrorDisplayCell(row, col int) bool {
	if !m.isFormulaDisplayCell(row, col) {
		return false
	}

	value := m.computedCellValue(row, col)
	return value == formulaErrorDisplay || value == formulaCycleDisplay
}

func (m model) evaluateCell(row, col int, ctx *formulaEvalContext) (formulaValue, error) {
	if ctx == nil {
		ctx = &formulaEvalContext{visiting: make(map[cellKey]bool)}
	}
	if ctx.visiting == nil {
		ctx.visiting = make(map[cellKey]bool)
	}

	key := cellKey{row: row, col: col}
	raw := m.cellValue(row, col)
	if !isFormulaCell(raw) {
		return parseScalarCellValue(raw), nil
	}
	if ctx.visiting[key] {
		return formulaValue{}, errCircularReference
	}

	ctx.visiting[key] = true
	defer delete(ctx.visiting, key)

	return m.evaluateFormula(raw[1:], key, ctx)
}

func (m model) evaluateFormula(input string, current cellKey, ctx *formulaEvalContext) (formulaValue, error) {
	parser := formulaParser{
		input:   strings.TrimSpace(input),
		model:   m,
		ctx:     ctx,
		current: current,
	}
	if parser.input == "" {
		return formulaValue{}, fmt.Errorf("empty formula")
	}

	return parser.parse()
}

func numberForArithmetic(value formulaValue) (float64, error) {
	switch value.kind {
	case formulaBlankValue:
		return 0, nil
	case formulaNumberValue:
		return value.number, nil
	case formulaTextValue:
		trimmed := strings.TrimSpace(value.text)
		if trimmed == "" {
			return 0, nil
		}
		number, err := strconv.ParseFloat(trimmed, 64)
		if err == nil {
			return number, nil
		}
	}

	return 0, fmt.Errorf("non-numeric value")
}

func numberForSum(value formulaValue) (float64, bool) {
	switch value.kind {
	case formulaBlankValue:
		return 0, false
	case formulaNumberValue:
		return value.number, true
	case formulaTextValue:
		trimmed := strings.TrimSpace(value.text)
		if trimmed == "" {
			return 0, false
		}
		number, err := strconv.ParseFloat(trimmed, 64)
		if err == nil {
			return number, true
		}
	}

	return 0, false
}

func applyArithmetic(op byte, left, right formulaValue) (formulaValue, error) {
	leftNumber, err := numberForArithmetic(left)
	if err != nil {
		return formulaValue{}, err
	}

	rightNumber, err := numberForArithmetic(right)
	if err != nil {
		return formulaValue{}, err
	}

	switch op {
	case '+':
		return numberFormulaValue(leftNumber + rightNumber), nil
	case '-':
		return numberFormulaValue(leftNumber - rightNumber), nil
	case '*':
		return numberFormulaValue(leftNumber * rightNumber), nil
	case '/':
		if rightNumber == 0 {
			return formulaValue{}, fmt.Errorf("division by zero")
		}
		return numberFormulaValue(leftNumber / rightNumber), nil
	default:
		return formulaValue{}, fmt.Errorf("unsupported operator %q", op)
	}
}

func (p *formulaParser) parse() (formulaValue, error) {
	value, err := p.parseExpression()
	if err != nil {
		return formulaValue{}, err
	}

	p.skipSpaces()
	if !p.done() {
		return formulaValue{}, fmt.Errorf("unexpected token %q", p.peek())
	}

	return value, nil
}

func (p *formulaParser) parseExpression() (formulaValue, error) {
	value, err := p.parseTerm()
	if err != nil {
		return formulaValue{}, err
	}

	for {
		p.skipSpaces()
		if p.done() {
			return value, nil
		}

		switch p.peek() {
		case '+', '-':
			op := p.consume()
			right, err := p.parseTerm()
			if err != nil {
				return formulaValue{}, err
			}
			value, err = applyArithmetic(op, value, right)
			if err != nil {
				return formulaValue{}, err
			}
		default:
			return value, nil
		}
	}
}

func (p *formulaParser) parseTerm() (formulaValue, error) {
	value, err := p.parseUnary()
	if err != nil {
		return formulaValue{}, err
	}

	for {
		p.skipSpaces()
		if p.done() {
			return value, nil
		}

		switch p.peek() {
		case '*', '/':
			op := p.consume()
			right, err := p.parseUnary()
			if err != nil {
				return formulaValue{}, err
			}
			value, err = applyArithmetic(op, value, right)
			if err != nil {
				return formulaValue{}, err
			}
		default:
			return value, nil
		}
	}
}

func (p *formulaParser) parseUnary() (formulaValue, error) {
	p.skipSpaces()
	if p.done() {
		return formulaValue{}, fmt.Errorf("unexpected end of formula")
	}

	switch p.peek() {
	case '+':
		p.consume()
		return p.parseUnary()
	case '-':
		p.consume()
		value, err := p.parseUnary()
		if err != nil {
			return formulaValue{}, err
		}

		number, err := numberForArithmetic(value)
		if err != nil {
			return formulaValue{}, err
		}

		return numberFormulaValue(-number), nil
	default:
		return p.parsePrimary()
	}
}

func (p *formulaParser) parsePrimary() (formulaValue, error) {
	p.skipSpaces()
	if p.done() {
		return formulaValue{}, fmt.Errorf("unexpected end of formula")
	}

	switch ch := p.peek(); {
	case ch == '(':
		p.consume()
		value, err := p.parseExpression()
		if err != nil {
			return formulaValue{}, err
		}
		if !p.match(')') {
			return formulaValue{}, fmt.Errorf("missing closing parenthesis")
		}
		return value, nil
	case isFormulaNumberStart(ch):
		return p.parseNumber()
	case isFormulaIdentifierStart(ch):
		identifier := p.parseIdentifier()
		p.skipSpaces()
		if p.match('(') {
			return p.parseFunctionCall(identifier)
		}

		ref, ok := parseCellRef(identifier)
		if !ok {
			return formulaValue{}, fmt.Errorf("unknown identifier %q", identifier)
		}

		return p.model.evaluateCell(ref.row, ref.col, p.ctx)
	default:
		return formulaValue{}, fmt.Errorf("unexpected token %q", ch)
	}
}

func (p *formulaParser) parseNumber() (formulaValue, error) {
	start := p.pos
	hasDigits := false

	if p.peek() == '.' {
		p.pos++
	}
	for !p.done() && isDigit(p.peek()) {
		hasDigits = true
		p.pos++
	}
	if !p.done() && p.peek() == '.' {
		p.pos++
		for !p.done() && isDigit(p.peek()) {
			hasDigits = true
			p.pos++
		}
	}
	if !hasDigits {
		return formulaValue{}, fmt.Errorf("invalid number")
	}
	if !p.done() && (p.peek() == 'e' || p.peek() == 'E') {
		p.pos++
		if !p.done() && (p.peek() == '+' || p.peek() == '-') {
			p.pos++
		}
		exponentDigits := false
		for !p.done() && isDigit(p.peek()) {
			exponentDigits = true
			p.pos++
		}
		if !exponentDigits {
			return formulaValue{}, fmt.Errorf("invalid number")
		}
	}

	number, err := strconv.ParseFloat(p.input[start:p.pos], 64)
	if err != nil {
		return formulaValue{}, err
	}

	return numberFormulaValue(number), nil
}

func (p *formulaParser) parseFunctionCall(name string) (formulaValue, error) {
	function, ok := resolveAggregateFunction(name)
	if !ok {
		return formulaValue{}, fmt.Errorf("unknown function %q", name)
	}

	var state aggregateState
	p.skipSpaces()
	if p.match(')') {
		return finalizeAggregateFunction(function, state)
	}

	for {
		contribution, err := p.parseAggregateArgument()
		if err != nil {
			return formulaValue{}, err
		}
		state.merge(contribution)

		p.skipSpaces()
		if p.match(')') {
			return finalizeAggregateFunction(function, state)
		}
		if !p.match(',') {
			return formulaValue{}, fmt.Errorf("expected comma or closing parenthesis")
		}
	}
}

func (p *formulaParser) parseAggregateArgument() (aggregateState, error) {
	start := p.pos
	if col, ok := p.parseColumnReferenceToken(); ok {
		p.skipSpaces()
		if p.done() || p.peek() == ',' || p.peek() == ')' {
			return p.aggregateColumnUntilCurrentRow(col)
		}
	}
	p.pos = start

	if refStart, ok := p.parseCellReferenceToken(); ok {
		p.skipSpaces()
		if p.match(':') {
			refEnd, ok := p.parseCellReferenceToken()
			if !ok {
				return aggregateState{}, fmt.Errorf("expected cell reference after range separator")
			}
			return p.aggregateRange(refStart, refEnd)
		}
	}
	p.pos = start

	if rowStart, ok := p.parseRowReferenceToken(); ok {
		p.skipSpaces()
		if p.match(':') {
			rowEnd, ok := p.parseRowReferenceToken()
			if !ok {
				return aggregateState{}, fmt.Errorf("expected row reference after range separator")
			}
			return p.aggregateRange(
				cellKey{row: rowStart, col: p.current.col},
				cellKey{row: rowEnd, col: p.current.col},
			)
		}
	}
	p.pos = start

	value, err := p.parseExpression()
	if err != nil {
		return aggregateState{}, err
	}

	var state aggregateState
	number, ok := numberForSum(value)
	if ok {
		state.add(number)
	}

	return state, nil
}

func (p *formulaParser) aggregateColumnUntilCurrentRow(col int) (aggregateState, error) {
	var state aggregateState
	for row := 0; row < p.current.row; row++ {
		value, err := p.model.evaluateCell(row, col, p.ctx)
		if err != nil {
			return aggregateState{}, err
		}
		number, ok := numberForSum(value)
		if ok {
			state.add(number)
		}
	}

	return state, nil
}

func (p *formulaParser) aggregateRange(start, end cellKey) (aggregateState, error) {
	top, bottom, left, right := normalizeCellRange(start, end)
	var state aggregateState

	for row := top; row <= bottom; row++ {
		for col := left; col <= right; col++ {
			value, err := p.model.evaluateCell(row, col, p.ctx)
			if err != nil {
				return aggregateState{}, err
			}
			number, ok := numberForSum(value)
			if ok {
				state.add(number)
			}
		}
	}

	return state, nil
}

func resolveAggregateFunction(name string) (aggregateFunction, bool) {
	switch strings.ToUpper(name) {
	case "SUM":
		return aggregateFunctionSum, true
	case "AVG":
		return aggregateFunctionAvg, true
	case "MIN":
		return aggregateFunctionMin, true
	case "MAX":
		return aggregateFunctionMax, true
	case "COUNT":
		return aggregateFunctionCount, true
	default:
		return 0, false
	}
}

func finalizeAggregateFunction(function aggregateFunction, state aggregateState) (formulaValue, error) {
	switch function {
	case aggregateFunctionSum:
		return numberFormulaValue(state.sum), nil
	case aggregateFunctionAvg:
		if state.count == 0 {
			return formulaValue{}, fmt.Errorf("AVG requires at least one numeric value")
		}
		return numberFormulaValue(state.sum / float64(state.count)), nil
	case aggregateFunctionMin:
		if state.count == 0 {
			return formulaValue{}, fmt.Errorf("MIN requires at least one numeric value")
		}
		return numberFormulaValue(state.min), nil
	case aggregateFunctionMax:
		if state.count == 0 {
			return formulaValue{}, fmt.Errorf("MAX requires at least one numeric value")
		}
		return numberFormulaValue(state.max), nil
	case aggregateFunctionCount:
		return numberFormulaValue(float64(state.count)), nil
	default:
		return formulaValue{}, fmt.Errorf("unsupported aggregate function")
	}
}

func (s *aggregateState) add(number float64) {
	if s.count == 0 {
		s.min = number
		s.max = number
	} else {
		s.min = min(s.min, number)
		s.max = max(s.max, number)
	}
	s.sum += number
	s.count++
}

func (s *aggregateState) merge(other aggregateState) {
	if other.count == 0 {
		return
	}
	if s.count == 0 {
		s.min = other.min
		s.max = other.max
	} else {
		s.min = min(s.min, other.min)
		s.max = max(s.max, other.max)
	}
	s.sum += other.sum
	s.count += other.count
}

func (p *formulaParser) parseCellReferenceToken() (cellKey, bool) {
	start := p.pos
	p.skipSpaces()

	identifierStart := p.pos
	for !p.done() && isLetter(p.peek()) {
		p.pos++
	}
	if p.pos == identifierStart {
		p.pos = start
		return cellKey{}, false
	}

	digitStart := p.pos
	for !p.done() && isDigit(p.peek()) {
		p.pos++
	}
	if p.pos == digitStart {
		p.pos = start
		return cellKey{}, false
	}

	ref, ok := parseCellRef(p.input[identifierStart:p.pos])
	if !ok {
		p.pos = start
		return cellKey{}, false
	}

	return ref, true
}

func (p *formulaParser) parseRowReferenceToken() (int, bool) {
	start := p.pos
	p.skipSpaces()

	digitStart := p.pos
	for !p.done() && isDigit(p.peek()) {
		p.pos++
	}
	if p.pos == digitStart {
		p.pos = start
		return 0, false
	}

	row, err := strconv.Atoi(p.input[digitStart:p.pos])
	if err != nil || row < 1 || row > maxRows {
		p.pos = start
		return 0, false
	}

	return row - 1, true
}

func (p *formulaParser) parseColumnReferenceToken() (int, bool) {
	start := p.pos
	p.skipSpaces()

	labelStart := p.pos
	for !p.done() && isLetter(p.peek()) {
		p.pos++
	}
	if p.pos == labelStart {
		p.pos = start
		return 0, false
	}
	if !p.done() && isDigit(p.peek()) {
		p.pos = start
		return 0, false
	}

	col, ok := parseColumnRef(p.input[labelStart:p.pos])
	if !ok {
		p.pos = start
		return 0, false
	}

	return col, true
}

func (p *formulaParser) parseIdentifier() string {
	start := p.pos
	for !p.done() && (isLetter(p.peek()) || isDigit(p.peek())) {
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *formulaParser) skipSpaces() {
	for !p.done() && p.peek() == ' ' {
		p.pos++
	}
}

func (p *formulaParser) match(ch byte) bool {
	p.skipSpaces()
	if p.done() || p.peek() != ch {
		return false
	}
	p.pos++
	return true
}

func (p *formulaParser) consume() byte {
	ch := p.input[p.pos]
	p.pos++
	return ch
}

func (p *formulaParser) peek() byte {
	return p.input[p.pos]
}

func (p *formulaParser) done() bool {
	return p.pos >= len(p.input)
}

func isFormulaIdentifierStart(ch byte) bool {
	return isLetter(ch)
}

func isFormulaNumberStart(ch byte) bool {
	return isDigit(ch) || ch == '.'
}
