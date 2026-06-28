package game

import (
	"image/color"
	"math"

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

// planet-view palette
var (
	colBackground            = color.RGBA{R: 10, G: 10, B: 20, A: 255}
	colPlanetBody            = color.RGBA{R: 5, G: 5, B: 10, A: 255}    // near-black interior
	colPlanetEdge            = color.RGBA{R: 50, G: 130, B: 50, A: 255} // green rim ring
	colNodeFree              = color.RGBA{R: 40, G: 160, B: 60, A: 255}
	colNodeReserved          = color.RGBA{R: 32, G: 130, B: 48, A: 255}
	colNodeClaimed           = color.RGBA{R: 20, G: 100, B: 35, A: 255}
	colTownFieldBase         = color.RGBA{R: 120, G: 82, B: 40, A: 150}  // warm clay wedge fill
	colTownFieldEdge         = color.RGBA{R: 165, G: 115, B: 58, A: 120} // amber edge bands
	colTownFieldSlot         = color.RGBA{R: 200, G: 148, B: 72, A: 220} // available dwelling slot
	colTownFieldSlotOccupied = color.RGBA{R: 150, G: 80, B: 8, A: 255}   // occupied — darker richer amber
	colGhostOk               = color.RGBA{R: 200, G: 200, B: 255, A: 160}
	colGhostBad              = color.RGBA{R: 200, G: 80, B: 80, A: 80}
	colRouteFree             = color.RGBA{R: 160, G: 220, B: 255, A: 200} // base; alpha/width scaled by quality
	colRouteClaimed          = color.RGBA{R: 100, G: 130, B: 150, A: 90}  // uniform muted
	colPreviewLens           = color.RGBA{R: 125, G: 145, B: 170, A: 16}
	colPreviewDebug          = color.RGBA{R: 255, G: 220, B: 80, A: 180} // debug range markers

	colInfluenceDebugFill = color.RGBA{R: 20, G: 60, B: 160, A: 28}  // debug-only: transparent blue wedge for KindWaterInfluence
	colInfluenceDebugEdge = color.RGBA{R: 55, G: 130, B: 215, A: 80} // debug-only: water-blue rim
	colDockWedge          = color.RGBA{R: 95, G: 165, B: 235, A: 38} // dock dive-reach cone: faint water-blue fill
)

// planetViewPalette holds the planet-view colors that vary by planet identity.
type planetViewPalette struct {
	background color.RGBA
	edge       color.RGBA
	body       color.RGBA
}

// atmosphereGrads holds per-planet-type alpha-falloff gradients for the
// completion glow. Built once at init; keyed by atmosphere colour variant.
var (
	gradAtmosphereStart colorgrad.Gradient
	gradAtmosphereA     colorgrad.Gradient
	gradAtmosphereB     colorgrad.Gradient
	gradAtmosphereWater colorgrad.Gradient
)

func init() {
	gradAtmosphereStart = buildAtmosphereGrad(colAtmosphereStart)
	gradAtmosphereA = buildAtmosphereGrad(colAtmosphereA)
	gradAtmosphereB = buildAtmosphereGrad(colAtmosphereB)
	gradAtmosphereWater = buildAtmosphereGrad(colAtmosphereWater)
}

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
	case p.Kind == PlanetUnknown && p.Completed:
		completedAt = p.CompletedAt
	case p.Kind == PlanetStarting && w.System.Unlocked:
		completedAt = p.CompletedAt
	default:
		return
	}

	base, atmosGrad := atmosphereFor(p)

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

func atmosphereFor(p SystemPlanet) (color.RGBA, colorgrad.Gradient) {
	switch {
	case p.Kind == PlanetUnknown:
		return colAtmosphereWater, gradAtmosphereWater
	case p.Kind == PlanetEcho && p.LayoutID == 1:
		return colAtmosphereB, gradAtmosphereB
	case p.Kind == PlanetEcho:
		return colAtmosphereA, gradAtmosphereA
	default:
		return colAtmosphereStart, gradAtmosphereStart
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
	// Paint field-specific rim colors over the base rim; the body circle below
	// will cover the field sector interiors, leaving only the rim band visible.
	for _, f := range w.Planet.Fields {
		if rimCol, ok := fieldRimColor(f.Kind); ok {
			start := f.CenterAngle - f.HalfArc
			end := f.CenterAngle + f.HalfArc
			drawFieldSector(scene, cx, cy, r, start, end, rimCol)
		}
	}
	vector.FillCircle(scene, cx, cy, r-rimWidth, pal.body, false)

	// Resource field interior fill: stable composition, not node-spawn progress.
	for _, f := range w.Planet.Fields {
		drawResourceFieldFill(scene, w.Planet, f, r-rimWidth)
		if debug && f.Kind == KindWaterInfluence {
			start := f.CenterAngle - f.HalfArc
			end := f.CenterAngle + f.HalfArc
			drawFilledSector(scene, cx, cy, r-rimWidth, start, end, colInfluenceDebugFill)
			drawFieldSectorBand(scene, cx, cy, r-rimWidth-0.5, 1.5, start, end, colInfluenceDebugEdge)
		}
		if len(w.Buildings) == 0 {
			drawPreFoundingPulse(scene, w.Planet, f, r, w.SimTime)
		}
		drawResourceFieldPulse(scene, w, f, r-rimWidth, pv != nil)
	}
	// Town field: settlement wedge anchored to the Town Hall, drawn over the forest
	// field but under nodes/buildings/workers so it transforms the planet interior.
	drawTownField(scene, w, r-rimWidth)
	// Dock dive-reach cones: drawn under nodes/buildings like the town field.
	// L1 = annular sector 1/3 from rim; L2+ = full sector to center.
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			if b.Level >= 2 {
				drawDockReachSector(scene, cx, cy, 0, r,
					b.Angle-dockWedgeHalfArc, b.Angle+dockWedgeHalfArc)
			} else {
				drawDockReachSector(scene, cx, cy, r*2/3, r,
					b.Angle-dockWedgeHalfArc, b.Angle+dockWedgeHalfArc)
			}
		}
	}

	// resource nodes — interior sparkles as blue + shapes; rim nodes as pine trees
	for _, n := range w.Nodes {
		if n.Interior {
			col := colSparkle
			if n.OwnerID != -1 {
				col = colSparkleClaimed
			}
			if pulseActive(w, n.Pulse) {
				col = brighten(col, 45)
			}
			drawSparkle(scene, n, col, growthNodeVisualScale(w, n), growthNodeVisualAlpha(w, n), w.SimTime)
			continue
		}
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
		alpha := growthNodeVisualAlpha(w, n)
		if alpha > 0 && w.growthCue.WaterInfluenced {
			col = blendColor(col, colWaterGrowthTint, alpha)
			alpha = 0
		}
		drawPineTree(scene, n, col, growthNodeVisualScale(w, n), alpha)
	}

	// placement preview — route lines and ghost, drawn above nodes/below buildings
	if pv != nil {
		drawPreview(scene, w.Planet, pv, debug)
	}

	// buildings
	for _, b := range w.Buildings {
		switch b.Kind {
		case KindTownHall:
			col := colTownHall
			if pulseActive(w, b.Pulse) {
				col = brighten(col, 40)
			}
			drawTownHallArt(scene, w.Planet, b.Angle, col)
			drawTownGrowthGauge(scene, w.Planet, b, w.Economy.TownGrowth, w.Economy.TownGrowthCap)
		case KindDock:
			col := colDock
			if pulseActive(w, b.Pulse) {
				col = brighten(col, 40)
			}
			drawDockArt(scene, w.Planet, b.Angle, col, b.Level)
		default: // KindLoggingCamp
			col := colBuilding
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
	slots := idleTowerSlots(w.Planet, th, idleCount)
	slotIdx := 0
	for _, wk := range w.Workers {
		if workerUsesIdleHome(wk) && th != nil {
			sp := slots[slotIdx]
			slotIdx++
			drawWorker(scene, sp.X, sp.Y, th.Angle, colWorkerLaden)
		} else {
			drawWorker(scene, wk.Pos.X, wk.Pos.Y, wk.Angle, workerColor(w, wk))
		}
	}
}
