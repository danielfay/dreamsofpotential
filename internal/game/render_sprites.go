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

	sparkleSpriteOnce sync.Once
	sparkleSpriteImg  *ebiten.Image

	sparkleAseOnce sync.Once
	sparkleAse_    *goaseprite.File

	transferButtonSpriteOnce sync.Once
	transferButtonSpriteImg  *ebiten.Image

	starfieldSpriteOnce sync.Once
	starfieldSpriteImg  *ebiten.Image
)

func starfieldSprite() *ebiten.Image {
	starfieldSpriteOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(assets.StarfieldPNG))
		if err != nil {
			panic(err)
		}
		starfieldSpriteImg = ebiten.NewImageFromImage(img)
	})
	return starfieldSpriteImg
}

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

func sparkleSprite() *ebiten.Image {
	sparkleSpriteOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(assets.SparklePNG))
		if err != nil {
			panic(err)
		}
		sparkleSpriteImg = ebiten.NewImageFromImage(img)
	})
	return sparkleSpriteImg
}

func sparkleAse() *goaseprite.File {
	sparkleAseOnce.Do(func() {
		sparkleAse_ = goaseprite.Read(assets.SparkleJSON)
	})
	return sparkleAse_
}

func transferButtonSprite() *ebiten.Image {
	transferButtonSpriteOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(assets.TransferButtonPNG))
		if err != nil {
			panic(err)
		}
		transferButtonSpriteImg = ebiten.NewImageFromImage(img)
	})
	return transferButtonSpriteImg
}

// drawSparkle draws an animated water sparkle sprite centered at n.Pos.
// The 4-frame cycle shifts through white/blue/pink, staggered per node ID
// so sparkles twinkle independently. claimed dims at low alpha; pulsed and
// alphaBoost both brighten (pulse = field pulse, alphaBoost = growth cue).
func drawSparkle(scene *ebiten.Image, n *ResourceNode, claimed bool, pulsed bool, visualScale float32, alphaBoost uint8, simTime float64) {
	ase := sparkleAse()
	frameDur := float64(ase.Frames[0].Duration)
	// Pick size tier: frames 0–3 = large, 4–7 = medium, 8–11 = small.
	sizeBase := 0
	if n.Size < 0.55 {
		sizeBase = 8
	} else if n.Size < 0.72 {
		sizeBase = 4
	}
	frameIdx := sizeBase + (int(simTime/frameDur)+int(n.ID))%4
	fr := ase.Frames[frameIdx]
	fw, fh := int(ase.FrameWidth), int(ase.FrameHeight)
	frame := sparkleSprite().SubImage(image.Rect(fr.X, fr.Y, fr.X+fw, fr.Y+fh)).(*ebiten.Image)

	scale := float64(visualScale)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(fw)/2, -float64(fh)/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(n.Pos.X, n.Pos.Y)
	if claimed {
		op.ColorScale.ScaleAlpha(0.45)
	}
	if pulsed {
		op.ColorScale.Scale(1.35, 1.35, 1.35, 1.0)
	}
	if alphaBoost > 0 {
		boost := 1.0 + float32(alphaBoost)/128.0
		op.ColorScale.Scale(boost, boost, boost, 1.0)
	}
	scene.DrawImage(frame, op)
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
