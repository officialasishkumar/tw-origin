package spec

import (
	"strconv"
	"strings"
)

// evalIf evaluates an %if expression whose macros have already been
// expanded. ok is false when the expression still contains macros or uses
// syntax the evaluator does not know; the caller then takes the branch.
func evalIf(expr string) (val bool, ok bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" || strings.Contains(expr, "%") {
		return false, false
	}
	p := &parser{toks: tokenize(expr)}
	if p.toks == nil {
		return false, false
	}
	v, ok := p.or()
	if !ok || p.pos != len(p.toks) {
		return false, false
	}
	return v.truthy(), true
}

type value struct {
	s     string
	n     int64
	isInt bool
}

func (v value) truthy() bool {
	if v.isInt {
		return v.n != 0
	}
	return v.s != ""
}

func (v value) str() string {
	if v.isInt {
		return strconv.FormatInt(v.n, 10)
	}
	return v.s
}

type token struct {
	kind byte // 'n' number, 's' string, 'o' operator
	text string
}

func tokenize(expr string) []token {
	var toks []token
	for i := 0; i < len(expr); {
		c := expr[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '"':
			j := i + 1
			for j < len(expr) && expr[j] != '"' {
				j++
			}
			if j >= len(expr) {
				return nil
			}
			toks = append(toks, token{'s', expr[i+1 : j]})
			i = j + 1
		case c == 'v' && i+1 < len(expr) && expr[i+1] == '"': // v"1.2" version literal
			i++
		case c >= '0' && c <= '9':
			j := i
			for j < len(expr) && expr[j] >= '0' && expr[j] <= '9' {
				j++
			}
			if j < len(expr) && (isWord(expr[j]) || expr[j] == '.') { // 1.2.3 or 0abc: a string
				for j < len(expr) && (isWord(expr[j]) || expr[j] == '.' || expr[j] == '-' || expr[j] == '+') {
					j++
				}
				toks = append(toks, token{'s', expr[i:j]})
			} else {
				toks = append(toks, token{'n', expr[i:j]})
			}
			i = j
		case isWord(c):
			j := i
			for j < len(expr) && (isWord(expr[j]) || expr[j] == '.' || expr[j] == '-' || expr[j] == '+') {
				j++
			}
			toks = append(toks, token{'s', expr[i:j]}) // bare word: compared as a string
			i = j
		default:
			op := matchOp(expr[i:])
			if op == "" {
				return nil
			}
			toks = append(toks, token{'o', op})
			i += len(op)
		}
	}
	return toks
}

var operators = []string{"&&", "||", "==", "!=", "<=", ">=", "<", ">", "!", "(", ")", "+", "-", "*", "/"}

func matchOp(s string) string {
	for _, op := range operators {
		if strings.HasPrefix(s, op) {
			return op
		}
	}
	return ""
}

type parser struct {
	toks []token
	pos  int
}

func (p *parser) peekOp(ops ...string) (string, bool) {
	if p.pos >= len(p.toks) || p.toks[p.pos].kind != 'o' {
		return "", false
	}
	for _, op := range ops {
		if p.toks[p.pos].text == op {
			return op, true
		}
	}
	return "", false
}

func (p *parser) or() (value, bool) {
	l, ok := p.and()
	if !ok {
		return l, false
	}
	for {
		if _, is := p.peekOp("||"); !is {
			return l, true
		}
		p.pos++
		r, ok := p.and()
		if !ok {
			return l, false
		}
		l = boolVal(l.truthy() || r.truthy())
	}
}

func (p *parser) and() (value, bool) {
	l, ok := p.cmp()
	if !ok {
		return l, false
	}
	for {
		if _, is := p.peekOp("&&"); !is {
			return l, true
		}
		p.pos++
		r, ok := p.cmp()
		if !ok {
			return l, false
		}
		l = boolVal(l.truthy() && r.truthy())
	}
}

func (p *parser) cmp() (value, bool) {
	l, ok := p.add()
	if !ok {
		return l, false
	}
	op, is := p.peekOp("==", "!=", "<=", ">=", "<", ">")
	if !is {
		return l, true
	}
	p.pos++
	r, ok := p.add()
	if !ok {
		return l, false
	}
	var c int
	if l.isInt && r.isInt {
		switch {
		case l.n < r.n:
			c = -1
		case l.n > r.n:
			c = 1
		}
	} else {
		c = strings.Compare(l.str(), r.str())
	}
	switch op {
	case "==":
		return boolVal(c == 0), true
	case "!=":
		return boolVal(c != 0), true
	case "<":
		return boolVal(c < 0), true
	case ">":
		return boolVal(c > 0), true
	case "<=":
		return boolVal(c <= 0), true
	}
	return boolVal(c >= 0), true
}

func (p *parser) add() (value, bool) {
	l, ok := p.mul()
	if !ok {
		return l, false
	}
	for {
		op, is := p.peekOp("+", "-")
		if !is {
			return l, true
		}
		p.pos++
		r, ok := p.mul()
		if !ok {
			return l, false
		}
		switch {
		case op == "+" && !(l.isInt && r.isInt):
			l = value{s: l.str() + r.str()}
		case op == "+":
			l = value{n: l.n + r.n, isInt: true}
		case l.isInt && r.isInt:
			l = value{n: l.n - r.n, isInt: true}
		default:
			return l, false
		}
	}
}

func (p *parser) mul() (value, bool) {
	l, ok := p.unary()
	if !ok {
		return l, false
	}
	for {
		op, is := p.peekOp("*", "/")
		if !is {
			return l, true
		}
		p.pos++
		r, ok := p.unary()
		if !ok || !l.isInt || !r.isInt || (op == "/" && r.n == 0) {
			return l, false
		}
		if op == "*" {
			l = value{n: l.n * r.n, isInt: true}
		} else {
			l = value{n: l.n / r.n, isInt: true}
		}
	}
}

func (p *parser) unary() (value, bool) {
	if _, is := p.peekOp("!"); is {
		p.pos++
		v, ok := p.unary()
		return boolVal(!v.truthy()), ok
	}
	if _, is := p.peekOp("-"); is {
		p.pos++
		v, ok := p.unary()
		if !ok || !v.isInt {
			return v, false
		}
		return value{n: -v.n, isInt: true}, true
	}
	if _, is := p.peekOp("("); is {
		p.pos++
		v, ok := p.or()
		if !ok {
			return v, false
		}
		if _, is := p.peekOp(")"); !is {
			return v, false
		}
		p.pos++
		return v, true
	}
	if p.pos >= len(p.toks) {
		return value{}, false
	}
	t := p.toks[p.pos]
	p.pos++
	switch t.kind {
	case 'n':
		n, err := strconv.ParseInt(t.text, 10, 64)
		if err != nil {
			return value{s: t.text}, true
		}
		return value{n: n, isInt: true}, true
	case 's':
		return value{s: t.text}, true
	}
	return value{}, false
}

func boolVal(b bool) value {
	if b {
		return value{n: 1, isInt: true}
	}
	return value{isInt: true}
}
