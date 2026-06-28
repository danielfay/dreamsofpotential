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
