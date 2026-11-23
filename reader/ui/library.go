package ui

import (
	"fmt"
	"image"
	"strings"
	"time"

	blitz "shamsainkengine"

	"shamsainkreader/library"
)

// A book's row in the library, as the reader knows it.
type Book struct {
	Title, Author string
	Read          int    // percent read; -1 when not known
	New           bool   // never opened: "new" shown (not in the New filter, where every book is)
	Narrated      bool   // it has narration (media overlays): a headphones mark after its title
	Finished      bool   // read to the end: no bar
	RTL           bool   // its title reads right to left
	When          string // a date shown at the row's end in place of the share read: when added (New), finished (Finished); "" for none
}

// The library's rows, for templates/library.html.
type libraryRow struct {
	ID, Title, Author string
	Read, Width, Dir  string // the share read as shown, its bar's width in per cent ("" for no bar), the row's direction
	New               bool   // New: "new" drawn as a tag
	When              string // a date (added, finished), small and grey, unframed
	Narrated          bool   // a headphones mark after the title
}

// The words that are the library's filters' labels, in the order of library.Filters (their names, in the buttons' ids);
// the sorts' are their names.
var filterWords = []string{"reading", "new-books", "finished", "all"}

// What the library's head shows besides the bar: the filter chosen (an index of library.Filters) and how many books each
// has, and which of the panels is open: the menu (under the
// bar) or the sort panel (under the filters).
type LibraryView struct {
	Filter         int
	Counts         [4]int
	MenuOn, SortOn bool
}

// LibraryHead draws the library's head (docs/design/system.html): the bar, its title the library's, and under it
// the filters and the sort button, with no line under them. Its buttons are the bar's, filter-reading, filter-new,
// filter-finished and filter-all, and list-sort.
func LibraryHead(topBar TopBar, v LibraryView, fonts *blitz.Fonts, w int) *Panel {
	topBar.Shelf = "library" // the bar's choice of Library and Catalogues in place of its title
	topBar.MenuOn = v.MenuOn // no start icon: search comes back with a keyboard
	var fs []button
	for i, name := range library.Filters {
		fs = append(fs, button{ID: "filter-" + name, Label: word(filterWords[i]), Count: num(v.Counts[i]), On: i == v.Filter})
	}
	html, ids := render("library-head.html", struct {
		Filters []button
		SortOn  bool
	}{fs, v.SortOn})
	filters := drawPanel(html, ids, fonts, w, 300)
	nh := filters.img.Rect.Dy()
	for y := nh - 4; y < nh; y++ { // no line: drawPanel's under it rubbed out
		for x := 0; x < w; x++ {
			filters.img.Pix[y*filters.img.Stride+x] = 255
		}
	}
	return stack(DrawTopBar(topBar, fonts, w), filters)
}

// stack is the panel top with the panel bottom under it, as one; the buttons of both, and under is where the
// bottom starts.
func stack(top, bottom *Panel) *Panel {
	th := top.img.Rect.Dy()
	m := &Panel{img: image.NewGray(image.Rect(0, 0, top.img.Rect.Dx(), th+bottom.img.Rect.Dy())), buttons: map[string]image.Rectangle{}, under: th}
	copy(m.img.Pix, top.img.Pix)
	copy(m.img.Pix[th*m.img.Stride:], bottom.img.Pix)
	for id, r := range top.buttons {
		m.buttons[id] = r
	}
	for id, r := range bottom.buttons {
		m.buttons[id] = r.Add(image.Pt(0, th))
	}
	return m
}

// emptyWords are what a library with no books in its filter says, for templates/library.html: a title, and when the
// library has no books at all a line around the folder they go in (Hint, Folder, After).
type emptyWords struct{ Title, Hint, Folder, After string }

// NewLibrary lays out the library (docs/design/system.html): the head (LibraryHead), a row a book (each
// in its title's direction, all as high as each other), and a pager at the bottom. A tap on a row is EntryAt. With
// no books in the filter chosen, one placeholder in their place: "No books here"; and with folder (as the device's drive
// shows it; "" when other filters have books) that EPUB books go in it.
func NewLibrary(books []Book, folder string, head *Panel, fonts *blitz.Fonts, w, h int) *List {
	var rows []libraryRow
	for i, b := range books {
		r := libraryRow{ID: fmt.Sprint("e", i), Title: b.Title, Author: b.Author, New: b.New, Narrated: b.Narrated, When: b.When, Dir: "ltr"}
		if b.RTL {
			r.Dir = "rtl"
		}
		switch {
		case b.When != "":
		case b.New:
			r.Read = word("new")
		case b.Finished:
			r.Width = "100"
		case b.Read >= 0: // the share read as a bar alone, no number
			r.Width = fmt.Sprint(b.Read)
		}
		rows = append(rows, r)
	}
	var none *emptyWords
	if len(rows) == 0 {
		none = &emptyWords{Title: word("no-books")}
		if folder != "" {
			// the sentence around the folder, which the template keeps left to right in a Farsi one
			none.Folder = folder
			none.Hint, none.After, _ = strings.Cut(word("no-books-hint", "\x00"), "\x00")
		}
	}
	col := fitTitles(rows, 0, fonts, w)
	html, _ := render("library.html", struct {
		Rows    []libraryRow
		Empty   *emptyWords
		Measure bool
		Col     int
	}{rows, none, false, col})
	return NewList(head, html, len(books), -1, true, false, fonts, w, h)
}

// fitTitles cuts each title too wide for its row's first line, the share read at the line's end, to the most letters
// that fit and "…" (the engine does not clip text: a long title ran under the bar, 2026-11-08); the whole title stays
// in the book's details. The rows are measured in a page of their own (the list's options, titles on one line): a title
// too wide is cut by its share of the room, the cut ones measured again and cut a little more until each fits; a few
// layouts, not one a letter. A row with nothing at its title's end keeps end px free there (a catalogue's button).
//
// The share read, a tag or a date sits at the right end of every row, in one column whatever the title's direction
// (2026-11-23, the principle "The interface's frame, the content's direction"): col is that column's width with the gap
// before it, from the widest of them, 0 when no row has one; a right-to-left title ends before it (library.html, --col).
func fitTitles(rows []libraryRow, end int, fonts *blitz.Fonts, w int) (col int) {
	const margin, gap = 40, 20 // the list's margin (NewList); between the title and the share read
	full := make([][]rune, len(rows))
	keep := make([]int, len(rows)) // letters kept, -1 for the whole title
	for i := range rows {
		full[i], keep[i] = []rune(rows[i].Title), -1
	}
	todo := make([]int, len(rows))
	for i := range todo {
		todo[i] = i
	}
	for pass := 0; pass < 8 && len(todo) > 0; pass++ {
		var some []libraryRow
		for _, i := range todo {
			some = append(some, rows[i])
		}
		ms := measureTitles(some, fonts, w)
		if pass == 0 {
			for _, m := range ms {
				if m.end.Dx() > 0 {
					col = max(col, m.end.Dx()+gap)
				}
			}
		}
		var next []int
		for j, i := range todo {
			m := ms[j]
			var room int
			if rows[i].Dir == "rtl" { // the title from the right, ending before the column; to the row's start less end px
				room = m.title.Max.X - col - (margin + end)
			} else {
				limit := w - margin - end
				if m.end.Dx() > 0 {
					limit = m.end.Min.X - gap
				}
				room = limit - m.title.Min.X
			}
			width := m.title.Dx()
			if width <= room || width == 0 {
				continue
			}
			n := keep[i]
			if n < 0 { // the first cut: by the share of the room, as letters are about as wide as each other
				n = int(float64(len(full[i])) * float64(room) / float64(width))
			} else {
				n -= max(1, n/20)
			}
			keep[i] = max(n, 1)
			rows[i].Title = strings.TrimSpace(string(full[i][:keep[i]])) + "…"
			if keep[i] > 1 {
				next = append(next, i)
			}
		}
		todo = next
	}
	return col
}

// A row as measured: its title's line, and what is at the row's end (the share read, a tag, a date; empty for none).
type measured struct{ title, end image.Rectangle }

// measureTitles lays out rows with their titles on one line, no column kept (a right-to-left title ends at the row's
// right), and says where each title's line and the thing at the row's end are, in px.
func measureTitles(rows []libraryRow, fonts *blitz.Fonts, w int) []measured {
	html, _ := render("library.html", struct {
		Rows    []libraryRow
		Empty   *emptyWords
		Measure bool
		Col     int
	}{rows, nil, true, 0})
	b := blitz.New([]string{"list.html"}, readFor(html), blitz.Options{Width: w, Height: 1200, Margin: 40, FontSize: 44}, fonts)
	defer b.Close()
	b.Layout(0)
	ms := make([]measured, len(rows))
	for i, r := range rows {
		if _, ts := b.Find(0, r.ID+"-t"); len(ts) > 0 {
			ms[i].title = union(ts)
		}
		if _, ps := b.Find(0, r.ID+"-p"); len(ps) > 0 {
			ms[i].end = union(ps)
		}
	}
	return ms
}

// AddedWhen is a date in a book's row (when it was added, or finished), from now: today, yesterday, a number of days within a
// week, else the day and month (and the year, if not this year's).
func AddedWhen(added, now time.Time) string {
	y1, m1, d1 := added.Date()
	y2, m2, d2 := now.Date()
	days := int(time.Date(y2, m2, d2, 0, 0, 0, 0, time.UTC).Sub(time.Date(y1, m1, d1, 0, 0, 0, 0, time.UTC)).Hours() / 24)
	switch {
	case days <= 0:
		return word("today")
	case days == 1:
		return word("yesterday")
	case days < 7:
		return word("days-ago", num(days))
	case y1 == y2:
		return digits(added.Format("2 Jan"))
	}
	return digits(added.Format("2 Jan 2006"))
}

// segmentsOf is the buttons of one choice of names (ids prefix+name, labels from the words), the one at index on drawn
// chosen.
func segmentsOf(prefix string, names, labels []string, on int) []button {
	var bs []button
	for i, n := range names {
		bs = append(bs, button{ID: prefix + n, Label: word(labels[i]), On: i == on})
	}
	return bs
}

// DrawMenu draws the bar's menu: a narrow list hanging from the bar's menu button, directly under the bar (under px from
// the top), at the bar's end (the left when the interface is right to left, in a Farsi book): the settings that suit
// where it is, named by which, and leave to Kobo's interface (leave), a row an entry. In the library which is "device":
// Settings (the button settings, DrawLibrarySettings); in a book "reading": Reading settings (reading-settings,
// DrawReadingSettings).
func DrawMenu(under int, which string, fonts *blitz.Fonts, w int) *Panel {
	d := struct {
		Dir   string
		Which string
		Dev   bool // dev mode, in the library: the Developer entry
	}{"ltr", which, DevMode && which == "device"}
	if RTL() {
		d.Dir = "rtl"
	}
	html, ids := render("library-menu.html", d)
	m := drawPanelIn(html, ids, "menu-list", fonts, w, 600) // at most: cut to the list
	m.y0 += under
	return m
}

// LibrarySettings is what the Settings panel shows: the interface follows the book's language, the sleep timeout in
// minutes, whether the device has page buttons, and whether they are reversed.
type LibrarySettings struct {
	Follow         bool
	SleepMin       int
	HasButtons     bool
	ReverseButtons bool
	Version        string // the reader's build: Updates' second line, small and grey (no Updates row when empty)
}

// DrawLibrarySettings draws the Settings panel, directly under the bar (under px from the top), in place of the menu it
// was opened from: no title, rows with the setting's name at the start and its switch or stepper at the far end:
// "Sleep after" (sleep-dec and sleep-inc), then the switches: "Follow book language" (lang-follow), on a device with
// page buttons "Reverse page buttons" (buttons-reverse); under them Updates, the version as its second line, and Check
// (update-check) at the row's end.
func DrawLibrarySettings(st LibrarySettings, under int, fonts *blitz.Fonts, w int) *Panel {
	sleep := word("min", num(st.SleepMin))
	html, ids := render("library-settings.html", struct {
		LibrarySettings
		Sleep string
	}{st, sleep})
	m := drawPanel(html, ids, fonts, w, 600) // at most: cut to its last row
	m.y0 = under
	return m
}

// Developer is what the Developer panel shows (dev mode only): Wi-Fi's switch (WiFiOn: on or connecting) and its line
// ("off", "connecting...", "on: ADDRESS"); the development SSH's state, its script's line ("SSH on: ADDRESS, port 2222"
// or "SSH off"; empty when the device has no dev/ssh.sh).
type Developer struct {
	WiFiOn    bool
	WiFi, SSH string
}

// SSHOn tells whether the development SSH is on, by its state's line.
func (d Developer) SSHOn() bool { return strings.HasPrefix(d.SSH, "SSH on") }

// DrawDeveloper draws the Developer panel (templates/library-developer.html) directly under the bar (under px from
// the top), in place of the menu it was opened from: "Wi-Fi" (dev-wifi) with its state, then "SSH" (dev-ssh) with its address while on.
func DrawDeveloper(d Developer, under int, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("library-developer.html", d)
	m := drawPanel(html, ids, fonts, w, 600) // at most: cut to its last row
	m.y0 = under
	return m
}

// DrawLibrarySort draws the sort panel directly under the head (under px from the top): how the books are sorted
// (sort, an index of library.Sorts; buttons sort-recent and the like).
func DrawLibrarySort(sort, under int, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("library-sort.html", struct{ Sorts []button }{segmentsOf("sort-", library.Sorts, library.Sorts, sort)})
	m := drawPanel(html, ids, fonts, w, 600) // at most: cut to its buttons
	m.y0 = under
	return m
}

// DrawBookMenu draws a book's menu (templates/library-book-menu.html), for the row of a book held in the library: Mark as
// finished unless it is (book-finish); Mark as new when it was opened (book-new);
// Details (book-details) and Remove book (book-remove). Hang puts it by the row.
func DrawBookMenu(finished, opened bool, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("library-book-menu.html", struct{ Finished, Opened bool }{finished, opened})
	return drawPanelIn(html, ids, "book-menu", fonts, w, 600) // at most: cut to the list
}

// What a book's details show: the library's record of it and its place file's.
type BookDetails struct {
	Title, Author, Lang, Source, File string
	RTL                               bool  // its title reads right to left
	Size                              int64 // bytes
	Copies                            int   // its files (identical copies, one book); 1 or 0: one
	Narrated, Opened, Finished        bool
	Read                              int // percent read; -1 when not known
	Added, LastRead                   time.Time
}

// DrawBookDetails draws a book's details (templates/library-book-details.html), with no buttons: any tap closes it.
func DrawBookDetails(d BookDetails, fonts *blitz.Fonts, w int) *Panel {
	date := func(t time.Time) string {
		if t.IsZero() {
			return word("never")
		}
		return digits(t.Format("2 Jan 2006"))
	}
	lang := d.Lang
	switch d.Lang {
	case "fa", "en":
		lang = word("lang-" + d.Lang)
	case "":
		lang = "—"
	}
	read := word("not-opened")
	switch {
	case d.Finished:
		read = word("finished")
	case d.Opened && d.Read >= 0:
		read = word("percent", num(d.Read))
	case d.Opened:
		read = "—"
	}
	yes := map[bool]string{true: word("yes"), false: word("no")}
	rows := [][2]string{
		{word("read-share"), read},
		{word("last-read"), date(d.LastRead)},
		{word("added"), date(d.Added)},
		{word("language"), lang},
		{word("narrated"), yes[d.Narrated]},
	}
	if d.Source != "" {
		rows = append(rows, [2]string{word("source"), d.Source})
	}
	rows = append(rows, [2]string{word("file"), d.File})
	if d.Copies > 1 {
		rows = append(rows, [2]string{word("copies"), num(d.Copies)})
	}
	rows = append(rows,
		[2]string{word("size"), word("mb", decimal(fmt.Sprintf("%.1f", float64(d.Size)/(1<<20))))})
	data := struct {
		Title, Author, Dir string
		Rows               [][2]string
	}{d.Title, d.Author, "ltr", rows}
	if d.RTL {
		data.Dir = "rtl"
	}
	html, ids := render("library-book-details.html", data)
	return drawPanelIn(html, ids, "book-details-box", fonts, w, 1200)
}

// DrawNotice draws a notice (templates/notice.html: the dialog component), the words of key, after the icon named
// icon when not "" (a crossed-out Wi-Fi when it failed), a grey line under them when note is given (a version: never
// inside the sentence, 2026-11-20, the maintainer: no Latin popping out of a Farsi one), and Close (its button
// notice-close), w px wide at most; the caller places it (SetY).
func DrawNotice(key, icon string, fonts *blitz.Fonts, w int, note ...string) *Panel {
	d := struct{ Text, Icon, Note string }{word(key), icon, ""}
	if len(note) > 0 {
		d.Note = ltr(note[0])
	}
	html, ids := render("notice.html", d)
	return drawPanelIn(html, ids, "notice-box", fonts, w, 900)
}

// ltr is s kept left to right inside a sentence (a version, "v0.4.0-beta", in a Farsi one): Unicode's isolate marks.
func ltr(s string) string { return "\u2066" + s + "\u2069" }

// DrawUpdateAvailable draws the question when a newer version is out (templates/update-available.html): "A new version
// is available.", the version on a grey line of its own, then Update (update-install, its download icon) and Later
// (update-later).
func DrawUpdateAvailable(version string, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("update-available.html", struct{ Text, Version string }{word("update-available"), ltr(version)})
	return drawPanelIn(html, ids, "update-box", fonts, w, 900)
}

// DrawUpdateProgress draws an update under way (templates/update-progress.html): downloading the new version, share per
// cent of it, with a bar and Cancel (update-cancel); with share below 0, restarting with it, no buttons. The version on
// a grey line of its own.
func DrawUpdateProgress(version string, share int, fonts *blitz.Fonts, w int) *Panel {
	d := struct {
		Text, Version string
		Share         int
	}{word("update-restarting"), ltr(version), share}
	if share >= 0 {
		d.Text = word("update-downloading", word("percent", num(share)))
	}
	html, ids := render("update-progress.html", d)
	return drawPanelIn(html, ids, "update-progress-box", fonts, w, 900)
}

// DrawWiFiAsk draws the question before Wi-Fi is turned on (templates/wifi-ask.html): the words of key (what it is for:
// "Wi-Fi is off. Turn it on to ...?"), then its buttons wifi-on and wifi-cancel; the caller places it (SetY).
func DrawWiFiAsk(key string, fonts *blitz.Fonts, w int) *Panel {
	html, ids := render("wifi-ask.html", struct{ Key string }{key})
	return drawPanelIn(html, ids, "wifi-ask-box", fonts, w, 900)
}

// DrawBookRemove draws the question before a book is removed (templates/library-book-remove.html): its buttons
// book-remove-yes and book-remove-no. copies are its files (more than one: identical copies, all removed).
func DrawBookRemove(title string, rtl bool, copies int, fonts *blitz.Fonts, w int) *Panel {
	note := word("remove-book-note")
	if copies > 1 {
		note = word("remove-book-note-copies", num(copies))
	}
	d := struct{ Title, Dir, Note string }{title, "ltr", note}
	if rtl {
		d.Dir = "rtl"
	}
	html, ids := render("library-book-remove.html", d)
	return drawPanelIn(html, ids, "book-remove-box", fonts, w, 900)
}
