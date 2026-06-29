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
	"github.com/solarlune/goaseprite"
)

var (
	workerSpriteOnce sync.Once
	workerSpriteImg  *ebiten.Image

	treeSpriteOnce sync.Once
	treeSpriteImg  *ebiten.Image

	campSpriteOnce sync.Once
	campSpriteImg  *ebiten.Image

	dockSpriteOnce sync.Once
	dockSpriteImg  *ebiten.Image

	dockAseOnce sync.Once
	dockAse_    *goaseprite.File

	townHallBaseSpriteOnce sync.Once
	townHallBaseSpriteImg  *ebiten.Image

	townHallFireSpriteOnce sync.Once
	townHallFireSpriteImg  *ebiten.Image

	townHallFireAseOnce sync.Once
	townHallFireAse_    *goaseprite.File
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

func dockSprite() *ebiten.Image {
	dockSpriteOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(assets.DockPNG))
		if err != nil {
			panic(err)
		}
		dockSpriteImg = ebiten.NewImageFromImage(img)
	})
	return dockSpriteImg
}

func dockAse() *goaseprite.File {
	dockAseOnce.Do(func() {
		dockAse_ = goaseprite.Read(assets.DockJSON)
	})
	return dockAse_
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

const dockSpriteRimY = 4

// drawDockArt draws the dock sprite anchored on the rim, with posts outward and
// the deck straddling the shoreline. Level 2 uses the upgraded rail frame.
func drawDockArt(scene *ebiten.Image, p Planet, angle float64, col color.RGBA, level int) {
	ase := dockAse()
	frameIdx := 0
	if level >= 2 && len(ase.Frames) > 1 {
		frameIdx = 1
	}
	fr := ase.Frames[frameIdx]
	fw, fh := int(ase.FrameWidth), int(ase.FrameHeight)
	frame := dockSprite().SubImage(image.Rect(fr.X, fr.Y, fr.X+fw, fr.Y+fh)).(*ebiten.Image)
	rimPt := p.RimPoint(angle)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(fw)/2, -float64(dockSpriteRimY))
	op.GeoM.Rotate(angle + math.Pi/2)
	op.GeoM.Translate(rimPt.X, rimPt.Y)
	op.ColorScale.Scale(
		float32(col.R)/255,
		float32(col.G)/255,
		float32(col.B)/255,
		float32(col.A)/255,
	)
	scene.DrawImage(frame, op)
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

func townHallFireAse() *goaseprite.File {
	townHallFireAseOnce.Do(func() {
		townHallFireAse_ = goaseprite.Read(assets.TownHallFireJSON)
	})
	return townHallFireAse_
}

const (
	townHallBaseW = 17
	townHallBaseH = 9
	// fire columns 6–10 of the base; offset = -(baseW/2) + 6 = -8.5 + 6 = -2.5
	townHallFireOffX = -2.5
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

	// Animated fire frame — timing and geometry from goaseprite data.
	fireData := townHallFireAse()
	frameDur := float64(fireData.Frames[0].Duration)
	frameIdx := int(simTime/frameDur) % len(fireData.Frames)
	fr := fireData.Frames[frameIdx]
	fw, fh := int(fireData.FrameWidth), int(fireData.FrameHeight)
	fire := townHallFireSprite()
	fireFrame := fire.SubImage(image.Rect(fr.X, fr.Y, fr.X+fw, fr.Y+fh)).(*ebiten.Image)
	op2 := &ebiten.DrawImageOptions{}
	op2.GeoM.Translate(townHallFireOffX, -float64(fh))
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

	fireData := townHallFireAse()
	fr0 := fireData.Frames[0]
	fw, fh := int(fireData.FrameWidth), int(fireData.FrameHeight)
	fire := townHallFireSprite()
	fireFrame := fire.SubImage(image.Rect(fr0.X, fr0.Y, fr0.X+fw, fr0.Y+fh)).(*ebiten.Image)
	op2 := &ebiten.DrawImageOptions{}
	op2.GeoM.Translate(townHallFireOffX, -float64(fh))
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
