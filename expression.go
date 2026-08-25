package orm

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type valueLookup func(name string) (any, bool)

func evalExpression(expression string, lookup valueLookup) (bool, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return false, fmt.Errorf("goark-orm: dynamic SQL test expression is empty")
	}
	tokens, err := scanExpressionTokens(expression)
	if err != nil {
		return false, err
	}
	parser := expressionParser{tokens: tokens, lookup: lookup}
	out, err := parser.parseOr()
	if err != nil {
		return false, err
	}
	if !parser.done() {
		return false, fmt.Errorf("goark-orm: unexpected dynamic SQL token %q", parser.peek().value)
	}
	return out, nil
}

type expressionTokenKind int

const (
	expressionTokenEOF expressionTokenKind = iota
	expressionTokenWord
	expressionTokenString
	expressionTokenOperator
	expressionTokenLParen
	expressionTokenRParen
)

type expressionToken struct {
	kind  expressionTokenKind
	value string
}

type expressionParser struct {
	tokens []expressionToken
	pos    int
	lookup valueLookup
}

func scanExpressionTokens(expression string) ([]expressionToken, error) {
	tokens := make([]expressionToken, 0, 8)
	for index := 0; index < len(expression); {
		ch := expression[index]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			index++
			continue
		}
		switch ch {
		case '(':
			tokens = append(tokens, expressionToken{kind: expressionTokenLParen, value: "("})
			index++
			continue
		case ')':
			tokens = append(tokens, expressionToken{kind: expressionTokenRParen, value: ")"})
			index++
			continue
		case '\'', '"':
			value, next, err := scanExpressionString(expression, index)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, expressionToken{kind: expressionTokenString, value: value})
			index = next
			continue
		}
		if index+1 < len(expression) {
			switch expression[index : index+2] {
			case "==", "!=", ">=", "<=", "&&", "||":
				tokens = append(tokens, expressionToken{kind: expressionTokenOperator, value: expression[index : index+2]})
				index += 2
				continue
			}
		}
		if ch == '!' || ch == '>' || ch == '<' {
			tokens = append(tokens, expressionToken{kind: expressionTokenOperator, value: expression[index : index+1]})
			index++
			continue
		}
		start := index
		for index < len(expression) && !isExpressionDelimiter(expression[index]) {
			index++
		}
		if start == index {
			return nil, fmt.Errorf("goark-orm: invalid dynamic SQL expression near %q", expression[index:])
		}
		if index+1 < len(expression) && expression[index] == '(' && expression[index+1] == ')' {
			index += 2
		}
		tokens = append(tokens, expressionToken{kind: expressionTokenWord, value: expression[start:index]})
	}
	tokens = append(tokens, expressionToken{kind: expressionTokenEOF})
	return tokens, nil
}

func scanExpressionString(expression string, start int) (string, int, error) {
	quote := expression[start]
	for index := start + 1; index < len(expression); index++ {
		if expression[index] == '\\' && quote == '"' {
			index++
			continue
		}
		if expression[index] == quote {
			return expression[start : index+1], index + 1, nil
		}
	}
	return "", start, fmt.Errorf("goark-orm: unterminated dynamic SQL string literal")
}

func isExpressionDelimiter(ch byte) bool {
	switch ch {
	case ' ', '\t', '\r', '\n', '(', ')', '!', '>', '<', '=':
		return true
	default:
		return false
	}
}

func (p *expressionParser) parseOr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for p.matchKeyword("or") || p.matchOperator("||") {
		right, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		left = left || right
	}
	return left, nil
}

func (p *expressionParser) parseAnd() (bool, error) {
	left, err := p.parseUnary()
	if err != nil {
		return false, err
	}
	for p.matchKeyword("and") || p.matchOperator("&&") {
		right, err := p.parseUnary()
		if err != nil {
			return false, err
		}
		left = left && right
	}
	return left, nil
}

func (p *expressionParser) parseUnary() (bool, error) {
	if p.matchKeyword("not") || p.matchOperator("!") {
		value, err := p.parseUnary()
		return !value, err
	}
	return p.parsePrimary()
}

func (p *expressionParser) parsePrimary() (bool, error) {
	if p.matchKind(expressionTokenLParen) {
		value, err := p.parseOr()
		if err != nil {
			return false, err
		}
		if !p.matchKind(expressionTokenRParen) {
			return false, fmt.Errorf("goark-orm: dynamic SQL expression missing closing parenthesis")
		}
		return value, nil
	}
	return p.parseComparison()
}

func (p *expressionParser) parseComparison() (bool, error) {
	left, err := p.consumeValue()
	if err != nil {
		return false, err
	}
	if operator, ok := p.matchComparisonOperator(); ok {
		right, err := p.consumeValue()
		if err != nil {
			return false, err
		}
		return compareExpression(left, operator, right, p.lookup)
	}
	value, ok, err := expressionValue(left, p.lookup)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	return truthy(value), nil
}

func (p *expressionParser) consumeValue() (string, error) {
	token := p.peek()
	switch token.kind {
	case expressionTokenWord:
		if strings.EqualFold(token.value, "len") && p.nextKindIs(expressionTokenLParen) {
			return p.consumeLengthCall()
		}
		p.pos++
		return token.value, nil
	case expressionTokenString:
		p.pos++
		return token.value, nil
	default:
		return "", fmt.Errorf("goark-orm: expected dynamic SQL value, got %q", token.value)
	}
}

func (p *expressionParser) consumeLengthCall() (string, error) {
	p.pos += 2
	token := p.peek()
	if token.kind != expressionTokenWord {
		return "", fmt.Errorf("goark-orm: len() requires a dynamic SQL parameter path")
	}
	name := token.value
	p.pos++
	if !p.matchKind(expressionTokenRParen) {
		return "", fmt.Errorf("goark-orm: len() missing closing parenthesis")
	}
	return "len(" + name + ")", nil
}

func (p *expressionParser) nextKindIs(kind expressionTokenKind) bool {
	return p.pos+1 < len(p.tokens) && p.tokens[p.pos+1].kind == kind
}

func (p *expressionParser) matchComparisonOperator() (string, bool) {
	token := p.peek()
	if token.kind != expressionTokenOperator {
		return "", false
	}
	switch token.value {
	case "==", "!=", ">", ">=", "<", "<=":
		p.pos++
		return token.value, true
	default:
		return "", false
	}
}

func (p *expressionParser) matchKeyword(keyword string) bool {
	token := p.peek()
	if token.kind == expressionTokenWord && strings.EqualFold(token.value, keyword) {
		p.pos++
		return true
	}
	return false
}

func (p *expressionParser) matchOperator(operator string) bool {
	token := p.peek()
	if token.kind == expressionTokenOperator && token.value == operator {
		p.pos++
		return true
	}
	return false
}

func (p *expressionParser) matchKind(kind expressionTokenKind) bool {
	if p.peek().kind == kind {
		p.pos++
		return true
	}
	return false
}

func (p *expressionParser) peek() expressionToken {
	if p.pos >= len(p.tokens) {
		return expressionToken{kind: expressionTokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *expressionParser) done() bool {
	return p.peek().kind == expressionTokenEOF
}

func evalValueExpression(expression string, lookup valueLookup) (any, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, fmt.Errorf("goark-orm: dynamic SQL value expression is empty")
	}
	parts := splitValueExpression(expression, '+')
	if len(parts) > 1 {
		var builder strings.Builder
		for _, part := range parts {
			value, ok, err := expressionValue(part, lookup)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("SQL value expression variable %q is missing", part)
			}
			if value != nil {
				builder.WriteString(fmt.Sprint(comparableValue(value)))
			}
		}
		return builder.String(), nil
	}
	value, ok, err := expressionValue(expression, lookup)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("SQL value expression variable %q is missing", expression)
	}
	return value, nil
}

func splitValueExpression(expression string, operator rune) []string {
	var out []string
	inQuote := rune(0)
	start := 0
	for index, r := range expression {
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			inQuote = r
			continue
		}
		if r == operator {
			out = append(out, strings.TrimSpace(expression[start:index]))
			start = index + 1
		}
	}
	if len(out) == 0 {
		return []string{expression}
	}
	out = append(out, strings.TrimSpace(expression[start:]))
	return out
}

func splitExpression(expression string, operator string) []string {
	var out []string
	inQuote := rune(0)
	token := " " + operator + " "
	start := 0
	for index, r := range expression {
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			inQuote = r
			continue
		}
		if strings.HasPrefix(expression[index:], token) {
			out = append(out, strings.TrimSpace(expression[start:index]))
			start = index + len(token)
		}
	}
	if len(out) == 0 {
		return []string{expression}
	}
	out = append(out, strings.TrimSpace(expression[start:]))
	return out
}

func cutOperator(expression string, operator string) (string, string, bool) {
	inQuote := rune(0)
	for index, r := range expression {
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			inQuote = r
			continue
		}
		if strings.HasPrefix(expression[index:], operator) {
			return strings.TrimSpace(expression[:index]), strings.TrimSpace(expression[index+len(operator):]), true
		}
	}
	return "", "", false
}

func compareExpressionValues(left string, right string, lookup valueLookup) (bool, error) {
	leftValue, _, err := expressionValue(left, lookup)
	if err != nil {
		return false, err
	}
	rightValue, _, err := expressionValue(right, lookup)
	if err != nil {
		return false, err
	}
	if isNilValue(leftValue) || isNilValue(rightValue) {
		return isNilValue(leftValue) && isNilValue(rightValue), nil
	}
	leftComparable := comparableValue(leftValue)
	rightComparable := comparableValue(rightValue)
	if reflect.TypeOf(leftComparable) == reflect.TypeOf(rightComparable) {
		return reflect.DeepEqual(leftComparable, rightComparable), nil
	}
	return fmt.Sprint(leftComparable) == fmt.Sprint(rightComparable), nil
}

func compareExpression(left string, operator string, right string, lookup valueLookup) (bool, error) {
	switch operator {
	case "==":
		return compareExpressionValues(left, right, lookup)
	case "!=":
		equal, err := compareExpressionValues(left, right, lookup)
		return !equal, err
	case ">", ">=", "<", "<=":
		order, ok, err := compareExpressionOrder(left, right, lookup)
		if err != nil || !ok {
			return false, err
		}
		switch operator {
		case ">":
			return order > 0, nil
		case ">=":
			return order >= 0, nil
		case "<":
			return order < 0, nil
		case "<=":
			return order <= 0, nil
		}
	}
	return false, fmt.Errorf("goark-orm: unsupported dynamic SQL operator %q", operator)
}

func compareExpressionOrder(left string, right string, lookup valueLookup) (int, bool, error) {
	leftValue, leftOK, err := expressionValue(left, lookup)
	if err != nil {
		return 0, false, err
	}
	rightValue, rightOK, err := expressionValue(right, lookup)
	if err != nil {
		return 0, false, err
	}
	if !leftOK || !rightOK || isNilValue(leftValue) || isNilValue(rightValue) {
		return 0, false, nil
	}
	leftNumber, leftNumberOK := numericExpressionValue(leftValue)
	rightNumber, rightNumberOK := numericExpressionValue(rightValue)
	if leftNumberOK && rightNumberOK {
		switch {
		case leftNumber > rightNumber:
			return 1, true, nil
		case leftNumber < rightNumber:
			return -1, true, nil
		default:
			return 0, true, nil
		}
	}
	leftText, leftStringOK := comparableValue(leftValue).(string)
	rightText, rightStringOK := comparableValue(rightValue).(string)
	if leftStringOK && rightStringOK {
		return strings.Compare(leftText, rightText), true, nil
	}
	return 0, false, fmt.Errorf("goark-orm: dynamic SQL operator requires numeric or string values")
}

func numericExpressionValue(value any) (float64, bool) {
	switch item := value.(type) {
	case int:
		return float64(item), true
	case int8:
		return float64(item), true
	case int16:
		return float64(item), true
	case int32:
		return float64(item), true
	case int64:
		return float64(item), true
	case uint:
		return float64(item), true
	case uint8:
		return float64(item), true
	case uint16:
		return float64(item), true
	case uint32:
		return float64(item), true
	case uint64:
		return float64(item), true
	case uintptr:
		return float64(item), true
	case float32:
		return float64(item), true
	case float64:
		return item, true
	case string:
		parsed, err := strconv.ParseFloat(item, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func expressionValue(raw string, lookup valueLookup) (any, bool, error) {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	switch {
	case lower == "nil" || lower == "null":
		return nil, true, nil
	case lower == "true":
		return true, true, nil
	case lower == "false":
		return false, true, nil
	case strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'") && len(raw) >= 2:
		return strings.TrimSuffix(strings.TrimPrefix(raw, "'"), "'"), true, nil
	case strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\""):
		value, err := strconv.Unquote(raw)
		if err != nil {
			return nil, false, fmt.Errorf("goark-orm: invalid dynamic SQL string literal %q", raw)
		}
		return value, true, nil
	case strings.HasPrefix(lower, "len(") && strings.HasSuffix(raw, ")"):
		value, ok := expressionLength(strings.TrimSpace(raw[4:len(raw)-1]), lookup)
		return value, ok, nil
	case isExpressionNumberCandidate(raw):
		value, err := parseExpressionNumber(raw)
		if err != nil {
			return nil, false, err
		}
		return value, true, nil
	case raw != "":
		if value, ok := expressionSizeProperty(raw, lookup); ok {
			return value, true, nil
		}
		value, ok := lookup(raw)
		return value, ok, nil
	}
	return nil, false, nil
}

func isExpressionNumberCandidate(raw string) bool {
	if raw == "" {
		return false
	}
	if raw[0] == '-' || raw[0] == '+' {
		if len(raw) == 1 {
			return false
		}
		raw = raw[1:]
	}
	return raw[0] == '.' || (raw[0] >= '0' && raw[0] <= '9')
}

func parseExpressionNumber(raw string) (any, error) {
	if strings.ContainsAny(raw, ".eE") {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("goark-orm: invalid dynamic SQL numeric literal %q", raw)
		}
		return parsed, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("goark-orm: invalid dynamic SQL numeric literal %q", raw)
	}
	return parsed, nil
}

func expressionSizeProperty(raw string, lookup valueLookup) (any, bool) {
	for _, suffix := range []string{".size()", ".length()", ".size", ".length"} {
		if strings.HasSuffix(raw, suffix) {
			return expressionLength(strings.TrimSpace(raw[:len(raw)-len(suffix)]), lookup)
		}
	}
	return nil, false
}

func expressionLength(raw string, lookup valueLookup) (any, bool) {
	if raw == "" {
		return nil, false
	}
	value, ok := lookup(raw)
	if !ok {
		return nil, false
	}
	if isNilValue(value) {
		return 0, true
	}
	current := reflect.ValueOf(value)
	current, nilValue := dereferenceParameterValue(current)
	if nilValue {
		return 0, true
	}
	switch current.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return current.Len(), true
	default:
		return nil, false
	}
}

func comparableValue(value any) any {
	switch item := value.(type) {
	case []byte:
		return string(item)
	default:
		return item
	}
}

func truthy(value any) bool {
	if isNilValue(value) {
		return false
	}
	switch item := value.(type) {
	case bool:
		return item
	case string:
		return item != ""
	case []byte:
		return len(item) > 0
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map:
		return rv.Len() > 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() != 0
	default:
		return true
	}
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
