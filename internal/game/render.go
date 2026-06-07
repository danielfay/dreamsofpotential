package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// virtW / virtH is the low-res canvas size. The window (640×480) is exactly
// 2× this, so we get clean integer nearest-neighbour scaling with no artifacts.
const (
	virtW = 320
	virtH = 240
)

// scaleX / scaleY are the factors from world space to screen space.
// With a 640×480 window over a 320×240 canvas this is always 2.
const scaleX = 2
const scaleY = 2

// screenToWorld converts a native screen position (e.g. from ebiten.CursorPosition)
// to the low-res world coordinate. With a plain 2× stretch and no letterbox the
// offset is zero.
func screenToWorld(sx, sy int) Vec {
	return Vec{X: float64(sx) / scaleX, Y: float64(sy) / scaleY}
}

// palette
var (
	colBackground  = color.RGBA{R: 10, G: 10, B: 20, A: 255}   // near-black space
	colPlanet      = color.RGBA{R: 60, G: 100, B: 60, A: 255}   // muted green
	colPlanetHi    = color.RGBA{R: 80, G: 130, B: 80, A: 255}   // lighter highlight arc
	colForest      = color.RGBA{R: 30, G: 140, B: 50, A: 255}   // bright green cluster
	colBuilding    = color.RGBA{R: 140, G: 90, B: 50, A: 255}   // brown camp
	colWorkerEmpty = color.RGBA{R: 220, G: 200, B: 150, A: 255} // pale tan
	colWorkerLaden = color.RGBA{R: 255, G: 240, B: 80, A: 255}  // bright yellow
	colGhostOk     = color.RGBA{R: 200, G: 200, B: 255, A: 160} // translucent blue — valid placement
)

// DrawWorld renders the complete world state onto the low-res scene image.
// ghostPos is non-nil during build-placement mode and draws a preview camp.
func DrawWorld(scene *ebiten.Image, w *World, ghostPos *Vec) {
	// --- background ---
	scene.Fill(colBackground)

	cx, cy := float32(w.Planet.Center.X), float32(w.Planet.Center.Y)
	r := float32(w.Planet.Radius)

	// --- planet disc (two circles for a chunky 2-tone look) ---
	vector.DrawFilledCircle(scene, cx, cy, r, colPlanet, false)
	// Offset highlight circle gives the impression of a light source upper-left.
	vector.DrawFilledCircle(scene, cx-r*0.15, cy-r*0.15, r*0.75, colPlanetHi, false)

	// --- forest (small cluster of solid blocks) ---
	fx, fy := w.Forest.Pos.X, w.Forest.Pos.Y
	for _, offset := range [][2]float32{{0, 0}, {4, -3}, {-4, -2}, {3, 3}, {-3, 4}} {
		vector.DrawFilledRect(scene,
			float32(fx)+offset[0]-2, float32(fy)+offset[1]-2, 5, 5,
			colForest, false)
	}

	// --- buildings ---
	for _, b := range w.Buildings {
		vector.DrawFilledRect(scene,
			float32(b.Pos.X)-3, float32(b.Pos.Y)-3, 7, 7,
			colBuilding, false)
	}

	// --- workers ---
	for _, b := range w.Buildings {
		for _, wk := range b.Workers {
			col := colWorkerEmpty
			if wk.Carried > 0 {
				col = colWorkerLaden
			}
			vector.DrawFilledRect(scene,
				float32(wk.Pos.X)-1, float32(wk.Pos.Y)-1, 3, 3,
				col, false)
		}
	}

	// --- ghost building during placement mode ---
	if ghostPos != nil {
		vector.DrawFilledRect(scene,
			float32(ghostPos.X)-3, float32(ghostPos.Y)-3, 7, 7,
			colGhostOk, false)
	}
}
