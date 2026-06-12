package game

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const dt = 1.0 / 60.0
const autoSavePeriod = 10.0 // seconds between autosaves

const (
	holdNone    = 0
	holdNurture = 2
)

// Game is the root ebiten game object.
type Game struct {
	world                     *World
	scene                     *ebiten.Image // low-res 320×240 canvas; scaled up to the window
	ui                        *ebitenui.UI
	hud                       *HUD
	placing                   bool              // true while waiting for player to click a camp location
	freePlacing               bool              // debug-only: next placement ignores camp cost
	preview                   *placementPreview // current frame's placement preview; nil when not placing
	showMenu                  bool              // true when the settings overlay is open
	debug                     bool              // F3 — verbose debug panel; session-only, not persisted
	debugSection              int               // selected debug panel section; session-only
	pulseTime                 float64           // seconds remaining on the unaffordable-cost flash
	pulseTarget               int               // which button pulses: 0=none, 1=build, 2=capacity, 3=nurture
	rejectTime                float64           // seconds remaining on invalid placement feedback
	screenW                   int               // current screen dimensions, updated each Draw()
	screenH                   int
	hudScale                  int         // integer view scale at last HUD build; triggers rebuild on change
	hudDigits                 int         // digit count of wood at last HUD build; triggers rebuild on grow
	saveTimer                 float64     // counts down to next autosave
	nurtureConfirmLeft        float64     // seconds remaining on the nurture success flash
	nurtureAttentionCooldown  float64     // counts down to next attention pulse fire
	nurtureAttentionPulseLeft float64     // seconds remaining on the current attention flash
	holdAction                int         // current held purchase action (holdNone, holdNurture, …)
	holdTimer                 float64     // counts down to next repeat fire
	holdDuration              float64     // total seconds the current hold has been active
	importCh                  chan *World // receives a validated world from an import goroutine
	dialogOpen                atomic.Bool // true while a file dialog is open; blocks UI input

	// reveal state (transient — not saved)
	revealActive  bool
	revealElapsed float64

	// system-view button rects in native screen space (set during drawOverlay; read by handleInput)
	sysEnterRect  sysRect // enter-planet tray button
	sysReturnRect sysRect // return-to-system button in planet view

	// double-click tracking for system-view planet zoom
	sysDoubleClickPlanet int       // index of last clicked planet (-1 = none)
	sysDoubleClickTime   time.Time // time of that click
}

// sysRect is a simple native-space hit-test rectangle.
type sysRect struct{ x, y, w, h float32 }

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
		importCh:                 make(chan *World, 1),
		sysDoubleClickPlanet:     -1,
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

func systemRateText(rate float64) string {
	if math.Abs(rate-math.Round(rate)) < 0.05 {
		return fmt.Sprintf("%.0f/s", rate)
	}
	return fmt.Sprintf("%.1f/s", rate)
}

// activateHold fires action once immediately and, if it succeeds, starts the
// hold-to-repeat timer. Called from button PressedHandlers.
func (g *Game) activateHold(action int) {
	if g.tryHoldAction(action) {
		g.holdAction = action
		g.holdTimer = holdInitialDelay
	}
}

// tryHoldAction executes the purchase action and returns true on success.
func (g *Game) tryHoldAction(action int) bool {
	switch action {
	case holdNurture:
		if nurtureField(g.world, KindWood) {
			g.nurtureConfirmLeft = nurtureConfirmDuration
			return true
		}
		g.pulseTime = pulseDuration
		g.pulseTarget = 3
	}
	return false
}

func (g *Game) Update() error {
	if ebiten.IsWindowBeingClosed() {
		_ = Save(g.world)
		os.Exit(0)
	}

	// Apply any world imported via the file dialog goroutine.
	select {
	case imported := <-g.importCh:
		g.world = imported
		g.placing = false
		g.freePlacing = false
		g.showMenu = false
		_ = Save(g.world)
	default:
	}

	// While a native or web file dialog is open, freeze all UI interaction so
	// the player can't open a second dialog or close the menu mid-operation.
	if g.dialogOpen.Load() {
		return nil
	}

	g.handleGlobalInput()
	if g.showMenu {
		g.preview = nil
		g.hud.Refresh(g.world, g.placing, g.debug, g.debugSection, g.preview, g.showMenu)
		g.ui.Update()
		return nil
	}

	// Reveal: lock out normal input and tick the animation timer.
	if g.revealActive {
		g.revealElapsed += dt
		g.world.Economy.Wood += abstractIncome(g.world) * dt
		if g.revealElapsed >= revealDuration {
			g.revealActive = false
		}
		g.hud.Refresh(g.world, false, g.debug, g.debugSection, nil, g.showMenu)
		return nil
	}

	g.ui.Update()

	if g.world.System.View == ViewSystem {
		g.handleSystemInput()
	} else {
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
	}

	justUnlocked := Tick(g.world, dt)
	if justUnlocked {
		g.revealActive = true
		g.revealElapsed = 0
		g.placing = false
		g.freePlacing = false
		g.holdAction = holdNone
		g.holdDuration = 0
	}

	if g.world.System.View == ViewPlanet {
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

	bgCol := colBackground
	if g.world.System.View == ViewSystem || g.revealActive {
		bgCol = colSysBackground
	}
	screen.Fill(bgCol)

	if g.showMenu {
		g.ui.Draw(screen)
		return
	}

	switch {
	case g.revealActive:
		drawReveal(g.scene, g.world, g.revealElapsed)
	case g.world.System.View == ViewSystem:
		drawSystemView(g.scene, g.world, g.debug)
	default:
		DrawWorld(g.scene, g.world, g.preview, g.debug)
	}

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

	if g.revealActive {
		return // reveal has no overlay yet (system HUD fades in after reveal)
	}

	if g.world.System.View == ViewSystem {
		g.drawSystemOverlay(screen)
		return
	}

	// Planet view: return-to-system button after unlock.
	if g.world.System.Unlocked {
		g.drawReturnToSystemButton(screen)
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
		if g.nurtureAttentionPulseLeft > 0 {
			// Square pulse: a rect expands from the button centre outward, fading as it grows.
			// t=1 at fire, t=0 at end; expand is the inverse so the rect starts small.
			t := float32(g.nurtureAttentionPulseLeft / nurtureAttentionPulseDur)
			expand := 1 - t
			cx := srx + srw/2
			cy := sry + srh/2
			halfW := srw / 2 * expand * 1.35
			halfH := srh / 2 * expand * 1.35
			if halfW > 0.5 {
				alpha := uint8(220 * t)
				col := color.RGBA{R: 120, G: 255, B: 150, A: alpha}
				vector.StrokeRect(screen, cx-halfW, cy-halfH, halfW*2, halfH*2, 1.5, col, false)
			}
		}
		if g.nurtureConfirmLeft > 0 {
			t := float32(g.nurtureConfirmLeft / nurtureConfirmDuration)
			alpha := uint8(210 * t)
			vector.FillRect(screen, srx, sry, srw, srh, color.RGBA{R: 200, G: 255, B: 210, A: alpha}, false)
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
			pr := g.hud.buildTownCapacityBtn.GetWidget().Rect
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
			color.RGBA{R: 100, G: 62, B: 36, A: 230}) // colBuilding, A:230
	}
	if hasTownHall && !townFieldFull(g.world) {
		drawButtonProgress(screen, g.hud.buildTownCapacityBtn,
			affordabilityFrac(g.world.Economy.Wood, townCapacityCost(g.world)),
			color.RGBA{R: 230, G: 145, B: 70, A: 230})
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

// ── System-view overlay (native screen space) ─────────────────────────────────

// drawSystemOverlay draws the system HUD and selected-planet tray.
func (g *Game) drawSystemOverlay(screen *ebiten.Image) {
	scale, _, _ := viewGeom(g.screenW, g.screenH)
	face := g.hud.sysface

	// Global wood amount and rate — top-left, drawn on screen at native resolution.
	if g.world.ResourceDiscovered || g.world.System.Unlocked {
		hudStr := fmt.Sprintf("%.0f (%s)", g.world.Economy.Wood, systemRateText(abstractIncome(g.world)))
		swSize := float32(8 * scale)
		swX := float32(4 * scale)
		swY := float32(4 * scale)
		vector.FillRect(screen, swX, swY, swSize, swSize, colWoodResource, false)
		textX := swX + swSize + float32(3*scale)
		_, textH := text.Measure(hudStr, face, 0)
		textY := swY + swSize - float32(textH)
		drawSysText(screen, hudStr, textX, textY, colWoodLabel, face)
	}

	// Bottom tray for selected planet.
	sel := g.world.System.Selected
	if sel < 0 || sel >= len(g.world.System.Planets) {
		g.sysEnterRect = sysRect{}
		return
	}
	p := g.world.System.Planets[sel]
	if p.Kind == PlanetUnknown {
		g.sysEnterRect = sysRect{}
		return
	}

	// Tray background.
	const trayVH = float64(20)
	const trayVW = float64(90)
	tw, th := float32(trayVW*scale), float32(trayVH*scale)
	tx := float32(g.screenW)/2 - tw/2
	ty := float32(g.screenH) - th - float32(4*scale)
	vector.FillRect(screen, tx, ty, tw, th, color.RGBA{R: 8, G: 8, B: 18, A: 210}, false)
	vector.StrokeRect(screen, tx, ty, tw, th, 1, color.RGBA{R: 60, G: 60, B: 90, A: 200}, false)

	// Planet swatch — small colored square.
	swSize := float32(10 * scale)
	swX := tx + float32(5*scale)
	swY := ty + (th-swSize)/2
	vector.FillRect(screen, swX, swY, swSize, swSize, sysSwatchColor(p), false)

	// Rate text — align the measured text box bottom with the swatch bottom so
	// both elements have the same bottom margin inside the tray.
	rateStr := fmt.Sprintf("%.1f/s", p.AbstractRate)
	_, rateH := text.Measure(rateStr, face, 0)
	rateY := swY + swSize - float32(rateH)
	drawSysText(screen, rateStr, swX+swSize+float32(4*scale), rateY, colWoodLabel, face)

	// Enter-planet button: only for the starting planet.
	g.sysEnterRect = sysRect{}
	if p.Kind == PlanetStarting {
		btnSize := float32(14 * scale)
		btnX := tx + tw - btnSize - float32(5*scale)
		btnY := ty + (th-btnSize)/2
		vector.FillRect(screen, btnX, btnY, btnSize, btnSize, color.RGBA{R: 30, G: 80, B: 50, A: 240}, false)
		vector.StrokeRect(screen, btnX, btnY, btnSize, btnSize, 1, color.RGBA{R: 80, G: 200, B: 100, A: 200}, false)
		bCx := btnX + btnSize/2
		bCy := btnY + btnSize/2
		bR := float32(4 * scale)
		vector.FillCircle(screen, bCx, bCy, bR, color.RGBA{R: 60, G: 160, B: 80, A: 220}, false)
		drawSystemOrbitRing(screen, bCx, bCy, bR+float32(2*scale), 1, color.RGBA{R: 200, G: 240, B: 200, A: 160})
		g.sysEnterRect = sysRect{x: btnX, y: btnY, w: btnSize, h: btnSize}
	}
}

func sysSwatchColor(p SystemPlanet) color.RGBA {
	switch p.Kind {
	case PlanetStarting, PlanetEcho:
		return colWoodResource
	default:
		return color.RGBA{R: 60, G: 60, B: 80, A: 255}
	}
}

func drawSysText(target *ebiten.Image, s string, x, y float32, col color.RGBA, face text.Face) {
	if face == nil {
		return
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(col)
	text.Draw(target, s, face, op)
}

// drawReturnToSystemButton draws and records the top-right return-to-system button.
func (g *Game) drawReturnToSystemButton(screen *ebiten.Image) {
	scale, _, _ := viewGeom(g.screenW, g.screenH)
	sp := float32(scale)
	btnSize := float32(16 * scale)
	btnX := float32(g.screenW) - btnSize - float32(4*scale)
	btnY := float32(3 * scale)

	vector.FillRect(screen, btnX, btnY, btnSize, btnSize, color.RGBA{R: 15, G: 30, B: 40, A: 220}, false)
	vector.StrokeRect(screen, btnX, btnY, btnSize, btnSize, 1, color.RGBA{R: 60, G: 100, B: 140, A: 200}, false)
	// Small planet with outward dots.
	bCx := btnX + btnSize/2
	bCy := btnY + btnSize/2
	bR := float32(4 * sp)
	vector.FillCircle(screen, bCx, bCy, bR, color.RGBA{R: 60, G: 140, B: 180, A: 220}, false)
	drawSystemOrbitRing(screen, bCx, bCy, bR+float32(3*sp), 1, color.RGBA{R: 140, G: 200, B: 220, A: 150})
	// Outward tick marks.
	for _, a := range []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2} {
		ox := bCx + (bR+float32(6*sp))*float32(math.Cos(a))
		oy := bCy + (bR+float32(6*sp))*float32(math.Sin(a))
		vector.FillRect(screen, ox-sp/2, oy-sp/2, sp, sp, color.RGBA{R: 140, G: 200, B: 220, A: 120}, false)
	}

	g.sysReturnRect = sysRect{x: btnX, y: btnY, w: btnSize, h: btnSize}
}
