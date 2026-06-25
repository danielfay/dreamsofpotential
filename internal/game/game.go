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
	nurtureAttentionCooldown       float64 // counts down to next Nurture attention pulse
	nurtureAttentionPulseLeft      float64 // seconds remaining on the Nurture attention flash
	dockUpgradeAttentionCooldown   float64 // counts down to next dock-upgrade attention pulse
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
	sysAwakenRect sysRect // awaken-echo tray button
	sysReturnRect sysRect // return-to-system button in planet view

	// double-click tracking for system-view planet zoom
	sysDoubleClickPlanet int       // index of last clicked planet (-1 = none)
	sysDoubleClickTime   time.Time // time of that click

	// planet-view selected building (dock tray with upgrade action)
	selectedBuildingID int     // index into w.Buildings; -1 = none
	dockUpgradeRect    sysRect // upgrade button hit-test rect in native screen space
	dockTrayRect       sysRect // entire tray background rect (clicking anywhere dismisses)
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
		nurtureAttentionCooldown:     nurtureAttentionInterval,
		dockUpgradeAttentionCooldown: nurtureAttentionInterval,
		importCh:                 make(chan *World, 1),
		sysDoubleClickPlanet:     -1,
		selectedBuildingID:       -1,
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
		if nurtureField(g.world) {
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
		clearTransientUI(g)
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
			if nurtureAttentionActive(g.world) {
				g.nurtureAttentionPulseLeft = nurtureAttentionPulseDur
			}
		}
		g.dockUpgradeAttentionCooldown -= dt
		if g.dockUpgradeAttentionCooldown <= 0 {
			g.dockUpgradeAttentionCooldown = nurtureAttentionInterval
			if dock := dockUpgradeAttentionDock(g.world); dock != nil {
				activatePulse(g.world, &dock.Pulse)
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

	var bgCol color.RGBA
	switch {
	case g.world.System.View == ViewSystem || g.revealActive:
		bgCol = colSysBackground
	default:
		bgCol = activePlanetPalette(g.world).background
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
		if fp := g.world.Planet.FieldProgress[f.Kind]; fp != nil && fp.Cap > 0 {
			frac = float32(fp.EXP / fp.Cap)
			if frac > 1 {
				frac = 1
			}
		}
		r := g.hud.resourceHUD.GetWidget().Rect
		x := float32(r.Min.X)
		y := float32(r.Max.Y) + 2
		w := float32(r.Max.X - r.Min.X)
		const h = float32(3)
		vector.StrokeRect(screen, x, y, w, h, 1, colWoodGaugeFrame, false)
		if frac > 0 {
			vector.FillRect(screen, x, y, w*frac, h, colWoodGaugeFill, false)
		}
		if g.world.growthCue.GaugeAfterglow > 0 {
			t := g.world.growthCue.GaugeAfterglow / growthGaugeAfterglowTime
			col := colGrowthGaugeAfterglow
			col.A = uint8(120 * t)
			vector.FillRect(screen, x, y, w, h, col, false)
		}
		if g.world.growthCue.GaugeRelease > 0 {
			t := g.world.growthCue.GaugeRelease / growthGaugeReleaseTime
			col := colGrowthGaugeRelease
			col.A = uint8(210 * t)
			vector.StrokeRect(screen, x-1, y-1, w+2, h+2, 1, col, false)
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
				col := colNurtureAttention
				col.A = uint8(220 * t)
				vector.StrokeRect(screen, cx-halfW, cy-halfH, halfW*2, halfH*2, 1.5, col, false)
			}
		}
		if g.nurtureConfirmLeft > 0 {
			t := float32(g.nurtureConfirmLeft / nurtureConfirmDuration)
			col := colNurtureConfirm
			col.A = uint8(210 * t)
			vector.FillRect(screen, srx, sry, srw, srh, col, false)
		}
	}

	// Selected-building dock tray.
	g.drawDockTray(screen)

	// Unaffordable-cost pulse flash: fades out over pulseDuration seconds.
	if g.pulseTime > 0 {
		colPulse := colCostPulse
		colPulse.A = uint8(200 * g.pulseTime / pulseDuration)
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

	// Global resources — top-left, drawn on screen at native resolution.
	if g.world.ResourceDiscovered || g.world.System.Unlocked {
		swSize := float32(8 * scale)
		swX := float32(4 * scale)
		swY := float32(4 * scale)
		rowGap := swSize + float32(3*scale)

		// Wood row.
		woodStr := fmt.Sprintf("%.0f (%s)", g.world.Economy.Wood, systemRateText(abstractIncome(g.world)))
		vector.FillRect(screen, swX, swY, swSize, swSize, colWoodResource, false)
		textX := swX + swSize + float32(3*scale)
		_, textH := text.Measure(woodStr, face, 0)
		drawSysText(screen, woodStr, textX, swY+swSize-float32(textH), colWoodLabel, face)

		// Water row: shown after first water delivery.
		nextRowY := swY + rowGap
		if g.world.Economy.WaterDiscovered {
			waterStr := fmt.Sprintf("%.0f (%s)", g.world.Economy.Water, systemRateText(abstractWaterIncome(g.world)))
			vector.FillRect(screen, swX, nextRowY, swSize, swSize, colSparkle, false)
			_, wTextH := text.Measure(waterStr, face, 0)
			drawSysText(screen, waterStr, textX, nextRowY+swSize-float32(wTextH),
				color.RGBA{R: 100, G: 200, B: 255, A: 220}, face)
			nextRowY += rowGap
		}

		// Earned Potential circles — one per kind, shown only after first award.
		potRow := nextRowY
		potR := float32(4 * scale)
		potCols := []struct {
			kind PotentialKind
			col  color.RGBA
		}{
			{PotentialForest, colForestPotential},
			{PotentialWater, colWaterPotential},
		}
		potX := swX + potR
		for _, pk := range potCols {
			count, earned := g.world.Economy.Potential[pk.kind]
			if !earned {
				continue
			}
			drawPotentialCircle(screen, potX, potRow+potR, potR, pk.col)
			countStr := fmt.Sprintf("%d", count)
			cw, ch := text.Measure(countStr, face, 0)
			drawSysText(screen, countStr, potX+potR+float32(2*scale), potRow+potR-float32(ch)/2, pk.col, face)
			potX += potR*2 + float32(cw) + float32(8*scale)
		}
	}

	// Bottom tray for selected planet.
	sel := g.world.System.Selected
	if sel < 0 || sel >= len(g.world.System.Planets) {
		g.sysEnterRect = sysRect{}
		return
	}
	p := g.world.System.Planets[sel]

	// Tray background.
	const trayVH = float64(20)
	const trayVW = float64(90)
	tw, th := float32(trayVW*scale), float32(trayVH*scale)
	tx := float32(g.screenW)/2 - tw/2
	ty := float32(g.screenH) - th - float32(4*scale)
	vector.FillRect(screen, tx, ty, tw, th, colSysTrayFill, false)
	vector.StrokeRect(screen, tx, ty, tw, th, 1, colSysTrayBorder, false)

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

	// Right side of tray: awaken button for un-awakened echoes, enter button for zoomable planets.
	g.sysEnterRect = sysRect{}
	g.sysAwakenRect = sysRect{}
	btnSize := float32(14 * scale)
	btnX := tx + tw - btnSize - float32(5*scale)
	btnY := ty + (th-btnSize)/2

	if (p.Kind == PlanetEcho || p.Kind == PlanetUnknown) && !p.Awakened {
		// Awaken button: burst/sparkle glyph, enabled or greyed by affordability.
		canAff := canAwaken(g.world, sel)
		fillCol, rimCol, glyphCol := colAwakenFill, colAwakenRim, colAwakenGlyph
		if !canAff {
			fillCol, rimCol, glyphCol = colAwakenFillDim, colAwakenRimDim, colAwakenGlyphDim
		}
		vector.FillRect(screen, btnX, btnY, btnSize, btnSize, fillCol, false)
		vector.StrokeRect(screen, btnX, btnY, btnSize, btnSize, 1, rimCol, false)

		// Four-point burst glyph.
		cx2 := btnX + btnSize/2
		cy2 := btnY + btnSize/2
		ray := float32(4 * scale)
		half := float32(1.5 * scale)
		vector.StrokeLine(screen, cx2-ray, cy2-ray, cx2+ray, cy2+ray, half, glyphCol, false)
		vector.StrokeLine(screen, cx2+ray, cy2-ray, cx2-ray, cy2+ray, half, glyphCol, false)
		vector.FillCircle(screen, cx2, cy2, half+float32(scale), glyphCol, false)

		// Cost labels to the left of the button: one circle+number per required Potential.
		// Items are drawn right-to-left from the button edge; the cost map is iterated in
		// a fixed display order so layout is deterministic.
		type potCostItem struct {
			kind     PotentialKind
			col      color.RGBA
			labelCol color.RGBA
		}
		potOrder := []potCostItem{
			{PotentialWater, colWaterPotential, colWaterPotentialLabel},
			{PotentialForest, colForestPotential, colForestPotentialLabel},
		}
		costStr := "1"
		_, costH := text.Measure(costStr, face, 0)
		costY := btnY + (btnSize-float32(costH))/2
		circR := float32(3 * scale)
		circY := btnY + btnSize/2
		awakenCostMap := planetAwakenCost(g.world, sel)
		curRight := btnX - float32(5*scale) // right edge of next item, working leftward
		for _, pk := range potOrder {
			if awakenCostMap[pk.kind] == 0 {
				continue
			}
			// Draw circle just left of curRight.
			circX := curRight - circR
			drawPotentialCircle(screen, circX, circY, circR, pk.col)
			costW, _ := text.Measure(costStr, face, 0)
			drawSysText(screen, costStr, circX-circR-float32(3*scale)-float32(costW), costY, pk.labelCol, face)
			curRight = circX - circR - float32(costW) - float32(6*scale)
		}
		g.sysAwakenRect = sysRect{x: btnX, y: btnY, w: btnSize, h: btnSize}

	} else if p.zoomable() {
		// Enter button: planet circle glyph.
		vector.FillRect(screen, btnX, btnY, btnSize, btnSize, colEnterFill, false)
		vector.StrokeRect(screen, btnX, btnY, btnSize, btnSize, 1, colEnterRim, false)
		bCx := btnX + btnSize/2
		bCy := btnY + btnSize/2
		bR := float32(4 * scale)
		vector.FillCircle(screen, bCx, bCy, bR, colEnterGlyph, false)
		drawSystemOrbitRing(screen, bCx, bCy, bR+float32(2*scale), 1, colEnterOrbit)
		g.sysEnterRect = sysRect{x: btnX, y: btnY, w: btnSize, h: btnSize}
	}
}

// drawPotentialCircle draws a single Potential token circle centered at (cx, cy).
func drawPotentialCircle(dst *ebiten.Image, cx, cy, r float32, col color.RGBA) {
	vector.FillCircle(dst, cx, cy, r, col, false)
}

func sysSwatchColor(p SystemPlanet) color.RGBA {
	switch p.Kind {
	case PlanetStarting, PlanetEcho:
		return colWoodResource
	case PlanetUnknown:
		if p.Awakened {
			return colSysFrontierSwatch
		}
		return colSysUnknownSwatch
	default:
		return colSysUnknownSwatch
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

// clearTransientUI resets all mid-action player state. Call on any view transition
// so placement, holds, and menu state don't bleed across planet switches.
func clearTransientUI(g *Game) {
	g.placing = false
	g.freePlacing = false
	g.holdAction = holdNone
	g.holdDuration = 0
	g.showMenu = false
	g.selectedBuildingID = -1
	g.dockUpgradeRect = sysRect{}
}

// drawReturnToSystemButton draws and records the top-right return-to-system button.
func (g *Game) drawReturnToSystemButton(screen *ebiten.Image) {
	scale, _, _ := viewGeom(g.screenW, g.screenH)
	sp := float32(scale)
	btnSize := float32(16 * scale)
	btnX := float32(g.screenW) - btnSize - float32(4*scale)
	btnY := float32(3 * scale)

	vector.FillRect(screen, btnX, btnY, btnSize, btnSize, colReturnFill, false)
	vector.StrokeRect(screen, btnX, btnY, btnSize, btnSize, 1, colReturnRim, false)
	// Small planet with outward dots.
	bCx := btnX + btnSize/2
	bCy := btnY + btnSize/2
	bR := float32(4 * sp)
	vector.FillCircle(screen, bCx, bCy, bR, colReturnGlyph, false)
	drawSystemOrbitRing(screen, bCx, bCy, bR+float32(3*sp), 1, colReturnOrbit)
	// Outward tick marks.
	for _, a := range []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2} {
		ox := bCx + (bR+float32(6*sp))*float32(math.Cos(a))
		oy := bCy + (bR+float32(6*sp))*float32(math.Sin(a))
		vector.FillRect(screen, ox-sp/2, oy-sp/2, sp, sp, colReturnDot, false)
	}

	g.sysReturnRect = sysRect{x: btnX, y: btnY, w: btnSize, h: btnSize}
}

// drawDockTray draws a bottom tray when a dock building is selected.
// Shows dock identity on the left and an upgrade action on the right.
// Level 1 docks show the L1→L2 upgrade cost (Wood + Water). Level 2 docks
// show a quiet Level-3 tease (unnamed future resource, not purchasable).
func (g *Game) drawDockTray(screen *ebiten.Image) {
	if g.selectedBuildingID < 0 || g.selectedBuildingID >= len(g.world.Buildings) {
		g.dockUpgradeRect = sysRect{}
		g.dockTrayRect = sysRect{}
		return
	}
	b := g.world.Buildings[g.selectedBuildingID]
	if b.Kind != KindDock {
		g.dockUpgradeRect = sysRect{}
		g.dockTrayRect = sysRect{}
		return
	}

	scale, _, _ := viewGeom(g.screenW, g.screenH)
	face := g.hud.sysface
	if face == nil {
		g.dockUpgradeRect = sysRect{}
		g.dockTrayRect = sysRect{}
		return
	}

	const trayVH = float64(20)
	const trayVW = float64(108)
	tw := float32(trayVW * scale)
	th := float32(trayVH * scale)
	tx := float32(g.screenW)/2 - tw/2
	ty := float32(g.screenH) - th - float32(4*scale)

	vector.FillRect(screen, tx, ty, tw, th, colSysTrayFill, false)
	vector.StrokeRect(screen, tx, ty, tw, th, 1, colSysTrayBorder, false)
	g.dockTrayRect = sysRect{x: tx, y: ty, w: tw, h: th}

	sp := float32(scale)

	// ── Left section: dock identity ───────────────────────────────────────────
	swSize := float32(10 * scale)
	swX := tx + 5*sp
	swY := ty + (th-swSize)/2
	vector.FillRect(screen, swX, swY, swSize, swSize, colDock, false)

	label := "Dock"
	if b.Extension {
		label = "Dock (ext)"
	}
	_, lH := text.Measure(label, face, 0)
	lY := swY + swSize - float32(lH)
	drawSysText(screen, label, swX+swSize+4*sp, lY, colWoodLabel, face)

	// Level label.
	lvlLabel := fmt.Sprintf("L%d", b.Level)
	if b.Level == 0 {
		lvlLabel = "L1"
	}
	_, lvlH := text.Measure(lvlLabel, face, 0)
	lvlY := swY - float32(lvlH) + float32(lvlH)*0.5
	_ = lvlY
	drawSysText(screen, lvlLabel, swX+swSize+4*sp, swY-float32(lvlH), colSysTrayBorder, face)

	// ── Right section: upgrade action ─────────────────────────────────────────
	const actionW = float64(52)
	axLeft := tx + tw - float32(actionW*scale)
	actionH := th - 4*sp
	ayTop := ty + 2*sp

	// Separator line.
	vector.StrokeLine(screen, axLeft-1, ty+2*sp, axLeft-1, ty+th-2*sp, 1, colSysTrayBorder, false)

	if b.Level >= 2 {
		// Level-3 tease: grey square + label, non-interactive.
		g.dockUpgradeRect = sysRect{}
		greyCol := color.RGBA{R: 60, G: 60, B: 80, A: 200}
		qSize := float32(8 * scale)
		qX := axLeft + 4*sp
		qY := ayTop + (actionH-qSize)/2
		vector.FillRect(screen, qX, qY, qSize, qSize, greyCol, false)
		drawSysText(screen, "L3 ???", qX+qSize+3*sp, qY, greyCol, face)
	} else {
		// Level-1 upgrade button: fills from bottom as resources accumulate.
		btnX := axLeft + 2*sp
		btnW := tw - float32(actionW*scale) - 4*sp
		btnH := actionH

		affordable := canUpgradeDock(g.world, b)

		// Dark base.
		vector.FillRect(screen, btnX, ayTop, btnW, btnH, colSysTrayFill, false)

		// Fill from bottom to top based on the scarcer of the two resources.
		woodFrac := affordabilityFrac(g.world.Economy.Wood, dockL2WoodCost)
		waterFrac := affordabilityFrac(g.world.Economy.Water, dockL2WaterCost)
		frac := woodFrac
		if waterFrac < frac {
			frac = waterFrac
		}
		if frac > 0 && frac < 1 {
			fillH := btnH * frac
			vector.FillRect(screen, btnX, ayTop+btnH-fillH, btnW, fillH,
				color.RGBA{R: 30, G: 55, B: 100, A: 200}, false)
		} else if affordable {
			vector.FillRect(screen, btnX, ayTop, btnW, btnH,
				color.RGBA{R: 30, G: 55, B: 100, A: 200}, false)
		}

		borderCol := colSysTrayBorder
		if affordable {
			borderCol = color.RGBA{R: 80, G: 120, B: 200, A: 220}
		}
		vector.StrokeRect(screen, btnX, ayTop, btnW, btnH, 1, borderCol, false)

		// Attention pulse: glow when dock's pulse is active.
		if pulseActive(g.world, b.Pulse) {
			atCol := colNurtureAttention
			atCol.A = 180
			vector.StrokeRect(screen, btnX-1, ayTop-1, btnW+2, btnH+2, 1.5, atCol, false)
		}

		g.dockUpgradeRect = sysRect{x: btnX, y: ayTop, w: btnW, h: btnH}

		// Cost icons: always use full color so they're visible against the dark/fill background.
		iconSize := float32(6 * scale)
		iconY := ayTop + (btnH-iconSize)/2
		cx := btnX + 4*sp

		// Arrow "▲".
		drawSysText(screen, "▲", cx, iconY, colWoodLabel, face)
		_, ah := text.Measure("▲", face, 0)
		cx += float32(ah) + 2*sp

		// Wood cost icon + amount.
		vector.FillRect(screen, cx, iconY, iconSize, iconSize, colWoodResource, false)
		cx += iconSize + 2*sp
		woodStr := fmt.Sprintf("%.0f", dockL2WoodCost)
		drawSysText(screen, woodStr, cx, iconY, colWoodLabel, face)
		ww, _ := text.Measure(woodStr, face, 0)
		cx += float32(ww) + 3*sp

		// Water cost icon + amount.
		vector.FillRect(screen, cx, iconY, iconSize, iconSize, colSparkle, false)
		cx += iconSize + 2*sp
		waterStr := fmt.Sprintf("%.0f", dockL2WaterCost)
		drawSysText(screen, waterStr, cx, iconY, color.RGBA{R: 100, G: 190, B: 240, A: 220}, face)
	}
}
