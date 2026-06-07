package game

import (
	"image/color"
	"math"

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
const scaleX = 2
const scaleY = 2

// screenToWorld converts a native screen position to low-res world coordinates.
func screenToWorld(sx, sy int) Vec {
	return Vec{X: float64(sx) / scaleX, Y: float64(sy) / scaleY}
}

// palette
var (
	colBackground  = color.RGBA{R: 10, G: 10, B: 20, A: 255}
	colPlanet      = color.RGBA{R: 60, G: 100, B: 60, A: 255}
	colPlanetHi    = color.RGBA{R: 80, G: 130, B: 80, A: 255}
	colNodeFree    = color.RGBA{R: 30, G: 140, B: 50, A: 255}
	colNodeClaimed = color.RGBA{R: 20, G: 95, B: 35, A: 255}
	colBuilding    = color.RGBA{R: 140, G: 90, B: 50, A: 255}
	colWorkerEmpty = color.RGBA{R: 220, G: 200, B: 150, A: 255}
	colWorkerLaden = color.RGBA{R: 255, G: 240, B: 80, A: 255}
	colGhostOk     = color.RGBA{R: 200, G: 200, B: 255, A: 160}
)

// kindFillColor returns a translucent interior fill colour for a resource kind.
func kindFillColor(k ResourceKind) color.RGBA {
	if k == KindWood {
		return color.RGBA{R: 30, G: 160, B: 60, A: 70}
	}
	return color.RGBA{R: 200, G: 200, B: 200, A: 70}
}

// DrawWorld renders the complete world state onto the low-res scene image.
// ghostPos is non-nil during build-placement mode and draws a preview camp.
func DrawWorld(scene *ebiten.Image, w *World, ghostPos *Vec) {
	scene.Fill(colBackground)

	cx, cy := float32(w.Planet.Center.X), float32(w.Planet.Center.Y)
	r := float32(w.Planet.Radius)

	// planet disc
	vector.FillCircle(scene, cx, cy, r, colPlanet, false)
	vector.FillCircle(scene, cx-r*0.15, cy-r*0.15, r*0.75, colPlanetHi, false)

	// resource field interior fill — core→edge wedge showing progress to next node
	for _, f := range w.Planet.Fields {
		if f.Counter <= 0 {
			continue
		}
		fillR := r * float32(f.Counter/f.Cap)
		drawFilledSector(scene, cx, cy, fillR,
			f.CenterAngle-f.HalfArc, f.CenterAngle+f.HalfArc,
			kindFillColor(f.Kind))
	}

	// resource nodes
	for _, n := range w.Nodes {
		col := colNodeFree
		if n.OwnerID != -1 {
			col = colNodeClaimed
		}
		vector.FillRect(scene,
			float32(n.Pos.X)-2, float32(n.Pos.Y)-2, 5, 5, col, false)
	}

	// buildings
	for _, b := range w.Buildings {
		vector.FillRect(scene,
			float32(b.Pos.X)-3, float32(b.Pos.Y)-3, 7, 7, colBuilding, false)
	}

	// workers (global pool)
	for _, wk := range w.Workers {
		col := colWorkerEmpty
		if wk.Carried > 0 {
			col = colWorkerLaden
		}
		vector.FillRect(scene,
			float32(wk.Pos.X)-1, float32(wk.Pos.Y)-1, 3, 3, col, false)
	}

	// ghost building during placement mode
	if ghostPos != nil {
		vector.FillRect(scene,
			float32(ghostPos.X)-3, float32(ghostPos.Y)-3, 7, 7, colGhostOk, false)
	}
}

// drawFilledSector draws a filled wedge from (cx,cy) spanning startAngle..endAngle
// out to radius fillR, in the given colour.
func drawFilledSector(scene *ebiten.Image, cx, cy, fillR float32, startAngle, endAngle float64, col color.RGBA) {
	if fillR <= 0 {
		return
	}
	const steps = 16
	var path vector.Path
	path.MoveTo(cx, cy)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		angle := startAngle + (endAngle-startAngle)*t
		path.LineTo(cx+fillR*float32(math.Cos(angle)), cy+fillR*float32(math.Sin(angle)))
	}
	path.Close()
	drawOp := &vector.DrawPathOptions{}
	drawOp.ColorScale.ScaleWithColor(col)
	vector.FillPath(scene, &path, nil, drawOp)
}
