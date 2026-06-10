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
// Sets dialogOpen for the duration so the game ignores input while the dialog is up.
func exportSaveDialog(g *Game) {
	g.dialogOpen.Store(true)
	go func() {
		defer g.dialogOpen.Store(false)
		path, err := zenity.SelectFileSave(
			zenity.Filename(exportFilename()),
			zenity.FileFilters{{Name: "JSON", Patterns: []string{"*.json"}}},
		)
		if err != nil || path == "" {
			return
		}
		_ = SaveTo(g.world, path)
	}()
}

// importSaveDialog opens a file picker, validates the chosen JSON as a compatible save,
// and sends the loaded world to g.importCh.
// Sets dialogOpen for the duration so the game ignores input while the dialog is up.
func importSaveDialog(g *Game) {
	g.dialogOpen.Store(true)
	go func() {
		defer g.dialogOpen.Store(false)
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
