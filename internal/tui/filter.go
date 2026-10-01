package tui

import "github.com/sahilm/fuzzy"

type match struct {
	index   int   // position in the original item list
	matched []int // byte offsets of the characters the query hit
}

// filterItems returns the items fuzzy-matching query, best first. An empty
// query returns everything in its original order.
func filterItems(query string, items []string) []match {
	if query == "" {
		out := make([]match, len(items))
		for i := range items {
			out[i] = match{index: i}
		}
		return out
	}
	results := fuzzy.Find(query, items)
	out := make([]match, len(results))
	for i, r := range results {
		out[i] = match{index: r.Index, matched: r.MatchedIndexes}
	}
	return out
}

// putMatch writes s with the matched characters in bold rose. The first
// dimLen bytes (a folder prefix) are drawn in dimFg so names stand out.
func (g *grid) putMatch(x, y int, s string, m match, base style, dimLen int, dimFg color) int {
	hit := make(map[int]bool, len(m.matched))
	for _, i := range m.matched {
		hit[i] = true
	}
	for i, r := range s {
		st := base
		switch {
		case hit[i]:
			st.fg, st.bold = cMagenta, true
		case i < dimLen:
			st.fg, st.bold = dimFg, false
		}
		x = g.put(x, y, string(r), st)
	}
	return x
}
