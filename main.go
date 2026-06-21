package main

import (
	"os"

	"github.com/mappu/miqt/qt6"
)

func main() {
	qt6.NewQApplication(os.Args)

	window := qt6.NewQMainWindow(nil)
	window.SetWindowTitle("Goldfish")
	window.Resize(640, 480)

	label := qt6.NewQLabel5("Hello from miqt 🐟", window.QWidget)
	label.SetAlignment(qt6.AlignCenter)
	window.SetCentralWidget(label.QWidget)

	window.Show()

	qt6.QApplication_Exec()
}
