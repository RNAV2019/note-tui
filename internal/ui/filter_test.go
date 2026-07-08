package ui

import "testing"

func TestFilterItemsEmptyQueryKeepsOrder(t *testing.T) {
	items := []string{"uni/algos/b-trees", "uni/networks/tcp", "personal/misc/todo"}
	got := FilterItems("", items)
	if len(got) != 3 || got[0].Value != "uni/algos/b-trees" {
		t.Fatalf("got %+v", got)
	}
}

func TestFilterItemsFuzzyMatchesAndRanks(t *testing.T) {
	items := []string{"uni/networks/tcp", "uni/algos/b-trees", "personal/misc/btree-redux"}
	got := FilterItems("btr", items)
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %+v", got)
	}
	for _, m := range got {
		if m.Value == "uni/networks/tcp" {
			t.Error("tcp should not match 'btr'")
		}
		if len(m.MatchedIndexes) == 0 {
			t.Error("expected matched indexes for highlighting")
		}
	}
}
