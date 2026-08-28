package runtime

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

type valueLookup func(name string) (any, bool)

func evalExpression(expression string, lookup valueLookup) (bool, error) {
	value, err := evalValueExpression(expression, lookup)
	if err != nil {
		return false, err
	}
	return truthy(value), nil
}

func evalValueExpression(expression string, lookup valueLookup) (any, error) {
	plan, err := compileExpressionPlan(expression)
	if err != nil {
		return nil, err
	}
	return plan.evalValue(lookup)
}

func (p expressionPlan) evalValue(lookup valueLookup) (any, error) {
	parser := expressionParser{tokens: p.tokens, lookup: lookup}
	value, err := parser.parseTernary()
	if err != nil {
		return nil, err
	}
	if !parser.done() {
		return nil, fmt.Errorf("goark-orm: unexpected dynamic SQL token %q", parser.peek().value)
	}
	return value, nil
}

type expressionTokenKind int

const (
	expressionTokenEOF expressionTokenKind = iota
	expressionTokenWord
	expressionTokenString
	expressionTokenNumber
	expressionTokenOperator
	expressionTokenLParen
	expressionTokenRParen
	expressionTokenLBrace
	expressionTokenRBrace
	expressionTokenComma
	expressionTokenQuestion
	expressionTokenColon
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
		case '{':
			tokens = append(tokens, expressionToken{kind: expressionTokenLBrace, value: "{"})
			index++
			continue
		case '}':
			tokens = append(tokens, expressionToken{kind: expressionTokenRBrace, value: "}"})
			index++
			continue
		case ',':
			tokens = append(tokens, expressionToken{kind: expressionTokenComma, value: ","})
			index++
			continue
		case '?':
			tokens = append(tokens, expressionToken{kind: expressionTokenQuestion, value: "?"})
			index++
			continue
		case ':':
			tokens = append(tokens, expressionToken{kind: expressionTokenColon, value: ":"})
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
		if strings.ContainsRune("!><+-*/%", rune(ch)) {
			tokens = append(tokens, expressionToken{kind: expressionTokenOperator, value: expression[index : index+1]})
			index++
			continue
		}
		if isExpressionNumberStart(expression, index) {
			value, next := scanExpressionNumber(expression, index)
			tokens = append(tokens, expressionToken{kind: expressionTokenNumber, value: value})
			index = next
			continue
		}
		start := index
		for index < len(expression) && !isExpressionDelimiter(expression[index]) {
			index++
		}
		if start == index {
			return nil, fmt.Errorf("goark-orm: invalid dynamic SQL expression near %q", expression[index:])
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

func isExpressionNumberStart(expression string, index int) bool {
	ch := expression[index]
	if ch >= '0' && ch <= '9' {
		return true
	}
	return ch == '.' && index+1 < len(expression) && expression[index+1] >= '0' && expression[index+1] <= '9'
}

func scanExpressionNumber(expression string, start int) (string, int) {
	index := start
	for index < len(expression) && expression[index] >= '0' && expression[index] <= '9' {
		index++
	}
	if index < len(expression) && expression[index] == '.' {
		index++
		for index < len(expression) && expression[index] >= '0' && expression[index] <= '9' {
			index++
		}
	}
	if index < len(expression) && (expression[index] == 'e' || expression[index] == 'E') {
		index++
		if index < len(expression) && (expression[index] == '+' || expression[index] == '-') {
			index++
		}
		for index < len(expression) && expression[index] >= '0' && expression[index] <= '9' {
			index++
		}
	}
	return expression[start:index], index
}

func isExpressionDelimiter(ch byte) bool {
	switch ch {
	case ' ', '\t', '\r', '\n', '(', ')', '{', '}', ',', '?', ':', '!', '>', '<', '=', '+', '-', '*', '/', '%':
		return true
	default:
		return false
	}
}

func (p *expressionParser) parseTernary() (any, error) {
	condition, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if !p.matchKind(expressionTokenQuestion) {
		return condition, nil
	}
	trueValue, err := p.parseTernary()
	if err != nil {
		return nil, err
	}
	if !p.matchKind(expressionTokenColon) {
		return nil, fmt.Errorf("goark-orm: dynamic SQL ternary expression missing ':'")
	}
	falseValue, err := p.parseTernary()
	if err != nil {
		return nil, err
	}
	if truthy(condition) {
		return trueValue, nil
	}
	return falseValue, nil
}

func (p *expressionParser) parseOr() (any, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.matchKeyword("or") || p.matchOperator("||") {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = truthy(left) || truthy(right)
	}
	return left, nil
}

func (p *expressionParser) parseAnd() (any, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.matchKeyword("and") || p.matchOperator("&&") {
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = truthy(left) && truthy(right)
	}
	return left, nil
}

func (p *expressionParser) parseComparison() (any, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	for {
		if operator, ok := p.matchComparisonOperator(); ok {
			right, err := p.parseAdditive()
			if err != nil {
				return nil, err
			}
			left, err = compareExpressionAny(left, operator, right)
			if err != nil {
				return nil, err
			}
			continue
		}
		if p.matchKeyword("in") {
			right, err := p.parseAdditive()
			if err != nil {
				return nil, err
			}
			left = expressionContains(right, left)
			continue
		}
		if p.matchNotIn() {
			right, err := p.parseAdditive()
			if err != nil {
				return nil, err
			}
			left = !expressionContains(right, left)
			continue
		}
		return left, nil
	}
}

func (p *expressionParser) parseAdditive() (any, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.matchOperator("+"):
			right, err := p.parseMultiplicative()
			if err != nil {
				return nil, err
			}
			left, err = addExpressionValues(left, right)
			if err != nil {
				return nil, err
			}
		case p.matchOperator("-"):
			right, err := p.parseMultiplicative()
			if err != nil {
				return nil, err
			}
			left, err = numericBinaryExpression(left, right, "-")
			if err != nil {
				return nil, err
			}
		default:
			return left, nil
		}
	}
}

func (p *expressionParser) parseMultiplicative() (any, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.matchOperator("*"):
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left, err = numericBinaryExpression(left, right, "*")
			if err != nil {
				return nil, err
			}
		case p.matchOperator("/"):
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left, err = numericBinaryExpression(left, right, "/")
			if err != nil {
				return nil, err
			}
		case p.matchOperator("%"):
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left, err = numericBinaryExpression(left, right, "%")
			if err != nil {
				return nil, err
			}
		default:
			return left, nil
		}
	}
}

func (p *expressionParser) parseUnary() (any, error) {
	switch {
	case p.matchKeyword("not") || p.matchOperator("!"):
		value, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return !truthy(value), nil
	case p.matchEmptyOperator():
		value, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return isEmptyExpressionValue(value), nil
	case p.matchOperator("-"):
		value, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		number, ok := numericExpressionValue(value)
		if !ok {
			return nil, fmt.Errorf("goark-orm: unary '-' requires numeric value")
		}
		return -number, nil
	case p.matchOperator("+"):
		value, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		number, ok := numericExpressionValue(value)
		if !ok {
			return nil, fmt.Errorf("goark-orm: unary '+' requires numeric value")
		}
		return number, nil
	default:
		return p.parsePrimary()
	}
}

func (p *expressionParser) parsePrimary() (any, error) {
	if p.matchKind(expressionTokenLParen) {
		value, err := p.parseTernary()
		if err != nil {
			return nil, err
		}
		if !p.matchKind(expressionTokenRParen) {
			return nil, fmt.Errorf("goark-orm: dynamic SQL expression missing closing parenthesis")
		}
		return p.parsePostfix(value)
	}
	if p.matchKind(expressionTokenLBrace) {
		value, err := p.parseListLiteral()
		if err != nil {
			return nil, err
		}
		return p.parsePostfix(value)
	}
	token := p.peek()
	switch token.kind {
	case expressionTokenWord:
		p.pos++
		if strings.HasPrefix(token.value, ".") {
			return nil, fmt.Errorf("goark-orm: unexpected dynamic SQL method %q", token.value)
		}
		if p.matchKind(expressionTokenLParen) {
			value, err := p.parseCall(token.value)
			if err != nil {
				return nil, err
			}
			return p.parsePostfix(value)
		}
		value, _, err := expressionValue(token.value, p.lookup)
		if err != nil {
			return nil, err
		}
		return p.parsePostfix(value)
	case expressionTokenString, expressionTokenNumber:
		p.pos++
		value, _, err := expressionValue(token.value, p.lookup)
		if err != nil {
			return nil, err
		}
		return p.parsePostfix(value)
	default:
		return nil, fmt.Errorf("goark-orm: expected dynamic SQL value, got %q", token.value)
	}
}

func (p *expressionParser) parsePostfix(value any) (any, error) {
	for {
		token := p.peek()
		if token.kind != expressionTokenWord || !strings.HasPrefix(token.value, ".") {
			return value, nil
		}
		method := strings.TrimPrefix(token.value, ".")
		p.pos++
		if p.matchKind(expressionTokenLParen) {
			args, err := p.parseCallArguments()
			if err != nil {
				return nil, err
			}
			value, err = evalExpressionMethod(value, method, args)
			if err != nil {
				return nil, err
			}
			continue
		}
		switch strings.ToLower(strings.TrimSpace(method)) {
		case "size", "length":
			length, ok := expressionLengthValue(value)
			if !ok {
				return nil, fmt.Errorf("goark-orm: %s property requires collection, map, array or string", method)
			}
			value = length
		case "empty":
			value = isEmptyExpressionValue(value)
		default:
			return nil, fmt.Errorf("goark-orm: dynamic SQL property %q is not allowed", method)
		}
	}
}

func (p *expressionParser) parseListLiteral() ([]any, error) {
	out := make([]any, 0)
	if p.matchKind(expressionTokenRBrace) {
		return out, nil
	}
	for {
		value, err := p.parseTernary()
		if err != nil {
			return nil, err
		}
		out = append(out, value)
		if p.matchKind(expressionTokenRBrace) {
			return out, nil
		}
		if !p.matchKind(expressionTokenComma) {
			return nil, fmt.Errorf("goark-orm: dynamic SQL list literal missing comma")
		}
	}
}

func (p *expressionParser) parseCall(name string) (any, error) {
	args, err := p.parseCallArguments()
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if receiver, method, ok := splitExpressionMethod(name); ok {
		value, _, err := expressionValue(receiver, p.lookup)
		if err != nil {
			return nil, err
		}
		return evalExpressionMethod(value, method, args)
	}
	return evalExpressionFunction(name, args)
}

func (p *expressionParser) parseCallArguments() ([]any, error) {
	args := make([]any, 0, 2)
	if p.matchKind(expressionTokenRParen) {
		return args, nil
	}
	for {
		value, err := p.parseTernary()
		if err != nil {
			return nil, err
		}
		args = append(args, value)
		if p.matchKind(expressionTokenRParen) {
			return args, nil
		}
		if !p.matchKind(expressionTokenComma) {
			return nil, fmt.Errorf("goark-orm: dynamic SQL function call missing comma")
		}
	}
}

func splitExpressionMethod(name string) (string, string, bool) {
	index := strings.LastIndex(name, ".")
	if index <= 0 || index >= len(name)-1 {
		return "", "", false
	}
	return strings.TrimSpace(name[:index]), strings.TrimSpace(name[index+1:]), true
}

func evalExpressionFunction(name string, args []any) (any, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "len", "size":
		if len(args) != 1 {
			return nil, fmt.Errorf("goark-orm: %s() requires one argument", name)
		}
		length, ok := expressionLengthValue(args[0])
		if !ok {
			return nil, fmt.Errorf("goark-orm: %s() requires collection, map, array or string", name)
		}
		return length, nil
	default:
		return nil, fmt.Errorf("goark-orm: dynamic SQL function %q is not allowed", name)
	}
}

func evalExpressionMethod(receiver any, method string, args []any) (any, error) {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "size", "length":
		if len(args) != 0 {
			return nil, fmt.Errorf("goark-orm: %s() requires no arguments", method)
		}
		length, ok := expressionLengthValue(receiver)
		if !ok {
			return nil, fmt.Errorf("goark-orm: %s() requires collection, map, array or string", method)
		}
		return length, nil
	case "isempty":
		if len(args) != 0 {
			return nil, fmt.Errorf("goark-orm: isEmpty() requires no arguments")
		}
		return isEmptyExpressionValue(receiver), nil
	case "contains":
		if len(args) != 1 {
			return nil, fmt.Errorf("goark-orm: contains() requires one argument")
		}
		return expressionContains(receiver, args[0]), nil
	case "containskey":
		if len(args) != 1 {
			return nil, fmt.Errorf("goark-orm: containsKey() requires one argument")
		}
		return expressionMapContainsKey(receiver, args[0]), nil
	case "containsvalue":
		if len(args) != 1 {
			return nil, fmt.Errorf("goark-orm: containsValue() requires one argument")
		}
		return expressionMapContainsValue(receiver, args[0]), nil
	case "startswith":
		if len(args) != 1 {
			return nil, fmt.Errorf("goark-orm: startsWith() requires one argument")
		}
		return strings.HasPrefix(expressionString(receiver), expressionString(args[0])), nil
	case "endswith":
		if len(args) != 1 {
			return nil, fmt.Errorf("goark-orm: endsWith() requires one argument")
		}
		return strings.HasSuffix(expressionString(receiver), expressionString(args[0])), nil
	case "tolowercase":
		if len(args) != 0 {
			return nil, fmt.Errorf("goark-orm: toLowerCase() requires no arguments")
		}
		return strings.ToLower(expressionString(receiver)), nil
	case "touppercase":
		if len(args) != 0 {
			return nil, fmt.Errorf("goark-orm: toUpperCase() requires no arguments")
		}
		return strings.ToUpper(expressionString(receiver)), nil
	case "trim":
		if len(args) != 0 {
			return nil, fmt.Errorf("goark-orm: trim() requires no arguments")
		}
		return strings.TrimSpace(expressionString(receiver)), nil
	case "equals":
		if len(args) != 1 {
			return nil, fmt.Errorf("goark-orm: equals() requires one argument")
		}
		return compareExpressionEqual(receiver, args[0]), nil
	case "equalsignorecase":
		if len(args) != 1 {
			return nil, fmt.Errorf("goark-orm: equalsIgnoreCase() requires one argument")
		}
		return strings.EqualFold(expressionString(receiver), expressionString(args[0])), nil
	default:
		return nil, fmt.Errorf("goark-orm: dynamic SQL method %q is not allowed", method)
	}
}

func (p *expressionParser) matchComparisonOperator() (string, bool) {
	token := p.peek()
	if token.kind == expressionTokenOperator {
		switch token.value {
		case "==", "!=", ">", ">=", "<", "<=":
			p.pos++
			return token.value, true
		}
	}
	if token.kind == expressionTokenWord {
		switch strings.ToLower(token.value) {
		case "eq":
			p.pos++
			return "==", true
		case "ne", "neq":
			p.pos++
			return "!=", true
		case "gt":
			p.pos++
			return ">", true
		case "gte", "ge":
			p.pos++
			return ">=", true
		case "lt":
			p.pos++
			return "<", true
		case "lte", "le":
			p.pos++
			return "<=", true
		}
	}
	return "", false
}

func (p *expressionParser) matchNotIn() bool {
	if p.pos+1 >= len(p.tokens) {
		return false
	}
	left := p.tokens[p.pos]
	right := p.tokens[p.pos+1]
	if left.kind == expressionTokenWord && right.kind == expressionTokenWord &&
		strings.EqualFold(left.value, "not") && strings.EqualFold(right.value, "in") {
		p.pos += 2
		return true
	}
	return false
}

func (p *expressionParser) matchEmptyOperator() bool {
	if !p.nextTokenCanStartValue() {
		return false
	}
	return p.matchKeyword("empty")
}

func (p *expressionParser) nextTokenCanStartValue() bool {
	if p.pos+1 >= len(p.tokens) {
		return false
	}
	switch p.tokens[p.pos+1].kind {
	case expressionTokenWord, expressionTokenString, expressionTokenNumber, expressionTokenLParen, expressionTokenLBrace:
		return true
	case expressionTokenOperator:
		return p.tokens[p.pos+1].value == "!" || p.tokens[p.pos+1].value == "-" || p.tokens[p.pos+1].value == "+"
	default:
		return false
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

func compareExpressionAny(left any, operator string, right any) (bool, error) {
	switch operator {
	case "==":
		return compareExpressionEqual(left, right), nil
	case "!=":
		return !compareExpressionEqual(left, right), nil
	case ">", ">=", "<", "<=":
		order, ok, err := compareExpressionOrderValues(left, right)
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

func compareExpressionEqual(left any, right any) bool {
	if isNilValue(left) || isNilValue(right) {
		return isNilValue(left) && isNilValue(right)
	}
	leftComparable := comparableValue(left)
	rightComparable := comparableValue(right)
	if reflect.TypeOf(leftComparable) == reflect.TypeOf(rightComparable) {
		return reflect.DeepEqual(leftComparable, rightComparable)
	}
	return fmt.Sprint(leftComparable) == fmt.Sprint(rightComparable)
}

func compareExpressionOrderValues(left any, right any) (int, bool, error) {
	if isNilValue(left) || isNilValue(right) {
		return 0, false, nil
	}
	leftNumber, leftNumberOK := numericExpressionValue(left)
	rightNumber, rightNumberOK := numericExpressionValue(right)
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
	leftText, leftStringOK := comparableValue(left).(string)
	rightText, rightStringOK := comparableValue(right).(string)
	if leftStringOK && rightStringOK {
		return strings.Compare(leftText, rightText), true, nil
	}
	return 0, false, fmt.Errorf("goark-orm: dynamic SQL operator requires numeric or string values")
}

func addExpressionValues(left any, right any) (any, error) {
	if expressionIsString(left) || expressionIsString(right) {
		return expressionString(left) + expressionString(right), nil
	}
	leftNumber, leftOK := numericExpressionValue(left)
	rightNumber, rightOK := numericExpressionValue(right)
	if !leftOK || !rightOK {
		return nil, fmt.Errorf("goark-orm: dynamic SQL '+' requires numeric or string values")
	}
	return leftNumber + rightNumber, nil
}

func numericBinaryExpression(left any, right any, operator string) (any, error) {
	leftNumber, leftOK := numericExpressionValue(left)
	rightNumber, rightOK := numericExpressionValue(right)
	if !leftOK || !rightOK {
		return nil, fmt.Errorf("goark-orm: dynamic SQL operator %s requires numeric values", operator)
	}
	switch operator {
	case "-":
		return leftNumber - rightNumber, nil
	case "*":
		return leftNumber * rightNumber, nil
	case "/":
		if rightNumber == 0 {
			return nil, fmt.Errorf("goark-orm: dynamic SQL division by zero")
		}
		return leftNumber / rightNumber, nil
	case "%":
		if rightNumber == 0 {
			return nil, fmt.Errorf("goark-orm: dynamic SQL modulo by zero")
		}
		return math.Mod(leftNumber, rightNumber), nil
	default:
		return nil, fmt.Errorf("goark-orm: unsupported numeric operator %q", operator)
	}
}

func expressionContains(collection any, item any) bool {
	if isNilValue(collection) {
		return false
	}
	if text, ok := comparableValue(collection).(string); ok {
		return strings.Contains(text, expressionString(item))
	}
	current := reflect.ValueOf(collection)
	current, nilValue := dereferenceParameterValue(current)
	if nilValue {
		return false
	}
	switch current.Kind() {
	case reflect.Array, reflect.Slice:
		for index := 0; index < current.Len(); index++ {
			if compareExpressionEqual(current.Index(index).Interface(), item) {
				return true
			}
		}
		return false
	case reflect.Map:
		return expressionMapContainsKey(collection, item)
	default:
		return false
	}
}

func expressionMapContainsKey(value any, key any) bool {
	if isNilValue(value) {
		return false
	}
	current := reflect.ValueOf(value)
	current, nilValue := dereferenceParameterValue(current)
	if nilValue || current.Kind() != reflect.Map {
		return false
	}
	lookup := reflect.ValueOf(key)
	if !lookup.IsValid() {
		return false
	}
	keyType := current.Type().Key()
	if !lookup.Type().AssignableTo(keyType) {
		if !lookup.Type().ConvertibleTo(keyType) {
			return false
		}
		lookup = lookup.Convert(keyType)
	}
	return current.MapIndex(lookup).IsValid()
}

func expressionMapContainsValue(value any, item any) bool {
	if isNilValue(value) {
		return false
	}
	current := reflect.ValueOf(value)
	current, nilValue := dereferenceParameterValue(current)
	if nilValue || current.Kind() != reflect.Map {
		return false
	}
	iter := current.MapRange()
	for iter.Next() {
		if compareExpressionEqual(iter.Value().Interface(), item) {
			return true
		}
	}
	return false
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
	for _, suffix := range []string{".isEmpty()", ".empty"} {
		if strings.HasSuffix(raw, suffix) {
			value, ok := expressionLength(strings.TrimSpace(raw[:len(raw)-len(suffix)]), lookup)
			if !ok {
				return nil, false
			}
			return value == 0, true
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
	return expressionLengthValue(value)
}

func expressionLengthValue(value any) (int, bool) {
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
		return 0, false
	}
}

func isEmptyExpressionValue(value any) bool {
	if isNilValue(value) {
		return true
	}
	if length, ok := expressionLengthValue(value); ok {
		return length == 0
	}
	return !truthy(value)
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

func comparableValue(value any) any {
	switch item := value.(type) {
	case []byte:
		return string(item)
	default:
		return item
	}
}

func expressionIsString(value any) bool {
	switch comparableValue(value).(type) {
	case string:
		return true
	default:
		return false
	}
}

func expressionString(value any) string {
	if isNilValue(value) {
		return ""
	}
	return fmt.Sprint(comparableValue(value))
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
