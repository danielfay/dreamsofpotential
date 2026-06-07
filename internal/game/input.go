package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// handleInput processes build-placement input. It must be called after
// g.ui.Update() so that EbitenUI has already consumed any widget clicks
// this frame, preventing a HUD button click from simultaneously placing
// a camp on the world.
func (g *Game) handleInput() {
	if !g.placing {
		return
	}

	// Cancel placement with Escape or right-click.
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.placing = false
		return
	}

	// Confirm placement on left-click outside the HUD.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if g.hud.pointInHUD(mx, my) {
			return // click was on the HUD panel; ignore
		}
		wp := screenToWorld(mx, my)
		if !onDisc(wp, g.world.Planet) {
			return // clicked off the planet; ignore (ghost turns red as feedback)
		}
		placeCamp(g.world, wp)
		g.placing = false
	}
}

// placeCamp deducts the camp cost and appends a new Building at world position pos.
func placeCamp(w *World, pos Vec) {
	cost := CampCost(w)
	w.Economy.Wood -= cost
	w.Economy.CampsBought++
	w.Buildings = append(w.Buildings, &Building{Pos: pos})
}

// ghostPos returns the current world-space cursor position when in placement mode,
// or nil when not placing. Used by render.go to draw the preview building.
func (g *Game) ghostPos() *Vec {
	if !g.placing {
		return nil
	}
	mx, my := ebiten.CursorPosition()
	wp := screenToWorld(mx, my)
	return &wp
}
