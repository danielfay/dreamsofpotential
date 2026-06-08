package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// handleInput processes global keys and build-placement input. It must be
// called after g.ui.Update() so that EbitenUI has already consumed any widget
// clicks this frame, preventing a HUD button click from simultaneously placing
// a camp on the world.
func (g *Game) handleInput() {
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		g.debug = !g.debug
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}

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
	// Any direction snaps to the rim. For the first camp, buyCamp enforces a
	// local-node check; if it fails we stay in placement mode so the player can
	// retry at a better angle.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if g.hud.pointInHUD(mx, my, g.debug) {
			return // click was on the HUD panel; ignore
		}
		wp := g.screenToWorld(mx, my)
		theta := g.world.Planet.AngleOf(wp)
		if buyCamp(g.world, theta) {
			g.placing = false
		}
	}
}

// buyCamp attempts to place a camp at the given rim angle. Returns true if the
// camp was placed. The first camp (CampsBought==0) is free but requires at
// least one free resource node within previewArc radians; later camps cost
// CampCost and skip the local-node check.
func buyCamp(w *World, angle float64) bool {
	if !buildPreview(w, angle).Valid {
		return false
	}
	cost := CampCost(w)
	if w.Economy.Wood < cost {
		return false
	}
	w.Economy.Wood -= cost
	w.Economy.CampsBought++
	w.Buildings = append(w.Buildings, &Building{
		Angle: angle,
		Pos:   w.Planet.RimPoint(angle),
	})
	return true
}

// curPlacementPreview returns the placement preview for the current cursor
// position, or nil when not placing or the cursor is too far from the rim.
func (g *Game) curPlacementPreview() *placementPreview {
	if !g.placing {
		return nil
	}
	mx, my := ebiten.CursorPosition()
	wp := g.screenToWorld(mx, my)
	center := g.world.Planet.Center
	radius := g.world.Planet.Radius
	dist := wp.Dist(center)
	if dist < radius-rimSnapBand || dist > radius+rimSnapBand {
		return nil
	}
	angle := g.world.Planet.AngleOf(wp)
	pv := buildPreview(g.world, angle)
	return &pv
}
