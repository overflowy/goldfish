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

	// The menu-bar item is the primary control surface; it repaints the overlay
	// immediately after any action.
	tray := ui.NewTray(sess, overlay.Refresh)

	// Poll the session a few times a second so the countdown, overtime flip,
	// one-shot chime, and menu enabled-states stay live without the session
	// having to push events.
	ticker := qt6.NewQTimer()
	ticker.OnTimeout(func() {
		overlay.Refresh()
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
