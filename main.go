// Goldfish is a macOS focus aid: a Pomodoro-style overlay that floats over other
// apps so the current time-box and intention stay in view. It runs as a menu-bar
// agent (no dock icon) whose single window is the always-present overlay.
package main

import (
	"os"
	"time"

	"goldfish/internal/config"
	"goldfish/internal/session"
	"goldfish/internal/ui"

	"github.com/mappu/miqt/qt6"
)

func main() {
	qt6.NewQApplication(os.Args)
	// The overlay is the only window; it must never be the reason the app quits,
	// and hiding it must not end the process. Quit is via the menu-bar item.
	qt6.QGuiApplication_SetQuitOnLastWindowClosed(false)

	cfg := config.Load()
	sess := session.New(durationsFromConfig(cfg))

	overlay := ui.NewOverlay(sess, func(x, y int) {
		cfg.WindowX, cfg.WindowY = x, y
		_ = cfg.Save() // a failed position save is not worth interrupting focus
	})
	overlay.Show(cfg.WindowX, cfg.WindowY)

	installTray()

	// Poll the session a few times a second so the countdown, overtime flip, and
	// one-shot chime stay live without the session having to push events.
	ticker := qt6.NewQTimer()
	ticker.OnTimeout(overlay.Refresh)
	ticker.Start(250)

	qt6.QApplication_Exec()
}

func durationsFromConfig(cfg config.Config) session.Durations {
	return session.Durations{
		Focus:     time.Duration(cfg.FocusMinutes) * time.Minute,
		Break:     time.Duration(cfg.BreakMinutes) * time.Minute,
		LongBreak: time.Duration(cfg.LongBreakMinutes) * time.Minute,
	}
}

// installTray adds the menu-bar item whose only job is to quit the agent (the
// overlay handles all session control). The tray reference is intentionally not
// retained: it lives for the process lifetime via Qt's ownership.
func installTray() {
	tray := qt6.NewQSystemTrayIcon()
	tray.SetIcon(trayIcon())
	tray.SetToolTip("Goldfish")

	menu := qt6.NewQMenu2()
	quit := qt6.NewQAction2("Quit Goldfish")
	quit.OnTriggered(qt6.QCoreApplication_Quit)
	menu.QWidget.AddAction(quit)

	tray.SetContextMenu(menu)
	tray.Show()
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
