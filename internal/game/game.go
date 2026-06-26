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
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const dt = 1.0 / 60.0
const autoSavePeriod = 10.0 // seconds between autosaves

const (
	holdNone    = 0
	holdNurture = 2
)

const (
	costPulseWood = 1 << iota
	costPulseWater
	costPulseNurture
)

// Game is the root ebiten game object.
type Game struct {
	world                        *World
	scene                        *ebiten.Image // low-res 320×240 canvas; scaled up to the window
	ui                           *ebitenui.UI
	hud                          *HUD
	placing                      bool              // true while waiting for player to click a camp location
	freePlacing                  bool              // debug-only: next placement ignores camp cost
	preview                      *placementPreview // current frame's placement preview; nil when not placing
	showMenu                     bool              // true when the settings overlay is open
	debug                        bool              // F3 — verbose debug panel; session-only, not persisted
	debugSection                 int               // selected debug panel section; session-only
	pulseTime                    float64           // seconds remaining on the unaffordable-cost flash
	pulseTarget                  int               // costPulse* bitmask for unaffordable-cost flash targets
	rejectTime                   float64           // seconds remaining on invalid placement feedback
	screenW                      int               // current screen dimensions, updated each Draw()
	screenH                      int
	hudScale                     int         // integer view scale at last HUD build; triggers rebuild on change
	hudDigits                    int         // digit count of wood at last HUD build; triggers rebuild on grow
	saveTimer                    float64     // counts down to next autosave
	nurtureConfirmLeft           float64     // seconds remaining on the nurture success flash
	nurtureAttentionCooldown     float64     // counts down to next Nurture attention pulse
	nurtureAttentionPulseLeft    float64     // seconds remaining on the Nurture attention flash
	workerRatioAttentionCooldown float64     // counts down to next worker-ratio attention pulse
	workerRatioAttentionLeft     float64     // seconds remaining on the worker-ratio attention flash
	workerRatioAttentionReady    bool        // previous-frame state for first-available worker-ratio attention
	dockUpgradeAttentionCooldown float64     // counts down to next dock-upgrade attention pulse
	holdAction                   int         // current held purchase action (holdNone, holdNurture, …)
	holdTimer                    float64     // counts down to next repeat fire
	holdDuration                 float64     // total seconds the current hold has been active
	importCh                     chan *World // receives a validated world from an import goroutine
	dialogOpen                   atomic.Bool // true while a file dialog is open; blocks UI input

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

	// planet-view selected building (dock tray or town hall tray)
	selectedBuildingID          int     // index into w.Buildings; -1 = none
	dockUpgradeRect             sysRect // upgrade button hit-test rect in native screen space
	dockTrayRect                sysRect // entire dock tray background rect (clicking anywhere dismisses)
	thCapacityRect              sysRect // capacity button in TH tray
	thTrayRect                  sysRect // entire TH tray background rect
	thCapacityAttentionCooldown float64 // counts down to next TH capacity attention pulse
	thCapacityAttentionLeft     float64 // seconds remaining on the TH first-house attention ripple
	thFirstCapacityBuildable    bool    // previous-frame state for the first house teaching pulse

	// labor focus control overlay
	showFocusControl bool
	focusDraftWater  int     // candidate water-worker count while control is open
	focusCtrlRect    sysRect // entire control background rect
	focusSlotsRect   sysRect // the clickable/draggable worker-slot bar
	focusConfirmRect sysRect
	focusCancelRect  sysRect
	focusDragging    bool
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
		world:                        w,
		scene:                        ebiten.NewImage(virtW, virtH),
		hudScale:                     initialScale,
		hudDigits:                    woodDigits(w.Economy.Wood),
		saveTimer:                    autoSavePeriod,
		nurtureAttentionCooldown:     nurtureAttentionInterval,
		workerRatioAttentionCooldown: nurtureAttentionInterval,
		dockUpgradeAttentionCooldown: nurtureAttentionInterval,
		thCapacityAttentionCooldown:  nurtureAttentionInterval,
		importCh:                     make(chan *World, 1),
		sysDoubleClickPlanet:         -1,
		selectedBuildingID:           -1,
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
		g.flashCostTargets(costPulseNurture)
	}
	return false
}

func (g *Game) flashCostTargets(targets int) {
	if targets == 0 {
		return
	}
	g.pulseTime = pulseDuration
	g.pulseTarget = targets
}

func missingCostTargets(w *World, woodCost, waterCost float64) int {
	targets := 0
	if woodCost > 0 && w.Economy.Wood < woodCost {
		targets |= costPulseWood
	}
	if waterCost > 0 && w.Economy.Water < waterCost {
		targets |= costPulseWater
	}
	return targets
}

func missingPlacementCostTargets(w *World, pv *placementPreview) int {
	switch pv.Kind {
	case KindDock:
		if pv.Extension {
			return missingCostTargets(w, dockExtWoodCost, dockExtWaterCost)
		}
		return missingCostTargets(w, dockShoreCost, 0)
	case KindLoggingCamp:
		return missingCostTargets(w, CampCost(w), 0)
	}
	return 0
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
		// Auto-placement: always in placement mode when in planet view.
		// Debug mode retains manual button control.
		if !g.debug {
			g.placing = true
		}
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
		if g.workerRatioAttentionLeft > 0 {
			g.workerRatioAttentionLeft -= dt
		}
		workerRatioReady := workerRatioAttentionActive(g.world)
		if workerRatioReady && !g.workerRatioAttentionReady {
			g.workerRatioAttentionLeft = nurtureAttentionPulseDur
		}
		g.workerRatioAttentionReady = workerRatioReady
		g.nurtureAttentionCooldown -= dt
		if g.nurtureAttentionCooldown <= 0 {
			g.nurtureAttentionCooldown = nurtureAttentionInterval
			if nurtureAttentionActive(g.world) {
				g.nurtureAttentionPulseLeft = nurtureAttentionPulseDur
			}
		}
		g.workerRatioAttentionCooldown -= dt
		if g.workerRatioAttentionCooldown <= 0 {
			g.workerRatioAttentionCooldown = nurtureAttentionInterval
			if workerRatioReady {
				g.workerRatioAttentionLeft = nurtureAttentionPulseDur
			}
		}
		g.dockUpgradeAttentionCooldown -= dt
		if g.dockUpgradeAttentionCooldown <= 0 {
			g.dockUpgradeAttentionCooldown = nurtureAttentionInterval
			if dock := dockUpgradeAttentionDock(g.world); dock != nil {
				activatePulse(g.world, &dock.Pulse)
			}
		}
		firstCapacityBuildable := firstTownCapacityBuildable(g.world)
		if firstCapacityBuildable && !g.thFirstCapacityBuildable {
			g.fireTownCapacityAttention()
		}
		g.thFirstCapacityBuildable = firstCapacityBuildable
		if g.thCapacityAttentionLeft > 0 {
			g.thCapacityAttentionLeft -= dt
		}
		g.thCapacityAttentionCooldown -= dt
		if g.thCapacityAttentionCooldown <= 0 {
			g.thCapacityAttentionCooldown = nurtureAttentionInterval
			if firstTownCapacityBuildable(g.world) {
				g.fireTownCapacityAttention()
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

func (g *Game) fireTownCapacityAttention() {
	if townHall(g.world) == nil {
		return
	}
	g.thCapacityAttentionLeft = nurtureAttentionPulseDur
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
		if g.world.growthCue.Kind == KindWood && g.world.growthCue.GaugeAfterglow > 0 {
			t := g.world.growthCue.GaugeAfterglow / growthGaugeAfterglowTime
			col := colGrowthGaugeAfterglow
			col.A = uint8(120 * t)
			vector.FillRect(screen, x, y, w, h, col, false)
		}
		if g.world.growthCue.Kind == KindWood && g.world.growthCue.GaugeRelease > 0 {
			t := g.world.growthCue.GaugeRelease / growthGaugeReleaseTime
			col := colGrowthGaugeRelease
			col.A = uint8(210 * t)
			vector.StrokeRect(screen, x-1, y-1, w+2, h+2, 1, col, false)
		}

		// Water gauge bar beneath the water HUD icon+number.
		if g.world.Economy.WaterDiscovered {
			var waterFrac float32
			if fp := g.world.Planet.FieldProgress[KindWater]; fp != nil && fp.Cap > 0 {
				waterFrac = float32(fp.EXP / fp.Cap)
				if waterFrac > 1 {
					waterFrac = 1
				}
			}
			wr := g.hud.waterHUD.GetWidget().Rect
			wx := float32(wr.Min.X)
			wy := float32(wr.Max.Y) + 2
			ww := float32(wr.Max.X - wr.Min.X)
			vector.StrokeRect(screen, wx, wy, ww, h, 1, colWaterGaugeFrame, false)
			if waterFrac > 0 {
				vector.FillRect(screen, wx, wy, ww*waterFrac, h, colWaterGaugeFill, false)
			}
			if g.world.growthCue.Kind == KindWater && g.world.growthCue.GaugeAfterglow > 0 {
				t := g.world.growthCue.GaugeAfterglow / growthGaugeAfterglowTime
				col := colGrowthGaugeAfterglow
				col.A = uint8(120 * t)
				vector.FillRect(screen, wx, wy, ww, h, col, false)
			}
			if g.world.growthCue.Kind == KindWater && g.world.growthCue.GaugeRelease > 0 {
				t := g.world.growthCue.GaugeRelease / growthGaugeReleaseTime
				col := colGrowthGaugeRelease
				col.A = uint8(210 * t)
				vector.StrokeRect(screen, wx-1, wy-1, ww+2, h+2, 1, col, false)
			}
		}

		sr := g.hud.resourceSquare.GetWidget().Rect
		srx := float32(sr.Min.X)
		sry := float32(sr.Min.Y)
		srw := float32(sr.Max.X - sr.Min.X)
		srh := float32(sr.Max.Y - sr.Min.Y)
		if g.nurtureAttentionPulseLeft > 0 {
			drawAttentionRipple(screen, srx+srw/2, sry+srh/2, srw, srh,
				g.nurtureAttentionPulseLeft, nurtureAttentionPulseDur, colNurtureAttention, 0)
		}
		if g.nurtureConfirmLeft > 0 {
			t := float32(g.nurtureConfirmLeft / nurtureConfirmDuration)
			col := colNurtureConfirm
			col.A = uint8(210 * t)
			vector.FillRect(screen, srx, sry, srw, srh, col, false)
		}

		if g.workerRatioAttentionLeft > 0 && g.hud.workerSquare != nil {
			wr := g.hud.workerSquare.GetWidget().Rect
			wrx := float32(wr.Min.X)
			wry := float32(wr.Min.Y)
			wrw := float32(wr.Max.X - wr.Min.X)
			wrh := float32(wr.Max.Y - wr.Min.Y)
			drawAttentionRipple(screen, wrx+wrw/2, wry+wrh/2, wrw, wrh,
				g.workerRatioAttentionLeft, nurtureAttentionPulseDur, colNurtureAttention, 0)
		}
	}

	g.drawTownHallAttention(screen)

	// Selected-building trays.
	g.drawDockTray(screen)
	g.drawTownHallTray(screen)

	// Worker HUD slider icon + colored breakdown (shown when focus is active).
	g.drawWorkerHUDOverlay(screen)

	// Labor focus control overlay.
	g.drawFocusControl(screen)

	// Unaffordable-cost pulse flash: fades out over pulseDuration seconds.
	if g.pulseTime > 0 {
		colPulse := colCostPulse
		colPulse.A = uint8(200 * g.pulseTime / pulseDuration)
		if g.pulseTarget&costPulseWood != 0 && g.hud.resourceHUD != nil {
			pr := g.hud.resourceHUD.GetWidget().Rect
			vector.StrokeRect(screen,
				float32(pr.Min.X)-2, float32(pr.Min.Y)-2,
				float32(pr.Max.X-pr.Min.X)+4, float32(pr.Max.Y-pr.Min.Y)+4,
				2, colPulse, false)
		}
		if g.pulseTarget&costPulseWater != 0 && g.hud.waterHUD != nil {
			pr := g.hud.waterHUD.GetWidget().Rect
			vector.StrokeRect(screen,
				float32(pr.Min.X)-2, float32(pr.Min.Y)-2,
				float32(pr.Max.X-pr.Min.X)+4, float32(pr.Max.Y-pr.Min.Y)+4,
				2, colPulse, false)
		}
		if g.pulseTarget&costPulseNurture != 0 {
			pr := g.hud.resourceSquare.GetWidget().Rect
			vector.StrokeRect(screen,
				float32(pr.Min.X)-2, float32(pr.Min.Y)-2,
				float32(pr.Max.X-pr.Min.X)+4, float32(pr.Max.Y-pr.Min.Y)+4,
				2, colPulse, false)
		}
	}
}

// drawAffordabilityProgress was used to fill disabled HUD buttons from bottom to top.
// Camp and capacity buttons have moved to the world (auto-placement) and TH tray respectively,
// so this is now a no-op retained for potential future use.
func (g *Game) drawAffordabilityProgress(_ *ebiten.Image) {}

func affordabilityFrac(wood, cost float64) float32 {
	if cost <= 0 || wood >= cost {
		return 1
	}
	if wood <= 0 {
		return 0
	}
	return float32(wood / cost)
}

func (g *Game) worldToScreen(v Vec) (float32, float32) {
	scale, offX, offY := viewGeom(g.screenW, g.screenH)
	return float32(offX + v.X*scale), float32(offY + v.Y*scale)
}

func (g *Game) drawTownHallAttention(screen *ebiten.Image) {
	if g.thCapacityAttentionLeft <= 0 || g.world.System.View != ViewPlanet {
		return
	}
	th := townHall(g.world)
	if th == nil {
		return
	}
	scale, _, _ := viewGeom(g.screenW, g.screenH)
	center := insetPoint(g.world.Planet, th.Angle, float64(townHallBldInset))
	cx, cy := g.worldToScreen(center)
	baseW := float32(townHallBldHalfW*2+6) * float32(scale)
	baseH := float32(townHallBldHalfH*2+6) * float32(scale)
	drawAttentionRipple(screen, cx, cy, baseW, baseH,
		g.thCapacityAttentionLeft, nurtureAttentionPulseDur, colTownHall, 0.25)
}

func drawAttentionRipple(screen *ebiten.Image, cx, cy, baseW, baseH float32, timeLeft, duration float64, baseColor color.RGBA, startScale float32) {
	if timeLeft <= 0 || duration <= 0 {
		return
	}
	t := float32(timeLeft / duration)
	expand := 1 - t
	halfW := baseW / 2 * (startScale + expand*1.35)
	halfH := baseH / 2 * (startScale + expand*1.35)
	if halfW <= 0.5 || halfH <= 0.5 {
		return
	}
	col := baseColor
	col.A = uint8(220 * t)
	vector.StrokeRect(screen, cx-halfW, cy-halfH, halfW*2, halfH*2, 1.5, col, false)
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

// sysTrayRateSpec describes one resource row in the system planet tray.
// Add an entry here to display any new resource — the tray loop handles the rest.
type sysTrayRateSpec struct {
	getRate func(SystemPlanet) float64
	textCol color.RGBA
}

var sysTrayRateSpecs = []sysTrayRateSpec{
	{func(p SystemPlanet) float64 { return p.AbstractRate }, colWoodLabel},
	{func(p SystemPlanet) float64 { return p.AbstractWaterRate }, color.RGBA{R: 100, G: 200, B: 255, A: 220}},
}

// drawSystemOverlay draws the system HUD and selected-planet tray.
func (g *Game) drawSystemOverlay(screen *ebiten.Image) {
	scale, _, _ := viewGeom(g.screenW, g.screenH)
	face := g.hud.sysface
	systemHUD := func(base float64) float32 {
		return scaledHUDFloat(scale, systemBottomHUDScale, base)
	}
	systemTopHUD := func(base float64) float32 {
		return scaledHUDFloat(scale, systemTopHUDScale, base)
	}

	// Global resources — full-width top band. Square resources occupy the top
	// row; matching Potential circles sit beneath them by resource colour/type.
	if g.world.ResourceDiscovered || g.world.System.Unlocked {
		type systemResourceColumn struct {
			showSquare    bool
			squareText    string
			squareCol     color.RGBA
			squareTextCol color.RGBA
			showCircle    bool
			circleText    string
			circleCol     color.RGBA
			circleTextCol color.RGBA
			width         float32
		}

		squareSize := systemTopHUD(systemTopHUDSquareBase)
		circleR := systemTopHUD(systemTopHUDCircleBase)
		textGap := systemTopHUD(bottomHUDTextGapBase)
		columnGap := systemTopHUD(systemTopHUDColumnGapBase)

		mkColumn := func(showSquare bool, squareText string, squareCol, squareTextCol color.RGBA, potKind PotentialKind, circleCol, circleTextCol color.RGBA) systemResourceColumn {
			count, earned := g.world.Economy.Potential[potKind]
			col := systemResourceColumn{
				showSquare:    showSquare,
				squareText:    squareText,
				squareCol:     squareCol,
				squareTextCol: squareTextCol,
				showCircle:    earned,
				circleText:    fmt.Sprintf("%d", int(math.Floor(count))),
				circleCol:     circleCol,
				circleTextCol: circleTextCol,
			}
			if showSquare {
				tw, _ := text.Measure(squareText, face, 0)
				col.width = squareSize + textGap + float32(tw)
			}
			if earned {
				tw, _ := text.Measure(col.circleText, face, 0)
				w := circleR*2 + textGap + float32(tw)
				if w > col.width {
					col.width = w
				}
			}
			return col
		}

		columns := []systemResourceColumn{
			mkColumn(
				g.world.ResourceDiscovered || g.world.System.Unlocked,
				fmt.Sprintf("%.0f (%s)", g.world.Economy.Wood, systemRateText(abstractIncome(g.world))),
				colWoodResource, colWoodLabel,
				PotentialForest, colForestPotential, colForestPotentialLabel,
			),
			mkColumn(
				g.world.Economy.WaterDiscovered,
				fmt.Sprintf("%.0f (%s)", g.world.Economy.Water, systemRateText(abstractWaterIncome(g.world))),
				colSparkle, color.RGBA{R: 100, G: 200, B: 255, A: 220},
				PotentialWater, colWaterPotential, colWaterPotentialLabel,
			),
		}

		var visibleCols []systemResourceColumn
		totalW := float32(0)
		for _, col := range columns {
			if !col.showSquare && !col.showCircle {
				continue
			}
			if len(visibleCols) > 0 {
				totalW += columnGap
			}
			totalW += col.width
			visibleCols = append(visibleCols, col)
		}

		if len(visibleCols) > 0 {
			h := systemTopHUD(systemTopHUDHeightBase)
			x := float32(0)
			y := float32(0)
			w := float32(g.screenW)
			vector.FillRect(screen, x, y, w, h, colSysTrayFill, false)
			vector.StrokeLine(screen, x, h, x+w, h, 1, colSysTrayBorder, false)

			paddingY := systemTopHUD(systemTopHUDPaddingVBase)
			rowGap := systemTopHUD(systemTopHUDRowGapBase)
			topY := paddingY
			bottomY := topY + squareSize + rowGap
			cx := float32(g.screenW)/2 - totalW/2
			for i, col := range visibleCols {
				if i > 0 {
					cx += columnGap
				}
				if col.showSquare {
					rowW := squareSize
					if col.squareText != "" {
						tw, _ := text.Measure(col.squareText, face, 0)
						rowW += textGap + float32(tw)
					}
					rowX := cx + (col.width-rowW)/2
					vector.FillRect(screen, rowX, topY, squareSize, squareSize, col.squareCol, false)
					if col.squareText != "" {
						_, th := text.Measure(col.squareText, face, 0)
						drawSysText(screen, col.squareText, rowX+squareSize+textGap, topY+squareSize-float32(th), col.squareTextCol, face)
					}
				}
				if col.showCircle {
					rowW := circleR * 2
					if col.circleText != "" {
						tw, _ := text.Measure(col.circleText, face, 0)
						rowW += textGap + float32(tw)
					}
					rowX := cx + (col.width-rowW)/2
					circleY := bottomY + circleR
					drawPotentialCircle(screen, rowX+circleR, circleY, circleR, col.circleCol)
					if col.circleText != "" {
						_, th := text.Measure(col.circleText, face, 0)
						drawSysText(screen, col.circleText, rowX+circleR*2+textGap, circleY-float32(th)/2, col.circleTextCol, face)
					}
				}
				cx += col.width
			}
		}
	}

	// Bottom tray for selected planet.
	sel := g.world.System.Selected
	if sel < 0 || sel >= len(g.world.System.Planets) {
		g.sysEnterRect = sysRect{}
		g.sysAwakenRect = sysRect{}
		return
	}
	p := g.world.System.Planets[sel]

	// Tray background.
	th := systemHUD(bottomHUDHeightBase)
	tx := float32(0)
	tw := float32(g.screenW)
	ty := float32(g.screenH) - th
	vector.FillRect(screen, tx, ty, tw, th, colSysTrayFill, false)
	vector.StrokeLine(screen, tx, ty, tx+tw, ty, 1, colSysTrayBorder, false)

	// Planet swatch — small colored square.
	swSize := systemHUD(bottomHUDSwatchBase)
	swY := ty + (th-swSize)/2

	// Resource rates — draw each non-zero rate as coloured text, left-to-right.
	// Text bottom is aligned with the swatch bottom for a consistent baseline.
	_, rateH := text.Measure("0", face, 0)
	type rateItem struct {
		label   string
		textCol color.RGBA
		width   float32
	}
	var rateItems []rateItem
	for _, spec := range sysTrayRateSpecs {
		rate := spec.getRate(p)
		if rate <= 0 {
			continue
		}
		rateStr := fmt.Sprintf("%.1f/s", rate)
		rw, _ := text.Measure(rateStr, face, 0)
		rateItems = append(rateItems, rateItem{
			label:   rateStr,
			textCol: spec.textCol,
			width:   float32(rw),
		})
	}
	if len(rateItems) == 0 {
		// Newly awakened planet with no measured rate yet — show wood placeholder.
		placeholder := "-.--/s"
		pw, _ := text.Measure(placeholder, face, 0)
		rateItems = append(rateItems, rateItem{
			label:   placeholder,
			textCol: colWoodLabel,
			width:   float32(pw),
		})
	}

	g.sysEnterRect = sysRect{}
	g.sysAwakenRect = sysRect{}
	btnSize := systemHUD(bottomHUDButtonBase)
	btnY := ty + (th-btnSize)/2
	showAwaken := (p.Kind == PlanetEcho || p.Kind == PlanetUnknown) && !p.Awakened
	showEnter := !showAwaken && p.zoomable()

	type potCostItem struct {
		kind     PotentialKind
		col      color.RGBA
		labelCol color.RGBA
	}
	potOrder := []potCostItem{
		{PotentialWater, colWaterPotential, colWaterPotentialLabel},
		{PotentialForest, colForestPotential, colForestPotentialLabel},
	}
	awakenCostMap := planetAwakenCost(g.world, sel)
	costStr := "1"
	costW, costH := text.Measure(costStr, face, 0)
	circR := systemHUD(bottomHUDCircleBase)
	costItemGap := systemHUD(bottomHUDCostGapBase)
	costWidth := float32(0)
	if showAwaken {
		for _, pk := range potOrder {
			if awakenCostMap[pk.kind] == 0 {
				continue
			}
			if costWidth > 0 {
				costWidth += costItemGap
			}
			costWidth += float32(costW) + systemHUD(3) + circR*2
		}
	}

	rateGap := systemHUD(bottomHUDRateGapBase)
	contentGap := systemHUD(bottomHUDContentGapBase)
	totalW := swSize
	if len(rateItems) > 0 {
		totalW += contentGap
		for i, item := range rateItems {
			if i > 0 {
				totalW += rateGap
			}
			totalW += item.width
		}
	}
	if costWidth > 0 {
		totalW += contentGap + costWidth
	}
	if showAwaken || showEnter {
		totalW += contentGap + btnSize
	}

	cx := float32(g.screenW)/2 - totalW/2
	swX := cx
	drawSystemTrayPlanetSwatch(screen, p, swX, swY, swSize)
	cx += swSize + contentGap

	rateY := swY + swSize - float32(rateH)
	for i, item := range rateItems {
		if i > 0 {
			cx += rateGap
		}
		drawSysText(screen, item.label, cx, rateY, item.textCol, face)
		cx += item.width
	}
	cx += contentGap

	if showAwaken && costWidth > 0 {
		costY := btnY + (btnSize-float32(costH))/2
		circY := btnY + btnSize/2
		drewCost := false
		for _, pk := range potOrder {
			if awakenCostMap[pk.kind] == 0 {
				continue
			}
			if drewCost {
				cx += costItemGap
			}
			drawSysText(screen, costStr, cx, costY, pk.labelCol, face)
			cx += float32(costW) + systemHUD(3) + circR
			drawPotentialCircle(screen, cx, circY, circR, pk.col)
			cx += circR
			drewCost = true
		}
		cx += contentGap
	}

	btnX := cx

	if showAwaken {
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
		ray := systemHUD(4)
		half := systemHUD(1.5)
		vector.StrokeLine(screen, cx2-ray, cy2-ray, cx2+ray, cy2+ray, half, glyphCol, false)
		vector.StrokeLine(screen, cx2+ray, cy2-ray, cx2-ray, cy2+ray, half, glyphCol, false)
		vector.FillCircle(screen, cx2, cy2, half+float32(scale), glyphCol, false)
		g.sysAwakenRect = sysRect{x: btnX, y: btnY, w: btnSize, h: btnSize}

	} else if showEnter {
		// Enter button: planet circle glyph.
		vector.FillRect(screen, btnX, btnY, btnSize, btnSize, colEnterFill, false)
		vector.StrokeRect(screen, btnX, btnY, btnSize, btnSize, 1, colEnterRim, false)
		bCx := btnX + btnSize/2
		bCy := btnY + btnSize/2
		bR := systemHUD(4)
		vector.FillCircle(screen, bCx, bCy, bR, colEnterGlyph, false)
		drawSystemOrbitRing(screen, bCx, bCy, bR+systemHUD(2), 1, colEnterOrbit)
		g.sysEnterRect = sysRect{x: btnX, y: btnY, w: btnSize, h: btnSize}
	}
}

// drawPotentialCircle draws a single Potential token circle centered at (cx, cy).
func drawPotentialCircle(dst *ebiten.Image, cx, cy, r float32, col color.RGBA) {
	vector.FillCircle(dst, cx, cy, r, col, false)
}

func drawSystemTrayPlanetSwatch(screen *ebiten.Image, p SystemPlanet, x, y, size float32) {
	if systemPlanetHybridSwatch(p) {
		halfW := size / 2
		vector.FillRect(screen, x, y, halfW, size, colWoodResource, false)
		vector.FillRect(screen, x+halfW, y, size-halfW, size, colWaterPotential, false)
		return
	}
	vector.FillRect(screen, x, y, size, size, sysSwatchColor(p), false)
}

func systemPlanetHybridSwatch(p SystemPlanet) bool {
	return p.AbstractRate > 0 && p.AbstractWaterRate > 0
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

func workerRatioAttentionActive(w *World) bool {
	return !w.WorkerRatioSeen && laborFocusControlAvailable(w)
}

// clearTransientUI resets all mid-action player state. Call on any view transition
// so placement, holds, and menu state don't bleed across planet switches.
func clearTransientUI(g *Game) {
	g.placing = false
	g.freePlacing = false
	g.holdAction = holdNone
	g.holdDuration = 0
	g.showMenu = false
	g.showFocusControl = false
	g.closeBuildingTray()
	g.workerRatioAttentionReady = false
	g.thCapacityAttentionLeft = 0
	g.thFirstCapacityBuildable = false
}

func (g *Game) closeBuildingTray() {
	g.selectedBuildingID = -1
	g.dockUpgradeRect = sysRect{}
	g.dockTrayRect = sysRect{}
	g.thCapacityRect = sysRect{}
	g.thTrayRect = sysRect{}
}

// openFocusControl opens the labor focus control, initialising the draft split
// from the current LaborFocus (or a 1:rest default if none has been set).
func (g *Game) openFocusControl() {
	total := len(g.world.Workers)
	if total == 0 {
		return
	}
	if !laborFocusControlAvailable(g.world) {
		return
	}
	g.world.WorkerRatioSeen = true
	g.workerRatioAttentionLeft = 0
	g.workerRatioAttentionReady = false
	if w, ok := g.world.LaborFocus[KindWater]; ok {
		g.focusDraftWater = w
	} else {
		g.focusDraftWater = 1
	}
	g.showFocusControl = true
	g.focusDragging = false
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
// Level 1 docks show L1→L2 affordability via fill progress. Level 2 docks show
// a quiet Level-3 tease (unnamed future resource, not purchasable).
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
	selectedHUD := func(base float64) float32 {
		return scaledHUDFloat(scale, selectedBuildingHUDScale, base)
	}

	th := selectedHUD(bottomHUDHeightBase)
	tx := float32(0)
	tw := float32(g.screenW)
	ty := float32(g.screenH) - th

	vector.FillRect(screen, tx, ty, tw, th, colSysTrayFill, false)
	vector.StrokeLine(screen, tx, ty, tx+tw, ty, 1, colSysTrayBorder, false)
	g.dockTrayRect = sysRect{x: tx, y: ty, w: tw, h: th}

	sp := selectedHUD(1)

	// ── Centered row: dock identity + action ──────────────────────────────────
	swSize := selectedHUD(bottomHUDSwatchBase)
	swY := ty + (th-swSize)/2

	// Level pips: N upward triangles (1 = L1, 2 = L2).
	level := b.Level
	if level == 0 {
		level = 1
	}
	pipHalfW := selectedHUD(3)
	pipGap := selectedHUD(2)
	pipCY := ty + th/2
	pipsW := float32(level)*(pipHalfW*2) + float32(level-1)*pipGap

	actionWpx := selectedHUD(selectedHUDActionWidthBase)
	actionInset := selectedHUD(selectedHUDActionInsetBase)
	actionH := th - actionInset*2
	ayTop := ty + actionInset
	contentGap := selectedHUD(selectedHUDContentGapBase)
	identityGap := selectedHUD(selectedHUDIdentityGapBase)
	totalW := swSize + identityGap + pipsW + contentGap + actionWpx
	cx := float32(g.screenW)/2 - totalW/2

	swX := cx
	vector.FillRect(screen, swX, swY, swSize, swSize, colDock, false)
	pipX := swX + swSize + identityGap + pipHalfW
	for i := 0; i < level; i++ {
		drawUpTriangle(screen, pipX+float32(i)*(pipHalfW*2+pipGap), pipCY, pipHalfW, colSysTrayBorder)
	}

	axLeft := swX + swSize + identityGap + pipsW + contentGap

	if b.Level >= 2 {
		// Level-3 tease: grey square + label, non-interactive.
		g.dockUpgradeRect = sysRect{}
		greyCol := color.RGBA{R: 60, G: 60, B: 80, A: 200}
		qSize := selectedHUD(8)
		qX := axLeft + 4*sp
		qY := ayTop + (actionH-qSize)/2
		vector.FillRect(screen, qX, qY, qSize, qSize, greyCol, false)
		// 3 dim triangles hint at a future third level.
		pipHalfW := selectedHUD(3)
		pipGap := selectedHUD(2)
		pipCY := qY + qSize/2
		pipX := qX + qSize + 3*sp + pipHalfW
		for i := 0; i < 3; i++ {
			drawUpTriangle(screen, pipX+float32(i)*(pipHalfW*2+pipGap), pipCY, pipHalfW, greyCol)
		}
	} else {
		// Level-1 upgrade button: fills from bottom as resources accumulate.
		btnX := axLeft
		btnW := actionWpx
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
	}
}

// drawTownHallTray draws a bottom tray when the Town Hall is selected.
// Mirrors the dock tray: TH swatch + one level pip, then a capacity action button.
// When the town field is full the button is greyed out and non-interactive.
func (g *Game) drawTownHallTray(screen *ebiten.Image) {
	if g.selectedBuildingID < 0 || g.selectedBuildingID >= len(g.world.Buildings) {
		g.thCapacityRect = sysRect{}
		g.thTrayRect = sysRect{}
		return
	}
	b := g.world.Buildings[g.selectedBuildingID]
	if b.Kind != KindTownHall {
		g.thCapacityRect = sysRect{}
		g.thTrayRect = sysRect{}
		return
	}

	scale, _, _ := viewGeom(g.screenW, g.screenH)
	selectedHUD := func(base float64) float32 {
		return scaledHUDFloat(scale, selectedBuildingHUDScale, base)
	}

	th := selectedHUD(bottomHUDHeightBase)
	tx := float32(0)
	tw := float32(g.screenW)
	ty := float32(g.screenH) - th

	vector.FillRect(screen, tx, ty, tw, th, colSysTrayFill, false)
	vector.StrokeLine(screen, tx, ty, tx+tw, ty, 1, colSysTrayBorder, false)
	g.thTrayRect = sysRect{x: tx, y: ty, w: tw, h: th}

	swSize := selectedHUD(bottomHUDSwatchBase)
	swY := ty + (th-swSize)/2

	// One level pip: TH is always level 1.
	pipHalfW := selectedHUD(3)
	pipCY := ty + th/2
	pipsW := pipHalfW * 2

	actionWpx := selectedHUD(selectedHUDActionWidthBase)
	actionInset := selectedHUD(selectedHUDActionInsetBase)
	actionH := th - actionInset*2
	ayTop := ty + actionInset
	contentGap := selectedHUD(selectedHUDContentGapBase)
	identityGap := selectedHUD(selectedHUDIdentityGapBase)

	totalW := swSize + identityGap + pipsW + contentGap + actionWpx
	cx := float32(g.screenW)/2 - totalW/2

	swX := cx
	vector.FillRect(screen, swX, swY, swSize, swSize, colTownHall, false)
	drawUpTriangle(screen, swX+swSize+identityGap+pipHalfW, pipCY, pipHalfW, colSysTrayBorder)

	axLeft := swX + swSize + identityGap + pipsW + contentGap

	isFull := townFieldFull(g.world)
	if isFull {
		// Greyed out — not interactive.
		g.thCapacityRect = sysRect{}
		vector.FillRect(screen, axLeft, ayTop, actionWpx, actionH,
			color.RGBA{R: 30, G: 30, B: 45, A: 200}, false)
		vector.StrokeRect(screen, axLeft, ayTop, actionWpx, actionH, 1, colSysTrayBorder, false)
	} else {
		cost := townCapacityCost(g.world)
		affordable := g.world.Economy.Wood >= cost
		btnX := axLeft
		btnW := actionWpx
		btnH := actionH

		// Dark base.
		vector.FillRect(screen, btnX, ayTop, btnW, btnH, colSysTrayFill, false)

		// Fill from bottom to top as wood accumulates.
		frac := affordabilityFrac(g.world.Economy.Wood, cost)
		if frac > 0 && frac < 1 {
			fillH := btnH * frac
			vector.FillRect(screen, btnX, ayTop+btnH-fillH, btnW, fillH,
				color.RGBA{R: 120, G: 75, B: 30, A: 200}, false)
		} else if affordable {
			vector.FillRect(screen, btnX, ayTop, btnW, btnH,
				color.RGBA{R: 120, G: 75, B: 30, A: 200}, false)
		}

		borderCol := colSysTrayBorder
		if affordable {
			borderCol = color.RGBA{R: 200, G: 120, B: 50, A: 220}
		}
		vector.StrokeRect(screen, btnX, ayTop, btnW, btnH, 1, borderCol, false)

		// Attention glow mirrors the first-house teaching ripple.
		if g.thCapacityAttentionLeft > 0 {
			atCol := colNurtureAttention
			atCol.A = 180
			vector.StrokeRect(screen, btnX-1, ayTop-1, btnW+2, btnH+2, 1.5, atCol, false)
		}

		g.thCapacityRect = sysRect{x: btnX, y: ayTop, w: btnW, h: btnH}
	}
}

// drawWorkerHUDOverlay replaces the plain worker icon and ratio text with a
// mini slider icon and per-kind worker counts once LaborFocus is set.
func (g *Game) drawWorkerHUDOverlay(screen *ebiten.Image) {
	w := g.world
	if !laborFocusControlAvailable(w) {
		return
	}
	if g.hud == nil {
		return
	}

	nWood, nWater, nIdle := activeWorkerHUDCounts(w)

	// ── Compact focus glyph (drawn over workerSquare button) ──────────────
	sqR := g.hud.workerSquare.GetWidget().Rect
	sqX := float32(sqR.Min.X)
	sqY := float32(sqR.Min.Y)
	sqW := float32(sqR.Dx())
	sqH := float32(sqR.Dy())

	vector.FillRect(screen, sqX, sqY, sqW, sqH, colSysTrayFill, false)
	vector.StrokeRect(screen, sqX, sqY, sqW, sqH, 1, colWorkerLaden, false)

	inset := sqH * 0.22
	innerX := sqX + inset
	innerY := sqY + inset
	innerW := sqW - inset*2
	innerH := sqH - inset*2
	halfGap := float32(1)
	halfW := (innerW - halfGap) / 2
	vector.FillRect(screen, innerX, innerY, halfW, innerH, colWoodResource, false)
	vector.FillRect(screen, innerX+halfW+halfGap, innerY, halfW, innerH, colSparkle, false)
	vector.StrokeLine(screen, innerX+halfW+halfGap/2, innerY, innerX+halfW+halfGap/2, innerY+innerH, 1, colWorkerLaden, false)

	if len(w.LaborFocus) == 0 {
		return
	}

	// ── Worker distribution meter (drawn over workerRatio text area) ──────
	txtR := g.hud.workerRatio.GetWidget().Rect
	txtX := float32(txtR.Min.X)
	txtW := float32(txtR.Dx())

	bgPad := float32(2)
	vector.FillRect(screen, txtX-bgPad, sqY, txtW+bgPad*2, sqH, colSysTrayFill, false)

	barH := sqH * 0.48
	barY := sqY + (sqH-barH)/2
	barX := txtX
	barW := txtW
	vector.StrokeRect(screen, barX, barY, barW, barH, 1, colSysTrayBorder, false)

	total := nWood + nWater + nIdle
	if total == 0 {
		return
	}
	type workerSeg struct {
		count int
		col   color.RGBA
	}
	segments := []workerSeg{
		{nWood, colWoodResource},
		{nWater, color.RGBA{R: colSparkle.R, G: colSparkle.G, B: colSparkle.B, A: 240}},
		{nIdle, colWorkerLaden},
	}
	nonZero := 0
	for _, seg := range segments {
		if seg.count > 0 {
			nonZero++
		}
	}
	fillX := barX + 1
	fillW := barW - 2
	remainingW := fillW
	drawn := 0
	for _, seg := range segments {
		if seg.count <= 0 {
			continue
		}
		drawn++
		w := remainingW
		if drawn < nonZero {
			w = fillW * float32(seg.count) / float32(total)
			if w < 2 {
				w = 2
			}
			if w > remainingW {
				w = remainingW
			}
		}
		vector.FillRect(screen, fillX, barY+1, w, barH-2, seg.col, false)
		fillX += w
		remainingW -= w
		if remainingW <= 0 {
			break
		}
	}
}

// drawFocusControl renders the two-resource labor focus overlay and records
// hit-test rects for input handling.
func (g *Game) drawFocusControl(screen *ebiten.Image) {
	if !g.showFocusControl {
		g.focusCtrlRect = sysRect{}
		g.focusSlotsRect = sysRect{}
		g.focusConfirmRect = sysRect{}
		g.focusCancelRect = sysRect{}
		return
	}

	total := len(g.world.Workers)
	if total == 0 {
		g.showFocusControl = false
		return
	}

	scale, _, _ := viewGeom(g.screenW, g.screenH)
	sp := float32(scale)
	face := g.hud.sysface
	if face == nil {
		return
	}

	nWater := g.focusDraftWater
	if nWater < 0 {
		nWater = 0
	}
	if nWater > total {
		nWater = total
	}
	nWood := total - nWater

	const (
		trackW   = float64(86)
		trackH   = float64(5)
		hitH     = float64(18)
		knobW    = float64(5)
		labelW   = float64(14)
		labelGap = float64(4)
		padding  = float64(8)
		btnSize  = float64(16)
		btnGap   = float64(6)
	)

	panelW := trackW + 2*labelW + 2*labelGap + 2*padding
	panelH := padding + hitH + padding*0.75 + btnSize + padding

	pw := float32(panelW * float64(scale))
	ph := float32(panelH * float64(scale))
	px := float32(g.screenW)/2 - pw/2
	py := float32(g.screenH)/2 - ph/2

	vector.FillRect(screen, px, py, pw, ph, colSysTrayFill, false)
	vector.StrokeRect(screen, px, py, pw, ph, 1, colSysTrayBorder, false)
	g.focusCtrlRect = sysRect{x: px, y: py, w: pw, h: ph}

	// Fixed-width split slider: green=wood, blue=water.
	labelWpx := float32(labelW * float64(scale))
	labelGapPx := float32(labelGap * float64(scale))
	trackX := px + float32(padding*float64(scale)) + labelWpx + labelGapPx
	trackWpx := float32(trackW * float64(scale))
	trackHpx := float32(trackH * float64(scale))
	hitHpx := float32(hitH * float64(scale))
	trackY := py + float32(padding*float64(scale)) + (hitHpx-trackHpx)/2
	splitT := float32(nWood) / float32(total)
	splitX := trackX + trackWpx*splitT

	labelCol := color.RGBA{R: 205, G: 220, B: 215, A: 235}
	labelY := trackY + trackHpx/2 - 4*sp
	woodStr := fmt.Sprintf("%d", nWood)
	woodTextW, _ := text.Measure(woodStr, face, 0)
	drawSysText(screen, woodStr, trackX-labelGapPx-float32(woodTextW), labelY, labelCol, face)
	waterStr := fmt.Sprintf("%d", nWater)
	drawSysText(screen, waterStr, trackX+trackWpx+labelGapPx, labelY, labelCol, face)

	vector.FillRect(screen, trackX, trackY, trackWpx, trackHpx, color.RGBA{R: 18, G: 28, B: 36, A: 230}, false)
	if nWood > 0 {
		vector.FillRect(screen, trackX, trackY, splitX-trackX, trackHpx, colWoodResource, false)
	}
	if nWater > 0 {
		vector.FillRect(screen, splitX, trackY, trackX+trackWpx-splitX, trackHpx, colSparkle, false)
	}
	vector.StrokeRect(screen, trackX, trackY, trackWpx, trackHpx, 1, color.RGBA{R: 95, G: 115, B: 125, A: 210}, false)

	knobWpx := float32(knobW * float64(scale))
	knobHpx := float32(14 * scale)
	knobX := splitX - knobWpx/2
	knobY := trackY + trackHpx/2 - knobHpx/2
	vector.FillRect(screen, knobX, knobY, knobWpx, knobHpx, color.RGBA{R: 225, G: 235, B: 225, A: 240}, false)
	vector.StrokeRect(screen, knobX, knobY, knobWpx, knobHpx, 1, color.RGBA{R: 30, G: 45, B: 40, A: 230}, false)

	g.focusSlotsRect = sysRect{x: trackX, y: trackY - (hitHpx-trackHpx)/2, w: trackWpx, h: hitHpx}

	// Confirm and cancel buttons
	btnY := py + ph - float32((padding+btnSize)*float64(scale))
	btnW := float32(btnSize * float64(scale))
	gap := float32(btnGap * float64(scale))
	btnStartX := px + pw/2 - btnW - gap/2
	g.focusConfirmRect = sysRect{x: btnStartX, y: btnY, w: btnW, h: btnW}
	g.focusCancelRect = sysRect{x: btnStartX + btnW + gap, y: btnY, w: btnW, h: btnW}

	confirmCol := color.RGBA{R: 24, G: 76, B: 42, A: 215}
	cancelCol := color.RGBA{R: 78, G: 30, B: 38, A: 215}
	vector.FillRect(screen, g.focusConfirmRect.x, g.focusConfirmRect.y, g.focusConfirmRect.w, g.focusConfirmRect.h, confirmCol, false)
	vector.StrokeRect(screen, g.focusConfirmRect.x, g.focusConfirmRect.y, g.focusConfirmRect.w, g.focusConfirmRect.h, 1, colSysTrayBorder, false)
	vector.FillRect(screen, g.focusCancelRect.x, g.focusCancelRect.y, g.focusCancelRect.w, g.focusCancelRect.h, cancelCol, false)
	vector.StrokeRect(screen, g.focusCancelRect.x, g.focusCancelRect.y, g.focusCancelRect.w, g.focusCancelRect.h, 1, colSysTrayBorder, false)

	iconPad := 4 * sp
	checkCol := color.RGBA{R: 178, G: 245, B: 190, A: 245}
	vector.StrokeLine(screen,
		g.focusConfirmRect.x+iconPad, g.focusConfirmRect.y+g.focusConfirmRect.h*0.55,
		g.focusConfirmRect.x+g.focusConfirmRect.w*0.43, g.focusConfirmRect.y+g.focusConfirmRect.h-iconPad,
		2, checkCol, false)
	vector.StrokeLine(screen,
		g.focusConfirmRect.x+g.focusConfirmRect.w*0.43, g.focusConfirmRect.y+g.focusConfirmRect.h-iconPad,
		g.focusConfirmRect.x+g.focusConfirmRect.w-iconPad, g.focusConfirmRect.y+iconPad,
		2, checkCol, false)

	xCol := color.RGBA{R: 245, G: 175, B: 180, A: 245}
	vector.StrokeLine(screen,
		g.focusCancelRect.x+iconPad, g.focusCancelRect.y+iconPad,
		g.focusCancelRect.x+g.focusCancelRect.w-iconPad, g.focusCancelRect.y+g.focusCancelRect.h-iconPad,
		2, xCol, false)
	vector.StrokeLine(screen,
		g.focusCancelRect.x+g.focusCancelRect.w-iconPad, g.focusCancelRect.y+iconPad,
		g.focusCancelRect.x+iconPad, g.focusCancelRect.y+g.focusCancelRect.h-iconPad,
		2, xCol, false)
}

// handleFocusControlInput processes mouse events while the focus control is open.
func (g *Game) handleFocusControlInput() {
	total := len(g.world.Workers)
	mx, my := ebiten.CursorPosition()
	fmx, fmy := float32(mx), float32(my)

	inRect := func(r sysRect) bool {
		return r.w > 0 && fmx >= r.x && fmx < r.x+r.w && fmy >= r.y && fmy < r.y+r.h
	}

	// Update draft split when mouse is pressed/dragged in the slot bar.
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if inRect(g.focusSlotsRect) || g.focusDragging {
			if inRect(g.focusSlotsRect) {
				g.focusDragging = true
			}
			if g.focusDragging && g.focusSlotsRect.w > 0 {
				t := (fmx - g.focusSlotsRect.x) / g.focusSlotsRect.w
				if t < 0 {
					t = 0
				}
				if t > 1 {
					t = 1
				}
				rawWood := int(math.Round(float64(t) * float64(total)))
				nWater := total - rawWood
				if nWater < 0 {
					nWater = 0
				}
				if nWater > total {
					nWater = total
				}
				g.focusDraftWater = nWater
			}
		}
	} else {
		g.focusDragging = false
	}

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}

	if inRect(g.focusConfirmRect) {
		nWater := g.focusDraftWater
		nWood := total - nWater
		g.world.LaborFocus = map[ResourceKind]int{
			KindWater: nWater,
			KindWood:  nWood,
		}
		g.world.SavedLaborRatio = map[ResourceKind]int{
			KindWater: nWater,
			KindWood:  nWood,
		}
		g.showFocusControl = false
		return
	}
	if inRect(g.focusCancelRect) || !inRect(g.focusCtrlRect) {
		g.showFocusControl = false
		return
	}
}
