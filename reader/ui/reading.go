package ui

import (
	"fmt"
	"strings"

	blitz "shamsainkengine"
)

// The panels over a book's page: the Text sheet, the dock, the question before a highlight is removed.
const sheetHeight = 900 // px, at most

// What the Text sheet shows: the reader builds it from its settings and the book.
type Text struct {
	Tab        string  // the tab shown: "text" (size, spacing, margins) or "page" (the page turns)
	FontSize   float32 // px
	LineHeight float32 // the one set; 0: the book's own
	Lines      float32 // the line spacing as laid out now (shown when LineHeight is 0)
	Margin     int     // px
	Dir        string  // the direction set for this book: "", "rtl" or "ltr"
}

// The sheet's numbers and choices, for templates/text.html.
type textData struct {
	Tab                            string
	Size, LinesName, Lines, Margin string
	Dir                            string
}

// DrawText draws the Text sheet for x at the bottom of a screen h px high, a line on its top edge. Its buttons:
// tab-text and tab-page, then the tab's (size-, lines- and margin-dec and -inc; or dir-, dir-rtl and dir-ltr), and
// reset.
func DrawText(x Text, fonts *blitz.Fonts, w, h int) *Panel {
	// the line spacing as it is, and whether it is the book's own
	lines, linesName := x.Lines, word("line-spacing-book")
	if x.LineHeight > 0 {
		lines, linesName = x.LineHeight, word("line-spacing")
	}
	html, ids := render("text.html", textData{Tab: x.Tab,
		Size: word("px", decimal(fmt.Sprintf("%g", x.FontSize))), LinesName: linesName, Lines: word("times", decimal(fmt.Sprintf("%.2f", lines))),
		Margin: word("px", num(x.Margin)), Dir: x.Dir,
	})
	return atBottom(drawPanel(html, ids, fonts, w, sheetHeight), h)
}

// atBottom puts a panel drawn by drawPanel at the bottom of a screen h px high, its line moved to its top edge.
// DrawHighlightMenu draws the menu by words chosen on the page (templates/highlight-menu.html): Highlight (hl-make),
// or Remove highlight (hl-remove) when made is a highlight held; Listen from here (hl-listen) when the book is
// narrated. Hang puts it by the words.
func DrawHighlightMenu(made, narrated bool, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("highlight-menu.html", struct{ Made, Narrated bool }{made, narrated})
	return drawPanelIn(html, ids, "highlight-menu", fonts, w, 400) // at most: cut to the list
}

// drawPanel draws a panel over the top of the page from html, as high as its buttons need (at most h px; the last
// thing on a panel is a button), a line under it, its buttons by ids.
// Where the reader is, for the dock: the chapter's title, the page in its document, the page in the book (Total 0
// until every document is paginated) and the share of the book read; Back is the page to go back to after a jump (0:
// none); Ticks the shares of the book where its chapters start.
type Progress struct {
	Chapter     string
	At, Of      int
	Page, Total int
	Share       float64
	Back        int
	Ticks       []float64
	RTL         bool
	Narrated    bool // it can be read aloud: the dock's Listen
}

const dockHeight = 700 // px, at most

// The dock's words and numbers, for templates/dock.html.
type dockData struct {
	Chapter, Page, Back, Left string // Back: the way back's label, "" for none
	Dir, Side, Width          string // the book's direction and the side it starts from (left, right), its filled share in per cent
	Ticks                     []string
	Narrated                  bool
}

// DrawDock draws the dock at the bottom of a screen h px high, opened with the bar: where the reader is, a bar to tap
// to go anywhere in the book between the chapters before and after, the way back after a jump, and the tools. Its
// buttons: chapter-prev, bar, chapter-next, back (after a jump), contents, text and read-aloud (if narrated).
func DrawDock(p Progress, fonts *blitz.Fonts, w, h int) *Panel {
	pct := func(f float64) string { return fmt.Sprintf("%.1f", 100*min(max(f, 0), 1)) }
	d := dockData{Chapter: p.Chapter, Page: word("dock-page", p.Page),
		Dir: "ltr", Side: "left", Width: pct(p.Share), Narrated: p.Narrated}
	if p.Total > 0 {
		d.Page = word("dock-page-of", p.Page, p.Total)
	}
	if p.RTL {
		d.Dir, d.Side = "rtl", "right"
	}
	for _, t := range p.Ticks {
		if t > 0 && t < 1 {
			d.Ticks = append(d.Ticks, pct(t))
		}
	}
	switch left := p.Of - p.At; {
	case p.Back > 0:
		d.Back = word("back-to-page", p.Back)
	case left <= 0:
		d.Left = word("last-page")
	case left == 1:
		d.Left = word("page-left", left)
	default:
		d.Left = word("pages-left", left)
	}
	draw := func() *Panel {
		html, ids := render("dock.html", d)
		return drawPanel(html, ids, fonts, w, dockHeight)
	}
	m := draw()
	// the engine does not clip text: a chapter title wider than its room is cut to the most letters that fit, then "…"
	fits := func() bool { return m.buttons["where-title-text"].Dx() <= m.buttons["where-title"].Dx() }
	if !fits() {
		title := []rune(p.Chapter)
		lo, hi := 0, len(title) // the most letters kept: lo fits, hi does not
		for hi-lo > 1 {
			mid := (lo + hi) / 2
			d.Chapter = strings.TrimSpace(string(title[:mid])) + "…"
			if m = draw(); fits() {
				lo = mid
			} else {
				hi = mid
			}
		}
		d.Chapter = strings.TrimSpace(string(title[:lo])) + "…"
		m = draw()
	}
	delete(m.buttons, "where-title")
	delete(m.buttons, "where-title-text")
	return atBottom(m, h)
}
