package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	FocusMinutes     int
	BreakMinutes     int
	LongBreakMinutes int
	WindowX          int
	WindowY          int
	AutoStartBreaks  bool
	AutoStartFocus   bool
}

// Default returns the opinionated classic-Pomodoro durations with an unset
// window position and auto-start-breaks enabled.
func Default() Config {
	return Config{
		FocusMinutes:     25,
		BreakMinutes:     5,
		LongBreakMinutes: 15,
		WindowX:          -1,
		WindowY:          -1,
		AutoStartBreaks:  true,
		AutoStartFocus:   true,
	}
}

// Path resolves the config file location, honouring XDG_CONFIG_HOME. Order:
//  1. $XDG_CONFIG_HOME/goldfish/config.toml
//  2. $HOME/.config/goldfish/config.toml
//  3. $HOME/.goldfish/config.toml      (fallback when ~/.config is unavailable)
func Path() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "goldfish", "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// No home at all: keep it beside the process rather than crashing.
		return "goldfish-config.toml"
	}
	dotConfig := filepath.Join(home, ".config")
	if fi, err := os.Stat(dotConfig); err == nil && fi.IsDir() {
		return filepath.Join(dotConfig, "goldfish", "config.toml")
	}
	return filepath.Join(home, ".goldfish", "config.toml")
}

// Load reads the config at Path. A missing file is not an error — it yields the
// defaults so first launch just works. Unknown keys and unparseable values are
// ignored so a partially hand-edited file never blocks startup.
func Load() Config {
	cfg := Default()
	f, err := os.Open(Path())
	if err != nil {
		return cfg
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		atoi := func() (int, bool) { n, err := strconv.Atoi(val); return n, err == nil }
		parseBool := func() (bool, bool) { b, err := strconv.ParseBool(val); return b, err == nil }
		switch key {
		case "focus_minutes":
			if n, ok := atoi(); ok {
				cfg.FocusMinutes = n
			}
		case "break_minutes":
			if n, ok := atoi(); ok {
				cfg.BreakMinutes = n
			}
		case "long_break_minutes":
			if n, ok := atoi(); ok {
				cfg.LongBreakMinutes = n
			}
		case "window_x":
			if n, ok := atoi(); ok {
				cfg.WindowX = n
			}
		case "window_y":
			if n, ok := atoi(); ok {
				cfg.WindowY = n
			}
		case "auto_start_breaks":
			if b, ok := parseBool(); ok {
				cfg.AutoStartBreaks = b
			}
		case "auto_start_focus":
			if b, ok := parseBool(); ok {
				cfg.AutoStartFocus = b
			}
		}
	}
	_ = sc.Err() // keep whatever parsed; never block startup on a read error
	return cfg.sanitised()
}

// Save writes the config, creating the parent directory as needed. Callers may
// reasonably ignore the error — a failed save isn't worth interrupting a session.
func (c Config) Save() error {
	c = c.sanitised()
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Goldfish configuration\n")
	b.WriteString("# Durations are in minutes.\n\n")
	fmt.Fprintf(&b, "focus_minutes = %d\n", c.FocusMinutes)
	fmt.Fprintf(&b, "break_minutes = %d\n", c.BreakMinutes)
	fmt.Fprintf(&b, "long_break_minutes = %d\n\n", c.LongBreakMinutes)
	b.WriteString("# Overlay position; -1 means default (top-right).\n")
	fmt.Fprintf(&b, "window_x = %d\n", c.WindowX)
	fmt.Fprintf(&b, "window_y = %d\n\n", c.WindowY)
	b.WriteString("# Auto-start the next phase when the current one ends.\n")
	fmt.Fprintf(&b, "auto_start_breaks = %t\n", c.AutoStartBreaks)
	fmt.Fprintf(&b, "auto_start_focus = %t\n", c.AutoStartFocus)
	return os.WriteFile(p, []byte(b.String()), 0o644)
}

// Sanitised clamps durations to at least one minute so a corrupt or zero value
// can never produce a phase that is "over" the instant it starts.
func (c Config) sanitised() Config {
	if c.FocusMinutes < 1 {
		c.FocusMinutes = 1
	}
	if c.BreakMinutes < 1 {
		c.BreakMinutes = 1
	}
	if c.LongBreakMinutes < 1 {
		c.LongBreakMinutes = 1
	}
	return c
}
