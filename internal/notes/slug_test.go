package notes

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Graph Theory", "graph-theory"},
		{"Lecture 2: B-Trees!", "lecture-2-b-trees"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"already-fine", "already-fine"},
		{"CAPS_and_underscores", "caps-and-underscores"},
		{"éàccents", "ccents"},
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSlugifyEmpty(t *testing.T) {
	if got := Slugify("!!!"); got != "" {
		t.Errorf("unsluggable input should return empty, got %q", got)
	}
}
