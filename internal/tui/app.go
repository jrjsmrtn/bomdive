// SPDX-FileCopyrightText: 2026 Georges Martin <jrjsmrtn@gmail.com>
//
// SPDX-License-Identifier: Apache-2.0

// Package tui draws the Miller-column view.
//
// It is a THIN painting layer. Every decision — what a column contains, where the
// cursor is, which way the view faces, what the detail pane shows — belongs to
// internal/columns, which has no widget dependency. If this library is ever
// replaced, nothing about the view's behaviour changes.
package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/jrjsmrtn/bomdive/internal/bom"
	"github.com/jrjsmrtn/bomdive/internal/columns"
	"github.com/rivo/tview"
)

// visibleColumns is how many panes are shown at once. Descending past this scrolls
// the chain left; the path line keeps the part that scrolled off visible.
const visibleColumns = 3

// detailPageSize is how far PgDn moves the detail pane. Deliberately not a full
// screen: overlapping by a few rows keeps a long property list readable.
const detailPageSize = 10

type ui struct {
	app    *tview.Application
	model  *columns.Model
	source string

	header    *tview.TextView
	panes     *tview.Flex
	detail    *tview.TextView
	status    *tview.TextView
	filter    *tview.InputField
	root      *tview.Flex
	filtering bool

	// Two overlays, deliberately separate. `?` EXPLAINS this document — what the
	// status label means here, why the leftmost column shows what it shows. `H`
	// LISTS THE KEYS. They answer different questions ("why does it say that" and
	// "what can I press"), and merging them buries the contextual half under a key
	// table the reader has usually already learned.
	//
	// pages is what puts them OVER the view rather than replacing it: a reader
	// asking what "partial" means should not lose their place to find out.
	overlayView *tview.TextView
	pages       *tview.Pages
	overlay     overlayKind

	// overlayName, overlayLines and overlayScroll let the overlay report and move
	// when its text does not fit — the key table runs past a short terminal, and a
	// pane cut off with no indicator looks complete. Same rule as the detail pane.
	overlayName   string
	overlayLines  int
	overlayScroll int

	// width is the screen width, captured by the before-draw hook. tview exposes no
	// GetScreen, so this is the only honest way to know how wide a pane may be.
	width int

	// height is the screen height, captured alongside width.
	//
	// The detail pane's inner height is DERIVED from it rather than read from the
	// widget. GetInnerRect in a before-draw hook reports the PREVIOUS frame's
	// layout, so the same run yielded 8 or 20 depending on draw timing, and the
	// overflow indicator appeared or vanished accordingly. Deriving it from a
	// layout this file controls is deterministic; querying it was not.
	height int

	// detailScroll is the detail pane's first visible row. Reset whenever the
	// selection changes, since an offset carried across components would show the
	// middle of one record while its header says another.
	detailScroll int
}

// Run opens the column view over a loaded BOM and blocks until the user quits.
func Run(g *bom.Graph, source string) error { return run(g, source, nil) }

// run takes an optional screen so tests can drive the view on a tcell simulation
// screen and read back what was actually drawn. "It compiles" is not evidence that
// a view renders.
func run(g *bom.Graph, source string, screen tcell.Screen) error {
	u := newUI(g, source)
	if screen != nil {
		u.app.SetScreen(screen)
	}
	u.app.SetInputCapture(u.keys)
	// Rebuild the panes whenever the terminal width changes, including the FIRST
	// draw — before which tview reports no geometry at all.
	u.app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		w, h := screen.Size()
		if w != u.width || h != u.height {
			u.width, u.height = w, h
			u.redraw()
		}
		return false
	})
	u.redraw()
	return u.app.SetRoot(u.pages, true).EnableMouse(false).Run()
}

func newUI(g *bom.Graph, source string) *ui {
	u := &ui{
		app:    tview.NewApplication(),
		model:  columns.New(g),
		source: source,
		header: tview.NewTextView().SetDynamicColors(true),
		detail: tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		status: tview.NewTextView().SetDynamicColors(true),
		panes:  tview.NewFlex(),
	}
	u.detail.SetBorder(true).SetTitle(" detail ")

	u.filter = tview.NewInputField().SetLabel("filter: ").
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEscape {
				u.model.Filter("")
			}
			u.filtering = false
			u.root.RemoveItem(u.filter)
			u.app.SetFocus(u.root)
			u.redraw()
		})
	u.filter.SetChangedFunc(func(text string) {
		u.model.Filter(text)
		u.redraw()
	})

	body := tview.NewFlex().
		AddItem(u.panes, 0, 3, true).
		AddItem(u.detail, 0, 2, false)

	u.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(u.header, 2, 0, false).
		AddItem(body, 0, 1, true).
		AddItem(u.status, 1, 0, false)

	u.overlayView = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	u.overlayView.SetBorder(true)

	u.pages = tview.NewPages().AddPage("main", u.root, true, true)

	return u
}

// centred puts a primitive in the middle of the screen, rows tall, which is what
// tview offers instead of a floating window. A fixed row count rather than a
// proportion, so the box hugs its content: an overlay two-thirds empty reads as
// something that failed to load.
func centred(p tview.Primitive, rows int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, rows, 0, true).
			AddItem(nil, 0, 1, false), 0, 8, true).
		AddItem(nil, 0, 1, false)
}

// overlayWidth is the help box's inner width: 8/10 of the screen, less its border.
func (u *ui) overlayWidth() int {
	w := u.width
	if w <= 0 {
		w = 80
	}
	if inner := (w * 8 / 10) - 2; inner > 20 {
		return inner
	}
	return 20
}

// overlayRows is how tall the box must be to hold the text, wrapping included, and
// never taller than the screen. Same measurement the detail pane needs, for the
// same reason: a pane sized by guesswork is either clipped or mostly empty.
func (u *ui) overlayRows(text string) int {
	rows := u.overlayContentRows(text) + 2 // the border
	if u.height > 2 && rows > u.height-2 {
		return u.height - 2
	}
	return rows
}

// overlayContentRows is how many rows the text occupies, wrapping included and
// border excluded.
func (u *ui) overlayContentRows(text string) int {
	w := u.overlayWidth()
	rows := 0
	for _, line := range strings.Split(text, "\n") {
		rows += wrappedRows(stripTags(line), w)
	}
	return rows
}

// stripTags removes tview colour tags, which occupy no cells and so must not be
// counted when measuring how far a line wraps.
func stripTags(s string) string { return colourTag.ReplaceAllString(s, "") }

var colourTag = regexp.MustCompile(`\[[a-zA-Z-]+\]`)

// toggleHelp opens or closes the overlay. The text is rebuilt on OPEN rather than
// on redraw: it describes the current selection's context, and a stale help screen
// explaining a component the cursor has left is worse than none.
// overlayKind is which of the two overlays is up, if either.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayExplain
	overlayHelp
)

// showOverlay opens one overlay, switches between them, or closes. Asking for the
// one already open closes it, so the key that opened it also dismisses it.
func (u *ui) showOverlay(kind overlayKind) {
	u.pages.RemovePage("overlay")
	if kind == u.overlay || kind == overlayNone {
		u.overlay = overlayNone
		return
	}
	u.overlay = kind
	u.overlayScroll = 0

	name, text := "what this means", u.explainText()
	if kind == overlayHelp {
		name, text = "keys", u.helpText()
	}
	u.overlayName = name
	u.overlayLines = u.overlayContentRows(text)
	u.overlayView.SetText(text)
	// Rebuilt rather than shown: the box is sized to the text it holds, and the
	// screen may have been resized since the last time it was opened.
	u.pages.AddPage("overlay", centred(u.overlayView, u.overlayRows(text)), true, true)
	u.paintOverlay()
}

// scrollOverlay moves the overlay and CLAMPS, so its end cannot be scrolled past
// into blankness that reads as "nothing here".
func (u *ui) scrollOverlay(by int) {
	max := u.overlayLines - u.visibleOverlayRows()
	if max < 0 {
		max = 0
	}
	u.overlayScroll += by
	if u.overlayScroll > max {
		u.overlayScroll = max
	}
	if u.overlayScroll < 0 {
		u.overlayScroll = 0
	}
	u.paintOverlay()
}

func (u *ui) paintOverlay() {
	u.overlayView.ScrollTo(u.overlayScroll, 0)
	u.overlayView.SetTitle(u.overlayTitle())
}

// visibleOverlayRows is the box's inner height: what it was allotted, less border.
func (u *ui) visibleOverlayRows() int {
	if h := u.overlayBoxRows() - 2; h > 0 {
		return h
	}
	return 1
}

// overlayBoxRows is what centred was given: the content plus border, capped by the
// screen. Recomputed rather than stored, so a resize cannot leave it stale.
func (u *ui) overlayBoxRows() int {
	rows := u.overlayLines + 2
	if u.height > 2 && rows > u.height-2 {
		return u.height - 2
	}
	return rows
}

// overlayTitle names the overlay, and says where you are in it when it does not
// fit. Without that a clipped key table is indistinguishable from a complete one.
func (u *ui) overlayTitle() string {
	closer := "?"
	if u.overlay == overlayHelp {
		closer = "H"
	}
	h := u.visibleOverlayRows()
	if u.overlayLines <= h {
		return fmt.Sprintf(" %s — %s or Esc to close ", u.overlayName, closer)
	}
	shown := u.overlayScroll + h
	if shown > u.overlayLines {
		shown = u.overlayLines
	}
	more := ""
	if u.overlayScroll > 0 {
		more += "↑"
	}
	if shown < u.overlayLines {
		more += "↓"
	}
	return fmt.Sprintf(" %s %d-%d of %d %s — ⇞⇟ scroll, %s or Esc to close ",
		u.overlayName, u.overlayScroll+1, shown, u.overlayLines, more, closer)
}

func (u *ui) keys(ev *tcell.EventKey) *tcell.EventKey {
	if u.filtering {
		return ev // the input field owns the keyboard
	}
	// While an overlay is up it owns the keyboard, except for quitting and for the
	// other overlay's key — any navigation key would move a view the reader cannot
	// see. Anything else closes, so no keypress leaves the reader stuck.
	if u.overlay != overlayNone {
		switch {
		case ev.Key() == tcell.KeyCtrlC, ev.Rune() == 'q':
			u.app.Stop()
		case ev.Rune() == '?':
			u.showOverlay(overlayExplain)
		case ev.Rune() == 'H':
			u.showOverlay(overlayHelp)
		// Scrolling must NOT close: an overlay that dismisses itself when you try to
		// read the rest of it is worse than one that never scrolled.
		case ev.Key() == tcell.KeyPgDn, ev.Key() == tcell.KeyCtrlD:
			u.scrollOverlay(+detailPageSize)
		case ev.Key() == tcell.KeyPgUp, ev.Key() == tcell.KeyCtrlU:
			u.scrollOverlay(-detailPageSize)
		case ev.Rune() == 'J':
			u.scrollOverlay(+1)
		case ev.Rune() == 'K':
			u.scrollOverlay(-1)
		default:
			u.showOverlay(overlayNone)
		}
		return nil
	}
	switch ev.Key() {
	case tcell.KeyUp:
		u.model.Up()
		u.detailScroll = 0
	case tcell.KeyDown:
		u.model.Down()
		u.detailScroll = 0
	case tcell.KeyRight, tcell.KeyEnter:
		u.model.Right()
		u.detailScroll = 0
	case tcell.KeyLeft:
		u.model.Left()
		u.detailScroll = 0
	case tcell.KeyTab:
		u.model.ToggleDirection()
		u.detailScroll = 0
	// The detail pane needs its own keys: the arrows drive the columns, and a real
	// HBOM device carries enough cdx:hbom:* properties to run off the bottom. A
	// pane cut off with no way down — and no sign there IS a down — looks complete,
	// which is the failure the coverage line exists to prevent.
	case tcell.KeyPgDn, tcell.KeyCtrlD:
		u.scrollDetail(+detailPageSize)
	case tcell.KeyPgUp, tcell.KeyCtrlU:
		u.scrollDetail(-detailPageSize)
	case tcell.KeyHome:
		u.detailScroll = 0
	case tcell.KeyEnd:
		u.scrollDetail(u.detailLines())
	case tcell.KeyEsc:
		u.model.Filter("")
	case tcell.KeyCtrlC:
		u.app.Stop()
		return nil
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'q':
			u.app.Stop()
			return nil
		case 'k':
			u.model.Up()
			u.detailScroll = 0
		case 'j':
			u.model.Down()
			u.detailScroll = 0
		case 'l':
			u.model.Right()
			u.detailScroll = 0
		case 'h':
			u.model.Left()
			u.detailScroll = 0
		case 'J':
			u.scrollDetail(+1)
		case 'K':
			u.scrollDetail(-1)
		case '?':
			u.showOverlay(overlayExplain)
			return nil
		// H, not h: h is Left, and this tool's premise is that ls/tree muscle memory
		// carries over. J and K are already the shifted forms of j and k.
		case 'H':
			u.showOverlay(overlayHelp)
			return nil
		// v switches between the component axis and the vulnerability axis
		// (ADR-0009). Not ⇥: that reverses edges, which is a different operation.
		case 'v':
			if !u.model.ToggleMode() {
				return nil // no vulnerability records: nothing to switch to
			}
			u.detailScroll = 0
		case '/':
			u.filtering = true
			u.filter.SetText(u.model.Active().Filter)
			u.root.AddItem(u.filter, 1, 0, true)
			u.app.SetFocus(u.filter)
			return nil
		default:
			return ev
		}
	default:
		return ev
	}
	u.redraw()
	return nil
}

// detailWidth is the detail pane's inner width, derived from the body Flex's 3:2
// split minus its two border columns.
func (u *ui) detailWidth() int {
	if u.width <= 0 {
		return 40
	}
	if w := (u.width * 2 / 5) - 2; w > 4 {
		return w
	}
	return 4
}

// detailLines is how many rows the pane will actually OCCUPY, wrapping included.
//
// It used to return len(Detail()) * 2, on the assumption that every field is a key
// row plus a value row. The pane wraps, and a real OBOM carries values of 70-plus
// characters in a pane around 55 wide — so the estimate undercounted, `max` clamped
// to zero, PgDn did nothing and the title claimed everything was visible. Reported
// from dogfooding an OBOM, which is exactly where long values live.
func (u *ui) detailLines() int {
	w := u.detailWidth()
	rows := 0
	for _, kv := range u.model.Detail() {
		rows += wrappedRows(kv.Key, w)
		rows += wrappedRows(kv.Value, w-2) // the value is indented two columns
	}
	return rows
}

// wrappedRows is how many terminal rows a string occupies at the given width.
func wrappedRows(s string, width int) int {
	if width < 1 {
		width = 1
	}
	n := utf8.RuneCountInString(s)
	if n == 0 {
		return 1
	}
	return (n + width - 1) / width
}

// scrollDetail moves the pane and CLAMPS, so the end of a list cannot be scrolled
// past into blankness that reads as "nothing here".
func (u *ui) scrollDetail(by int) {
	h := u.visibleDetailRows()
	max := u.detailLines() - h
	if max < 0 {
		max = 0
	}
	u.detailScroll += by
	if u.detailScroll > max {
		u.detailScroll = max
	}
	if u.detailScroll < 0 {
		u.detailScroll = 0
	}
}

func (u *ui) redraw() {
	cols := u.model.Columns()
	u.panes.Clear()

	// Show the rightmost panes; the path line carries what scrolled off the left.
	start := 0
	if len(cols) > visibleColumns {
		start = len(cols) - visibleColumns
	}
	// Width comes from the SCREEN, not from the Flex. redraw runs before the first
	// layout, when GetRect still reports zero — which silently truncated every label
	// to the 8-cell floor ("roots" became "…ots"). The simulation-screen test caught
	// it; nothing about the code looked wrong.
	screenW := u.width
	if screenW <= 0 {
		screenW = 80
	}
	// The detail pane takes 2/5 of the width (the body Flex is 3:2), so the columns
	// share the other 3/5 — divided by how many are ACTUALLY shown, not by the
	// maximum. Dividing by the maximum truncated a lone 46-wide column's entries to
	// 22 ("certificates" became "…tificates"), which is exactly the sort of thing
	// that looks fine in code and wrong on screen.
	shown := len(cols) - start
	if shown < 1 {
		shown = 1
	}
	paneW := (screenW * 3 / 5) / shown
	if paneW < 8 {
		paneW = 8
	}

	for i := start; i < len(cols); i++ {
		c := cols[i]
		list := tview.NewList().ShowSecondaryText(false)
		list.SetBorder(true).SetTitle(" " + truncTitle(c.Title, paneW-4) + " ")
		// rowW is paneW-3, one cell narrower than the box's inner width, because the
		// Flex distributes any rounding remainder and a pane can come out a cell
		// narrower than paneW. Overshooting would CLIP the arrow rather than wrap
		// it, silently removing the indicator at exactly the widths where the
		// columns are tightest.
		rowW := paneW - 3
		for _, n := range c.Visible() {
			label := u.model.RowLabel(n, paneW-6)
			list.AddItem(columns.Row(label, u.model.Descendable(n), rowW), "", 0, nil)
		}
		if i == len(cols)-1 {
			list.SetCurrentItem(c.Cursor)
			list.SetSelectedBackgroundColor(tcell.ColorTeal)
		} else {
			// A pane left of the cursor shows the step taken, dimmed.
			list.SetCurrentItem(c.Cursor)
			list.SetSelectedBackgroundColor(tcell.ColorDarkSlateGray)
		}
		u.panes.AddItem(list, 0, 1, false)
	}

	u.header.SetText(u.headerText())
	u.detail.SetText(u.detailText())
	u.detail.ScrollTo(u.detailScroll, 0)
	u.detail.SetTitle(u.detailTitle())
	u.status.SetText(u.statusText())
}

func (u *ui) headerText() string {
	id := u.model.Graph().Identity()
	// The direction is stated because forward and reverse are visually identical,
	// and a view that silently swapped them would lie (ADR-0007).
	path := strings.Join(u.model.Path(), " [darkgray]>[-] ")
	if path == "" {
		path = "[darkgray](nothing selected)[-]"
	}
	if mem, ok := u.model.Unloaded(); ok {
		line := "not loaded: " + mem.Reason()
		if u.width > 0 {
			line = truncHead(line, u.width-1)
		}
		return fmt.Sprintf("[red]%s[-]\n[darkgray]path:[-] %s", tview.Escape(line), path)
	}
	// The first line must FIT. The header is exactly two rows, so a first line that
	// wrapped pushed the path line off the screen: at 80 columns an OBOM's identity
	// plus "showing: dependencies" was already 82 characters. The identity is trimmed
	// rather than the direction, because the direction changes with every flip and
	// ADR-0007 requires it stated; the identity is constant, and `?` shows it whole.
	showing := u.model.Showing()
	identity := id.Describe()
	if w := u.width; w > 0 {
		room := w - utf8.RuneCountInString("   showing: "+showing) - 1
		identity = truncHead(identity, room)
	}
	return fmt.Sprintf("[white]%s[-]   showing: [yellow]%s[-]\n[darkgray]path:[-] %s",
		tview.Escape(identity), showing, path)
}

// truncHead keeps the HEAD of s, which for an identity is the part that matters —
// "OBOM (root operating-system; …" — unlike a component name, whose tail matters.
func truncHead(s string, w int) string {
	if w < 2 {
		w = 2
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return string(r[:w-1]) + columns.Ellipsis
}

// detailTitle says whether the pane is showing everything. Without it a truncated
// record is indistinguishable from a complete one.
// visibleDetailRows derives the detail pane's inner height from the screen and the
// fixed chrome this file lays out: a 2-row header, a 1-row status bar, and the
// pane's own top and bottom border.
const chromeRows = 2 + 1 + 2

func (u *ui) visibleDetailRows() int {
	if u.height <= 0 {
		return 20 // before the first frame; only affects the opening paint
	}
	if h := u.height - chromeRows; h > 0 {
		return h
	}
	return 1
}

func (u *ui) detailTitle() string {
	h := u.visibleDetailRows()
	total := u.detailLines()
	if total <= h {
		return " detail "
	}
	shown := u.detailScroll + h
	if shown > total {
		shown = total
	}
	more := ""
	if u.detailScroll > 0 {
		more += "↑"
	}
	if shown < total {
		more += "↓"
	}
	return fmt.Sprintf(" detail %d-%d of %d %s ", u.detailScroll+1, shown, total, more)
}

func (u *ui) detailText() string {
	var b strings.Builder
	for _, kv := range u.model.Detail() {
		fmt.Fprintf(&b, "[darkgray]%s[-]\n  %s\n", tview.Escape(kv.Key), tview.Escape(kv.Value))
	}
	if b.Len() == 0 {
		return "[darkgray](nothing selected)[-]"
	}
	return b.String()
}

// statusText carries the coverage line. ADR-0004 makes coverage a correctness
// guarantee across surfaces, and a TUI is a third surface — a column view that
// showed three of nine thousand components without saying so would be exactly the
// failure the text renderer is built to avoid.
func (u *ui) statusText() string {
	if u.model.Mode() == columns.ModeVulnerabilities {
		return u.vulnStatusText()
	}
	// A file that failed to load has no coverage to report; showing another document's
	// would describe the wrong file.
	if _, ok := u.model.Unloaded(); ok {
		return "[red]not loaded[-]  [darkgray]↑↓ nav  [-][white]?[-][darkgray]why [-]" +
			"[white]H[-][darkgray]keys q quit[-]"
	}
	c := u.model.Graph().Coverage()

	// One label per state, and RED reserved for the one state that is a defect in
	// the document. It previously read "partial" for a BOM with no `dependencies`
	// field, which is the opposite claim: nothing was missing, because nothing was
	// ever declared. See bom.GraphState.
	warn := ""
	switch st := c.State(); st {
	case bom.GraphPartial:
		warn = "  [red]" + st.Label() + "[-]"
	case bom.GraphDeclaredEmpty, bom.GraphUndeclared:
		warn = "  [yellow]" + st.Label() + "[-]"
	}
	// With nothing to inventory there is nothing for a graph to cover, so the graph
	// state is beside the point — "relations undeclared" on a standalone VEX is true
	// and answers a question nobody asked. Same precedence the notes use.
	if c.Components == 0 {
		warn = "  [yellow]no components[-]"
	}
	// Kept short on purpose: at 120 columns the longer form ran off the right edge
	// and silently lost "q quit", which is the one hint a user cannot do without.
	// That is also why the label is terse and `?` carries the explanation — the
	// status bar has no room to say what "partial" means, and a word a reader
	// cannot expand is a word that misleads.
	// The v hint only where there is something to switch to.
	vulns := ""
	if u.model.HasVulnerabilities() {
		vulns = "[-][white]v[-][darkgray]vulns "
	}
	return fmt.Sprintf("[darkgray]coverage[-] %d/%d (%d%%)%s  [darkgray]"+
		"↑↓ →← nav  ⇞⇟ detail  ⇥flip %s/filter [-][white]?[-][darkgray]why [-]"+
		"[white]H[-][darkgray]keys q quit[-]",
		c.InGraph, c.Components, c.Percent(), warn, vulns)
}

// explainText answers "why does it say that", for THIS document: what the status
// bar's one word means here, what the coverage numbers mean, and why the leftmost
// column shows what it shows. It carries no key table — that is `H`, and a reader
// who wants to know what "partial" means is not asking to be taught the keyboard.
func (u *ui) explainText() string {
	if u.model.Mode() == columns.ModeVulnerabilities {
		return u.vulnExplainText()
	}
	if mem, ok := u.model.Unloaded(); ok {
		return fmt.Sprintf("[red]not loaded[-]\n[darkgray]%s[-]\n\n%s\n\n"+
			"This file is, or says it is, a CycloneDX document, and it could not be read. It is "+
			"listed rather than skipped: a set that silently lost a document would show its links "+
			"as linked, not loaded, and never say why.\n\n[darkgray]this view[-]  %s\n\n"+
			"[darkgray]press H for the keys.[-]\n",
			tview.Escape(mem.Path), tview.Escape(mem.Reason()), u.axisText())
	}
	g := u.model.Graph()
	c := g.Coverage()

	var b strings.Builder
	fmt.Fprintf(&b, "[white]%s[-]\n[darkgray]%s[-]\n\n", g.Identity().Describe(), u.sourceText())

	label := c.State().Label()
	if label == "" {
		label = "complete"
	}
	fmt.Fprintf(&b, "[darkgray]the status bar says[-]  coverage %d/%d (%d%%)  [white]%s[-]\n\n",
		c.InGraph, c.Components, c.Percent(), label)
	if empty, isEmpty := g.Contents().ExplainEmpty(); isEmpty {
		fmt.Fprintf(&b, "%s\n\n", empty)
	} else {
		fmt.Fprintf(&b, "%s\n\n", c.Explain())
	}
	fmt.Fprintf(&b, "[darkgray]this view[-]  %s\n", u.axisText())

	// A SAMPLE, not the list. A public 837-component SBOM carries 278 dangling
	// refs; printing them all filled this overlay to the full screen and pushed the
	// explanation it exists for off the top.
	if n := len(c.Dangling); n > 0 {
		shown, omitted := c.SampleDangling()
		more := ""
		if omitted > 0 {
			more = fmt.Sprintf(", and %d more", omitted)
		}
		fmt.Fprintf(&b, "\n[red]%d dependsOn target(s) match no component[-] — the graph "+
			"references components this document does not contain, so a walk stops "+
			"short of them: %s%s\n", n, strings.Join(shown, ", "), more)
	}

	b.WriteString("\n[darkgray]press H for the keys.[-]\n")
	return b.String()
}

// helpText answers "what can I press". Static, unlike explainText — the keys do not
// depend on the document.
func (u *ui) helpText() string {
	return "[darkgray]navigate[-]\n" +
		"  ↑ ↓ / j k    move within a column\n" +
		"  → ← / l h    descend into a component, or back out\n" +
		"  ⇥            flip between depends-on and depended-on-by; on the vulnerability\n" +
		"               axis, between vulnerabilities and the components they affect\n" +
		"  v            switch between components and vulnerabilities, when the document\n" +
		"               has any; switching back returns you to where you were\n" +
		"  /            filter the current column; Esc clears the filter\n" +
		"\n[darkgray]detail pane[-]\n" +
		"  ⇞ ⇟          scroll a page; Ctrl-U and Ctrl-D do the same\n" +
		"  J K          scroll a line\n" +
		"  Home End     jump to either end\n" +
		"\n[darkgray]overlays[-]\n" +
		"  ?            explain THIS document — what the status label means here\n" +
		"  H            this list\n" +
		"  Esc          close an overlay\n" +
		"\n[darkgray]quit[-]\n" +
		"  q            quit; Ctrl-C does the same\n"
}

// axisText says what the entry column lists AND why that axis was chosen, because
// the choice is made from the document rather than by the user.
//
// One line per case, unwrapped: the pane wraps, and hard-wrapping inside text that
// is wrapped again produces a ragged column with orphaned fragments.
// filesAxisText says where the files came from, and names every file left out and why
// (ADR-0009 [D]). A skipped file is not a row, so this is the one place to find it.
func (u *ui) filesAxisText() string {
	s := u.model.Set()
	var b strings.Builder
	if dirs := s.Directories(); len(dirs) > 0 {
		fmt.Fprintf(&b, "the leftmost column lists the CycloneDX documents directly inside %s, "+
			"one level deep and sorted by name, and any file named beside it. BOM-Links resolve "+
			"among all of them, so a link reads differently depending on what else is in the "+
			"directory: one whose target two documents with different content claim reads "+
			"linked, ambiguous. ", tview.Escape(strings.Join(dirs, ", ")))
	} else {
		fmt.Fprintf(&b, "the leftmost column lists the %d documents named on the command line, "+
			"in the order given. BOM-Links resolve among all of them. ", s.Len())
	}
	b.WriteString("Descend into one to open it, and v shows every document's vulnerabilities.")
	failed := 0
	for _, m := range s.Members() {
		if m.Err != nil {
			failed++
		}
	}
	if failed > 0 {
		fmt.Fprintf(&b, "\n\n[red]%d file(s) not loaded[-]: each is a row marked \"not loaded\", "+
			"and its detail says why.", failed)
	}
	if sk := s.Skipped(); len(sk) > 0 {
		fmt.Fprintf(&b, "\n\n[yellow]%d file(s) skipped[-], because they are not CycloneDX documents:", len(sk))
		for _, x := range sk {
			fmt.Fprintf(&b, "\n  %s — %s", tview.Escape(filepath.Base(x.Path)), tview.Escape(x.Reason))
		}
	}
	return b.String()
}

// sourceText is the file the explanation is about: the one argument, or — with several
// named — the document under the cursor, since that is what the rest of `?` describes.
func (u *ui) sourceText() string {
	if len(u.model.Set().Members()) > 1 {
		return u.model.Graph().Path()
	}
	return u.source
}

func (u *ui) axisText() string {
	switch u.model.Axis() {
	case columns.AxisFiles:
		return u.filesAxisText()
	case columns.AxisCategories:
		return "the leftmost column lists `cdx:osquery:category` values, because this document " +
			"declares no dependency graph. That is the axis it does give you — descend into a " +
			"category to see its components."
	case columns.AxisRecords:
		return "this document lists no components, so the leftmost column shows what it does " +
			"carry: its vulnerability records. Descend into one to see what it affects; v groups " +
			"them."
	case columns.AxisSubject:
		return "this document lists no components and no vulnerability records, so the leftmost " +
			"column shows its subject — the product it describes. Its detail shows any " +
			"vulnerabilities other documents attribute to it."
	case columns.AxisFlat:
		return "the leftmost column lists every component flat. With no relations declared there " +
			"is nothing to descend into, and no categories to group by either."
	default:
		if _, synthetic := u.model.Graph().Roots(); synthetic {
			return "the leftmost column lists DERIVED roots — components nothing else depends " +
				"on. This document's declared root is not in its own dependency graph, so the " +
				"roots were inferred rather than read."
		}
		return "the leftmost column lists the document's declared roots. Descend to see what a " +
			"component depends on; ⇥ flips to what depends on it."
	}
}

func truncTitle(s string, w int) string {
	if w < 4 {
		w = 4
	}
	return columns.Truncate(s, w)
}
