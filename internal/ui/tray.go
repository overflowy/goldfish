package ui

import (
	"goldfish/internal/session"

	"github.com/mappu/miqt/qt6"
)

// Tray is the menu-bar item and the app's primary control surface: Start focus,
// Take a break, Pause/Resume, and Abandon, each enabled only when it applies to
// the current phase. Sync must be called whenever the session may have changed
// so the items reflect reality.
type Tray struct {
	icon    *qt6.QSystemTrayIcon
	session *session.Session

	startAct   *qt6.QAction
	breakAct   *qt6.QAction
	pauseAct   *qt6.QAction
	abandonAct *qt6.QAction

	onChange func() // redraw the overlay after an action
}

// NewTray builds the menu-bar item. onChange is invoked after any action so the
// overlay can repaint immediately rather than waiting for the next poll.
func NewTray(s *session.Session, onChange func()) *Tray {
	t := &Tray{
		icon:     qt6.NewQSystemTrayIcon(),
		session:  s,
		onChange: onChange,
	}
	t.icon.SetIcon(trayIcon())
	t.icon.SetToolTip("Goldfish")

	menu := qt6.NewQMenu2()

	t.startAct = qt6.NewQAction2("Start focus")
	t.startAct.OnTriggered(func() { t.do(t.startFocus) })
	menu.QWidget.AddAction(t.startAct)

	t.breakAct = qt6.NewQAction2("Take a break")
	t.breakAct.OnTriggered(func() { t.do(s.TakeBreak) })
	menu.QWidget.AddAction(t.breakAct)

	t.pauseAct = qt6.NewQAction2("Pause")
	t.pauseAct.OnTriggered(func() { t.do(t.pauseResume) })
	menu.QWidget.AddAction(t.pauseAct)

	t.abandonAct = qt6.NewQAction2("Abandon")
	t.abandonAct.OnTriggered(func() { t.do(s.Abandon) })
	menu.QWidget.AddAction(t.abandonAct)

	menu.AddSeparator()
	quit := qt6.NewQAction2("Quit Goldfish")
	quit.OnTriggered(qt6.QCoreApplication_Quit)
	menu.QWidget.AddAction(quit)

	t.icon.SetContextMenu(menu)
	t.icon.Show()
	t.Sync()
	return t
}

// do runs a transition then refreshes both the menu and the overlay.
func (t *Tray) do(action func()) {
	action()
	t.Sync()
	if t.onChange != nil {
		t.onChange()
	}
}

// startFocus is "Start focus" across phases: begin from Idle, or advance out of a
// break into the next focus block.
func (t *Tray) startFocus() {
	switch t.session.Phase() {
	case session.Idle:
		t.session.StartFocus()
	case session.Break, session.LongBreak:
		t.session.StartNextFocus()
	}
}

func (t *Tray) pauseResume() {
	if t.session.Paused() {
		t.session.Resume()
	} else {
		t.session.Pause()
	}
}

// Sync updates each item's enabled state and label to match the current phase.
func (t *Tray) Sync() {
	phase := t.session.Phase()
	focusing := phase == session.Focus

	// Start focus applies whenever we're not already focusing (Idle, or in a
	// break ready to advance).
	t.startAct.SetEnabled(!focusing)
	// Take a break and Abandon only make sense during a focus block.
	t.breakAct.SetEnabled(focusing)
	t.abandonAct.SetEnabled(focusing)
	// Pause toggles to Resume; available whenever a phase is running.
	t.pauseAct.SetEnabled(t.session.Running())
	if t.session.Paused() {
		t.pauseAct.SetText("Resume")
	} else {
		t.pauseAct.SetText("Pause")
	}
}

// trayIcon paints a small filled dot at runtime so Goldfish ships no image asset.
func trayIcon() *qt6.QIcon {
	pm := qt6.NewQPixmap2(22, 22)
	pm.FillWithFillColor(qt6.NewQColor6("transparent"))

	p := qt6.NewQPainter()
	p.Begin(pm.QPaintDevice)
	p.SetRenderHint(qt6.QPainter__Antialiasing)
	p.SetPen(qt6.NewQColor6("transparent"))
	p.SetBrush(qt6.NewQBrush3(qt6.NewQColor6("#f5a623")))
	p.DrawEllipse2(3, 3, 16, 16)
	p.End()

	return qt6.NewQIcon2(pm)
}
