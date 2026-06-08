package game

import (
	"image/color"
	"os"

	"github.com/ebitenui/ebitenui"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const dt = 1.0 / 60.0
const autoSavePeriod = 10.0 // seconds between autosaves

// Game is the root ebiten game object.
type Game struct {
	world       *World
	scene       *ebiten.Image // low-res 320×240 canvas; scaled up to the window
	ui          *ebitenui.UI
	hud         *HUD
	placing     bool    // true while waiting for player to click a camp location
	debug       bool    // F3 — verbose debug panel; session-only, not persisted
	pulseTime   float64 // seconds remaining on the unaffordable-cost flash
	pulseTarget int     // which button pulses: 0=none, 1=build, 2=worker
	screenW     int     // current screen dimensions, updated each Draw()
	screenH     int
	hudScale    int     // integer view scale at last HUD build; triggers rebuild on change
	saveTimer   float64 // counts down to next autosave
}

// New constructs and returns a ready-to-run Game.
// It loads a saved world from disk if one exists, otherwise starts fresh.
func New() (*Game, error) {
	w, err := Load()
	if err != nil {
		w = NewWorld()
	}
	const initialScale = 2
	g := &Game{
		world:     w,
		scene:     ebiten.NewImage(virtW, virtH),
		hudScale:  initialScale,
		saveTimer: autoSavePeriod,
	}
	hud, ui, err := buildHUD(g, initialScale)
	if err != nil {
		return nil, err
	}
	g.hud = hud
	g.ui = ui
	return g, nil
}

func (g *Game) Update() error {
	if ebiten.IsWindowBeingClosed() {
		_ = Save(g.world)
		os.Exit(0)
	}
	g.ui.Update()
	Step(g.world, dt)
	if g.pulseTime > 0 {
		g.pulseTime -= dt
	}
	g.saveTimer -= dt
	if g.saveTimer <= 0 {
		_ = Save(g.world)
		g.saveTimer = autoSavePeriod
	}
	g.hud.Refresh(g.world, g.placing, g.debug)
	g.handleInput()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.screenW, g.screenH = screen.Bounds().Dx(), screen.Bounds().Dy()

	if newScale := g.intScale(); newScale != g.hudScale {
		g.hudScale = newScale
		if hud, ui, err := buildHUD(g, newScale); err == nil {
			g.hud = hud
			g.ui = ui
		}
	}

	DrawWorld(g.scene, g.world, g.ghostPos())

	scale, offX, offY := viewGeom(g.screenW, g.screenH)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(offX, offY)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(g.scene, op)

	g.ui.Draw(screen)
	g.drawOverlay(screen)
}

// intScale returns the floor'd integer view scale, clamped to at least 1.
func (g *Game) intScale() int {
	scale, _, _ := viewGeom(g.screenW, g.screenH)
	s := int(scale)
	if s < 1 {
		s = 1
	}
	return s
}

// screenToWorld converts a native screen position to low-res world coordinates,
// accounting for the current letterbox/pillarbox offset and scale.
func (g *Game) screenToWorld(sx, sy int) Vec {
	scale, offX, offY := viewGeom(g.screenW, g.screenH)
	return Vec{
		X: (float64(sx) - offX) / scale,
		Y: (float64(sy) - offY) / scale,
	}
}

// drawOverlay draws normal-mode-only HUD affordances on top of EbitenUI in
// native screen space. Skipped in debug mode — overlays are a normal-mode
// feature. Widget Rects are valid here because ui.Draw already laid them out.
func (g *Game) drawOverlay(screen *ebiten.Image) {
	if g.debug {
		return
	}

	// Selected-outline around the build button while placement mode is active.
	if g.placing {
		r := g.hud.buildCampBtn.GetWidget().Rect
		vector.StrokeRect(screen,
			float32(r.Min.X)-2, float32(r.Min.Y)-2,
			float32(r.Max.X-r.Min.X)+4, float32(r.Max.Y-r.Min.Y)+4,
			2, colGhostOk, false)
	}

	// Gauge bar beneath the resource HUD icon+number, only after discovery.
	if g.world.ResourceDiscovered && len(g.world.Planet.Fields) > 0 {
		f := g.world.Planet.Fields[0]
		frac := float32(0)
		if f.Cap > 0 {
			frac = float32(f.Counter / f.Cap)
			if frac > 1 {
				frac = 1
			}
		}
		r := g.hud.resourceHUD.GetWidget().Rect
		x := float32(r.Min.X)
		y := float32(r.Max.Y) + 2
		w := float32(r.Max.X - r.Min.X)
		const h = float32(3)
		colFill := color.RGBA{R: 40, G: 160, B: 60, A: 180}
		colFrame := color.RGBA{R: 80, G: 80, B: 80, A: 180}
		vector.StrokeRect(screen, x, y, w, h, 1, colFrame, false)
		if frac > 0 {
			vector.FillRect(screen, x, y, w*frac, h, colFill, false)
		}
	}

	// Unaffordable-cost pulse flash: fades out over pulseDuration seconds.
	if g.pulseTime > 0 {
		alpha := uint8(200 * g.pulseTime / pulseDuration)
		colPulse := color.RGBA{R: 220, G: 60, B: 60, A: alpha}
		switch g.pulseTarget {
		case 1:
			pr := g.hud.buildCampBtn.GetWidget().Rect
			vector.StrokeRect(screen,
				float32(pr.Min.X)-2, float32(pr.Min.Y)-2,
				float32(pr.Max.X-pr.Min.X)+4, float32(pr.Max.Y-pr.Min.Y)+4,
				2, colPulse, false)
		case 2:
			pr := g.hud.buyWorkerBtn.GetWidget().Rect
			vector.StrokeRect(screen,
				float32(pr.Min.X)-2, float32(pr.Min.Y)-2,
				float32(pr.Max.X-pr.Min.X)+4, float32(pr.Max.Y-pr.Min.Y)+4,
				2, colPulse, false)
		}
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}
