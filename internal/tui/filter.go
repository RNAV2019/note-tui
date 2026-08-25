package tui

import (
	"strings"

	"github.com/sahilm/fuzzy"
)

type Match struct {
	Value          string
	MatchedIndexes []int
}

// FilterItems returns items fuzzy-matching query, best first.
// An empty query returns everything in original order.
func FilterItems(query string, items []string) []Match {
	if query == "" {
		out := make([]Match, len(items))
		for i, s := range items {
			out[i] = Match{Value: s}
		}
		return out
	}
	results := fuzzy.Find(query, items)
	out := make([]Match, len(results))
	for i, r := range results {
		out[i] = Match{Value: r.Str, MatchedIndexes: r.MatchedIndexes}
	}
	return out
}

// highlight underlines the characters of a match that the query hit.
func highlight(m Match) string {
	if len(m.MatchedIndexes) == 0 {
		return m.Value
	}
	idx := make(map[int]bool, len(m.MatchedIndexes))
	for _, i := range m.MatchedIndexes {
		idx[i] = true
	}
	var b strings.Builder
	for i, r := range m.Value {
		if idx[i] {
			b.WriteString(matchStyle.Render(string(r)))
		} else {
			b.WriteString(string(r))
		}
	}
	return b.String()
}
