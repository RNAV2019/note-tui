package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	previewMaxBytes = 256 << 10 // enough for any lecture note; larger files are cut
	previewMaxLines = 400
)

// preview is the read-only view of one note, cached until the selection or
// the note's mtime changes.
type preview struct {
	path    string
	modTime time.Time
	rel     string // path relative to the notes root
	size    int64
	lines   []string
	total   int // line count of the whole file
	err     error
}

func loadPreview(root, path string, modTime time.Time) preview {
	p := preview{path: path, modTime: modTime}
	if rel, err := filepath.Rel(root, path); err == nil {
		p.rel = rel
	} else {
		p.rel = path
	}
	info, err := os.Stat(path)
	if err != nil {
		p.err = err
		return p
	}
	p.size = info.Size()

	f, err := os.Open(path)
	if err != nil {
		p.err = err
		return p
	}
	defer f.Close()
	buf := make([]byte, previewMaxBytes)
	n, _ := f.Read(buf)
	data := buf[:n]

	p.total = bytes.Count(data, []byte("\n"))
	if n > 0 && data[n-1] != '\n' {
		p.total++
	}
	if int64(n) < p.size {
		// Estimate rather than read the rest of a huge file.
		p.total = int(int64(p.total) * p.size / int64(n))
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\t", "  ")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	if len(lines) > previewMaxLines {
		lines = lines[:previewMaxLines]
	}
	p.lines = lines
	return p
}

// humanSize renders a byte count as "4.2 KB".
func humanSize(n int64) string {
	switch {
	case n < 1<<10:
		return strconv.FormatInt(n, 10) + " B"
	case n < 1<<20:
		return trimFloat(float64(n)/(1<<10)) + " KB"
	default:
		return trimFloat(float64(n)/(1<<20)) + " MB"
	}
}

func trimFloat(f float64) string {
	return strings.TrimSuffix(strconv.FormatFloat(f, 'f', 1, 64), ".0")
}

// ---------------------------------------------------------------- highlight

var typstKeywords = map[string]bool{
	"let": true, "set": true, "show": true, "import": true, "include": true,
	"if": true, "else": true, "for": true, "while": true, "in": true,
	"return": true, "break": true, "continue": true, "context": true,
	"and": true, "or": true, "not": true, "none": true, "auto": true,
	"true": true, "false": true, "as": true,
}

// highlighter colours Typst source line by line. It is deliberately
// approximate: it only has to make a preview readable, not be a parser.
type highlighter struct {
	depth   int  // brackets opened in code mode and not yet closed
	stmt    bool // this line started a #statement, so the rest of it is code
	comment bool // inside a /* */ block
	raw     bool // inside a ``` block
}

func (h *highlighter) inCode() bool { return h.stmt || h.depth > 0 }

func (h *highlighter) line(s string) []seg {
	defer func() { h.stmt = false }()
	var out []seg
	emit := func(text string, st style) {
		if text == "" {
			return
		}
		if n := len(out); n > 0 && out[n-1].st == st {
			out[n-1].text += text
			return
		}
		out = append(out, seg{text, st})
	}

	trimmed := strings.TrimLeft(s, " ")
	if h.raw || strings.HasPrefix(trimmed, "```") {
		if strings.HasPrefix(trimmed, "```") {
			h.raw = !h.raw
		}
		emit(s, fg(cSubtle))
		return out
	}
	if !h.inCode() && !h.comment {
		if strings.HasPrefix(trimmed, "=") {
			rest := strings.TrimLeft(trimmed, "=")
			if rest == "" || rest[0] == ' ' {
				emit(s, bold(cLove))
				return out
			}
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "+ ") {
			lead := len(s) - len(trimmed)
			emit(s[:lead+1], fg(cRose))
			s = s[lead+1:]
		}
	}

	rs := []rune(s)
	i := 0
	span := func(end int) string { t := string(rs[i:end]); i = end; return t }
	for i < len(rs) {
		r := rs[i]
		switch {
		case h.comment:
			end := indexFrom(rs, i, "*/")
			if end < 0 {
				emit(span(len(rs)), fg(cSubtle))
				continue
			}
			h.comment = false
			emit(span(end+2), fg(cSubtle))

		case hasAt(rs, i, "//"):
			emit(span(len(rs)), fg(cSubtle))

		case hasAt(rs, i, "/*"):
			h.comment = true
			emit(span(i+2), fg(cSubtle))

		case r == '"':
			end := i + 1
			for end < len(rs) && rs[end] != '"' {
				if rs[end] == '\\' {
					end++
				}
				end++
			}
			emit(span(min(end+1, len(rs))), fg(cGold))

		case r == '$':
			end := indexFrom(rs, i+1, "$")
			if end < 0 {
				end = len(rs) - 1
			}
			emit(span(end+1), fg(cFoam))

		case r == '#' && i+1 < len(rs) && (isIdentStart(rs[i+1]) || rs[i+1] == '(' || rs[i+1] == '{' || rs[i+1] == '['):
			end := identEnd(rs, i+1)
			word := string(rs[i+1 : end])
			if typstKeywords[word] || word == "" {
				emit(span(end), fg(cIris))
			} else {
				emit(span(end), fg(cRose))
			}
			h.stmt = true // the rest of the line is code

		case h.inCode() && isIdentStart(r):
			end := identEnd(rs, i)
			word := string(rs[i:end])
			next := end
			for next < len(rs) && rs[next] == ' ' {
				next++
			}
			switch {
			case typstKeywords[word]:
				emit(span(end), fg(cIris))
			case next < len(rs) && rs[next] == ':':
				emit(span(end), fg(cFoam))
			case next < len(rs) && rs[next] == '(':
				emit(span(end), fg(cRose))
			default:
				emit(span(end), fg(cText))
			}

		case unicode.IsDigit(r) && (i == 0 || !isIdentPart(rs[i-1])):
			end := i
			for end < len(rs) && (unicode.IsDigit(rs[end]) || rs[end] == '.' || unicode.IsLetter(rs[end]) || rs[end] == '%') {
				end++
			}
			emit(span(end), fg(cRose))

		case !h.inCode() && (r == '*' || r == '_') && i+1 < len(rs) && rs[i+1] != ' ':
			end := indexFrom(rs, i+1, string(r))
			if end < 0 {
				emit(span(i+1), fg(cText))
				continue
			}
			if r == '*' {
				emit(span(end+1), bold(cText))
			} else {
				emit(span(end+1), fg(cRose))
			}

		default:
			if h.inCode() {
				switch r {
				case '(', '{', '[':
					h.depth++
				case ')', '}', ']':
					h.depth = max(h.depth-1, 0)
				}
			}
			emit(span(i+1), fg(cText))
		}
	}
	return out
}

func hasAt(rs []rune, i int, s string) bool {
	for j, r := range s {
		if i+j >= len(rs) || rs[i+j] != r {
			return false
		}
	}
	return true
}

func indexFrom(rs []rune, from int, s string) int {
	for i := from; i < len(rs); i++ {
		if hasAt(rs, i, s) {
			return i
		}
	}
	return -1
}

func isIdentStart(r rune) bool { return unicode.IsLetter(r) || r == '_' }
func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.'
}

func identEnd(rs []rune, i int) int {
	for i < len(rs) && isIdentPart(rs[i]) {
		i++
	}
	return i
}
