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
	costPulseForestCircle
	costPulseWaterCircle
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

	// system-view button rects in native screen space (set during drawOverlay; read by handleSystemInput)
	sysEnterRect  sysRect                   // enter-planet tray button
	sysAwakenRect sysRect                   // awaken-echo tray button
	sysReturnRect sysRect                   // return-to-system button in planet view
	sysInjectRect map[PotentialKind]sysRect // circle inject buttons by Potential family
	sysAllocRect  map[PotentialKind]sysRect // alloc pip strips by Potential family

	sysInjectDots []sysInjectDot // active circle-packet injection animations

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

func (r sysRect) contains(x, y float32) bool {
	return r.w > 0 && x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

// sysInjectDot is one particle in the circle-packet injection animation.
// Dots travel from the HUD circle toward the target planet and fade out.
type sysInjectDot struct {
	ox, oy float32 // origin screen pos
	tx, ty float32 // target planet center screen pos
	age    float32 // seconds elapsed
	life   float32 // total lifetime
	col    color.RGBA
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
		sysInjectRect:                make(map[PotentialKind]sysRect, len(resourceFamilies)),
		sysAllocRect:                 make(map[PotentialKind]sysRect, len(resourceFamilies)),
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

	// Advance inject-dot animations.
	alive := g.sysInjectDots[:0]
	for _, d := range g.sysInjectDots {
		d.age += dt
		if d.age < d.life {
			alive = append(alive, d)
		}
	}
	g.sysInjectDots = alive

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
		for i := range resourceFamilies {
			fam := &resourceFamilies[i]
			r := g.sysInjectRect[fam.Potential]
			if g.pulseTarget&fam.InjectCostPulse != 0 && r.w > 0 {
				vector.StrokeRect(screen, r.x-2, r.y-2, r.w+4, r.h+4, 2, colPulse, false)
			}
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
