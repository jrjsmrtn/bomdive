// Package tui draws the Miller-column view.
//
// It is a THIN painting layer. Every decision — what a column contains, where the
// cursor is, which way the view faces, what the detail pane shows — belongs to
// internal/columns, which has no widget dependency. If this library is ever
// replaced, nothing about the view's behaviour changes.
package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jrjsmrtn/lsxbom/internal/bom"
	"github.com/jrjsmrtn/lsxbom/internal/columns"
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
	return u.app.SetRoot(u.root, true).EnableMouse(false).Run()
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

	return u
}

func (u *ui) keys(ev *tcell.EventKey) *tcell.EventKey {
	if u.filtering {
		return ev // the input field owns the keyboard
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

// detailLines is how many rows the detail pane holds.
func (u *ui) detailLines() int { return len(u.model.Detail()) * 2 }

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
		for _, n := range c.Visible() {
			// A component may carry no name — seen in a real HBOM — and a blank row
			// is unselectable-looking and unsearchable. Fall back to the ref, which
			// is the only field guaranteed to be there.
			label := n.Name
			if label == "" {
				label = n.Ref
			}
			list.AddItem(columns.Label(label, n.Version, paneW-6), "", 0, nil)
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
	dir := "[yellow]" + u.model.Direction().String() + "[-]"
	path := strings.Join(u.model.Path(), " [darkgray]>[-] ")
	if path == "" {
		path = "[darkgray](nothing selected)[-]"
	}
	return fmt.Sprintf("[white]%s[-]   showing: %s\n[darkgray]path:[-] %s", id.Describe(), dir, path)
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
	c := u.model.Graph().Coverage()
	pct := 0
	if c.Components > 0 {
		pct = c.InGraph * 100 / c.Components
	}
	warn := ""
	if !c.Complete() {
		warn = "  [red]partial[-]"
	}
	if u.model.ByCategory() {
		warn = "  [yellow]no dependency graph — browsing categories[-]"
	}
	// Kept short on purpose: at 120 columns the longer form ran off the right edge
	// and silently lost "q quit", which is the one hint a user cannot do without.
	return fmt.Sprintf("[darkgray]coverage[-] %d/%d (%d%%)%s  [darkgray]"+
		"↑↓ →← nav  ⇞⇟ detail  ⇥flip /filter q quit[-]",
		c.InGraph, c.Components, pct, warn)
}

func truncTitle(s string, w int) string {
	if w < 4 {
		w = 4
	}
	return columns.Truncate(s, w)
}
