package ui

import (
	"goldfish/internal/session"

	"github.com/mappu/miqt/qt6"
)

type Tray struct {
	icon    *qt6.QSystemTrayIcon
	session *session.Session
	menu    *qt6.QMenu

	startAct     *qt6.QAction
	breakAct     *qt6.QAction
	pauseAct     *qt6.QAction
	abandonAct   *qt6.QAction
	resetAct     *qt6.QAction
	autoBreakAct *qt6.QAction
	autoFocusAct *qt6.QAction

	iconColor string

	onChange   func() // redraw the overlay after an action
	onSettings func() // persist the auto-start toggles
}

func NewTray(s *session.Session, onChange func(), onSettings func()) *Tray {
	t := &Tray{
		icon:       qt6.NewQSystemTrayIcon(),
		session:    s,
		onChange:   onChange,
		onSettings: onSettings,
	}
	t.iconColor = trayColor(s.Phase())
	t.icon.SetIcon(trayIcon(t.iconColor))
	t.icon.SetToolTip("Goldfish")

	menu := qt6.NewQMenu2()
	t.menu = menu

	t.startAct = qt6.NewQAction2("Start focus")
	t.startAct.OnTriggered(func() { t.do(s.StartFocus) })
	menu.QWidget.AddAction(t.startAct)

	t.breakAct = qt6.NewQAction2("Take a break")
	t.breakAct.OnTriggered(func() { t.do(s.TakeBreak) })
	menu.QWidget.AddAction(t.breakAct)

	t.pauseAct = qt6.NewQAction2("Pause")
	t.pauseAct.OnTriggered(func() { t.do(s.ToggleRun) })
	menu.QWidget.AddAction(t.pauseAct)

	t.abandonAct = qt6.NewQAction2("Abandon")
	t.abandonAct.OnTriggered(func() { t.do(s.Abandon) })
	menu.QWidget.AddAction(t.abandonAct)

	t.resetAct = qt6.NewQAction2("Reset")
	t.resetAct.OnTriggered(func() { t.do(s.Stop) })
	menu.QWidget.AddAction(t.resetAct)

	menu.AddSeparator()
	t.autoBreakAct = t.addCheckable(menu, "Auto-start breaks", s.AutoStartBreaks, s.SetAutoStartBreaks)
	t.autoFocusAct = t.addCheckable(menu, "Auto-start focus", s.AutoStartFocus, s.SetAutoStartFocus)

	menu.AddSeparator()
	quit := qt6.NewQAction2("Quit Goldfish")
	quit.OnTriggered(qt6.QCoreApplication_Quit)
	menu.QWidget.AddAction(quit)

	t.icon.SetContextMenu(menu)
	t.icon.Show()
	t.Sync()
	return t
}

func (t *Tray) addCheckable(menu *qt6.QMenu, label string, get func() bool, set func(bool)) *qt6.QAction {
	act := qt6.NewQAction2(label)
	act.SetCheckable(true)
	act.SetChecked(get())
	act.OnTriggered(func() {
		set(act.IsChecked())
		if t.onSettings != nil {
			t.onSettings()
		}
	})
	menu.QWidget.AddAction(act)
	return act
}

// PopupMenu raises the same menu at a global point (the overlay's right-click),
// syncing its state first.
func (t *Tray) PopupMenu(pos *qt6.QPoint) {
	t.Sync()
	t.menu.Popup(pos)
}

func (t *Tray) do(action func()) {
	action()
	t.Sync()
	if t.onChange != nil {
		t.onChange()
	}
}

func (t *Tray) Sync() {
	s := t.session

	// Enable each item straight from the session's capability predicates rather
	// than re-deriving the transition rules here.
	t.startAct.SetEnabled(s.CanStartFocus())
	t.breakAct.SetEnabled(s.CanTakeBreak())
	t.abandonAct.SetEnabled(s.CanAbandon())
	t.resetAct.SetEnabled(s.CanReset())
	t.pauseAct.SetEnabled(s.CanPauseResume())
	if s.Paused() {
		t.pauseAct.SetText("Resume")
	} else {
		t.pauseAct.SetText("Pause")
	}
	t.autoBreakAct.SetChecked(s.AutoStartBreaks())
	t.autoFocusAct.SetChecked(s.AutoStartFocus())

	if c := trayColor(s.Phase()); c != t.iconColor {
		t.iconColor = c
		t.icon.SetIcon(trayIcon(c))
	}
}

// trayColor is the phase's body colour, lightened to read against the menu bar.
func trayColor(p session.Phase) string {
	return lighten(cardColor(p), trayLighten)
}

// trayIcon paints a small filled dot so Goldfish ships no image asset.
func trayIcon(color string) *qt6.QIcon {
	pm := qt6.NewQPixmap2(22, 22)
	pm.FillWithFillColor(qt6.NewQColor6("transparent"))

	p := qt6.NewQPainter()
	p.Begin(pm.QPaintDevice)
	p.SetRenderHint(qt6.QPainter__Antialiasing)
	p.SetPen(qt6.NewQColor6("transparent"))
	p.SetBrush(qt6.NewQBrush3(qt6.NewQColor6(color)))
	p.DrawEllipse2(3, 3, 16, 16)
	p.End()

	return qt6.NewQIcon2(pm)
}
