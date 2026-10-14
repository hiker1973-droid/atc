package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/paulmach/orb"
)

// Foothold's campaign state, read from the save file the mission writes about
// once a minute (zoneCommander BattleCommander:saveToDisk). The file is plain
// Lua: "zonePersistance = {}" and then one statement per key, the zones as
//
//	zonePersistance["zones"]["Bassel Al-Assad"] = { ["side"]=1, ["lat_long"]={...}, ... }
//
// The mission also writes Missions/Saves/foothold.status, whose first line is
// the save file's path, "for external tools". SkyEye runs on the DCS server,
// so it reads both locally. Operator 2026-10-14: Command should task from what
// the F10 map shows ("do step 1").

// footholdZone is one zone as the save file has it.
type footholdZone struct {
	Name   string
	Side   int // 0 neutral, 1 red, 2 blue
	Active bool
	Pos    orb.Point // lon, lat; zero when the save has no lat_long
	Units  []string  // DCS type names of the units still alive
}

// footholdSave reads the save file on demand and keeps the last parse until
// the file changes.
type footholdSave struct {
	paths func() []string // candidate save files, best first

	mu    sync.Mutex
	path  string
	mod   time.Time
	zones []footholdZone
}

// Zones returns the current zones, re-reading the file when it has changed.
// A read or parse failure keeps the previous zones and returns the error.
func (f *footholdSave) Zones() ([]footholdZone, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var lastErr error
	for _, p := range f.paths() {
		st, err := os.Stat(p)
		if err != nil {
			lastErr = err
			continue
		}
		if p == f.path && st.ModTime().Equal(f.mod) {
			return f.zones, nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			lastErr = err
			continue
		}
		zones, err := parseFootholdSave(string(data))
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", p, err)
			continue
		}
		f.path, f.mod, f.zones = p, st.ModTime(), zones
		return zones, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no Foothold save file found")
	}
	return f.zones, lastErr
}

// footholdSavePaths lists where the save file may be: --foothold-save if set,
// else the path in Saves/foothold.status, else the newest footholdSyria*.lua,
// under each Missions folder SkyEye knows of.
func footholdSavePaths() []string {
	if flagFootholdSave != "" {
		return []string{flagFootholdSave}
	}
	var out []string
	for _, missions := range footholdMissionDirs() {
		saves := filepath.Join(missions, "Saves")
		if b, err := os.ReadFile(filepath.Join(saves, "foothold.status")); err == nil {
			if p := strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0]); p != "" {
				out = append(out, filepath.FromSlash(p))
			}
		}
		matches, _ := filepath.Glob(filepath.Join(saves, "foothold*.lua"))
		sort.Slice(matches, func(i, j int) bool {
			a, _ := os.Stat(matches[i])
			b, _ := os.Stat(matches[j])
			return a != nil && b != nil && a.ModTime().After(b.ModTime())
		})
		for _, m := range matches {
			if !strings.Contains(strings.ToLower(filepath.Base(m)), "rank") {
				out = append(out, m)
			}
		}
	}
	return out
}

// footholdMissionDirs is every "Missions" folder that the mission paths SkyEye
// was given sit under: --miz-path, the hook's current_mission.txt, --miz-dir.
func footholdMissionDirs() []string {
	var cands []string
	if flagMizPath != "" {
		cands = append(cands, flagMizPath)
	}
	if b, err := os.ReadFile(flagMizWatchFile); err == nil {
		cands = append(cands, strings.TrimSpace(string(b)))
	}
	if flagMizDir != "" {
		cands = append(cands, filepath.Join(flagMizDir, "x"))
	}
	seen := map[string]bool{}
	var out []string
	for _, c := range cands {
		for d := filepath.Dir(c); d != filepath.Dir(d); d = filepath.Dir(d) {
			if strings.EqualFold(filepath.Base(d), "Missions") {
				if !seen[strings.ToLower(d)] {
					seen[strings.ToLower(d)] = true
					out = append(out, d)
				}
				break
			}
		}
	}
	return out
}

// parseFootholdSave pulls the zones out of a Foothold save file.
func parseFootholdSave(src string) ([]footholdZone, error) {
	p := &luaParser{src: src}
	var zones []footholdZone
	for {
		p.skipSpace()
		if p.pos >= len(p.src) {
			break
		}
		keys, val, err := p.statement()
		if err != nil {
			return nil, err
		}
		if len(keys) != 2 || keys[0] != "zones" {
			continue
		}
		t, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		z := footholdZone{Name: keys[1], Side: int(luaNum(t["side"])), Active: t["active"] != false}
		if ll, ok := t["lat_long"].(map[string]interface{}); ok {
			z.Pos = orb.Point{luaNum(ll["longitude"]), luaNum(ll["latitude"])}
		}
		if ru, ok := t["remainingUnits"].(map[string]interface{}); ok {
			for _, slot := range sortedKeys(ru) {
				if units, ok := ru[slot].(map[string]interface{}); ok {
					for _, k := range sortedKeys(units) {
						if s, ok := units[k].(string); ok {
							z.Units = append(z.Units, s)
						}
					}
				}
			}
		}
		zones = append(zones, z)
	}
	sort.Slice(zones, func(i, j int) bool { return zones[i].Name < zones[j].Name })
	return zones, nil
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, ea := strconv.Atoi(keys[i])
		b, eb := strconv.Atoi(keys[j])
		if ea == nil && eb == nil {
			return a < b
		}
		return keys[i] < keys[j]
	})
	return keys
}

func luaNum(v interface{}) float64 {
	f, _ := v.(float64)
	return f
}

// luaParser reads the subset of Lua that Utils.saveTable writes: assignment
// statements to a name with bracketed keys, and values that are tables with
// [key]= entries, %q strings, numbers, booleans and nil. Tables come back as
// map[string]interface{} (numeric keys as their decimal text).
type luaParser struct {
	src string
	pos int
}

func (p *luaParser) errorf(format string, a ...interface{}) error {
	line := 1 + strings.Count(p.src[:p.pos], "\n")
	return fmt.Errorf("line %d: %s", line, fmt.Sprintf(format, a...))
}

func (p *luaParser) skipSpace() {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ';':
			p.pos++
		case strings.HasPrefix(p.src[p.pos:], "--"):
			if i := strings.IndexByte(p.src[p.pos:], '\n'); i >= 0 {
				p.pos += i + 1
			} else {
				p.pos = len(p.src)
			}
		default:
			return
		}
	}
}

// statement parses "name[k1][k2]... = value" and returns the bracket keys.
func (p *luaParser) statement() ([]string, interface{}, error) {
	if p.ident() == "" {
		return nil, nil, p.errorf("expected a name")
	}
	var keys []string
	for {
		p.skipSpace()
		if p.pos >= len(p.src) || p.src[p.pos] != '[' {
			break
		}
		p.pos++
		k, err := p.value()
		if err != nil {
			return nil, nil, err
		}
		keys = append(keys, luaKey(k))
		p.skipSpace()
		if !p.eat(']') {
			return nil, nil, p.errorf("expected ]")
		}
	}
	p.skipSpace()
	if !p.eat('=') {
		return nil, nil, p.errorf("expected =")
	}
	v, err := p.value()
	return keys, v, err
}

func (p *luaParser) eat(c byte) bool {
	if p.pos < len(p.src) && p.src[p.pos] == c {
		p.pos++
		return true
	}
	return false
}

func (p *luaParser) ident() string {
	start := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || p.pos > start && c >= '0' && c <= '9' {
			p.pos++
			continue
		}
		break
	}
	return p.src[start:p.pos]
}

func luaKey(v interface{}) string {
	switch k := v.(type) {
	case string:
		return k
	case float64:
		return strconv.FormatFloat(k, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(k)
	}
	return fmt.Sprint(v)
}

func (p *luaParser) value() (interface{}, error) {
	p.skipSpace()
	if p.pos >= len(p.src) {
		return nil, p.errorf("unexpected end of file")
	}
	switch c := p.src[p.pos]; {
	case c == '{':
		return p.table()
	case c == '"' || c == '\'':
		return p.str()
	case c == '-' || c == '.' || c >= '0' && c <= '9':
		return p.number()
	}
	switch id := p.ident(); id {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "nil":
		return nil, nil
	case "inf", "nan":
		p.skipParenSuffix()
		return 0.0, nil
	case "":
		return nil, p.errorf("unexpected %q", p.src[p.pos])
	default:
		return nil, p.errorf("unexpected name %q", id)
	}
}

func (p *luaParser) table() (interface{}, error) {
	p.pos++ // {
	t := map[string]interface{}{}
	n := 1
	for {
		p.skipSpace()
		if p.eat('}') {
			return t, nil
		}
		var key string
		if p.eat('[') {
			k, err := p.value()
			if err != nil {
				return nil, err
			}
			key = luaKey(k)
			p.skipSpace()
			if !p.eat(']') {
				return nil, p.errorf("expected ]")
			}
			p.skipSpace()
			if !p.eat('=') {
				return nil, p.errorf("expected =")
			}
		} else {
			save := p.pos
			if id := p.ident(); id != "" && id != "true" && id != "false" && id != "nil" {
				p.skipSpace()
				if p.eat('=') {
					key = id
				} else {
					p.pos = save
				}
			} else {
				p.pos = save
			}
			if key == "" {
				key = strconv.Itoa(n)
				n++
			}
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		if v != nil {
			t[key] = v
		}
		p.skipSpace()
		if !p.eat(',') && !p.eat(';') {
			p.skipSpace()
			if !p.eat('}') {
				return nil, p.errorf("expected , or }")
			}
			return t, nil
		}
	}
}

// str reads a quoted string with Lua's escapes (string.format %q writes \",
// \\, \r, \0 / \ddd, and a backslash before a real newline).
func (p *luaParser) str() (interface{}, error) {
	q := p.src[p.pos]
	p.pos++
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		p.pos++
		switch {
		case c == q:
			return b.String(), nil
		case c != '\\':
			b.WriteByte(c)
		case p.pos >= len(p.src):
			return nil, p.errorf("unterminated string")
		default:
			e := p.src[p.pos]
			p.pos++
			switch e {
			case 'n', '\n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				if e >= '0' && e <= '9' {
					d := int(e - '0')
					for i := 0; i < 2 && p.pos < len(p.src) && p.src[p.pos] >= '0' && p.src[p.pos] <= '9'; i++ {
						d = d*10 + int(p.src[p.pos]-'0')
						p.pos++
					}
					b.WriteByte(byte(d))
				} else {
					b.WriteByte(e)
				}
			}
		}
	}
	return nil, p.errorf("unterminated string")
}

// skipParenSuffix skips the "(ind)" of "-nan(ind)".
func (p *luaParser) skipParenSuffix() {
	if p.eat('(') {
		for p.pos < len(p.src) && p.src[p.pos] != ')' {
			p.pos++
		}
		p.eat(')')
	}
}

func (p *luaParser) number() (interface{}, error) {
	start := p.pos
	if p.src[p.pos] == '-' {
		p.pos++
		if id := p.ident(); id == "inf" || strings.HasPrefix(id, "nan") {
			p.skipParenSuffix()
			return 0.0, nil
		} else if id != "" {
			p.pos = start + 1
		}
	}
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c >= '0' && c <= '9' || c == '.' || c == 'e' || c == 'E' || c == 'x' || c == 'X' ||
			(c == '+' || c == '-') && (p.src[p.pos-1] == 'e' || p.src[p.pos-1] == 'E') ||
			c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' {
			p.pos++
			continue
		}
		break
	}
	// MSVC prints infinities and NaNs as 1.#INF / -1.#IND, newer CRTs as
	// -nan(ind); none matter for a zone, so they read as 0.
	if p.pos < len(p.src) && p.src[p.pos] == '#' {
		for p.pos < len(p.src) && p.src[p.pos] != ',' && p.src[p.pos] != '}' && p.src[p.pos] > ' ' {
			p.pos++
		}
		return 0.0, nil
	}
	text := p.src[start:p.pos]
	if f, err := strconv.ParseFloat(text, 64); err == nil {
		return f, nil
	}
	if i, err := strconv.ParseInt(text, 0, 64); err == nil {
		return float64(i), nil
	}
	if strings.Contains(text, "nan") || strings.Contains(text, "inf") {
		return 0.0, nil
	}
	return nil, p.errorf("bad number %q", text)
}
