// Goldfish is a macOS focus aid: a Pomodoro-style overlay that floats over other
// apps so the current time-box stays in view. It runs as a menu-bar agent (no
// dock icon) whose single window is the always-present overlay.
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
	qt6.QGuiApplication_SetQuitOnLastWindowClosed(false)

	cfg := config.Load()
	sess := session.New(durationsFromConfig(cfg))
	sess.SetAutoStartBreaks(cfg.AutoStartBreaks)
	sess.SetAutoStartFocus(cfg.AutoStartFocus)

	overlay := ui.NewOverlay(sess, func(x, y int) {
		cfg.WindowX, cfg.WindowY = x, y
		_ = cfg.Save() // a failed position save is not worth interrupting focus
	})
	overlay.Show(cfg.WindowX, cfg.WindowY)

	tray := ui.NewTray(sess, overlay.Refresh, func() {
		cfg.AutoStartBreaks = sess.AutoStartBreaks()
		cfg.AutoStartFocus = sess.AutoStartFocus()
		_ = cfg.Save()
	})
	overlay.OnContextMenu(tray.PopupMenu)

	// Poll a few times a second so the countdown, the auto-advance at zero, and
	// the menu states stay live without the session pushing events.
	ticker := qt6.NewQTimer()
	ticker.OnTimeout(func() {
		overlay.Tick()
		tray.Sync()
	})
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
