// Package ui renders the Overlay: the small, frameless, always-on-top window
// that is Goldfish's entire visible surface. It owns no timing logic — it polls
// a *session.Session each tick and draws whatever that reports. The only control
// it carries is a Start-focus button shown while Idle; everything else lives in
// the menu-bar item (see tray.go).
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
	startBtn   *qt6.QPushButton // shown only while Idle

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

	// The only on-overlay control: Start focus. It is shown while Idle and hidden
	// once a block is running (the "disappears after clicking" behaviour). All
	// other actions live in the menu bar, so the layout is otherwise fixed.
	o.startBtn = qt6.NewQPushButton3("Start focus")
	o.startBtn.SetObjectName(asv("primary"))
	o.startBtn.OnClicked(func() { o.session.StartFocus(); o.Refresh() })
	col.AddWidget(o.startBtn.QWidget)

	o.root.SetStyleSheet(globalQSS)

	o.installDrag()
	o.Refresh()
	return o
}

// Refresh redraws everything from the session. Called on a timer and after every
// action.
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
	o.card.SetStyleSheet(fmt.Sprintf("#card{background:%s;border-radius:14px;}", cardColor(phase)))

	o.startBtn.SetVisible(phase == session.Idle)
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

// installDrag wires body-dragging (frameless windows have no title bar). The
// position is persisted via onMove when the drag ends.
func (o *Overlay) installDrag() {
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
QPushButton#primary {
	background: rgba(255,255,255,0.22);
	color: #f5f5fa;
	border: none; border-radius: 6px;
	padding: 6px 10px; font-size: 13px; font-weight: 600;
}
QPushButton#primary:hover { background: rgba(255,255,255,0.30); }
`
