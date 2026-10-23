package debrief

import (
	"html"
	"regexp"
	"strings"
)

// HTML renders the Markdown Claude writes - headings, bullet and numbered
// lists, tables, bold, italic, code, rules and paragraphs - for the AAR page
// and its PDF. Everything is escaped first, so nothing in the text becomes
// markup of its own.
func HTML(md string) string {
	lines := strings.Split(strings.ReplaceAll(md, "\r", ""), "\n")
	var out []string
	var para []string
	list := ""
	flushPara := func() {
		if len(para) > 0 {
			out = append(out, "<p>"+inline(strings.Join(para, " "))+"</p>")
			para = nil
		}
	}
	closeList := func() {
		if list != "" {
			out = append(out, "</"+list+">")
			list = ""
		}
	}
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		t := strings.TrimSpace(l)
		switch {
		case t == "":
			flushPara()
			closeList()
		case headingRE.MatchString(l):
			flushPara()
			closeList()
			m := headingRE.FindStringSubmatch(l)
			n := string(rune('0' + len(m[1])))
			out = append(out, "<h"+n+">"+inline(m[2])+"</h"+n+">")
		case ruleRE.MatchString(l):
			flushPara()
			closeList()
			out = append(out, "<hr>")
		case strings.HasPrefix(t, "|") && i+1 < len(lines) && sepRE.MatchString(lines[i+1]):
			flushPara()
			closeList()
			var b strings.Builder
			b.WriteString("<table><thead><tr>")
			for _, c := range cells(t) {
				b.WriteString("<th>" + inline(c) + "</th>")
			}
			b.WriteString("</tr></thead><tbody>")
			i += 2
			for ; i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|"); i++ {
				b.WriteString("<tr>")
				for _, c := range cells(strings.TrimSpace(lines[i])) {
					b.WriteString("<td>" + inline(c) + "</td>")
				}
				b.WriteString("</tr>")
			}
			i--
			out = append(out, b.String()+"</tbody></table>")
		case itemRE.MatchString(l):
			flushPara()
			m := itemRE.FindStringSubmatch(l)
			kind := "ul"
			if strings.HasSuffix(m[1], ".") {
				kind = "ol"
			}
			if list != kind {
				closeList()
				out = append(out, "<"+kind+">")
				list = kind
			}
			out = append(out, "<li>"+inline(m[2])+"</li>")
		default:
			closeList()
			para = append(para, t)
		}
	}
	flushPara()
	closeList()
	return strings.Join(out, "\n")
}

var (
	headingRE = regexp.MustCompile(`^(#{1,4})\s+(.*)$`)
	ruleRE    = regexp.MustCompile(`^\s*(-{3,}|\*{3,}|_{3,})\s*$`)
	sepRE     = regexp.MustCompile(`^\s*\|?\s*:?-{2,}`)
	itemRE    = regexp.MustCompile(`^\s*([-*•]|\d+\.)\s+(.*)$`)
	codeRE    = regexp.MustCompile("`([^`]+)`")
	boldRE    = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicRE  = regexp.MustCompile(`(^|[^*\w])\*([^*\s][^*]*)\*`)
)

func cells(row string) []string {
	row = strings.TrimSuffix(strings.TrimPrefix(row, "|"), "|")
	parts := strings.Split(row, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func inline(s string) string {
	s = html.EscapeString(s)
	s = codeRE.ReplaceAllString(s, "<code>$1</code>")
	s = boldRE.ReplaceAllString(s, "<b>$1</b>")
	s = italicRE.ReplaceAllString(s, "$1<i>$2</i>")
	return s
}
