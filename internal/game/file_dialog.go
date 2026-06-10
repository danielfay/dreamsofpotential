//go:build !js

package game

import (
	"time"

	"github.com/ncruces/zenity"
)

func exportFilename() string {
	return time.Now().Format("dreams-2006-01-02-15-04.json")
}

// exportSaveDialog opens a save-file dialog and writes the world JSON to the chosen path.
// Runs in a goroutine so the game loop is not blocked.
func exportSaveDialog(w *World) {
	go func() {
		path, err := zenity.SelectFileSave(
			zenity.Filename(exportFilename()),
			zenity.FileFilters{{Name: "JSON", Patterns: []string{"*.json"}}},
		)
		if err != nil || path == "" {
			return
		}
		_ = SaveTo(w, path)
	}()
}

// importSaveDialog opens a file picker, validates the chosen JSON as a compatible save,
// and sends the loaded world to g.importCh. Runs in a goroutine.
func importSaveDialog(g *Game) {
	go func() {
		path, err := zenity.SelectFile(
			zenity.FileFilters{{Name: "JSON", Patterns: []string{"*.json"}}},
		)
		if err != nil || path == "" {
			return
		}
		world, err := LoadFrom(path)
		if err != nil {
			return
		}
		g.importCh <- world
	}()
}
