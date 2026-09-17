package tui

import "testing"

func TestFilterItems(t *testing.T) {
	items := []string{"uni/algos/b-trees", "uni/networks/tcp", "scratch/misc/idea"}
	if got := filterItems("", items); len(got) != 3 || got[2].index != 2 {
		t.Errorf("empty query = %+v, want every item in order", got)
	}
	got := filterItems("tcp", items)
	if len(got) != 1 || got[0].index != 1 {
		t.Fatalf("filter tcp = %+v", got)
	}
	if len(got[0].matched) != 3 {
		t.Errorf("matched = %v, want three offsets", got[0].matched)
	}
}

func TestPutMatchHighlightsHits(t *testing.T) {
	g := newGrid(10, 1)
	g.putMatch(0, 0, "ab/cd", match{matched: []int{3}}, fg(cText), 3, cSubtle)
	if c := g.at(3, 0); c.st.fg != cRose || !c.st.bold {
		t.Errorf("hit = %+v, want bold rose", c.st)
	}
	if c := g.at(0, 0); c.st.fg != cSubtle {
		t.Errorf("folder = %+v, want subtle", c.st)
	}
	if c := g.at(4, 0); c.st.fg != cText {
		t.Errorf("name = %+v, want text", c.st)
	}
}
