package game

import (
	"fmt"
	"image/color"
	"os"

	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const dt = 1.0 / 60.0
const autoSavePeriod = 10.0 // seconds between autosaves

const (
	holdNone      = 0
	holdBuyWorker = 1
)

// Game is the root ebiten game object.
type Game struct {
	world        *World
	scene        *ebiten.Image // low-res 320×240 canvas; scaled up to the window
	ui           *ebitenui.UI
	hud          *HUD
	placing      bool              // true while waiting for player to click a camp location
	freePlacing  bool              // debug-only: next placement ignores camp cost
	preview      *placementPreview // current frame's placement preview; nil when not placing
	showMenu     bool              // true when the settings overlay is open
	debug        bool              // F3 — verbose debug panel; session-only, not persisted
	debugSection int               // selected debug panel section; session-only
	pulseTime    float64           // seconds remaining on the unaffordable-cost flash
	pulseTarget  int               // which button pulses: 0=none, 1=build, 2=worker
	rejectTime   float64           // seconds remaining on invalid placement feedback
	screenW      int               // current screen dimensions, updated each Draw()
	screenH      int
	hudScale              int     // integer view scale at last HUD build; triggers rebuild on change
	hudDigits             int     // digit count of wood at last HUD build; triggers rebuild on grow
	saveTimer             float64 // counts down to next autosave
	nurtureConfirmLeft        float64 // seconds remaining on the nurture success flash
	nurtureAttentionCooldown  float64 // counts down to next attention pulse fire
	nurtureAttentionPulseLeft float64 // seconds remaining on the current attention flash
	holdAction                int     // current held purchase action (holdNone, holdNurture, …)
	holdTimer                 float64 // counts down to next repeat fire
	holdDuration              float64 // total seconds the current hold has been active
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
		world:                    w,
		scene:                    ebiten.NewImage(virtW, virtH),
		hudScale:                 initialScale,
		hudDigits:                woodDigits(w.Economy.Wood),
		saveTimer:                autoSavePeriod,
		nurtureAttentionCooldown: nurtureAttentionInterval,
	}
	hud, ui, err := buildHUD(g, initialScale)
	if err != nil {
		return nil, err
	}
	g.hud = hud
	g.ui = ui
	return g, nil
}

// woodDigits returns the number of decimal digits in the integer part of x,
// used to detect when the resource display needs more horizontal space.
func woodDigits(x float64) int {
	if x < 1 {
		return 1
	}
	n := 1
	for v := int(x); v >= 10; v /= 10 {
		n++
	}
	return n
}

// activateHold fires action once immediately and, if it succeeds, starts the
// hold-to-repeat timer. Called from button PressedHandlers.
func (g *Game) activateHold(action int) {
	if g.tryHoldAction(action) {
		g.holdAction = action
		g.holdTimer = holdInitialDelay
	}
}

// tryNurture attempts a single Nurture activation and shows success/fail feedback.
func (g *Game) tryNurture() {
	if nurtureField(g.world, KindWood) {
		g.nurtureConfirmLeft = nurtureConfirmDuration
	} else {
		g.pulseTime = pulseDuration
		g.pulseTarget = 3
	}
}

// tryHoldAction executes the purchase action and returns true on success.
func (g *Game) tryHoldAction(action int) bool {
	switch action {
	case holdBuyWorker:
		if buyWorker(g.world) {
			return true
		}
		if g.world.ResourceDiscovered && g.world.Economy.Wood < WorkerCost(g.world) {
			g.pulseTime = pulseDuration
			g.pulseTarget = 2
		}
	}
	return false
}

func (g *Game) Update() error {
	if ebiten.IsWindowBeingClosed() {
		_ = Save(g.world)
		os.Exit(0)
	}
	g.handleGlobalInput()
	if g.showMenu {
		g.preview = nil
		g.hud.Refresh(g.world, g.placing, g.debug, g.debugSection, g.preview, g.showMenu)
		g.ui.Update()
		return nil
	}

	g.ui.Update()
	g.handleInput()

	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.holdAction = holdNone
		g.holdDuration = 0
	}
	if g.holdAction != holdNone {
		g.holdDuration += dt
		g.holdTimer -= dt
		if g.holdTimer <= 0 {
			if g.tryHoldAction(g.holdAction) {
				interval := holdRepeatInterval - g.holdDuration*holdRampRate
				if interval < holdMinInterval {
					interval = holdMinInterval
				}
				g.holdTimer = interval
			} else {
				g.holdAction = holdNone
				g.holdDuration = 0
			}
		}
	}

	Step(g.world, dt)
	if g.pulseTime > 0 {
		g.pulseTime -= dt
	}
	if g.nurtureConfirmLeft > 0 {
		g.nurtureConfirmLeft -= dt
	}
	if g.nurtureAttentionPulseLeft > 0 {
		g.nurtureAttentionPulseLeft -= dt
	}
	g.nurtureAttentionCooldown -= dt
	if g.nurtureAttentionCooldown <= 0 {
		g.nurtureAttentionCooldown = nurtureAttentionInterval
		if nurtureAttentionActive(g.world, KindWood) {
			g.nurtureAttentionPulseLeft = nurtureAttentionPulseDur
		}
	}
	if g.rejectTime > 0 {
		g.rejectTime -= dt
		if g.rejectTime < 0 {
			g.rejectTime = 0
		}
	}
	g.saveTimer -= dt
	if g.saveTimer <= 0 {
		_ = Save(g.world)
		g.saveTimer = autoSavePeriod
	}
	g.preview = g.curPlacementPreview()
	g.hud.Refresh(g.world, g.placing, g.debug, g.debugSection, g.preview, g.showMenu)
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.screenW, g.screenH = screen.Bounds().Dx(), screen.Bounds().Dy()

	rebuildHUD := false
	if newScale := g.intScale(); newScale != g.hudScale {
		g.hudScale = newScale
		rebuildHUD = true
	}
	if d := woodDigits(g.world.Economy.Wood); d > g.hudDigits {
		g.hudDigits = d
		rebuildHUD = true
	}
	if rebuildHUD {
		if hud, ui, err := buildHUD(g, g.hudScale); err == nil {
			g.hud = hud
			g.ui = ui
			g.hud.Refresh(g.world, g.placing, g.debug, g.debugSection, g.preview, g.showMenu)
		}
	}

	if g.showMenu {
		screen.Fill(colBackground)
		g.ui.Draw(screen)
		return
	}

	DrawWorld(g.scene, g.world, g.preview, g.debug)

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

// drawOverlay draws HUD affordances on top of EbitenUI in native screen space.
// Widget Rects are valid here because ui.Draw already laid them out.
func (g *Game) drawOverlay(screen *ebiten.Image) {
	if g.showMenu || g.debug {
		return
	}

	g.drawAffordabilityProgress(screen)

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
			frac = float32(f.EXP / f.Cap)
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
		if g.world.growthCue.GaugeAfterglow > 0 {
			t := g.world.growthCue.GaugeAfterglow / growthGaugeAfterglowTime
			alpha := uint8(120 * t)
			vector.FillRect(screen, x, y, w, h, color.RGBA{R: 70, G: 210, B: 90, A: alpha}, false)
		}
		if g.world.growthCue.GaugeRelease > 0 {
			t := g.world.growthCue.GaugeRelease / growthGaugeReleaseTime
			alpha := uint8(210 * t)
			vector.StrokeRect(screen, x-1, y-1, w+2, h+2, 1, color.RGBA{R: 140, G: 255, B: 130, A: alpha}, false)
		}

		sr := g.hud.resourceSquare.GetWidget().Rect
		srx := float32(sr.Min.X)
		sry := float32(sr.Min.Y)
		srw := float32(sr.Max.X - sr.Min.X)
		srh := float32(sr.Max.Y - sr.Min.Y)
		charges := f.NurtureCharges
		if charges > 0 {
			// Active border: soft green stroke around the square.
			vector.StrokeRect(screen, srx-1, sry-1, srw+2, srh+2, 2,
				color.RGBA{R: 120, G: 255, B: 150, A: 200}, false)
			// Charge badge: count centered on the square.
			chargeStr := fmt.Sprintf("%d", charges)
			tw, th := text.Measure(chargeStr, g.hud.face, 0)
			op := &text.DrawOptions{}
			op.GeoM.Translate(float64(srx)+float64(srw)/2-tw/2, float64(sry)+float64(srh)/2-th/2)
			op.ColorScale.Scale(120.0/255.0, 1.0, 150.0/255.0, 1.0)
			text.Draw(screen, chargeStr, g.hud.face, op)
		} else {
			// Attention and confirm flashes only render when no charges are active
			// so they don't obscure the charge badge.
			if g.nurtureAttentionPulseLeft > 0 {
				t := float32(g.nurtureAttentionPulseLeft / nurtureAttentionPulseDur)
				alpha := uint8(90 * t)
				vector.FillRect(screen, srx, sry, srw, srh, color.RGBA{R: 120, G: 255, B: 150, A: alpha}, false)
			}
			if g.nurtureConfirmLeft > 0 {
				t := float32(g.nurtureConfirmLeft / nurtureConfirmDuration)
				alpha := uint8(210 * t)
				vector.FillRect(screen, srx, sry, srw, srh, color.RGBA{R: 200, G: 255, B: 210, A: alpha}, false)
			}
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
		case 3:
			pr := g.hud.resourceSquare.GetWidget().Rect
			vector.StrokeRect(screen,
				float32(pr.Min.X)-2, float32(pr.Min.Y)-2,
				float32(pr.Max.X-pr.Min.X)+4, float32(pr.Max.Y-pr.Min.Y)+4,
				2, colPulse, false)
		}
	}
}

// drawAffordabilityProgress fills disabled economy buttons from bottom to top
// as the player approaches the cost. Non-economy locks remain fully dim.
func (g *Game) drawAffordabilityProgress(screen *ebiten.Image) {
	hasTownHall := len(g.world.Buildings) > 0
	discovered := g.world.ResourceDiscovered
	if hasTownHall && discovered {
		drawButtonProgress(screen, g.hud.buildCampBtn, affordabilityFrac(g.world.Economy.Wood, CampCost(g.world)),
			color.RGBA{R: 140, G: 90, B: 50, A: 230})
	}
	if hasTownHall {
		workerLocked := g.world.Economy.WorkersBought > 0 && !discovered
		if !workerLocked {
			drawButtonProgress(screen, g.hud.buyWorkerBtn, affordabilityFrac(g.world.Economy.Wood, WorkerCost(g.world)),
				color.RGBA{R: 220, G: 200, B: 60, A: 230})
		}
	}
}

func affordabilityFrac(wood, cost float64) float32 {
	if cost <= 0 || wood >= cost {
		return 1
	}
	if wood <= 0 {
		return 0
	}
	return float32(wood / cost)
}

func drawButtonProgress(screen *ebiten.Image, btn *widget.Button, frac float32, col color.RGBA) {
	if frac <= 0 || frac >= 1 || !btn.GetWidget().Disabled {
		return
	}
	r := btn.GetWidget().Rect
	w := float32(r.Max.X - r.Min.X)
	h := float32(r.Max.Y-r.Min.Y) * frac
	if w <= 0 || h <= 0 {
		return
	}
	x := float32(r.Min.X)
	y := float32(r.Max.Y) - h
	vector.FillRect(screen, x, y, w, h, col, false)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}
