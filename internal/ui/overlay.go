package ui

import (
	"fmt"
	"strings"
	"time"

	"goldfish/internal/session"

	"github.com/mappu/miqt/qt6"
)

const (
	cardWidth  = 155
	edgeMargin = 16

	// rowGap is shared by the phase row (label → dots) and the timer row (time →
	// buttons) so the two gaps match.
	rowGap = 10
)

// asv wraps a Go string as the QAnyStringView value miqt expects for a few
// setters (e.g. SetObjectName).
func asv(s string) qt6.QAnyStringView { return *qt6.NewQAnyStringView3(s) }

// cardRadius/borderWidth: the body is painted by hand (antialiased) rather than
// via CSS border-radius, which Qt does not antialias — jagged over a light
// background behind.
const (
	cardRadius  = 14
	borderWidth = 1.5
)

const (
	opacityActive   = 1.0
	opacityInactive = 0.8
)

type Overlay struct {
	root    *qt6.QWidget
	session *session.Session

	bg string // current body colour (hex), read by the paintEvent

	phaseLabel *qt6.QLabel
	dotsLabel  *qt6.QLabel
	timeLabel  *qt6.QLabel

	onMove    func(x, y int)        // persist position after a drag
	onContext func(pos *qt6.QPoint) // raise the context menu at a point

	dragging       bool
	dragDX, dragDY int
}

func NewOverlay(s *session.Session, onMove func(x, y int)) *Overlay {
	o := &Overlay{session: s, onMove: onMove, bg: colorIdle}

	o.root = qt6.NewQWidget(nil)
	// CustomizeWindowHint grants only the capabilities we list — no minimize/
	// maximize/resize. Deliberately NOT Qt::Tool — on macOS a Tool window
	// auto-hides whenever the app is deactivated, which would make the overlay
	// vanish the moment the user clicks into another app.
	o.root.SetWindowFlags(qt6.FramelessWindowHint | qt6.WindowStaysOnTopHint | qt6.CustomizeWindowHint)
	o.root.SetAttribute2(qt6.WA_TranslucentBackground, true)
	o.root.SetFocusPolicy(qt6.StrongFocus) // accept keys when clicked (spacebar)
	o.root.SetFixedWidth(cardWidth)
	o.installPaint()
	o.installContextMenu()
	o.installActivation()
	o.installKeys()

	col := qt6.NewQVBoxLayout(o.root)
	col.SetContentsMargins(14, 12, 14, 12)
	col.SetSpacing(8)

	phaseRow := qt6.NewQHBoxLayout(nil)
	phaseRow.SetContentsMargins(0, 0, 0, 0)
	phaseRow.SetSpacing(rowGap)

	o.phaseLabel = qt6.NewQLabel2()
	o.phaseLabel.SetObjectName(asv("phase"))
	phaseRow.AddWidget3(o.phaseLabel.QWidget, 0, qt6.AlignVCenter)

	o.dotsLabel = qt6.NewQLabel2()
	o.dotsLabel.SetObjectName(asv("phase"))
	phaseRow.AddWidget3(o.dotsLabel.QWidget, 0, qt6.AlignVCenter)
	phaseRow.AddStretch()
	col.AddLayout(phaseRow.QLayout)

	// A fixed-pitch font plus zero-padded digits keeps the timer a constant width.
	// Set programmatically (not via the stylesheet) so the family actually resolves.
	o.timeLabel = qt6.NewQLabel2()
	o.timeLabel.SetObjectName(asv("time"))
	mono := qt6.NewQFont2("Menlo")
	mono.SetPixelSize(32)
	mono.SetBold(true)
	mono.SetFixedPitch(true)
	mono.SetStyleHint(qt6.QFont__Monospace)
	o.timeLabel.SetFont(mono)
	col.AddWidget(o.timeLabel.QWidget)

	o.root.SetStyleSheet(fmt.Sprintf(
		"QLabel#phase { color: %s; font-size: 12px; }\nQLabel#time { color: %s; }",
		colorPhaseText, colorTimeText))

	o.installDrag()
	o.Refresh()
	return o
}

// Tick advances the cycle and chimes on a hand-off, then redraws. Driven by the
// poll timer (see main.go).
func (o *Overlay) Tick() {
	if o.session.Tick() {
		playChime()
	}
	o.Refresh()
}

// Drives opacity from window activation events.
func (o *Overlay) installActivation() {
	o.root.OnChangeEvent(func(super func(e *qt6.QEvent), e *qt6.QEvent) {
		super(e)
		if e.Type() == qt6.QEvent__ActivationChange {
			o.applyOpacity()
		}
	})
}

func (o *Overlay) applyOpacity() {
	if o.root.IsActiveWindow() {
		o.root.SetWindowOpacity(opacityActive)
	} else {
		o.root.SetWindowOpacity(opacityInactive)
	}
}

// installKeys makes spacebar a play/pause toggle while the overlay is focused.
func (o *Overlay) installKeys() {
	o.root.OnKeyPressEvent(func(super func(e *qt6.QKeyEvent), e *qt6.QKeyEvent) {
		if e.Key() == int(qt6.Key_Space) {
			o.session.ToggleRun()
			o.Refresh()
			return
		}
		super(e)
	})
}

func (o *Overlay) Refresh() {
	phase := o.session.Phase()

	o.phaseLabel.SetText(phaseText(phase))
	if phase == session.Idle {
		o.dotsLabel.SetText("")
	} else {
		o.dotsLabel.SetText(dotsText(o.session.FocusDone()))
	}
	o.timeLabel.SetText(formatRemaining(o.session.Remaining()))

	if bg := cardColor(phase); bg != o.bg { // repaint only on a colour change
		o.bg = bg
		o.root.Update()
	}
}

func (o *Overlay) Show(savedX, savedY int) {
	o.root.Show()
	// Lock the height to the laid-out size so the window is fully fixed (width is
	// already fixed) and the user can't resize it.
	o.root.SetFixedHeight(o.root.Height())
	o.applyOpacity()
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

func (o *Overlay) installPaint() {
	o.root.OnPaintEvent(func(super func(e *qt6.QPaintEvent), e *qt6.QPaintEvent) {
		p := qt6.NewQPainter()
		p.Begin(o.root.QPaintDevice)
		p.SetRenderHint(qt6.QPainter__Antialiasing)
		pen := qt6.NewQPen3(qt6.NewQColor6(darken(o.bg, borderDarken)))
		pen.SetWidthF(borderWidth)
		p.SetPenWithPen(pen)
		p.SetBrush(qt6.NewQBrush3(qt6.NewQColor6(o.bg)))
		// Inset by half the pen width so the stroked border stays inside the window.
		const inset = borderWidth / 2
		rect := qt6.NewQRectF4(inset, inset, float64(o.root.Width())-2*inset, float64(o.root.Height())-2*inset)
		p.DrawRoundedRect(rect, cardRadius, cardRadius)
		p.End()
	})
}

func (o *Overlay) OnContextMenu(fn func(pos *qt6.QPoint)) { o.onContext = fn }

func (o *Overlay) installContextMenu() {
	o.root.OnContextMenuEvent(func(super func(e *qt6.QContextMenuEvent), e *qt6.QContextMenuEvent) {
		if o.onContext != nil {
			o.onContext(e.GlobalPos())
		}
	})
}

// Wires body-dragging (frameless windows have no title bar).
func (o *Overlay) installDrag() {
	o.root.OnMousePressEvent(func(super func(e *qt6.QMouseEvent), e *qt6.QMouseEvent) {
		if e.Button() != qt6.LeftButton {
			return // right-click is for the context menu
		}
		o.dragging = true
		o.dragDX = e.GlobalX() - o.root.X()
		o.dragDY = e.GlobalY() - o.root.Y()
	})
	o.root.OnMouseMoveEvent(func(super func(e *qt6.QMouseEvent), e *qt6.QMouseEvent) {
		if o.dragging {
			o.root.Move(e.GlobalX()-o.dragDX, e.GlobalY()-o.dragDY)
		}
	})
	o.root.OnMouseReleaseEvent(func(super func(e *qt6.QMouseEvent), e *qt6.QMouseEvent) {
		if o.dragging {
			o.dragging = false
			if o.onMove != nil {
				o.onMove(o.root.X(), o.root.Y())
			}
		}
	})
}

func formatRemaining(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int(d / time.Second)
	return fmt.Sprintf("%02d:%02d", total/60, total%60)
}

func phaseText(p session.Phase) string {
	switch p {
	case session.Focus:
		return "Focus"
	case session.Break:
		return "Break"
	case session.LongBreak:
		return "Long break"
	default:
		return "Ready to focus"
	}
}

func dotsText(focusDone int) string {
	var dots strings.Builder
	for i := range 4 {
		if i < focusDone {
			dots.WriteString("●")
		} else {
			dots.WriteString("○")
		}
	}
	return dots.String()
}

func cardColor(p session.Phase) string {
	switch p {
	case session.Focus:
		return colorFocus
	case session.Break:
		return colorBreak
	case session.LongBreak:
		return colorLongBreak
	default:
		return colorIdle
	}
}
