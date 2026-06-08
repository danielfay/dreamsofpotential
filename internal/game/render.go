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

// viewGeom returns the uniform scale and top-left offset that centres the
// virtual 320×240 canvas inside a screen of (screenW, screenH), preserving
// aspect ratio with letterbox/pillarbox bars as needed.
func viewGeom(screenW, screenH int) (scale, offX, offY float64) {
	sx := float64(screenW) / float64(virtW)
	sy := float64(screenH) / float64(virtH)
	scale = sx
	if sy < scale {
		scale = sy
	}
	offX = (float64(screenW) - float64(virtW)*scale) / 2
	offY = (float64(screenH) - float64(virtH)*scale) / 2
	return
}

// Building / worker render sizes.
const (
	campBldHalf      = float32(3.5) // half of 7×7 camp square
	campBldSize      = float32(7)
	townHallBldHalf  = float32(5.5) // half of 11×11 town hall square
	townHallBldSize  = float32(11)
	townHallBldInset = float32(5.5) // px inward from rim for town hall art center
	workerBldHalf    = float32(1)   // half of 3×3 worker square
	workerBldSize    = float32(3)
	idleMaxSlots     = 5 // max visible idle-worker spots near the town hall
)

// palette
var (
	colBackground    = color.RGBA{R: 10, G: 10, B: 20, A: 255}
	colPlanetBody    = color.RGBA{R: 5, G: 5, B: 10, A: 255}   // near-black interior
	colPlanetEdge    = color.RGBA{R: 50, G: 130, B: 50, A: 255} // green rim ring
	colNodeFree      = color.RGBA{R: 40, G: 160, B: 60, A: 255}
	colNodeClaimed   = color.RGBA{R: 20, G: 100, B: 35, A: 255}
	colTownHall      = color.RGBA{R: 190, G: 160, B: 110, A: 255} // warm stone
	colBuilding      = color.RGBA{R: 140, G: 90, B: 50, A: 255}
	colWorkerEmpty   = color.RGBA{R: 220, G: 200, B: 150, A: 255}
	colWorkerLaden   = color.RGBA{R: 255, G: 240, B: 80, A: 255}
	colGhostOk       = color.RGBA{R: 200, G: 200, B: 255, A: 160}
	colGhostBad      = color.RGBA{R: 200, G: 80, B: 80, A: 80}
	colRouteFree     = color.RGBA{R: 160, G: 220, B: 255, A: 200} // base; alpha/width scaled by quality
	colRouteClaimed  = color.RGBA{R: 100, G: 130, B: 150, A: 90}  // uniform muted
	colPreviewDebug  = color.RGBA{R: 255, G: 220, B: 80, A: 180}  // debug range markers
)

// kindFillColor returns a translucent interior fill colour for a resource kind.
func kindFillColor(k ResourceKind) color.RGBA {
	if k == KindWood {
		return color.RGBA{R: 30, G: 160, B: 60, A: 70}
	}
	return color.RGBA{R: 200, G: 200, B: 200, A: 70}
}

// DrawWorld renders the complete world state onto the low-res scene image.
// pv is non-nil during build-placement mode and drives the camp ghost and route
// line preview. debug enables the range-boundary markers.
func DrawWorld(scene *ebiten.Image, w *World, pv *placementPreview, debug bool) {
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
			vector.FillCircle(scene, cx, cy, fillR, col, false)
		} else {
			drawFilledSector(scene, cx, cy, fillR,
				f.CenterAngle-f.HalfArc, f.CenterAngle+f.HalfArc, col)
		}
	}

	// resource nodes — pine-tree shape; muted when in preview and claimed
	for _, n := range w.Nodes {
		col := colNodeFree
		if n.OwnerID != -1 {
			col = colNodeClaimed
		}
		if pv != nil {
			col = previewNodeColor(n, pv)
		}
		drawPineTree(scene, n, col)
	}

	// placement preview — route lines and ghost, drawn above nodes/below buildings
	if pv != nil {
		drawPreview(scene, w.Planet, pv, debug)
	}

	// buildings
	for _, b := range w.Buildings {
		if b.Kind == KindTownHall {
			ip := insetPoint(w.Planet, b.Angle, float64(townHallBldInset))
			vector.FillRect(scene,
				float32(ip.X)-townHallBldHalf, float32(ip.Y)-townHallBldHalf,
				townHallBldSize, townHallBldSize, colTownHall, false)
		} else {
			vector.FillRect(scene,
				float32(b.Pos.X)-campBldHalf, float32(b.Pos.Y)-campBldHalf,
				campBldSize, campBldSize, colBuilding, false)
		}
	}

	// workers — active ones at their sim position; idle ones at Town Hall cluster.
	th := townHall(w)
	idleCount := 0
	for _, wk := range w.Workers {
		if wk.NodeID == -1 {
			idleCount++
		}
	}
	slots := idleHomeSlots(w.Planet, th, idleCount)
	slotIdx := 0
	for _, wk := range w.Workers {
		col := colWorkerEmpty
		if wk.Carried > 0 {
			col = colWorkerLaden
		}
		if wk.NodeID == -1 && th != nil {
			if slotIdx < len(slots) {
				sp := slots[slotIdx]
				slotIdx++
				vector.FillRect(scene,
					float32(sp.X)-workerBldHalf, float32(sp.Y)-workerBldHalf,
					workerBldSize, workerBldSize, col, false)
			}
			// overflow workers omitted here; handled by drawIdleOverflow below
		} else {
			vector.FillRect(scene,
				float32(wk.Pos.X)-workerBldHalf, float32(wk.Pos.Y)-workerBldHalf,
				workerBldSize, workerBldSize, col, false)
		}
	}
	if th != nil && idleCount > idleMaxSlots {
		drawIdleOverflow(scene, w.Planet, th, idleCount-idleMaxSlots)
	}
}

// previewNodeColor returns the colour to draw node n while a placement preview
// is active. In-range free nodes are emphasised; in-range claimed nodes are
// muted; out-of-range nodes use normal colours.
func previewNodeColor(n *ResourceNode, pv *placementPreview) color.RGBA {
	inRange := math.Abs(normAngle(n.Angle-pv.Angle)) <= previewArc
	if !inRange {
		if n.OwnerID == -1 {
			return colNodeFree
		}
		return colNodeClaimed
	}
	if n.OwnerID == -1 {
		return color.RGBA{R: 80, G: 220, B: 100, A: 255} // brighter free
	}
	return color.RGBA{R: 15, G: 65, B: 25, A: 180} // deeper mute for claimed
}

// drawPreview draws route lines, the camp ghost, and (in debug mode) the range
// boundary for the given placement preview.
func drawPreview(scene *ebiten.Image, planet Planet, pv *placementPreview, debug bool) {
	radius := float32(planet.Radius)
	maxDist := float32(previewArc) * radius

	// Free-node route lines — quality-scaled brightness and width.
	for _, pr := range pv.Free {
		q := float32(1) - clamp32(float32(pr.Dist)/maxDist, 0, 1)
		a := uint8(80 + 120*q)
		col := color.RGBA{R: colRouteFree.R, G: colRouteFree.G, B: colRouteFree.B, A: a}
		w := 1.0 + 1.5*q
		drawRimArc(scene, planet, float32(pv.Angle), float32(pr.Node.Angle), w, col)
	}

	// Claimed-node route lines — uniform muted.
	for _, n := range pv.Claimed {
		drawRimArc(scene, planet, float32(pv.Angle), float32(n.Angle), 1.0, colRouteClaimed)
	}

	// Building ghost — validity-coloured; shape depends on kind.
	col := colGhostOk
	if !pv.Valid {
		col = colGhostBad
	}
	if pv.Kind == KindTownHall {
		ip := insetPoint(planet, pv.Angle, float64(townHallBldInset))
		vector.FillRect(scene,
			float32(ip.X)-townHallBldHalf, float32(ip.Y)-townHallBldHalf,
			townHallBldSize, townHallBldSize, col, false)
	} else {
		vector.FillRect(scene,
			float32(pv.Pos.X)-campBldHalf, float32(pv.Pos.Y)-campBldHalf,
			campBldSize, campBldSize, col, false)
	}

	// Debug: range boundary ticks at ±previewArc.
	if debug {
		cx, cy := float32(planet.Center.X), float32(planet.Center.Y)
		for _, side := range []float64{-previewArc, previewArc} {
			a := pv.Angle + side
			inner := float32(0.88)
			x0 := cx + radius*float32(math.Cos(a))
			y0 := cy + radius*float32(math.Sin(a))
			x1 := cx + radius*inner*float32(math.Cos(a))
			y1 := cy + radius*inner*float32(math.Sin(a))
			vector.StrokeLine(scene, x0, y0, x1, y1, 1.5, colPreviewDebug, false)
		}
	}
}

// drawRimArc strokes an arc from angle a to b along planet's rim with the
// given line width and colour, following the short way round.
func drawRimArc(scene *ebiten.Image, planet Planet, a, b, width float32, col color.RGBA) {
	const steps = 16
	delta := float32(normAngle(float64(b - a)))
	cx, cy := float32(planet.Center.X), float32(planet.Center.Y)
	r := float32(planet.Radius)

	var path vector.Path
	for i := 0; i <= steps; i++ {
		t := float32(i) / float32(steps)
		angle := a + delta*t
		x := cx + r*float32(math.Cos(float64(angle)))
		y := cy + r*float32(math.Sin(float64(angle)))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	sop := &vector.StrokeOptions{Width: width}
	drawOp := &vector.DrawPathOptions{}
	drawOp.ColorScale.ScaleWithColor(col)
	vector.StrokePath(scene, &path, sop, drawOp)
}

// clamp32 clamps a float32 to [lo, hi].
func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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

// insetPoint returns a world position stepped inward from the rim at angle by
// offset pixels toward the planet center.
func insetPoint(p Planet, angle, offset float64) Vec {
	rim := p.RimPoint(angle)
	return Vec{
		X: rim.X - math.Cos(angle)*offset,
		Y: rim.Y - math.Sin(angle)*offset,
	}
}

// idleHomeSlots returns up to idleMaxSlots distinct world positions for idle
// workers, arranged in a small 2-column grid inset inside the rim near th.
// Returns nil if th is nil or count ≤ 0. Count is capped at idleMaxSlots.
func idleHomeSlots(p Planet, th *Building, count int) []Vec {
	if th == nil || count <= 0 {
		return nil
	}
	if count > idleMaxSlots {
		count = idleMaxSlots
	}
	// Inward and tangent unit vectors at the Town Hall angle.
	cos := math.Cos(th.Angle)
	sin := math.Sin(th.Angle)
	inx, iny := -cos, -sin    // inward (toward planet center)
	tx, ty := -sin, cos       // tangent (counterclockwise along rim)
	// Anchor: 9 px inside the rim.
	rim := p.RimPoint(th.Angle)
	ax := rim.X + inx*9
	ay := rim.Y + iny*9
	// Grid: 2 columns × up to 3 rows, with a centred 5th slot.
	type off struct{ t, i float64 }
	slotOffsets := [idleMaxSlots]off{
		{-2.5, 0}, {+2.5, 0},   // row 0
		{-2.5, 4}, {+2.5, 4},   // row 1
		{0, 8},                  // row 2 (centred)
	}
	slots := make([]Vec, count)
	for i := 0; i < count; i++ {
		o := slotOffsets[i]
		slots[i] = Vec{
			X: ax + tx*o.t + inx*o.i,
			Y: ay + ty*o.t + iny*o.i,
		}
	}
	return slots
}

// drawIdleOverflow draws a compact dot on/inside the Town Hall for idle workers
// beyond the idleMaxSlots visible spots. Dot size and brightness scale subtly
// with overflowCount (bounded).
func drawIdleOverflow(scene *ebiten.Image, p Planet, th *Building, overflowCount int) {
	t := float32(overflowCount-1) / 19.0 // 0 at 1, 1 at 20+
	if t > 1 {
		t = 1
	}
	radius := float32(1.5) + 2.0*t
	bright := uint8(120 + uint8(100*t))
	col := color.RGBA{R: bright, G: bright, B: bright + 20, A: 200}
	ip := insetPoint(p, th.Angle, float64(townHallBldInset))
	vector.FillCircle(scene, float32(ip.X), float32(ip.Y), radius, col, false)
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
