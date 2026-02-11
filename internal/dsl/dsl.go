package dsl

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// ErrDivideByZero is returned when a DSL expression divides by zero.
var ErrDivideByZero = errors.New("DIVIDE_BY_ZERO")

// ============================================================
// Result
// ============================================================

// EvalResult holds the evaluation output.
type EvalResult struct {
	Value bool    // final boolean result
	Error string  // "DIVIDE_BY_ZERO" if error occurred
	Raw   float64 // raw numeric result before boolean conversion
}

// Evaluate parses and evaluates a DSL expression against a counts map.
// counts maps ident names (e.g. "person") to their detection count.
func Evaluate(expression string, counts map[string]float64) (EvalResult, error) {
	tokens, err := tokenize(expression)
	if err != nil {
		return EvalResult{}, fmt.Errorf("tokenize: %w", err)
	}

	p := &parser{tokens: tokens}
	node, err := p.parseExpr()
	if err != nil {
		return EvalResult{}, fmt.Errorf("parse: %w", err)
	}
	if p.pos < len(p.tokens) && p.tokens[p.pos].typ != tokEOF {
		return EvalResult{}, fmt.Errorf("parse: unexpected token '%s' at position %d", p.tokens[p.pos].val, p.pos)
	}

	val, err := eval(node, counts)
	if err != nil {
		if errors.Is(err, ErrDivideByZero) {
			return EvalResult{Value: false, Error: "DIVIDE_BY_ZERO", Raw: 0}, nil
		}
		return EvalResult{}, err
	}

	return EvalResult{Value: val != 0, Raw: val}, nil
}

// Validate checks if a DSL expression is syntactically valid and all idents exist in allowedIdents.
func Validate(expression string, allowedIdents []string) error {
	tokens, err := tokenize(expression)
	if err != nil {
		return fmt.Errorf("tokenize: %w", err)
	}

	p := &parser{tokens: tokens}
	node, err := p.parseExpr()
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if p.pos < len(p.tokens) && p.tokens[p.pos].typ != tokEOF {
		return fmt.Errorf("unexpected token '%s'", p.tokens[p.pos].val)
	}

	if allowedIdents != nil {
		allowed := make(map[string]bool, len(allowedIdents))
		for _, id := range allowedIdents {
			allowed[id] = true
		}
		for _, id := range collectIdents(node) {
			if !allowed[id] {
				return fmt.Errorf("unknown identifier '%s'", id)
			}
		}
	}

	return nil
}

// ============================================================
// Token
// ============================================================

type tokenType int

const (
	tokNumber tokenType = iota
	tokIdent
	tokPlus
	tokMinus
	tokStar
	tokSlash
	tokGT
	tokLT
	tokGTE
	tokLTE
	tokEQ
	tokNEQ
	tokAND
	tokOR
	tokNOT
	tokLParen
	tokRParen
	tokEOF
)

type token struct {
	typ tokenType
	val string
}

func tokenize(input string) ([]token, error) {
	var tokens []token
	i := 0
	runes := []rune(input)

	for i < len(runes) {
		ch := runes[i]

		// skip whitespace
		if unicode.IsSpace(ch) {
			i++
			continue
		}

		// numbers
		if unicode.IsDigit(ch) {
			start := i
			for i < len(runes) && unicode.IsDigit(runes[i]) {
				i++
			}
			// allow decimal
			if i < len(runes) && runes[i] == '.' {
				i++
				for i < len(runes) && unicode.IsDigit(runes[i]) {
					i++
				}
			}
			tokens = append(tokens, token{tokNumber, string(runes[start:i])})
			continue
		}

		// identifiers and keywords
		if unicode.IsLetter(ch) || ch == '_' {
			start := i
			for i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i]) || runes[i] == '_') {
				i++
			}
			word := string(runes[start:i])
			switch strings.ToUpper(word) {
			case "AND":
				tokens = append(tokens, token{tokAND, word})
			case "OR":
				tokens = append(tokens, token{tokOR, word})
			case "NOT":
				tokens = append(tokens, token{tokNOT, word})
			default:
				// identifier를 소문자로 변환
				tokens = append(tokens, token{tokIdent, strings.ToLower(word)})
			}
			continue
		}

		// two-char operators
		if i+1 < len(runes) {
			two := string(runes[i : i+2])
			switch two {
			case ">=":
				tokens = append(tokens, token{tokGTE, two})
				i += 2
				continue
			case "<=":
				tokens = append(tokens, token{tokLTE, two})
				i += 2
				continue
			case "==":
				tokens = append(tokens, token{tokEQ, two})
				i += 2
				continue
			case "!=":
				tokens = append(tokens, token{tokNEQ, two})
				i += 2
				continue
			}
		}

		// single-char operators
		switch ch {
		case '+':
			tokens = append(tokens, token{tokPlus, "+"})
		case '-':
			tokens = append(tokens, token{tokMinus, "-"})
		case '*':
			tokens = append(tokens, token{tokStar, "*"})
		case '/':
			tokens = append(tokens, token{tokSlash, "/"})
		case '>':
			tokens = append(tokens, token{tokGT, ">"})
		case '<':
			tokens = append(tokens, token{tokLT, "<"})
		case '(':
			tokens = append(tokens, token{tokLParen, "("})
		case ')':
			tokens = append(tokens, token{tokRParen, ")"})
		case '!':
			tokens = append(tokens, token{tokNOT, "!"})
		default:
			return nil, fmt.Errorf("unexpected character '%c' at position %d", ch, i)
		}
		i++
	}

	tokens = append(tokens, token{tokEOF, ""})
	return tokens, nil
}

// ============================================================
// AST
// ============================================================

type node interface {
	nodeTag()
}

type numberNode struct {
	value float64
}

type identNode struct {
	name string
}

type binaryNode struct {
	op    tokenType
	left  node
	right node
}

type unaryNode struct {
	op      tokenType
	operand node
}

func (numberNode) nodeTag() {}
func (identNode) nodeTag()  {}
func (binaryNode) nodeTag() {}
func (unaryNode) nodeTag()  {}

// ============================================================
// Parser (recursive descent, precedence climbing)
// ============================================================
//
// Precedence (low → high):
//   7. OR
//   6. AND
//   5. > < >= <= == !=
//   4. + -
//   3. * /
//   2. NOT / !
//   1. () / number / ident

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return token{tokEOF, ""}
}

func (p *parser) advance() token {
	t := p.peek()
	if t.typ != tokEOF {
		p.pos++
	}
	return t
}

func (p *parser) expect(typ tokenType) (token, error) {
	t := p.advance()
	if t.typ != typ {
		return t, fmt.Errorf("expected token type %d, got '%s'", typ, t.val)
	}
	return t, nil
}

// parseExpr = or
func (p *parser) parseExpr() (node, error) {
	return p.parseOr()
}

// or = and ( "OR" and )*
func (p *parser) parseOr() (node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek().typ == tokOR {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: tokOR, left: left, right: right}
	}
	return left, nil
}

// and = comparison ( "AND" comparison )*
func (p *parser) parseAnd() (node, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.peek().typ == tokAND {
		p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: tokAND, left: left, right: right}
	}
	return left, nil
}

// comparison = addition ( ( ">" | "<" | ">=" | "<=" | "==" | "!=" ) addition )?
func (p *parser) parseComparison() (node, error) {
	left, err := p.parseAddition()
	if err != nil {
		return nil, err
	}
	switch p.peek().typ {
	case tokGT, tokLT, tokGTE, tokLTE, tokEQ, tokNEQ:
		op := p.advance()
		right, err := p.parseAddition()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op.typ, left: left, right: right}
	}
	return left, nil
}

// addition = multiplication ( ("+" | "-") multiplication )*
func (p *parser) parseAddition() (node, error) {
	left, err := p.parseMultiplication()
	if err != nil {
		return nil, err
	}
	for p.peek().typ == tokPlus || p.peek().typ == tokMinus {
		op := p.advance()
		right, err := p.parseMultiplication()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op.typ, left: left, right: right}
	}
	return left, nil
}

// multiplication = unary ( ("*" | "/") unary )*
func (p *parser) parseMultiplication() (node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek().typ == tokStar || p.peek().typ == tokSlash {
		op := p.advance()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op.typ, left: left, right: right}
	}
	return left, nil
}

// unary = ("NOT" | "!") unary | primary
func (p *parser) parseUnary() (node, error) {
	if p.peek().typ == tokNOT {
		p.advance()
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return unaryNode{op: tokNOT, operand: operand}, nil
	}
	return p.parsePrimary()
}

// primary = "(" expr ")" | NUMBER | IDENT
func (p *parser) parsePrimary() (node, error) {
	t := p.peek()

	switch t.typ {
	case tokLParen:
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokRParen); err != nil {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		return expr, nil

	case tokNumber:
		p.advance()
		v, err := strconv.ParseFloat(t.val, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number '%s'", t.val)
		}
		return numberNode{value: v}, nil

	case tokIdent:
		p.advance()
		return identNode{name: t.val}, nil

	default:
		return nil, fmt.Errorf("unexpected token '%s'", t.val)
	}
}

// ============================================================
// Evaluator
// ============================================================

func eval(n node, counts map[string]float64) (float64, error) {
	switch v := n.(type) {
	case numberNode:
		return v.value, nil

	case identNode:
		return counts[v.name], nil // missing ident → 0

	case unaryNode:
		operand, err := eval(v.operand, counts)
		if err != nil {
			return 0, err
		}
		if operand == 0 {
			return 1, nil // NOT false → true
		}
		return 0, nil // NOT true → false

	case binaryNode:
		left, err := eval(v.left, counts)
		if err != nil {
			return 0, err
		}
		right, err := eval(v.right, counts)
		if err != nil {
			return 0, err
		}

		switch v.op {
		case tokPlus:
			return left + right, nil
		case tokMinus:
			return left - right, nil
		case tokStar:
			return left * right, nil
		case tokSlash:
			if right == 0 {
				return 0, ErrDivideByZero
			}
			return float64(int64(left) / int64(right)), nil // integer division
		case tokGT:
			return boolToFloat(left > right), nil
		case tokLT:
			return boolToFloat(left < right), nil
		case tokGTE:
			return boolToFloat(left >= right), nil
		case tokLTE:
			return boolToFloat(left <= right), nil
		case tokEQ:
			return boolToFloat(left == right), nil
		case tokNEQ:
			return boolToFloat(left != right), nil
		case tokAND:
			return boolToFloat(left != 0 && right != 0), nil
		case tokOR:
			return boolToFloat(left != 0 || right != 0), nil
		default:
			return 0, fmt.Errorf("unknown operator %d", v.op)
		}

	default:
		return 0, fmt.Errorf("unknown node type %T", n)
	}
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// collectIdents returns all identifier names in the AST.
func collectIdents(n node) []string {
	var idents []string
	switch v := n.(type) {
	case identNode:
		idents = append(idents, v.name)
	case unaryNode:
		idents = append(idents, collectIdents(v.operand)...)
	case binaryNode:
		idents = append(idents, collectIdents(v.left)...)
		idents = append(idents, collectIdents(v.right)...)
	}
	return idents
}
