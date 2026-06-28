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
