package ui

import "github.com/sahilm/fuzzy"

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
