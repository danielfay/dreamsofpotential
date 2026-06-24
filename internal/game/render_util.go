package game

import (
	"image/color"
	"math"

	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

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

func brighten(col color.RGBA, amount uint8) color.RGBA {
	if amount == 0 {
		return col
	}
	c, _ := colorful.MakeColor(color.RGBA{R: col.R, G: col.G, B: col.B, A: 255})
	result := c.BlendLab(colorful.Color{R: 1, G: 1, B: 1}, float64(amount)/255.0).Clamped()
	r8, g8, b8 := result.RGB255()
	return color.RGBA{R: r8, G: g8, B: b8, A: col.A}
}

// blendColor blends col toward target in Lab space by amount/255.
func blendColor(col color.RGBA, target color.RGBA, amount uint8) color.RGBA {
	if amount == 0 {
		return col
	}
	c, _ := colorful.MakeColor(color.RGBA{R: col.R, G: col.G, B: col.B, A: 255})
	t, _ := colorful.MakeColor(color.RGBA{R: target.R, G: target.G, B: target.B, A: 255})
	result := c.BlendLab(t, float64(amount)/255.0).Clamped()
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
