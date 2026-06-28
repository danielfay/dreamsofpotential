package game

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png"
	"math"
	"sync"

	"github.com/danielfay/dreamsofpotential/assets"
	"github.com/hajimehoshi/ebiten/v2"
)

var (
	workerSpriteOnce sync.Once
	workerSpriteImg  *ebiten.Image

	treeSpriteOnce sync.Once
	treeSpriteImg  *ebiten.Image

	campSpriteOnce sync.Once
	campSpriteImg  *ebiten.Image

	townHallBaseSpriteOnce sync.Once
	townHallBaseSpriteImg  *ebiten.Image

	townHallFireSpriteOnce sync.Once
	townHallFireSpriteImg  *ebiten.Image
)

func workerSprite() *ebiten.Image {
	workerSpriteOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(assets.WorkerPNG))
		if err != nil {
			panic(err)
		}
		workerSpriteImg = ebiten.NewImageFromImage(img)
	})
	return workerSpriteImg
}

func treeSprite() *ebiten.Image {
	treeSpriteOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(assets.TreePNG))
		if err != nil {
			panic(err)
		}
		treeSpriteImg = ebiten.NewImageFromImage(img)
	})
	return treeSpriteImg
}

// drawTreeSprite draws the tree sprite anchored at the rim point (n.Pos),
// extending outward. Drop-in replacement for drawPineTree.
func drawTreeSprite(scene *ebiten.Image, n *ResourceNode, col color.RGBA, visualScale float32, alphaBoost uint8) {
	if alphaBoost > 0 {
		col = brighten(col, alphaBoost)
	}
	s := float64(n.Size) * float64(visualScale)
	img := treeSprite()
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(w)/2, -float64(h))
	op.GeoM.Scale(s, s)
	op.GeoM.Rotate(n.Angle + math.Pi/2)
	op.GeoM.Translate(n.Pos.X, n.Pos.Y)
	op.ColorScale.Scale(
		float32(col.R)/255,
		float32(col.G)/255,
		float32(col.B)/255,
		float32(col.A)/255,
	)
	scene.DrawImage(img, op)
}

func campSprite() *ebiten.Image {
	campSpriteOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(assets.CampPNG))
		if err != nil {
			panic(err)
		}
		campSpriteImg = ebiten.NewImageFromImage(img)
	})
	return campSpriteImg
}

// drawCampSprite draws the logging camp sprite with base at the rim point,
// roof extending outward. pos is the rim point, angle is the outward normal.
func drawCampSprite(scene *ebiten.Image, pos Vec, angle float64, col color.RGBA) {
	img := campSprite()
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(w)/2, -float64(h))
	op.GeoM.Rotate(angle + math.Pi/2)
	op.GeoM.Translate(pos.X, pos.Y)
	op.ColorScale.Scale(
		float32(col.R)/255,
		float32(col.G)/255,
		float32(col.B)/255,
		float32(col.A)/255,
	)
	scene.DrawImage(img, op)
}

func townHallBaseSprite() *ebiten.Image {
	townHallBaseSpriteOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(assets.TownHallBasePNG))
		if err != nil {
			panic(err)
		}
		townHallBaseSpriteImg = ebiten.NewImageFromImage(img)
	})
	return townHallBaseSpriteImg
}

func townHallFireSprite() *ebiten.Image {
	townHallFireSpriteOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(assets.TownHallFirePNG))
		if err != nil {
			panic(err)
		}
		townHallFireSpriteImg = ebiten.NewImageFromImage(img)
	})
	return townHallFireSpriteImg
}

const (
	townHallBaseW     = 17
	townHallBaseH     = 9
	townHallFireW     = 5
	townHallFireH     = 9
	townHallFireCount = 4
	// fire columns 6–10 of the base; offset = -(baseW/2) + 6 = -8.5 + 6 = -2.5
	townHallFireOffX = -2.5
	// seconds per fire frame (~8 fps)
	townHallFrameSec = 0.12
)

// drawTownHallSprite draws the animated town hall: static houses + log base,
// then the current fire frame composited on top. Base at rim, fire outward.
func drawTownHallSprite(scene *ebiten.Image, planet Planet, angle float64, pulse bool, simTime float64) {
	rimPt := planet.RimPoint(angle)
	rot := angle + math.Pi/2

	// Static base: houses + log pile.
	base := townHallBaseSprite()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(townHallBaseW)/2, -float64(townHallBaseH))
	op.GeoM.Rotate(rot)
	op.GeoM.Translate(rimPt.X, rimPt.Y)
	if pulse {
		op.ColorScale.Scale(1.3, 1.3, 1.3, 1.0)
	}
	scene.DrawImage(base, op)

	// Animated fire frame.
	fire := townHallFireSprite()
	frameIdx := int(simTime/townHallFrameSec) % townHallFireCount
	fireFrame := fire.SubImage(image.Rect(
		frameIdx*townHallFireW, 0,
		(frameIdx+1)*townHallFireW, townHallFireH,
	)).(*ebiten.Image)
	op2 := &ebiten.DrawImageOptions{}
	op2.GeoM.Translate(townHallFireOffX, -float64(townHallFireH))
	op2.GeoM.Rotate(rot)
	op2.GeoM.Translate(rimPt.X, rimPt.Y)
	if pulse {
		op2.ColorScale.Scale(1.3, 1.3, 1.3, 1.0)
	}
	scene.DrawImage(fireFrame, op2)
}

// drawTownHallGhost draws the town hall placement ghost using frame 0 of the
// fire at reduced alpha, so the player can see the shape/colors while placing.
func drawTownHallGhost(scene *ebiten.Image, planet Planet, angle float64, col color.RGBA) {
	alpha := float32(col.A) / 255
	rimPt := planet.RimPoint(angle)
	rot := angle + math.Pi/2

	base := townHallBaseSprite()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(townHallBaseW)/2, -float64(townHallBaseH))
	op.GeoM.Rotate(rot)
	op.GeoM.Translate(rimPt.X, rimPt.Y)
	op.ColorScale.ScaleAlpha(alpha)
	scene.DrawImage(base, op)

	fire := townHallFireSprite()
	fireFrame := fire.SubImage(image.Rect(0, 0, townHallFireW, townHallFireH)).(*ebiten.Image)
	op2 := &ebiten.DrawImageOptions{}
	op2.GeoM.Translate(townHallFireOffX, -float64(townHallFireH))
	op2.GeoM.Rotate(rot)
	op2.GeoM.Translate(rimPt.X, rimPt.Y)
	op2.ColorScale.ScaleAlpha(alpha)
	scene.DrawImage(fireFrame, op2)
}

// drawWorker draws the worker sprite centered at (x, y), rotated so the
// body faces inward toward the planet center. rimAngle is the worker's
// current angle on the planet rim (wk.Angle).
func drawWorker(scene *ebiten.Image, x, y, rimAngle float64, col color.RGBA) {
	img := workerSprite()
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(w)/2, -float64(h)/2)
	op.GeoM.Rotate(rimAngle + math.Pi/2)
	op.GeoM.Translate(x, y)
	op.ColorScale.Scale(
		float32(col.R)/255,
		float32(col.G)/255,
		float32(col.B)/255,
		float32(col.A)/255,
	)
	scene.DrawImage(img, op)
}
