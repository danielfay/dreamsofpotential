package game

import (
	"image/color"
	"math"

	"github.com/tanema/gween/ease"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// previewNodeColor returns the colour to draw node n while a placement preview
// is active. In-range free nodes are emphasised; unavailable nodes keep their
// normal world colours so only muted route lines imply local competition.
func previewNodeColor(n *ResourceNode, pv *placementPreview) color.RGBA {
	inRange := math.Abs(normAngle(n.Angle-pv.Angle)) <= previewArc
	if inRange && n.OwnerID == -1 && n.ReservedByWorkerID == -1 {
		return color.RGBA{R: 80, G: 220, B: 100, A: 255} // brighter free
	}
	if n.ReservedByWorkerID != -1 && n.OwnerID == -1 {
		return colNodeReserved
	}
	if n.OwnerID != -1 {
		return colNodeClaimed
	}
	return colNodeFree
}

// drawPreview draws route lines, the camp ghost, and (in debug mode) the range
// boundary for the given placement preview.
func drawPreview(scene *ebiten.Image, planet Planet, pv *placementPreview, debug bool) {
	radius := float32(planet.Radius)
	maxDist := float32(previewArc) * radius
	routeRadius := radius + 6
	lensRadius := radius + 3

	if pv.Valid && pv.Kind == KindLoggingCamp {
		// Free-node route lines — quality-scaled brightness and width.
		for _, pr := range pv.Free {
			q := float32(1) - clamp32(float32(pr.Dist)/maxDist, 0, 1)
			a := uint8(80 + 120*q)
			col := color.RGBA{R: colRouteFree.R, G: colRouteFree.G, B: colRouteFree.B, A: a}
			w := 1.0 + 1.5*q
			drawArcAtRadius(scene, planet, routeRadius, float32(pv.Angle), float32(pr.Node.Angle), w, col)
		}

		// Claimed/reserved route lines — uniform muted.
		for _, n := range pv.Claimed {
			drawArcAtRadius(scene, planet, routeRadius, float32(pv.Angle), float32(n.Angle), 1.0, colRouteClaimed)
		}
		for _, n := range pv.Reserved {
			drawArcAtRadius(scene, planet, routeRadius, float32(pv.Angle), float32(n.Angle), 1.0, colRouteClaimed)
		}
	}

	// Building ghost — validity-coloured; shape depends on kind.
	col := colGhostOk
	if !pv.Valid {
		col = colGhostBad
	}
	if pv.Reject > 0 {
		boost := uint8(80 * pv.Reject)
		col = brighten(col, boost)
		col.A = uint8(100 + 80*pv.Reject)
	}
	footprintCol := color.RGBA{R: col.R, G: col.G, B: col.B, A: 70}
	footprintHalf := buildingHardHalfArc(pv.Kind, planet.Radius)
	footprintWidth := float32(3.0)
	if pv.Reject > 0 {
		footprintWidth += float32(2 * pv.Reject)
	}
	drawRimArc(scene, planet, float32(pv.Angle-footprintHalf), float32(pv.Angle+footprintHalf), footprintWidth, footprintCol)
	switch pv.Kind {
	case KindTownHall:
		drawTownHallArt(scene, planet, pv.Angle, col)
	case KindDock:
		// Dive-reach wedge: L1 annular sector anchored at the rim, reaching 1/3 inward.
		cx, cy := float32(planet.Center.X), float32(planet.Center.Y)
		drawDockReachSector(scene, cx, cy, radius*2/3, radius,
			pv.Angle-dockWedgeHalfArc, pv.Angle+dockWedgeHalfArc)
		drawDockArt(scene, planet, pv.Angle, col, 1) // preview always shows L1 visual
	default: // KindLoggingCamp
		vector.FillRect(scene,
			float32(pv.Pos.X)-campBldHalf, float32(pv.Pos.Y)-campBldHalf,
			campBldSize, campBldSize, col, false)
	}

	// Debug: range boundary ticks at ±previewArc.
	if debug {
		lensCol := colPreviewLens
		if !pv.Valid {
			lensCol.A = 10
		}
		drawArcAtRadius(scene, planet, lensRadius, float32(pv.Angle-previewArc), float32(pv.Angle+previewArc), 1.0, lensCol)

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
		for _, side := range []float64{-footprintHalf, footprintHalf} {
			a := pv.Angle + side
			x0 := cx + radius*float32(math.Cos(a))
			y0 := cy + radius*float32(math.Sin(a))
			x1 := cx + radius*0.93*float32(math.Cos(a))
			y1 := cy + radius*0.93*float32(math.Sin(a))
			vector.StrokeLine(scene, x0, y0, x1, y1, 1.0, col, false)
		}
		for _, n := range pv.Blocked {
			vector.FillCircle(scene, float32(n.Pos.X), float32(n.Pos.Y), 2.0, colGhostBad, false)
		}
	}
}

func growthNodeVisualScale(w *World, n *ResourceNode) float32 {
	if w.growthCue.NodeID != n.ID || w.growthCue.NodeDelay > 0 || w.growthCue.NodeCue <= 0 {
		return 1
	}
	progress := float32(1 - w.growthCue.NodeCue/growthNodeCueTime)
	progress = clamp32(progress, 0, 1)
	switch w.growthCue.Outcome {
	case growthOutcomeSpawnedNode:
		return 0.35 + 0.65*smoothStep(progress)
	case growthOutcomeUpgradedNode:
		return 1 + 0.16*float32(math.Sin(float64(progress)*math.Pi))
	default:
		return 1
	}
}

func growthNodeVisualAlpha(w *World, n *ResourceNode) uint8 {
	if w.growthCue.NodeID != n.ID || w.growthCue.NodeDelay > 0 || w.growthCue.NodeCue <= 0 {
		return 0
	}
	t := w.growthCue.NodeCue / growthNodeCueTime
	return uint8(55 * clamp32(float32(t), 0, 1))
}

func smoothStep(t float32) float32 {
	return ease.InOutSine(t, 0, 1, 1)
}

// drawPineTree draws a 3-layer pine tree at n.Pos oriented inward along the
// planet surface normal. Layer widths and spacing scale with n.Size.
func drawPineTree(scene *ebiten.Image, n *ResourceNode, col color.RGBA, visualScale float32, alphaBoost uint8) {
	s := float32(n.Size) * visualScale
	if alphaBoost > 0 {
		col = brighten(col, alphaBoost)
	}

	// Outward normal (away from planet center) and tangent (along rim).
	ix := float32(math.Cos(n.Angle))
	iy := float32(math.Sin(n.Angle))
	tx := float32(-math.Sin(n.Angle))
	ty := float32(math.Cos(n.Angle))

	cx, cy := float32(n.Pos.X), float32(n.Pos.Y)

	// Layer definitions: (half-width, inward offset of layer center).
	layers := [3][2]float32{
		{4 * s, 1.5},   // bottom — widest, at the rim
		{2.5 * s, 5},   // middle
		{1.5 * s, 8.5}, // top — narrowest, farthest inward
	}
	const halfH = float32(1.5) // half-height of each layer (3px tall)

	for _, l := range layers {
		hw, offset := l[0], l[1]
		lx := cx + ix*offset
		ly := cy + iy*offset
		drawOrientedRect(scene, lx, ly, tx, ty, ix, iy, hw, halfH, col)
	}
}

// drawSparkle draws an interior water sparkle as an animated + shape at n.Pos.
// simTime drives a gentle per-node size pulse; alphaBoost brightens during growth cues.
func drawSparkle(scene *ebiten.Image, n *ResourceNode, col color.RGBA, visualScale float32, alphaBoost uint8, simTime float64) {
	if alphaBoost > 0 {
		col = brighten(col, alphaBoost)
	}
	// Per-node phase offset so sparkles pulse independently.
	phase := float64(n.Pos.X)*0.17 + float64(n.Pos.Y)*0.13
	pulse := float32(1.0 + sparkleAnimAmp*math.Sin(simTime*sparkleAnimFreq+phase))
	r := float32(n.Size) * sparkleBaseDrawRadius * visualScale * pulse
	hw := r * sparkleArmWidthRatio
	cx, cy := float32(n.Pos.X), float32(n.Pos.Y)
	// Horizontal arm.
	vector.FillRect(scene, cx-r, cy-hw, 2*r, 2*hw, col, false)
	// Vertical arm.
	vector.FillRect(scene, cx-hw, cy-r, 2*hw, 2*r, col, false)
}
