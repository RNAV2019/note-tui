package notes

import (
	"strings"
	"time"
)

// DefaultTemplate is written to <notes>/.template.typ on bootstrap.
// Package version verified against typst 0.14.
const DefaultTemplate = `#import "@local/typdraw:0.1.0": td

#set page(margin: 2cm)
#set text(size: 11pt)
#set heading(numbering: "1.1")
#show raw.where(block: true): block.with(
  fill: luma(245),
  inset: 8pt,
  radius: 4pt,
  width: 100%,
)

#align(center)[
  #text(size: 20pt, weight: "bold")[{{title}}]
  #v(-4pt)
  #text(size: 11pt, fill: gray)[{{tag}} · {{date}}]
]
#line(length: 100%, stroke: 0.5pt + gray)
#v(8pt)

`

func RenderTemplate(tmpl, title, tag string, date time.Time) string {
	r := strings.NewReplacer(
		"{{title}}", title,
		"{{tag}}", tag,
		"{{date}}", date.Format("2006-01-02"),
	)
	return r.Replace(tmpl)
}
