package game

import (
	"image/color"
	"math"

	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/mazznoer/colorgrad"
	"github.com/tanema/gween/ease"

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
	townHallBldHalfW = float32(8)   // half-width along rim tangent (16px wide — fort shape)
	townHallBldHalfH = float32(4.5) // half-height along inward normal (9px tall)
	townHallBldInset = float32(5)   // px inward from rim for town hall art center
	workerBldHalf    = float32(1)   // half of 3×3 worker square
	workerBldSize    = float32(3)
	idleMaxSlots     = 5 // max visible idle-worker spots near the town hall
)

// palette
var (
	colBackground    = color.RGBA{R: 10, G: 10, B: 20, A: 255}
	colPlanetBody    = color.RGBA{R: 5, G: 5, B: 10, A: 255}    // near-black interior
	colPlanetEdge    = color.RGBA{R: 50, G: 130, B: 50, A: 255} // green rim ring
	colNodeFree      = color.RGBA{R: 40, G: 160, B: 60, A: 255}
	colNodeReserved  = color.RGBA{R: 32, G: 130, B: 48, A: 255}
	colNodeClaimed   = color.RGBA{R: 20, G: 100, B: 35, A: 255}
	colTownFieldBase = color.RGBA{R: 120, G: 82, B: 40, A: 150}  // warm clay wedge fill
	colTownFieldEdge = color.RGBA{R: 165, G: 115, B: 58, A: 120} // amber edge bands
	colTownFieldSlot         = color.RGBA{R: 200, G: 148, B: 72, A: 220} // available dwelling slot
	colTownFieldSlotOccupied = color.RGBA{R: 150, G: 80, B: 8, A: 255}  // occupied — darker richer amber, clearly distinct
	colGhostOk       = color.RGBA{R: 200, G: 200, B: 255, A: 160}
	colGhostBad      = color.RGBA{R: 200, G: 80, B: 80, A: 80}
	colRouteFree     = color.RGBA{R: 160, G: 220, B: 255, A: 200} // base; alpha/width scaled by quality
	colRouteClaimed  = color.RGBA{R: 100, G: 130, B: 150, A: 90}  // uniform muted
	colPreviewLens   = color.RGBA{R: 125, G: 145, B: 170, A: 16}
	colPreviewDebug  = color.RGBA{R: 255, G: 220, B: 80, A: 180} // debug range markers
)

// planetViewPalette holds the planet-view colors that vary by planet identity.
type planetViewPalette struct {
	background color.RGBA
	edge       color.RGBA
	body       color.RGBA
}

// activePlanetPalette returns a subtle palette variation for the active planet
// so echo planets feel distinct while keeping the same visual language.
func activePlanetPalette(w *World) planetViewPalette {
	if !w.System.Unlocked || w.Active == 0 {
		// Starting planet or pre-unlock: original green-on-black palette.
		return planetViewPalette{
			background: colBackground,
			edge:       colPlanetEdge,
			body:       colPlanetBody,
		}
	}
	switch w.System.Planets[w.Active].LayoutID {
	case 1: // echo layout 1 — warm yellow-green (forest)
		return planetViewPalette{
			background: color.RGBA{R: 10, G: 14, B: 6, A: 255},
			edge:       color.RGBA{R: 85, G: 145, B: 35, A: 255},
			body:       color.RGBA{R: 5, G: 8, B: 3, A: 255},
		}
	default: // echo layout 0 — cool blue-green (forest)
		return planetViewPalette{
			background: color.RGBA{R: 6, G: 14, B: 10, A: 255},
			edge:       color.RGBA{R: 35, G: 130, B: 75, A: 255},
			body:       color.RGBA{R: 3, G: 8, B: 5, A: 255},
		}
	}
}

// atmosphereGrads holds per-planet-type alpha-falloff gradients for the
// completion glow. Built once at init; keyed by atmosphere colour variant.
var (
	gradAtmosphereStart colorgrad.Gradient
	gradAtmosphereA     colorgrad.Gradient
	gradAtmosphereB     colorgrad.Gradient
)

func init() {
	gradAtmosphereStart = buildAtmosphereGrad(colAtmosphereStart)
	gradAtmosphereA = buildAtmosphereGrad(colAtmosphereA)
	gradAtmosphereB = buildAtmosphereGrad(colAtmosphereB)
}

// buildAtmosphereGrad returns a gradient from base@alpha20 (inner rim) to
// base@alpha0 (outer edge) for sampling ring opacities in drawPlanetAtmosphere.
func buildAtmosphereGrad(base color.RGBA) colorgrad.Gradient {
	g, err := colorgrad.NewGradient().
		Colors(
			colorgrad.Rgb8(base.R, base.G, base.B, 20),
			colorgrad.Rgb8(base.R, base.G, base.B, 0),
		).
		Build()
	if err != nil {
		panic(err)
	}
	return g
}

// drawPlanetAtmosphere draws a wide coloured glow behind the planet when it is
// complete. Multiple concentric transparent circles accumulate into a soft
// gradient that fills much of the screen. The glow expands from the rim during
// an intro animation and then breathes gently in steady state.
func drawPlanetAtmosphere(scene *ebiten.Image, w *World, cx, cy, r float32) {
	p := w.System.Planets[w.Active]

	var completedAt float64
	switch {
	case p.Kind == PlanetEcho && p.Completed:
		completedAt = p.CompletedAt
	case p.Kind == PlanetStarting && w.System.Unlocked:
		completedAt = p.CompletedAt
	default:
		return
	}

	// Choose atmosphere colour and gradient by planet type / layout.
	var (
		base      color.RGBA
		atmosGrad colorgrad.Gradient
	)
	switch {
	case p.Kind == PlanetEcho && p.LayoutID == 1:
		base = colAtmosphereB
		atmosGrad = gradAtmosphereB
	case p.Kind == PlanetEcho:
		base = colAtmosphereA
		atmosGrad = gradAtmosphereA
	default:
		base = colAtmosphereStart
		atmosGrad = gradAtmosphereStart
	}

	// Intro progress: 0→1 over atmosphereIntroDur seconds, quadratic ease-out.
	animAge := w.SimTime - completedAt
	rawProg := animAge / atmosphereIntroDur
	if rawProg > 1 {
		rawProg = 1
	}
	progress := ease.OutQuad(float32(rawProg), 0, 1, 1)

	// Gentle breathing in steady state.
	breath := float32(0.8 + 0.2*math.Sin(w.SimTime*atmosphereBreathFreq))

	// Ten concentric layers drawn outermost-first. FillCircle uses
	// ColorScaleModePremultipliedAlpha internally, so the RGB components must
	// be premultiplied by alpha — otherwise even a small alpha renders at full
	// colour. Premultiplying keeps each layer faint; they accumulate to ~40%
	// of the base colour at the rim and fade to near-zero at the screen edge.
	//
	// Alpha at each ring is sampled from a smooth gradient (inner=opaque →
	// outer=transparent), giving a cleaner falloff than a hard-coded table.
	const innerOff, outerOff = float32(3), float32(115)
	offsets := [10]float32{115, 95, 77, 61, 47, 35, 25, 16, 9, 3}
	for _, offset := range offsets {
		tNorm := float64((offset - innerOff) / (outerOff - innerOff))
		_, _, _, a32 := atmosGrad.At(tNorm).RGBA()
		rawA := float32(uint8(a32 >> 8))
		a := rawA * progress * breath
		if a <= 0 {
			continue
		}
		// Premultiply so FillCircle's premultiplied-alpha path works correctly.
		vector.FillCircle(scene, cx, cy, r+offset*progress, color.RGBA{
			R: uint8(float32(base.R) * a / 255),
			G: uint8(float32(base.G) * a / 255),
			B: uint8(float32(base.B) * a / 255),
			A: uint8(a),
		}, false)
	}
}

// DrawWorld renders the complete world state onto the low-res scene image.
// pv is non-nil during build-placement mode and drives the camp ghost and route
// line preview. debug enables the range-boundary markers.
func DrawWorld(scene *ebiten.Image, w *World, pv *placementPreview, debug bool) {
	pal := activePlanetPalette(w)
	scene.Fill(pal.background)

	cx, cy := float32(w.Planet.Center.X), float32(w.Planet.Center.Y)
	r := float32(w.Planet.Radius)

	// Completion atmosphere drawn first — planet body covers the interior.
	drawPlanetAtmosphere(scene, w, cx, cy, r)

	// planet: rim ring then dark body on top
	const rimWidth = float32(4)
	vector.FillCircle(scene, cx, cy, r, pal.edge, false)
	vector.FillCircle(scene, cx, cy, r-rimWidth, pal.body, false)

	// Resource field interior fill: stable composition, not node-spawn progress.
	for _, f := range w.Planet.Fields {
		drawResourceFieldFill(scene, w.Planet, f, r-rimWidth)
		if len(w.Buildings) == 0 {
			drawPreFoundingPulse(scene, w.Planet, f, r, w.SimTime)
		}
		drawResourceFieldPulse(scene, w, f, r-rimWidth, pv != nil)
	}
	// Town field: settlement wedge anchored to the Town Hall, drawn over the forest
	// field but under nodes/buildings/workers so it transforms the planet interior.
	drawTownField(scene, w, r-rimWidth)

	// resource nodes — pine-tree shape; muted when in preview and unavailable
	for _, n := range w.Nodes {
		col := colNodeFree
		if n.OwnerID != -1 {
			col = colNodeClaimed
		} else if n.ReservedByWorkerID != -1 {
			col = colNodeReserved
		}
		if pv != nil {
			col = previewNodeColor(n, pv)
		}
		if pulseActive(w, n.Pulse) {
			col = brighten(col, 45)
		}
		drawPineTree(scene, n, col, growthNodeVisualScale(w, n), growthNodeVisualAlpha(w, n))
	}

	// placement preview — route lines and ghost, drawn above nodes/below buildings
	if pv != nil {
		drawPreview(scene, w.Planet, pv, debug)
	}

	// buildings
	for _, b := range w.Buildings {
		col := colBuilding
		if b.Kind == KindTownHall {
			col = colTownHall
			if pulseActive(w, b.Pulse) {
				col = brighten(col, 40)
			}
			drawTownHallArt(scene, w.Planet, b.Angle, col)
			drawTownGrowthGauge(scene, w.Planet, b, w.Economy.TownGrowth, w.Economy.TownGrowthCap)
		} else {
			if pulseActive(w, b.Pulse) {
				col = brighten(col, 40)
			}
			vector.FillRect(scene,
				float32(b.Pos.X)-campBldHalf, float32(b.Pos.Y)-campBldHalf,
				campBldSize, campBldSize, col, false)
		}
	}

	// workers — active ones at their sim position; idle ones at Town Hall cluster.
	th := townHall(w)
	idleCount := 0
	for _, wk := range w.Workers {
		if workerUsesIdleHome(wk) {
			idleCount++
		}
	}
	slots := idleHomeSlots(w.Planet, th, idleCount)
	slotIdx := 0
	for _, wk := range w.Workers {
		col := workerColor(w, wk)
		if workerUsesIdleHome(wk) && th != nil {
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

	if pv.Valid {
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
	if pv.Kind == KindTownHall {
		drawTownHallArt(scene, planet, pv.Angle, col)
	} else {
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

func workerUsesIdleHome(wk *Worker) bool {
	switch wk.State {
	case StateIdleWaiting, StateSettling, StateReactionDelay:
		return true
	default:
		return false
	}
}

func workerColor(w *World, wk *Worker) color.RGBA {
	col := colWorkerEmpty
	if wk.State == StateReturningHome || wk.State == StateToIdleSpot {
		col = colWorkerReturn
	}
	if wk.Carried > 0 || wk.State == StateToBuilding || wk.State == StateUnloading {
		col = colWorkerLaden
	}
	if pulseActive(w, wk.Pulse) {
		col = brighten(col, 35)
	}
	return col
}

func brighten(col color.RGBA, amount uint8) color.RGBA {
	if amount == 0 {
		return col
	}
	c, _ := colorful.MakeColor(color.RGBA{R: col.R, G: col.G, B: col.B, A: 255})
	result := c.BlendLab(colorful.Color{R: 1, G: 1, B: 1}, float64(amount)/255.0).Clamped()
	r8, g8, b8 := result.RGB255()
	return color.RGBA{R: r8, G: g8, B: b8, A: col.A}
}

// drawRimArc strokes an arc from angle a to b along planet's rim with the
// given line width and colour, following the short way round.
func drawRimArc(scene *ebiten.Image, planet Planet, a, b, width float32, col color.RGBA) {
	drawArcAtRadius(scene, planet, float32(planet.Radius), a, b, width, col)
}

func drawArcAtRadius(scene *ebiten.Image, planet Planet, radius, a, b, width float32, col color.RGBA) {
	const steps = 16
	delta := float32(normAngle(float64(b - a)))
	cx, cy := float32(planet.Center.X), float32(planet.Center.Y)

	var path vector.Path
	for i := 0; i <= steps; i++ {
		t := float32(i) / float32(steps)
		angle := a + delta*t
		x := cx + radius*float32(math.Cos(float64(angle)))
		y := cy + radius*float32(math.Sin(float64(angle)))
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

// drawTownHallArt draws the Town Hall as a wide fort-shaped rectangle oriented
// along the rim (wide in the tangential direction, short inward).
func drawTownHallArt(scene *ebiten.Image, p Planet, angle float64, col color.RGBA) {
	ip := insetPoint(p, angle, float64(townHallBldInset))
	// Inward normal (toward planet center) and tangent (along rim, ccw).
	ix := float32(-math.Cos(angle))
	iy := float32(-math.Sin(angle))
	tx := float32(-math.Sin(angle))
	ty := float32(math.Cos(angle))
	drawOrientedRect(scene, float32(ip.X), float32(ip.Y), tx, ty, ix, iy,
		townHallBldHalfW, townHallBldHalfH, col)
}

// drawTownGrowthGauge draws a small progress bar below the Town Hall art,
// aligned with the rim tangent, showing Town Growth / TownGrowthCap.
func drawTownGrowthGauge(scene *ebiten.Image, p Planet, th *Building, growth, cap float64) {
	if cap <= 0 {
		return
	}
	frac := float32(growth / cap)
	if frac > 1 {
		frac = 1
	}
	const gaugeInset = float32(townHallBldInset) + float32(townHallBldHalfH) + 3
	anchor := insetPoint(p, th.Angle, float64(gaugeInset))
	ax, ay := float32(anchor.X), float32(anchor.Y)
	ix := float32(-math.Cos(th.Angle))
	iy := float32(-math.Sin(th.Angle))
	tx := float32(-math.Sin(th.Angle))
	ty := float32(math.Cos(th.Angle))
	const halfW = float32(townHallBldHalfW) - 2
	const halfH = float32(1)
	// Frame
	drawOrientedRect(scene, ax, ay, tx, ty, ix, iy, halfW, halfH, colTownGrowthGaugeFrame)
	// Fill (from one end along the tangent)
	if frac > 0 {
		fillHW := halfW * frac
		fcx := ax - tx*(halfW-fillHW)
		fcy := ay - ty*(halfW-fillHW)
		drawOrientedRect(scene, fcx, fcy, tx, ty, ix, iy, fillHW, halfH, colTownGrowthGaugeFill)
	}
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
	inx, iny := -cos, -sin // inward (toward planet center)
	tx, ty := -sin, cos    // tangent (counterclockwise along rim)
	// Anchor: 9 px inside the rim.
	rim := p.RimPoint(th.Angle)
	ax := rim.X + inx*9
	ay := rim.Y + iny*9
	// Grid: 2 columns × up to 3 rows, with a centred 5th slot.
	type off struct{ t, i float64 }
	slotOffsets := [idleMaxSlots]off{
		{-2.5, 0}, {+2.5, 0}, // row 0
		{-2.5, 4}, {+2.5, 4}, // row 1
		{0, 8}, // row 2 (centred)
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

// drawResourceFieldFill paints a full resource field as terrain composition.
// Node-spawn progress is shown in the HUD, so this stays visually stable.
func drawResourceFieldFill(scene *ebiten.Image, planet Planet, f *ResourceField, radius float32) {
	cx, cy := float32(planet.Center.X), float32(planet.Center.Y)
	start := f.CenterAngle - f.HalfArc
	end := f.CenterAngle + f.HalfArc

	if f.Kind == KindWood {
		drawForestFieldFill(scene, cx, cy, radius, start, end)
		return
	}
	if f.Kind == KindWater {
		drawWaterFieldFill(scene, cx, cy, radius, start, end)
		return
	}
	drawFieldSector(scene, cx, cy, radius, start, end, color.RGBA{R: 200, G: 200, B: 200, A: 54})
}

func drawResourceFieldPulse(scene *ebiten.Image, w *World, f *ResourceField, radius float32, placementActive bool) {
	if w.growthCue.FieldDelay > 0 || w.growthCue.FieldPulse <= 0 || w.growthCue.Kind != f.Kind {
		return
	}
	if math.Abs(normAngle(w.growthCue.CenterAngle-f.CenterAngle)) > 1e-9 ||
		math.Abs(w.growthCue.HalfArc-f.HalfArc) > 1e-9 {
		return
	}
	remaining := float32(w.growthCue.FieldPulse / growthFieldPulseTime)
	progress := 1 - clamp32(remaining, 0, 1)
	intensity := float32(math.Sin(float64(progress) * math.Pi))
	if placementActive {
		intensity *= 0.45
	}
	cx, cy := float32(w.Planet.Center.X), float32(w.Planet.Center.Y)
	start := f.CenterAngle - f.HalfArc
	end := f.CenterAngle + f.HalfArc
	ringAlpha := uint8(30 * intensity)
	if ringAlpha == 0 {
		return
	}

	outer := radius * (0.18 + 0.80*progress)
	inner := outer - radius*0.16
	if inner > 0 {
		drawFieldSectorBand(scene, cx, cy, inner, 1, start, end, color.RGBA{R: 95, G: 210, B: 108, A: uint8(float32(ringAlpha) * 0.55)})
	}
	drawFieldSectorBand(scene, cx, cy, outer, 2, start, end, color.RGBA{R: 118, G: 235, B: 124, A: ringAlpha})
}

// drawPreFoundingPulse draws a slow breathing ring on the field rim to signal
// wood potential before the Town Hall is placed. Fades in/out on a ~3 s cycle.
func drawPreFoundingPulse(scene *ebiten.Image, planet Planet, f *ResourceField, radius float32, simTime float64) {
	intensity := float32(0.4 + 0.6*math.Cos(simTime*1.5))
	alpha := uint8(38 * intensity)
	if alpha == 0 {
		return
	}
	cx, cy := float32(planet.Center.X), float32(planet.Center.Y)
	start := f.CenterAngle - f.HalfArc
	end := f.CenterAngle + f.HalfArc
	drawFieldSectorBand(scene, cx, cy, radius-1, 3, start, end, color.RGBA{R: 95, G: 210, B: 108, A: alpha})
}

// drawForestFieldFill layers low-alpha greens so the forest reads as a filled
// biome with subtle canopy texture instead of a flat progress disk.
func drawForestFieldFill(scene *ebiten.Image, cx, cy, radius float32, startAngle, endAngle float64) {
	drawFieldSector(scene, cx, cy, radius, startAngle, endAngle, color.RGBA{R: 8, G: 52, B: 28, A: 150})

	for _, ring := range []struct {
		r   float32
		col color.RGBA
	}{
		{radius * 0.94, color.RGBA{R: 44, G: 118, B: 56, A: 36}},
		{radius * 0.76, color.RGBA{R: 4, G: 34, B: 22, A: 36}},
		{radius * 0.51, color.RGBA{R: 42, G: 108, B: 52, A: 24}},
		{radius * 0.29, color.RGBA{R: 5, G: 38, B: 24, A: 24}},
	} {
		drawFieldSectorBand(scene, cx, cy, ring.r, 2, startAngle, endAngle, ring.col)
	}
	drawForestCanopyFlecks(scene, cx, cy, radius, startAngle, endAngle)
}

// drawWaterFieldFill layers low-alpha blues so the water reads as a filled
// biome with subtle ripple texture, mirroring drawForestFieldFill in blue.
func drawWaterFieldFill(scene *ebiten.Image, cx, cy, radius float32, startAngle, endAngle float64) {
	drawFieldSector(scene, cx, cy, radius, startAngle, endAngle, color.RGBA{R: 10, G: 40, B: 112, A: 190})

	for _, ring := range []struct {
		r   float32
		col color.RGBA
	}{
		{radius * 0.93, color.RGBA{R: 18, G: 72, B: 158, A: 38}},
		{radius * 0.75, color.RGBA{R: 6, G: 28, B: 88, A: 36}},
		{radius * 0.52, color.RGBA{R: 24, G: 90, B: 178, A: 26}},
		{radius * 0.30, color.RGBA{R: 8, G: 38, B: 108, A: 26}},
	} {
		drawFieldSectorBand(scene, cx, cy, ring.r, 2, startAngle, endAngle, ring.col)
	}
	drawWaterRippleFlecks(scene, cx, cy, radius, startAngle, endAngle)
}

// drawWaterRippleFlecks adds deterministic shimmer dots inside the water field,
// mirroring the forest canopy fleck approach but in blue tones.
func drawWaterRippleFlecks(scene *ebiten.Image, cx, cy, radius float32, startAngle, endAngle float64) {
	span := endAngle - startAngle
	for i := 0; i < 52; i++ {
		aFrac := math.Mod(float64(i)*0.38196601125+0.23, 1)
		rFrac := math.Sqrt(math.Mod(float64(i)*0.75487766625+0.31, 1))
		angle := startAngle + span*aFrac
		rr := radius * float32(0.12+0.78*rFrac)
		x := cx + rr*float32(math.Cos(angle))
		y := cy + rr*float32(math.Sin(angle))

		col := color.RGBA{R: 55, G: 130, B: 215, A: 44}
		if i%3 == 0 {
			col = color.RGBA{R: 8, G: 40, B: 118, A: 44}
		}
		size := float32(1)
		if i%9 == 0 {
			size = 2
		}
		vector.FillRect(scene, x-size/2, y-size/2, size, size, col, false)
	}
}

// drawFieldSector fills either a full circular field or a partial wedge.
func drawFieldSector(scene *ebiten.Image, cx, cy, radius float32, startAngle, endAngle float64, col color.RGBA) {
	if math.Abs(endAngle-startAngle-math.Pi*2) < 1e-9 {
		vector.FillCircle(scene, cx, cy, radius, col, false)
		return
	}
	drawFilledSector(scene, cx, cy, radius, startAngle, endAngle, col)
}

// drawFieldSectorBand strokes a narrow arc/ring segment inside a field.
func drawFieldSectorBand(scene *ebiten.Image, cx, cy, radius, width float32, startAngle, endAngle float64, col color.RGBA) {
	const steps = 48
	if radius <= 0 {
		return
	}
	var path vector.Path
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		angle := startAngle + (endAngle-startAngle)*t
		x := cx + radius*float32(math.Cos(angle))
		y := cy + radius*float32(math.Sin(angle))
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

// drawForestCanopyFlecks adds deterministic low-res texture inside the field.
func drawForestCanopyFlecks(scene *ebiten.Image, cx, cy, radius float32, startAngle, endAngle float64) {
	span := endAngle - startAngle
	for i := 0; i < 58; i++ {
		aFrac := math.Mod(float64(i)*0.38196601125+0.17, 1)
		rFrac := math.Sqrt(math.Mod(float64(i)*0.75487766625+0.11, 1))
		angle := startAngle + span*aFrac
		rr := radius * float32(0.15+0.78*rFrac)
		x := cx + rr*float32(math.Cos(angle))
		y := cy + rr*float32(math.Sin(angle))

		col := color.RGBA{R: 18, G: 82, B: 38, A: 46}
		if i%3 == 0 {
			col = color.RGBA{R: 4, G: 34, B: 22, A: 46}
		}
		size := float32(1)
		if i%11 == 0 {
			size = 2
		}
		vector.FillRect(scene, x-size/2, y-size/2, size, size, col, false)
	}
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

// ── System view ──────────────────────────────────────────────────────────────

// system-view palette
var (
	colSysBackground = color.RGBA{R: 4, G: 4, B: 12, A: 255}
	colSysStar       = color.RGBA{R: 140, G: 140, B: 160, A: 200}
	colSysStarting   = color.RGBA{R: 30, G: 105, B: 50, A: 255}   // deep forest green — awakened planet
	colSysStartRim   = color.RGBA{R: 100, G: 215, B: 115, A: 255} // bright active green rim
	colSysEchoA      = color.RGBA{R: 40, G: 140, B: 60, A: 255}   // dim green — dormant echo A
	colSysEchoB      = color.RGBA{R: 35, G: 120, B: 55, A: 255}   // dim green — dormant echo B
	colSysEchoRimA   = color.RGBA{R: 80, G: 210, B: 145, A: 200}  // muted cool blue-green rim — layout 0
	colSysEchoRimB   = color.RGBA{R: 155, G: 220, B: 70, A: 200}  // muted warm yellow-green rim — layout 1
	colSysEchoActiveRimA = color.RGBA{R: 80, G: 215, B: 145, A: 255} // bright cool blue-green — awakened/completed layout 0
	colSysEchoActiveRimB = color.RGBA{R: 155, G: 225, B: 70, A: 255} // bright warm yellow-green — awakened/completed layout 1
	colSysUnknown    = color.RGBA{R: 28, G: 28, B: 38, A: 255}    // dark silhouette
	colSysUnknownRim = color.RGBA{R: 50, G: 50, B: 70, A: 180}    // faint orbit tint
	colSysOrbit      = color.RGBA{R: 40, G: 40, B: 60, A: 80}     // faint orbit ellipse
	colSysSelect     = color.RGBA{R: 255, G: 240, B: 130, A: 255} // gold selection ring
	colRevealPulse   = color.RGBA{R: 240, G: 210, B: 80, A: 255}  // warm gold reveal pulse
	colRevealEdge    = color.RGBA{R: 120, G: 240, B: 130, A: 200} // green pulse edge
)

// starPositions returns deterministic dim star positions for the system background.
func starPositions() [][2]float32 {
	stars := make([][2]float32, 40)
	// Use golden-ratio distribution so stars look natural without random.Seed side-effects.
	const phi = 2.399
	for i := range stars {
		af := math.Mod(float64(i)*phi, math.Pi*2)
		rf := math.Sqrt(math.Mod(float64(i)*0.617, 1))
		stars[i][0] = float32(160 + 155*rf*math.Cos(af))
		stars[i][1] = float32(120 + 115*rf*math.Sin(af))
	}
	return stars
}

// drawSystemView renders the system-layer view onto scene.
func drawSystemView(scene *ebiten.Image, w *World, debug bool) {
	scene.Fill(colSysBackground)

	// Sparse star background.
	for _, s := range starPositions() {
		vector.FillRect(scene, s[0]-0.5, s[1]-0.5, 1, 1, colSysStar, false)
	}

	// Faint orbit ellipse for the unknown planet.
	if len(w.System.Planets) > 3 {
		unk := w.System.Planets[3]
		cx, cy := float32(unk.Pos.X), float32(unk.Pos.Y)
		// Very faint elliptical ring around the unknown position.
		drawSystemOrbitRing(scene, cx, cy, float32(unk.Radius)+8, 1.5, colSysOrbit)
	}

	// Draw planets (back to front: unknown, echoes, starting).
	drawOrder := []int{3, 2, 1, 0}
	for _, i := range drawOrder {
		if i >= len(w.System.Planets) {
			continue
		}
		p := w.System.Planets[i]
		drawSystemPlanet(scene, w, p, w.System.Selected == i, w.SimTime, 1.0, debug)
	}
}

// drawSystemPlanet renders one system-view planet disk with its rim ring.
// brightness is [0,1] and is used by the reveal flicker effect.
func drawSystemPlanet(scene *ebiten.Image, w *World, p SystemPlanet, selected bool, simTime float64, brightness float32, debug bool) {
	cx, cy := float32(p.Pos.X), float32(p.Pos.Y)
	r := float32(p.Radius)

	switch p.Kind {
	case PlanetStarting:
		body := scaleColor(colSysStarting, brightness)
		vector.FillCircle(scene, cx, cy, r, body, false)
		rimCol := scaleColor(colSysStartRim, brightness)
		drawSystemOrbitRing(scene, cx, cy, r, 2.5, rimCol)
		// Subtle afterglow on the awakened planet.
		glowAlpha := uint8(float32(18) * brightness)
		vector.FillCircle(scene, cx, cy, r+3, color.RGBA{R: 50, G: 180, B: 70, A: glowAlpha}, false)

	case PlanetEcho:
		col := colSysEchoA
		echoRim := colSysEchoRimA
		echoActiveRim := colSysEchoActiveRimA
		if p.RingColorIdx == 1 {
			col = colSysEchoB
			echoRim = colSysEchoRimB
			echoActiveRim = colSysEchoActiveRimB
		}
		body := scaleColor(col, brightness)
		vector.FillCircle(scene, cx, cy, r, body, false)
		switch {
		case p.Completed:
			glowAlpha := uint8(float32(18) * brightness)
			vector.FillCircle(scene, cx, cy, r+3, color.RGBA{R: echoActiveRim.R, G: echoActiveRim.G, B: echoActiveRim.B, A: glowAlpha}, false)
			rimCol := scaleColor(echoActiveRim, brightness)
			drawSystemOrbitRing(scene, cx, cy, r, 2.5, rimCol)
		case p.Awakened:
			// Active but incomplete: solid bright rim, no twinkle.
			rimCol := scaleColor(echoActiveRim, brightness)
			drawSystemOrbitRing(scene, cx, cy, r, 2.0, rimCol)
		default:
			// Dormant: twinkling rim in the layout's colour family.
			twinkle := float32(0.7 + 0.3*math.Sin(simTime*1.8+float64(p.RingColorIdx)*1.2))
			rimAlpha := uint8(float32(160) * twinkle * brightness)
			drawSystemOrbitRing(scene, cx, cy, r, 1.5, color.RGBA{R: echoRim.R, G: echoRim.G, B: echoRim.B, A: rimAlpha})
		}

	case PlanetUnknown:
		vector.FillCircle(scene, cx, cy, r, colSysUnknown, false)
		vector.FillCircle(scene, cx, cy, r, colSysUnknownRim, false)
		// Water Potential earned → blue-leaning shimmer hints at the frontier without unlocking it.
		if w.Economy.Potential[PotentialWater] > 0 {
			shimmer := float32(0.5 + 0.5*math.Sin(simTime*1.5))
			shimAlpha := uint8(float32(28) * shimmer * brightness)
			if shimAlpha > 0 {
				vector.FillCircle(scene, cx, cy, r+1, color.RGBA{R: 120, G: 160, B: 240, A: shimAlpha}, false)
			}
		}
	}

	if selected {
		drawSystemOrbitRing(scene, cx, cy, r+3, 2, colSysSelect)
	}
}

// scaleColor dims a colour toward black by brightness (0=black, 1=unchanged),
// blending in Lab space for perceptually even darkening.
func scaleColor(col color.RGBA, brightness float32) color.RGBA {
	if brightness >= 1 {
		return col
	}
	if brightness <= 0 {
		return color.RGBA{A: col.A}
	}
	c, _ := colorful.MakeColor(color.RGBA{R: col.R, G: col.G, B: col.B, A: 255})
	result := c.BlendLab(colorful.Color{}, float64(1-brightness)).Clamped()
	r8, g8, b8 := result.RGB255()
	return color.RGBA{R: r8, G: g8, B: b8, A: col.A}
}

func drawSystemOrbitRing(scene *ebiten.Image, cx, cy, radius, width float32, col color.RGBA) {
	const steps = 32
	var path vector.Path
	for i := 0; i <= steps; i++ {
		angle := float64(i) * math.Pi * 2 / steps
		x := cx + radius*float32(math.Cos(angle))
		y := cy + radius*float32(math.Sin(angle))
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

// ── Reveal animation ──────────────────────────────────────────────────────────

// drawReveal renders the one-time unlock reveal animation onto scene.
// Phase A (0..revealPhaseASecs): planet dims with a completion pulse.
// Phase B (revealPhaseASecs..revealDuration): system view appears with wave.
func drawReveal(scene *ebiten.Image, w *World, elapsed float64) {
	if elapsed < revealPhaseASecs {
		drawRevealPhaseA(scene, w, elapsed)
	} else {
		drawRevealPhaseB(scene, w, elapsed-revealPhaseASecs)
	}
}

func drawRevealPhaseA(scene *ebiten.Image, w *World, elapsed float64) {
	DrawWorld(scene, w, nil, false)
	t := float32(elapsed / revealPhaseASecs) // 0 → 1
	t = clamp32(t, 0, 1)

	// Progressive dim overlay.
	alpha := uint8(float32(160) * smoothStep(t))
	if alpha > 0 {
		scene.Fill(color.RGBA{R: 0, G: 0, B: 0, A: alpha})
	}

	// Completion pulse — warm gold expanding ring from planet center.
	cx, cy := float32(w.Planet.Center.X), float32(w.Planet.Center.Y)
	pRadius := float32(w.Planet.Radius) * (0.3 + 1.4*smoothStep(t))
	pulseAlpha := uint8(float32(200) * (1 - t))
	if pulseAlpha > 0 {
		drawSystemOrbitRing(scene, cx, cy, pRadius, 3, color.RGBA{R: colRevealPulse.R, G: colRevealPulse.G, B: colRevealPulse.B, A: pulseAlpha})
		edgeAlpha := uint8(float32(120) * (1 - t))
		drawSystemOrbitRing(scene, cx, cy, pRadius+4, 1.5, color.RGBA{R: colRevealEdge.R, G: colRevealEdge.G, B: colRevealEdge.B, A: edgeAlpha})
	}
}

func drawRevealPhaseB(scene *ebiten.Image, w *World, phaseElapsed float64) {
	phaseDur := revealDuration - revealPhaseASecs
	t := clamp32(float32(phaseElapsed/phaseDur), 0, 1) // 0 → 1 over Phase B

	// Compute wave radius.
	waveRadius := float32(phaseElapsed * revealWaveSpeedPxS)

	// Per-echo flicker state: brightness based on whether wave has passed.
	echoGlow := [2]float32{0, 0}
	if len(w.System.Planets) >= 3 {
		for ei := 0; ei < 2; ei++ {
			ep := w.System.Planets[1+ei]
			dist := float32(w.System.Planets[0].Pos.Dist(ep.Pos))
			if waveRadius > dist {
				// Flicker: on/off/on/off then steady. Use square-wave on (wave - dist) time.
				flickerTime := float32(phaseElapsed) - dist/float32(revealWaveSpeedPxS)
				flickerPhase := flickerTime * 8.0 // ~8 flickers per second
				flicker := float32(1)
				iPhase := int(flickerPhase)
				if iPhase < 4 && iPhase%2 == 1 {
					flicker = 0.2 // dim flicker
				}
				// Settle after 4 flickers.
				if flickerTime > 0.5 {
					flicker = 1.0
				}
				echoGlow[ei] = clamp32(flicker, 0, 1)
			}
		}
	}

	// Unknown shimmer state.
	unknownShimmer := float32(0)
	if len(w.System.Planets) >= 4 {
		unk := w.System.Planets[3]
		dist := float32(w.System.Planets[0].Pos.Dist(unk.Pos))
		if waveRadius > dist {
			shimmerTime := float32(phaseElapsed) - dist/float32(revealWaveSpeedPxS)
			// Single faint shimmer that fades out quickly.
			unknownShimmer = clamp32(1-shimmerTime*3, 0, 0.4)
		}
	}

	// Draw system view base.
	scene.Fill(colSysBackground)
	for _, s := range starPositions() {
		vector.FillRect(scene, s[0]-0.5, s[1]-0.5, 1, 1, colSysStar, false)
	}

	// Unknown with shimmer.
	if len(w.System.Planets) >= 4 {
		unk := w.System.Planets[3]
		cx, cy := float32(unk.Pos.X), float32(unk.Pos.Y)
		drawSystemOrbitRing(scene, cx, cy, float32(unk.Radius)+8, 1.5, colSysOrbit)
		vector.FillCircle(scene, cx, cy, float32(unk.Radius), colSysUnknown, false)
		if unknownShimmer > 0 {
			shimAlpha := uint8(float32(80) * unknownShimmer)
			vector.FillCircle(scene, cx, cy, float32(unk.Radius)+2, color.RGBA{R: 200, G: 200, B: 220, A: shimAlpha}, false)
		}
	}

	// Echoes with flicker.
	for ei := 0; ei < 2; ei++ {
		if 1+ei >= len(w.System.Planets) {
			break
		}
		drawSystemPlanet(scene, w, w.System.Planets[1+ei], false, w.SimTime, echoGlow[ei], false)
	}

	// Starting planet: bright, selected.
	if len(w.System.Planets) > 0 {
		drawSystemPlanet(scene, w, w.System.Planets[0], true, w.SimTime, 1.0, false)
	}

	// Expanding wave ring.
	if waveRadius > 0 && len(w.System.Planets) > 0 {
		sp := w.System.Planets[0]
		waveAlpha := uint8(float32(160) * clamp32(1-t*1.2, 0, 1))
		if waveAlpha > 0 {
			drawSystemOrbitRing(scene,
				float32(sp.Pos.X), float32(sp.Pos.Y),
				waveRadius, 2,
				color.RGBA{R: colRevealPulse.R, G: colRevealPulse.G, B: colRevealPulse.B, A: waveAlpha})
		}
	}

	// Fade in system UI overlay signal at end of Phase B.
	_ = t // available for future HUD fade-in if needed
}

// drawTownField renders the settlement wedge inside the planet at the Town Hall
// angle, with visible dwelling slots for built capacity. No-op until a Town
// Hall exists.
func drawTownField(scene *ebiten.Image, w *World, radius float32) {
	th := townHall(w)
	if th == nil {
		return
	}
	cx, cy := float32(w.Planet.Center.X), float32(w.Planet.Center.Y)
	start := th.Angle - townFieldHalfArc
	end := th.Angle + townFieldHalfArc

	// Warm clay wedge fill — full pizza slice from center to rim.
	drawFilledSector(scene, cx, cy, radius, start, end, colTownFieldBase)
	// Outer edge definition.
	drawFieldSectorBand(scene, cx, cy, radius-0.5, 1.5, start, end, colTownFieldEdge)

	// Dwelling slots — only built capacity is visible, so fresh towns start with
	// one house and fill in one purchase at a time.
	slots := townFieldSlots(w.Planet, th)
	if len(slots) == 0 {
		return
	}
	builtSlots := w.Economy.WorkerCapacity
	if builtSlots < 0 {
		builtSlots = 0
	}
	if builtSlots > len(slots) {
		builtSlots = len(slots)
	}
	occupiedSlots := len(w.Workers)
	if occupiedSlots > builtSlots {
		occupiedSlots = builtSlots
	}
	cos := float32(math.Cos(th.Angle))
	sin := float32(math.Sin(th.Angle))
	ix := -cos // inward
	iy := -sin
	tx := -sin // tangent
	ty := cos
	for i, pos := range slots[:builtSlots] {
		col := colTownFieldSlot
		if i < occupiedSlots {
			col = colTownFieldSlotOccupied
		}
		drawOrientedRect(scene, float32(pos.X), float32(pos.Y), tx, ty, ix, iy, 1.5, 1.5, col)
	}
}
