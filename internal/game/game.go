package game

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/ebitenui/ebitenui"
	"github.com/hajimehoshi/ebiten/v2"
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
	world                         *World
	scene                         *ebiten.Image // low-res 320×240 canvas; scaled up to the window
	ui                            *ebitenui.UI
	hud                           *HUD
	placing                       bool              // true while waiting for player to click a camp location
	freePlacing                   bool              // debug-only: next placement ignores camp cost
	pendingDestructive            bool              // true while waiting for second click to confirm a destructive camp
	pendingPreview                placementPreview  // locked ghost for the pending destructive placement
	pendingDestructiveFreePlacing bool              // freePlacing state at the time of the first click
	preview                       *placementPreview // current frame's placement preview; nil when not placing
	showMenu                      bool              // true when the settings overlay is open
	debug                         bool              // F3 — verbose debug panel; session-only, not persisted
	debugSection                  int               // selected debug panel section; session-only
	pulseTime                     float64           // seconds remaining on the unaffordable-cost flash
	pulseTarget                   int               // costPulse* bitmask for unaffordable-cost flash targets
	rejectTime                    float64           // seconds remaining on invalid placement feedback
	screenW                       int               // current screen dimensions, updated each Draw()
	screenH                       int
	hudScale                      int         // integer view scale at last HUD build; triggers rebuild on change
	hudDigits                     int         // digit count of wood at last HUD build; triggers rebuild on grow
	saveTimer                     float64     // counts down to next autosave
	nurtureToggleActive           bool        // true while the nurture auto-cycle is running
	nurtureConfirmLeft            float64     // seconds remaining on the nurture success flash
	nurtureAttentionCooldown      float64     // counts down to next Nurture attention pulse
	nurtureAttentionPulseLeft     float64     // seconds remaining on the Nurture attention flash
	workerRatioAttentionCooldown  float64     // counts down to next worker-ratio attention pulse
	workerRatioAttentionLeft      float64     // seconds remaining on the worker-ratio attention flash
	workerRatioAttentionReady     bool        // previous-frame state for first-available worker-ratio attention
	dockUpgradeAttentionCooldown  float64     // counts down to next dock-upgrade attention pulse
	holdAction                    int         // current held purchase action (holdNone, holdNurture, …)
	holdTimer                     float64     // counts down to next repeat fire
	holdDuration                  float64     // total seconds the current hold has been active
	importCh                      chan *World // receives a validated world from an import goroutine
	dialogOpen                    atomic.Bool // true while a file dialog is open; blocks UI input

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

	// planet-view selected building (dock tray)
	selectedBuildingID int     // index into w.Buildings; -1 = none
	dockUpgradeRect    sysRect // upgrade button hit-test rect in native screen space
	dockTrayRect       sysRect // entire dock tray background rect (clicking anywhere dismisses)

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

func (g *Game) ensureTransientMaps() {
	if g.sysInjectRect == nil {
		g.sysInjectRect = make(map[PotentialKind]sysRect, len(resourceFamilies))
	}
	if g.sysAllocRect == nil {
		g.sysAllocRect = make(map[PotentialKind]sysRect, len(resourceFamilies))
	}
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
		return missingCostTargets(w, dockExtWoodCost, dockExtWaterCost)
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

		if g.nurtureToggleActive {
			if nurtureField(g.world) {
				g.nurtureConfirmLeft = nurtureConfirmDuration
			} else if !g.world.ResourceDiscovered || !anyFieldCanSpawn(g.world) {
				g.nurtureToggleActive = false
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
	g.ensureTransientMaps()
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
	g.nurtureToggleActive = false
	g.showMenu = false
	g.showFocusControl = false
	g.closeBuildingTray()
	g.workerRatioAttentionReady = false
}

func (g *Game) closeBuildingTray() {
	g.selectedBuildingID = -1
	g.dockUpgradeRect = sysRect{}
	g.dockTrayRect = sysRect{}
}
