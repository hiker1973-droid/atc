package miz

import (
	"fmt"
	"strconv"
	"strings"
)

// parseLuaTable reads the data-only Lua DCS writes into a .miz ("mission =
// { ... }", "dictionary = { ... }"): tables, strings, numbers, booleans and
// nil. Tables come back as map[string]any; array entries are keyed "1",
// "2", ... as Lua numbers them. The weather code keeps its regexes; this is
// for the nested parts (coalitions, groups, units) a regex cannot follow.
func parseLuaTable(src string) (map[string]any, error) {
	i := strings.IndexByte(src, '{')
	if i < 0 {
		return nil, fmt.Errorf("no table")
	}
	p := &luaParser{s: src, i: i}
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	t, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not a table")
	}
	return t, nil
}

type luaParser struct {
	s string
	i int
}

func (p *luaParser) errf(format string, a ...any) error {
	return fmt.Errorf("lua at %d: %s", p.i, fmt.Sprintf(format, a...))
}

// skip passes whitespace and comments.
func (p *luaParser) skip() {
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			p.i++
		case strings.HasPrefix(p.s[p.i:], "--[["):
			if e := strings.Index(p.s[p.i:], "]]"); e >= 0 {
				p.i += e + 2
			} else {
				p.i = len(p.s)
			}
		case strings.HasPrefix(p.s[p.i:], "--"):
			if e := strings.IndexByte(p.s[p.i:], '\n'); e >= 0 {
				p.i += e + 1
			} else {
				p.i = len(p.s)
			}
		default:
			return
		}
	}
}

func (p *luaParser) value() (any, error) {
	p.skip()
	if p.i >= len(p.s) {
		return nil, p.errf("unexpected end")
	}
	c := p.s[p.i]
	switch {
	case c == '{':
		return p.table()
	case c == '"' || c == '\'':
		return p.str(c)
	case c == '[' && p.i+1 < len(p.s) && (p.s[p.i+1] == '[' || p.s[p.i+1] == '='):
		return p.longStr()
	case c == '-' || c == '.' || (c >= '0' && c <= '9'):
		return p.num()
	default:
		id := p.ident()
		switch id {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "nil":
			return nil, nil
		}
		return nil, p.errf("unexpected %q", id)
	}
}

func (p *luaParser) table() (any, error) {
	p.i++ // {
	t := map[string]any{}
	n := 0
	for {
		p.skip()
		if p.i >= len(p.s) {
			return nil, p.errf("unclosed table")
		}
		if p.s[p.i] == '}' {
			p.i++
			return t, nil
		}
		var key string
		switch {
		case p.s[p.i] == '[' && p.i+1 < len(p.s) && p.s[p.i+1] != '[' && p.s[p.i+1] != '=':
			p.i++
			k, err := p.value()
			if err != nil {
				return nil, err
			}
			p.skip()
			if p.i >= len(p.s) || p.s[p.i] != ']' {
				return nil, p.errf("want ]")
			}
			p.i++
			p.skip()
			if p.i >= len(p.s) || p.s[p.i] != '=' {
				return nil, p.errf("want =")
			}
			p.i++
			key = luaKey(k)
		case isIdentStart(p.s[p.i]):
			save := p.i
			id := p.ident()
			p.skip()
			if p.i < len(p.s) && p.s[p.i] == '=' && (p.i+1 >= len(p.s) || p.s[p.i+1] != '=') {
				p.i++
				key = id
			} else { // a bare value: true, false, nil
				p.i = save
			}
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		if key == "" {
			n++
			key = strconv.Itoa(n)
		}
		t[key] = v
		p.skip()
		if p.i < len(p.s) && (p.s[p.i] == ',' || p.s[p.i] == ';') {
			p.i++
		}
	}
}

func luaKey(k any) string {
	switch v := k.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return fmt.Sprint(k)
}

func (p *luaParser) str(q byte) (any, error) {
	p.i++
	var b strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == q:
			p.i++
			return b.String(), nil
		case c == '\\' && p.i+1 < len(p.s):
			p.i++
			switch e := p.s[p.i]; e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
			case '\n':
				b.WriteByte('\n')
			default:
				b.WriteByte(e)
			}
			p.i++
		default:
			b.WriteByte(c)
			p.i++
		}
	}
	return nil, p.errf("unclosed string")
}

func (p *luaParser) longStr() (any, error) {
	p.i++ // first [
	level := 0
	for p.i < len(p.s) && p.s[p.i] == '=' {
		level++
		p.i++
	}
	if p.i >= len(p.s) || p.s[p.i] != '[' {
		return nil, p.errf("bad long string")
	}
	p.i++
	end := "]" + strings.Repeat("=", level) + "]"
	e := strings.Index(p.s[p.i:], end)
	if e < 0 {
		return nil, p.errf("unclosed long string")
	}
	s := strings.TrimPrefix(p.s[p.i:p.i+e], "\n")
	p.i += e + len(end)
	return s, nil
}

func (p *luaParser) num() (any, error) {
	start := p.i
	for p.i < len(p.s) && strings.IndexByte("0123456789+-.eExXabcdefABCDEF", p.s[p.i]) >= 0 {
		p.i++
	}
	f, err := strconv.ParseFloat(p.s[start:p.i], 64)
	if err != nil {
		if n, err2 := strconv.ParseInt(p.s[start:p.i], 0, 64); err2 == nil {
			return float64(n), nil
		}
		return nil, p.errf("bad number %q", p.s[start:p.i])
	}
	return f, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func (p *luaParser) ident() string {
	start := p.i
	for p.i < len(p.s) && (isIdentStart(p.s[p.i]) || (p.s[p.i] >= '0' && p.s[p.i] <= '9')) {
		p.i++
	}
	if p.i == start && p.i < len(p.s) {
		p.i++ // never stall on an unexpected character
	}
	return p.s[start:p.i]
}

// Lua table helpers.

func tbl(v any, keys ...string) map[string]any {
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	m, _ := v.(map[string]any)
	return m
}

// items is a Lua array's values in order.
func items(m map[string]any) []any {
	var out []any
	for i := 1; ; i++ {
		v, ok := m[strconv.Itoa(i)]
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

func strOf(m map[string]any, k string) string {
	switch v := m[k].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

func numOf(m map[string]any, k string) float64 {
	f, _ := m[k].(float64)
	return f
}
