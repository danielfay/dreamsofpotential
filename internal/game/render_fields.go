package game

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// drawResourceFieldFill paints a full resource field as terrain composition.
// Node-spawn progress is shown in the HUD, so this stays visually stable.
func drawResourceFieldFill(scene *ebiten.Image, planet Planet, f *ResourceField, radius float32) {
	cx, cy := float32(planet.Center.X), float32(planet.Center.Y)
	start := f.CenterAngle - f.HalfArc
	end := f.CenterAngle + f.HalfArc

	if f.Kind == KindWaterInfluence {
		return
	}
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
	innerCol := color.RGBA{R: 95, G: 210, B: 108, A: uint8(float32(ringAlpha) * 0.55)}
	outerCol := color.RGBA{R: 118, G: 235, B: 124, A: ringAlpha}
	if w.growthCue.Kind == KindWater {
		innerCol = color.RGBA{R: 50, G: 160, B: 220, A: uint8(float32(ringAlpha) * 0.55)}
		outerCol = color.RGBA{R: 80, G: 190, B: 255, A: ringAlpha}
	} else if w.growthCue.WaterInfluenced {
		innerCol = color.RGBA{R: 45, G: 185, B: 140, A: uint8(float32(ringAlpha) * 0.55)}
		outerCol = color.RGBA{R: 60, G: 200, B: 150, A: ringAlpha}
	}
	if inner > 0 {
		drawFieldSectorBand(scene, cx, cy, inner, 1, start, end, innerCol)
	}
	drawFieldSectorBand(scene, cx, cy, outer, 2, start, end, outerCol)
}

// drawPreFoundingPulse draws a slow breathing ring on the field rim to signal
// wood potential before the Town Hall is placed. Fades in/out on a ~3 s cycle.
func drawPreFoundingPulse(scene *ebiten.Image, planet Planet, f *ResourceField, radius float32, simTime float64) {
	if f.Kind == KindWaterInfluence {
		return
	}
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

// drawAnnularSector draws a filled ring-slice from innerR to outerR spanning startAngle..endAngle.
func drawAnnularSector(scene *ebiten.Image, cx, cy, innerR, outerR float32, startAngle, endAngle float64, col color.RGBA) {
	if outerR <= 0 || innerR >= outerR {
		return
	}
	const steps = 32
	var path vector.Path
	path.MoveTo(cx+innerR*float32(math.Cos(startAngle)), cy+innerR*float32(math.Sin(startAngle)))
	path.LineTo(cx+outerR*float32(math.Cos(startAngle)), cy+outerR*float32(math.Sin(startAngle)))
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		angle := startAngle + (endAngle-startAngle)*t
		path.LineTo(cx+outerR*float32(math.Cos(angle)), cy+outerR*float32(math.Sin(angle)))
	}
	path.LineTo(cx+innerR*float32(math.Cos(endAngle)), cy+innerR*float32(math.Sin(endAngle)))
	for i := steps - 1; i >= 0; i-- {
		t := float64(i) / float64(steps)
		angle := startAngle + (endAngle-startAngle)*t
		path.LineTo(cx+innerR*float32(math.Cos(angle)), cy+innerR*float32(math.Sin(angle)))
	}
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
