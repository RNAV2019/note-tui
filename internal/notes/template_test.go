package notes

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultTemplateHasPlaceholders(t *testing.T) {
	for _, ph := range []string{"{{title}}", "{{tag}}", "{{date}}"} {
		if !strings.Contains(DefaultTemplate, ph) {
			t.Errorf("DefaultTemplate missing %s", ph)
		}
	}
}

func TestRenderTemplate(t *testing.T) {
	tmpl := "= {{title}}\n{{tag}} on {{date}}\n"
	got := RenderTemplate(tmpl, "B-Trees", "algorithms", time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC))
	want := "= B-Trees\nalgorithms on 2026-07-08\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
