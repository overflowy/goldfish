// Package ui renders the Overlay: the small, frameless, always-on-top window
// that is Goldfish's entire visible surface. It owns no timing logic — it polls
// a *session.Session each tick and draws whatever that reports, and turns clicks
// back into session transitions.
package ui

import (
	"fmt"
	"strings"
	"time"

	"goldfish/internal/session"

	"github.com/mappu/miqt/qt6"
)

const (
	cardWidth  = 230 // fixed overlay width in px
	edgeMargin = 16  // gap from screen edge for the default top-right placement
)

// asv wraps a Go string as the QAnyStringView value miqt expects for a few
// setters (e.g. SetObjectName).
func asv(s string) qt6.QAnyStringView { return *qt6.NewQAnyStringView3(s) }

// Overlay is the floating window and its widgets, bound to a session.
type Overlay struct {
	root    *qt6.QWidget // top-level frameless window (transparent)
	card    *qt6.QWidget // the visible rounded body
	session *session.Session

	phaseLabel *qt6.QLabel
	timeLabel  *qt6.QLabel
	intention  *qt6.QLineEdit

	primaryBtn *qt6.QPushButton
	pauseBtn   *qt6.QPushButton
	abandonBtn *qt6.QPushButton
	stopBtn    *qt6.QPushButton
	secondary  *qt6.QWidget // hover-revealed row holding the secondary buttons

	onMove func(x, y int) // persist position after a drag

	wasOvertime bool // edge-detect the zero-crossing to chime once

	dragging       bool
	dragDX, dragDY int
}

// NewOverlay builds the window for the given session. onMove is called with the
// new top-left whenever the user finishes dragging, so the caller can persist it.
func NewOverlay(s *session.Session, onMove func(x, y int)) *Overlay {
	o := &Overlay{session: s, onMove: onMove}

	o.root = qt6.NewQWidget(nil)
	// FramelessWindowHint: no title bar (we drag the body). WindowStaysOnTopHint:
	// float above other apps' normal windows. Deliberately NOT Qt::Tool — on
	// macOS a Tool window auto-hides whenever the app is deactivated, which would
	// make the overlay vanish the moment the user clicks into another app: the
	// exact opposite of an always-present focus anchor.
	o.root.SetWindowFlags(qt6.FramelessWindowHint | qt6.WindowStaysOnTopHint)
	o.root.SetAttribute2(qt6.WA_TranslucentBackground, true)
	o.root.SetFixedWidth(cardWidth)

	rootLayout := qt6.NewQVBoxLayout(o.root)
	rootLayout.SetContentsMargins(0, 0, 0, 0)

	o.card = qt6.NewQWidget(o.root)
	o.card.SetObjectName(asv("card"))
	rootLayout.AddWidget(o.card)

	col := qt6.NewQVBoxLayout(o.card)
	col.SetContentsMargins(14, 12, 14, 12)
	col.SetSpacing(8)

	o.phaseLabel = qt6.NewQLabel2()
	o.phaseLabel.SetObjectName(asv("phase"))
	col.AddWidget(o.phaseLabel.QWidget)

	o.timeLabel = qt6.NewQLabel2()
	o.timeLabel.SetObjectName(asv("time"))
	col.AddWidget(o.timeLabel.QWidget)

	o.intention = qt6.NewQLineEdit(o.card)
	o.intention.SetPlaceholderText("What are you working on?")
	o.intention.OnTextChanged(func(t string) { o.session.SetIntention(t) })
	col.AddWidget(o.intention.QWidget)

	// Primary action: always visible, label/effect depend on the phase.
	o.primaryBtn = qt6.NewQPushButton3("")
	o.primaryBtn.SetObjectName(asv("primary"))
	o.primaryBtn.OnClicked(o.onPrimary)
	col.AddWidget(o.primaryBtn.QWidget)

	// Secondary actions: hidden until the pointer is over the overlay.
	o.secondary = qt6.NewQWidget(o.card)
	secRow := qt6.NewQHBoxLayout(o.secondary)
	secRow.SetContentsMargins(0, 0, 0, 0)
	secRow.SetSpacing(8)

	o.pauseBtn = qt6.NewQPushButton3("Pause")
	o.pauseBtn.OnClicked(o.onPauseResume)
	secRow.AddWidget(o.pauseBtn.QWidget)

	o.abandonBtn = qt6.NewQPushButton3("Abandon")
	o.abandonBtn.OnClicked(func() { o.session.Abandon(); o.Refresh() })
	secRow.AddWidget(o.abandonBtn.QWidget)

	o.stopBtn = qt6.NewQPushButton3("Stop")
	o.stopBtn.OnClicked(func() { o.session.Stop(); o.Refresh() })
	secRow.AddWidget(o.stopBtn.QWidget)

	col.AddWidget(o.secondary)
	o.secondary.SetVisible(false)

	o.root.SetStyleSheet(globalQSS)

	o.installDragAndHover()
	o.Refresh()
	return o
}

// onPrimary dispatches the single primary button to the right transition for the
// current phase, then redraws for immediate feedback.
func (o *Overlay) onPrimary() {
	switch o.session.Phase() {
	case session.Idle:
		o.session.StartFocus()
	case session.Focus:
		o.session.TakeBreak()
	case session.Break, session.LongBreak:
		o.session.StartNextFocus()
	}
	o.Refresh()
}

func (o *Overlay) onPauseResume() {
	if o.session.Paused() {
		o.session.Resume()
	} else {
		o.session.Pause()
	}
	o.Refresh()
}

// Refresh redraws everything from the session. Called on a timer and after every
// action. It must never write to the intention field (that would fight the
// user's cursor) — the field is the source of truth for the intention text.
func (o *Overlay) Refresh() {
	phase := o.session.Phase()
	overtime := o.session.Overtime()

	// Chime exactly once at the zero-crossing (docs/adr/0001).
	if overtime && !o.wasOvertime {
		playChime()
	}
	o.wasOvertime = overtime

	o.phaseLabel.SetText(phaseText(phase, o.session.FocusDone(), overtime))
	o.timeLabel.SetText(formatRemaining(o.session.Remaining()))
	o.timeLabel.SetStyleSheet("color:" + timeColor(overtime) + ";")

	o.primaryBtn.SetText(primaryText(phase))
	o.card.SetStyleSheet(fmt.Sprintf("#card{background:%s;border-radius:14px;}", cardColor(phase)))

	// Which secondary buttons make sense in this phase.
	running := o.session.Running()
	o.pauseBtn.SetVisible(running)
	if o.session.Paused() {
		o.pauseBtn.SetText("Resume")
	} else {
		o.pauseBtn.SetText("Pause")
	}
	o.abandonBtn.SetVisible(phase == session.Focus)
	o.stopBtn.SetVisible(phase == session.Break || phase == session.LongBreak)
}

// Show places the overlay at the saved position (or default top-right) and shows
// it. savedX/savedY < 0 means "unset".
func (o *Overlay) Show(savedX, savedY int) {
	o.root.Show() // realise the window so width/height are valid
	x, y := savedX, savedY
	if x < 0 || y < 0 {
		x, y = o.defaultTopRight()
	}
	o.root.Move(x, y)
}

func (o *Overlay) defaultTopRight() (int, int) {
	geo := qt6.QGuiApplication_PrimaryScreen().AvailableGeometry()
	x := geo.X() + geo.Width() - o.root.Width() - edgeMargin
	y := geo.Y() + edgeMargin
	return x, y
}

// installDragAndHover wires body-dragging (frameless windows have no title bar)
// and the hover-reveal of the secondary controls.
func (o *Overlay) installDragAndHover() {
	o.card.OnMousePressEvent(func(super func(e *qt6.QMouseEvent), e *qt6.QMouseEvent) {
		o.dragging = true
		o.dragDX = e.GlobalX() - o.root.X()
		o.dragDY = e.GlobalY() - o.root.Y()
	})
	o.card.OnMouseMoveEvent(func(super func(e *qt6.QMouseEvent), e *qt6.QMouseEvent) {
		if o.dragging {
			o.root.Move(e.GlobalX()-o.dragDX, e.GlobalY()-o.dragDY)
		}
	})
	o.card.OnMouseReleaseEvent(func(super func(e *qt6.QMouseEvent), e *qt6.QMouseEvent) {
		if o.dragging {
			o.dragging = false
			if o.onMove != nil {
				o.onMove(o.root.X(), o.root.Y())
			}
		}
	})

	o.card.OnEnterEvent(func(super func(e *qt6.QEnterEvent), e *qt6.QEnterEvent) {
		o.secondary.SetVisible(true)
	})
	o.card.OnLeaveEvent(func(super func(e *qt6.QEvent), e *qt6.QEvent) {
		// Leaving the card onto a child button is not really leaving the
		// overlay — only hide once the cursor is outside the whole window.
		if !o.cursorInsideWindow() {
			o.secondary.SetVisible(false)
		}
	})
}

func (o *Overlay) cursorInsideWindow() bool {
	p := qt6.QCursor_Pos()
	x, y := o.root.X(), o.root.Y()
	w, h := o.root.Width(), o.root.Height()
	return p.X() >= x && p.X() < x+w && p.Y() >= y && p.Y() < y+h
}

// --- pure formatting helpers ----------------------------------------------

func formatRemaining(d time.Duration) string {
	neg := d < 0
	if neg {
		d = -d
	}
	total := int(d / time.Second)
	sign := ""
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%s%d:%02d", sign, total/60, total%60)
}

func phaseText(p session.Phase, focusDone int, overtime bool) string {
	var name string
	switch p {
	case session.Idle:
		return "Ready to focus"
	case session.Focus:
		name = "Focus"
	case session.Break:
		name = "Break"
	case session.LongBreak:
		name = "Long break"
	}
	var dots strings.Builder
	for i := range 4 {
		if i < focusDone {
			dots.WriteString("●")
		} else {
			dots.WriteString("○")
		}
	}
	if overtime {
		name += " · overtime"
	}
	return name + "   " + dots.String()
}

func primaryText(p session.Phase) string {
	switch p {
	case session.Focus:
		return "Take a break"
	default: // Idle, Break, LongBreak all start a focus block
		return "Start focus"
	}
}

func cardColor(p session.Phase) string {
	switch p {
	case session.Focus:
		return "#3b3470"
	case session.Break:
		return "#2f5d50"
	case session.LongBreak:
		return "#2f5266"
	default:
		return "#2a2a3a"
	}
}

func timeColor(overtime bool) string {
	if overtime {
		return "#ff6b6b"
	}
	return "#f5f5fa"
}

const globalQSS = `
QLabel#phase { color: #b8b8c8; font-size: 12px; }
QLabel#time { color: #f5f5fa; font-size: 34px; font-weight: 600; }
QLineEdit {
	background: rgba(255,255,255,0.08);
	color: #f5f5fa;
	border: none; border-radius: 6px;
	padding: 5px 8px; font-size: 13px;
}
QPushButton {
	background: rgba(255,255,255,0.12);
	color: #f5f5fa;
	border: none; border-radius: 6px;
	padding: 6px 10px; font-size: 13px;
}
QPushButton:hover { background: rgba(255,255,255,0.20); }
QPushButton#primary { background: rgba(255,255,255,0.22); font-weight: 600; }
QPushButton#primary:hover { background: rgba(255,255,255,0.30); }
`
