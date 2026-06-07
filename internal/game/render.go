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
	colPlanetBody  = color.RGBA{R: 5, G: 5, B: 10, A: 255}   // near-black interior
	colPlanetEdge  = color.RGBA{R: 50, G: 130, B: 50, A: 255} // green rim ring
	colNodeFree    = color.RGBA{R: 40, G: 160, B: 60, A: 255}
	colNodeClaimed = color.RGBA{R: 20, G: 100, B: 35, A: 255}
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

	// planet: green outer ring then black body on top
	const rimWidth = float32(4)
	vector.FillCircle(scene, cx, cy, r, colPlanetEdge, false)
	vector.FillCircle(scene, cx, cy, r-rimWidth, colPlanetBody, false)

	// resource field interior fill — core→edge showing progress to next node
	for _, f := range w.Planet.Fields {
		if f.Counter <= 0 {
			continue
		}
		fillR := (r - rimWidth) * float32(f.Counter/f.Cap)
		col := kindFillColor(f.Kind)
		if math.Abs(f.HalfArc-math.Pi) < 1e-9 {
			// Full surface: a simple filled circle is correct and fast.
			vector.FillCircle(scene, cx, cy, fillR, col, false)
		} else {
			drawFilledSector(scene, cx, cy, fillR,
				f.CenterAngle-f.HalfArc, f.CenterAngle+f.HalfArc, col)
		}
	}

	// resource nodes — pine-tree shape oriented inward along the rim normal
	for _, n := range w.Nodes {
		col := colNodeFree
		if n.OwnerID != -1 {
			col = colNodeClaimed
		}
		drawPineTree(scene, n, col)
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

// drawPineTree draws a 3-layer pine tree at n.Pos oriented inward along the
// planet surface normal. Layer widths and spacing scale with n.Size.
func drawPineTree(scene *ebiten.Image, n *ResourceNode, col color.RGBA) {
	s := float32(n.Size)

	// Outward normal (away from planet center) and tangent (along rim).
	ix := float32(math.Cos(n.Angle))
	iy := float32(math.Sin(n.Angle))
	tx := float32(-math.Sin(n.Angle))
	ty := float32(math.Cos(n.Angle))

	cx, cy := float32(n.Pos.X), float32(n.Pos.Y)

	// Layer definitions: (half-width, inward offset of layer center).
	layers := [3][2]float32{
		{4 * s, 1.5},  // bottom — widest, at the rim
		{2.5 * s, 5},  // middle
		{1.5 * s, 8.5}, // top — narrowest, farthest inward
	}
	const halfH = float32(1.5) // half-height of each layer (3px tall)

	for _, l := range layers {
		hw, offset := l[0], l[1]
		// Center of this layer.
		lx := cx + ix*offset
		ly := cy + iy*offset
		// Four corners of the oriented rectangle.
		drawOrientedRect(scene, lx, ly, tx, ty, ix, iy, hw, halfH, col)
	}
}

// drawOrientedRect fills an axis-oriented-in-world-space rectangle defined by
// its center (lx,ly), tangent direction (tx,ty), inward direction (ix,iy),
// half-width hw along the tangent, and half-height hh along inward.
func drawOrientedRect(scene *ebiten.Image, lx, ly, tx, ty, ix, iy, hw, hh float32, col color.RGBA) {
	var path vector.Path
	path.MoveTo(lx+tx*hw+ix*hh, ly+ty*hw+iy*hh)
	path.LineTo(lx-tx*hw+ix*hh, ly-ty*hw+iy*hh)
	path.LineTo(lx-tx*hw-ix*hh, ly-ty*hw-iy*hh)
	path.LineTo(lx+tx*hw-ix*hh, ly+ty*hw-iy*hh)
	path.Close()
	drawOp := &vector.DrawPathOptions{}
	drawOp.ColorScale.ScaleWithColor(col)
	vector.FillPath(scene, &path, nil, drawOp)
}

// drawFilledSector draws a filled wedge from (cx,cy) spanning startAngle..endAngle
// out to radius fillR, in the given colour.
func drawFilledSector(scene *ebiten.Image, cx, cy, fillR float32, startAngle, endAngle float64, col color.RGBA) {
	if fillR <= 0 {
		return
	}
	const steps = 32
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
