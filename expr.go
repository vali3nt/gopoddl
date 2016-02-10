package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

/////////////////////////////////////////////////////////////////////
/// Expression
/////////////////////////////////////////////////////////////////////

type Expression struct {
	lText     string            // text to search
	rText     string            // text where to search
	prefix    bool              // search like '%str'
	suffix    bool              // search like 'str%'
	not       bool              // revers value
	in        bool              // just for syntax check
	completed bool              // epression has all data
	result    bool              // current result
	data      map[string]string // data replace variable
}

// evaluate completed expression
func (e *Expression) Evaluate() error {
	res := false
	if !e.completed || !e.in {
		return errors.New("malformed expression: one of the argument is missing")
	}
	if e.prefix {
		res = strings.HasPrefix(e.rText, e.lText)
	} else if e.suffix {
		res = strings.HasSuffix(e.rText, e.lText)
	} else {
		res = strings.Contains(e.rText, e.lText)
	}
	if e.not {
		res = !res
	}
	e.result = res
	e.Reset() // result saved, no need other values
	return nil
}

// add left or rigt string
func (e *Expression) AddString(s string) error {
	if e.lText == "" {
		e.lText = s
	} else if e.rText == "" {
		e.rText = s
		e.completed = true
	} else {
		return errors.New("malformed expression: too many input arguments")
	}
	return nil
}

func (e *Expression) AddVariable(s string) error {
	if val, ok := e.data[s]; !ok {
		return errors.New("Variable " + s + " was not found in provided data")
	} else {
		if err := e.AddString(val); err != nil {
			return err
		}
	}
	return nil
}

func (e *Expression) AddKeyword(k string) error {
	switch k {
	case "in":
		if !e.in && e.lText != "" && e.rText == "" {
			e.in = true
		} else {
			return errors.New("malformed expression: " + k)
		}
	case "not":
		if !e.not && !e.in && e.lText != "" {
			e.not = true
		} else {
			return errors.New("malformed expression: " + k)
		}
	case "prefix":
		if !e.prefix && e.in && e.lText != "" && e.rText == "" {
			e.prefix = true
		} else {
			return errors.New("malformed expression: " + k)
		}
	case "suffix":
		if !e.suffix && e.in && e.lText != "" && e.rText == "" {
			e.suffix = true
		} else {
			return errors.New("malformed expression: " + k)
		}
	default:
		return errors.New("malformed expression: uknown keyword: " + k)
	}
	return nil
}

// reset experssion data to default
func (e *Expression) Reset() {
	e.lText = ""
	e.rText = ""
	e.prefix = false
	e.suffix = false
	e.not = false
	e.in = false
	e.completed = false
}

/////////////////////////////////////////////////////////////////////
/// Lexer
/////////////////////////////////////////////////////////////////////

type itemType int

const (
	itemExpr itemType = iota
	itemEOF
	itemError
	itemKeyword
	itemOpenBlock  // (
	itemCloseBlock // )
	itemAnd        // "and" keyword
	itemOr         // "or"  keyword
)

const eof = -1

type item struct {
	typ itemType
	err string
	val bool
}

type stateFn func(*lexer) stateFn

// lexer holds the state of the scanner.
type lexer struct {
	input      string     // the string being scanned.
	start      int        // start position of this item.
	pos        int        // current position in the input.
	width      int        // width of last rune read from input.
	items      chan item  // channel of scanned items.
	parenDepth int        // nesting depth of ( )
	expression Expression // hold current expression
}

func lex(input string, data map[string]string) *lexer {
	l := &lexer{
		input:      input,
		items:      make(chan item),
		expression: Expression{data: data},
	}
	go l.run() // Concurrently run state machine.
	return l
}

func (l *lexer) nextItem() item {
	item := <-l.items
	return item
}

// error returns an error token and terminates the scan
// by passing back a nil pointer that will be the next
// state, terminating l.run.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{
		typ: itemError,
		err: fmt.Sprintf(format, args...) + fmt.Sprintf(", pos : %d", l.pos),
	}
	return nil
}

func (l *lexer) emit(t itemType, s stateFn) stateFn {
	switch {
	case t > itemKeyword || t == itemEOF:
		l.items <- item{typ: t}
	case t == itemExpr:
		if l.expression.completed {
			if err := l.expression.Evaluate(); err != nil {
				return l.errorf("%s", err)
			} else {
				l.items <- item{val: l.expression.result}
				l.expression.Reset()
			}
		}
	}
	l.start = l.pos
	return s
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = w
	l.pos += l.width
	return r
}

// ignore skips over the pending input before this point.
func (l *lexer) ignore() {
	l.start = l.pos
}

// get current rune
func (l *lexer) current() rune {
	r, _ := utf8.DecodeRuneInString(l.input[l.pos:])
	return r
}

// backup steps back one rune.
// Can be called only once per call of next.
func (l *lexer) backup() {
	l.pos -= l.width
}

// peek returns but does not consume
// the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// accept consumes the next rune
// if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

func isEndOfLine(r rune) bool {
	return r == '\r' || r == '\n'
}

func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func (l *lexer) run() {
	for state := lexExpr; state != nil; {
		state = state(l)
	}
	close(l.items) // No more tokens will be delivered.
}

func (l *lexer) getVal() string {
	return l.input[l.start:l.pos]
}

func lexExpr(l *lexer) stateFn {
	for {
		switch r := l.next(); {
		case isSpace(r): // ignore spaces
			l.ignore()
			continue

		case r == eof || isEndOfLine(r): // exit condition
			if l.parenDepth != 0 {
				return l.errorf("unclosed left paren")
			}
			return l.emit(itemEOF, nil)

		case r == '\'' || r == '"': // handle single and double quotes
			l.backup()
			return lexQuote

		case r == '{':
			if tmp := l.next(); tmp == '{' { // handle vars: {{var}}
				return lexVariable
			} else {
				return l.errorf("unrecognized character: %#U", r)
			}

		case isAlphaNumeric(r): // handle any kywords, order will be checked later
			return lexKeyword
		case r == '(':
			l.parenDepth++
			return l.emit(itemOpenBlock, lexExpr)

		case r == ')':
			l.parenDepth--
			if l.parenDepth < 0 {
				return l.errorf("unexpected right paren %#U", r)
			}
			return l.emit(itemCloseBlock, lexExpr)
		default:
			return l.errorf("unrecognized character: %#U", r)
		}
	}
	return l.emit(itemEOF, nil)
}

func lexQuote(l *lexer) stateFn {
	curQuote := l.current() // take current rune to find it's pair
	l.pos += 1              // ignore left quote
	l.ignore()
	for {
		switch r := l.next(); {
		case isEndOfLine(r):
			return l.errorf("unterminated quoted string")
		case r == curQuote:
			l.backup() // exclude right quote
			if err := l.expression.AddString(l.getVal()); err != nil {
				return l.errorf("%s", err)
			}
			l.next() // ignore right quote
			//l.pos += 1
			l.ignore()
			return l.emit(itemExpr, lexExpr)
		}
	}
	return nil
}

func lexVariable(l *lexer) stateFn {
	l.ignore() // ignore left braket {{
	for {
		switch r := l.next(); {
		case r == '}':
			if tmp := l.next(); tmp == '}' {
				if l.start == l.pos {
					return l.errorf("variable name cannot be empty")
				}
				l.pos -= 2 // exclude right }}
				if err := l.expression.AddVariable(l.getVal()); err != nil {
					return l.errorf("%s", err)
				}
				l.pos += 2
				l.ignore() // ignore right }}
				return l.emit(itemExpr, lexExpr)
			}
		case isAlphaNumeric(r):
			// absorb.
		case isEndOfLine(r):
			return l.errorf("unexpected EOL")
		default:
			return l.errorf("unclosed left paren")
		}
	}
	return nil
}

func lexKeyword(l *lexer) stateFn {
	for { // TODO move braces here
		switch r := l.next(); {
		case !isAlphaNumeric(r): // consume all alphanumeric
			l.backup()
			w := l.getVal()
			switch w {
			case "and":
				return l.emit(itemAnd, lexExpr)
			case "or":
				return l.emit(itemOr, lexExpr)
			default:
				if err := l.expression.AddKeyword(w); err != nil {
					return l.errorf("%s", err)
				}
				return l.emit(itemExpr, lexExpr)
			}
		}
	}
	return nil
}

/////////////////////////////////////////////////////////////////////
/// Main
/////////////////////////////////////////////////////////////////////

// Evaluate filter and return boolean result
// filter example : "SOMETEXT" in {{token}}
func processFilter(l *lexer) (bool, error) {
	result := false
L:
	for {
		item := l.nextItem()
		switch item.typ {
		case itemEOF:
			break L
		case itemError:
			return false, errors.New(item.err)

		case itemExpr:
			result = item.val

		case itemAnd, itemOr:
			nextResult := l.nextItem()
			var v bool
			switch nextResult.typ {
			case itemExpr:
				v = nextResult.val
			case itemOpenBlock:
				if tmp, err := processFilter(l); err != nil {
					return false, err
				} else {
					v = tmp
				}
			case itemError:
				return false, errors.New(item.err)
			}
			if item.typ == itemAnd {
				result = result && v
			} else {
				result = result || v
			}

		case itemOpenBlock:
			if tmp, err := processFilter(l); err != nil {
				return false, err
			} else {
				result = tmp
			}
		case itemCloseBlock:
			break L
		}
	}
	return result, nil
}

// replace tokens in string, token names are keys in data map
func EvalFormat(s string, data map[string]string) string {
	re := regexp.MustCompile("{{[A-Za-z]+}}")
	res := re.ReplaceAllStringFunc(s, func(s string) string {

		if v, ok := data[s[2:len(s)-2]]; ok {
			return v
		} else {
			return ""
		}
	})
	return res
}

func EvalFilter(s string, data map[string]string) (bool, error) {
	l := lex(s, data)
	return processFilter(l)
}
