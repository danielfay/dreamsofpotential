package game

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const sysDoubleClickWindow = 350 * time.Millisecond

// handleSystemInput processes clicks and wheel in system view and,
// when in post-unlock planet view, the return-to-system affordances.
func (g *Game) handleSystemInput() {
	if g.showMenu {
		return
	}

	mx, my := ebiten.CursorPosition()

	// Allocation pip strips: left-click increments, right-click decrements by one step.
	if g.world.System.Unlocked {
		stepAlloc := func(r sysRect, get func() float64, set func(float64), delta int) bool {
			if r.w <= 0 {
				return false
			}
			if float32(mx) < r.x || float32(mx) >= r.x+r.w ||
				float32(my) < r.y || float32(my) >= r.y+r.h {
				return false
			}
			cur := int(math.Round(get() * 4))
			next := cur + delta
			if next < 0 {
				next = 0
			} else if next > 4 {
				next = 4
			}
			set(float64(next) / 4)
			return true
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if stepAlloc(g.sysAllocWoodRect,
				func() float64 { return g.world.SystemEconomy.WoodAllocPotential },
				func(v float64) { g.world.SystemEconomy.WoodAllocPotential = v }, +1) {
				return
			}
			if stepAlloc(g.sysAllocWaterRect,
				func() float64 { return g.world.SystemEconomy.WaterAllocPotential },
				func(v float64) { g.world.SystemEconomy.WaterAllocPotential = v }, +1) {
				return
			}
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
			if stepAlloc(g.sysAllocWoodRect,
				func() float64 { return g.world.SystemEconomy.WoodAllocPotential },
				func(v float64) { g.world.SystemEconomy.WoodAllocPotential = v }, -1) {
				return
			}
			if stepAlloc(g.sysAllocWaterRect,
				func() float64 { return g.world.SystemEconomy.WaterAllocPotential },
				func(v float64) { g.world.SystemEconomy.WaterAllocPotential = v }, -1) {
				return
			}
		}
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.world.System.Selected = -1
		return
	}

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}

	// Check awaken-echo tray button.
	if g.sysAwakenRect.w > 0 {
		if float32(mx) >= g.sysAwakenRect.x && float32(mx) < g.sysAwakenRect.x+g.sysAwakenRect.w &&
			float32(my) >= g.sysAwakenRect.y && float32(my) < g.sysAwakenRect.y+g.sysAwakenRect.h {
			awakenPlanet(g.world, g.world.System.Selected)
			return
		}
	}

	// Check enter-planet tray button (sysEnterRect set in drawOverlay previous frame).
	if g.sysEnterRect.w > 0 {
		if float32(mx) >= g.sysEnterRect.x && float32(mx) < g.sysEnterRect.x+g.sysEnterRect.w &&
			float32(my) >= g.sysEnterRect.y && float32(my) < g.sysEnterRect.y+g.sysEnterRect.h {
			idx := g.world.System.Selected
			switchToPlanet(g.world, idx)
			enterPlanetView(g.world)
			clearTransientUI(g)
			return
		}
	}

	// Circle inject buttons in the top HUD.
	if g.world.System.Unlocked && g.world.System.Selected >= 0 {
		if r := g.sysInjectWoodRect; r.w > 0 &&
			float32(mx) >= r.x && float32(mx) < r.x+r.w &&
			float32(my) >= r.y && float32(my) < r.y+r.h {
			if injectCirclePacket(g.world, PotentialForest) {
				g.spawnInjectDots(PotentialForest)
			} else {
				g.flashCostTargets(costPulseForestCircle)
			}
			return
		}
		if r := g.sysInjectWaterRect; r.w > 0 &&
			float32(mx) >= r.x && float32(mx) < r.x+r.w &&
			float32(my) >= r.y && float32(my) < r.y+r.h {
			if injectCirclePacket(g.world, PotentialWater) {
				g.spawnInjectDots(PotentialWater)
			} else {
				g.flashCostTargets(costPulseWaterCircle)
			}
			return
		}
	}

	// Planet selection: convert to virtual world coords and check disks.
	wp := g.screenToWorld(mx, my)
	for i, p := range g.world.System.Planets {
		if wp.Dist(p.Pos) <= p.Radius+3 {
			if i == g.sysDoubleClickPlanet &&
				time.Since(g.sysDoubleClickTime) < sysDoubleClickWindow {
				if p.zoomable() {
					// Double-click on an awakened/starting planet — enter it.
					switchToPlanet(g.world, i)
					enterPlanetView(g.world)
					g.world.System.Selected = i
					clearTransientUI(g)
					g.sysDoubleClickPlanet = -1
					return
				}
				if canAwaken(g.world, i) {
					// Double-click on an affordable dormant echo — awaken it.
					awakenPlanet(g.world, i)
					g.sysDoubleClickPlanet = -1
					return
				}
			}
			g.world.System.Selected = i
			g.sysDoubleClickPlanet = i
			g.sysDoubleClickTime = time.Now()
			return
		}
	}
}

// handlePlanetViewSystemReturn handles wheel-down and the return button
// in planet view when the system is already unlocked.
func (g *Game) handlePlanetViewSystemReturn() {
	if !g.world.System.Unlocked {
		return
	}
	returnToSystem := func() {
		parkActive(g.world)
		enterSystemView(g.world)
		clearTransientUI(g)
	}
	// Mouse wheel down (scroll out → system view).
	_, wy := ebiten.Wheel()
	if wy < 0 {
		returnToSystem()
		return
	}
	// Return button click (sysReturnRect set in drawOverlay previous frame).
	if g.sysReturnRect.w > 0 && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if float32(mx) >= g.sysReturnRect.x && float32(mx) < g.sysReturnRect.x+g.sysReturnRect.w &&
			float32(my) >= g.sysReturnRect.y && float32(my) < g.sysReturnRect.y+g.sysReturnRect.h {
			returnToSystem()
		}
	}
}

// handleGlobalInput processes keyboard-only commands that must take effect
// before EbitenUI lays out widgets for the frame.
func (g *Game) handleGlobalInput() {
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		g.debug = !g.debug
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}

	// Esc: in debug mode, cancel placement if active; otherwise toggle the settings menu.
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.debug && g.placing {
			g.placing = false
			g.freePlacing = false
		} else {
			g.showMenu = !g.showMenu
		}
		return
	}
}

// handleInput processes build-placement and building-selection input. It must
// be called after g.ui.Update() so that EbitenUI has already consumed any
// widget clicks this frame.
func (g *Game) handleInput() {
	if g.showMenu {
		return
	}
	if g.showFocusControl {
		g.handleFocusControlInput()
		return
	}
	g.handlePlanetViewSystemReturn()

	// Right-click: deselect any selected building; in debug mode also cancel placement.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		if g.debug && g.placing {
			g.placing = false
			g.freePlacing = false
		} else {
			g.closeBuildingTray()
		}
		return
	}

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := ebiten.CursorPosition()

	// TH tray: any click inside is consumed. The tray stays open so capacity can
	// be bought repeatedly while resources allow.
	if g.thTrayRect.w > 0 {
		tr := g.thTrayRect
		if float32(mx) >= tr.x && float32(mx) < tr.x+tr.w &&
			float32(my) >= tr.y && float32(my) < tr.y+tr.h {
			rx := g.thCapacityRect
			if rx.w > 0 && float32(mx) >= rx.x && float32(mx) < rx.x+rx.w &&
				float32(my) >= rx.y && float32(my) < rx.y+rx.h {
				if !buildTownCapacity(g.world) && g.world.Economy.Wood < townCapacityCost(g.world) {
					g.flashCostTargets(missingCostTargets(g.world, townCapacityCost(g.world), 0))
				}
			}
			return
		}
	}

	// Dock tray: any click inside is consumed. The tray stays open so future
	// multi-level upgrades can be bought repeatedly while resources allow.
	if g.dockTrayRect.w > 0 {
		tr := g.dockTrayRect
		if float32(mx) >= tr.x && float32(mx) < tr.x+tr.w &&
			float32(my) >= tr.y && float32(my) < tr.y+tr.h {
			rx := g.dockUpgradeRect
			if rx.w > 0 && float32(mx) >= rx.x && float32(mx) < rx.x+rx.w &&
				float32(my) >= rx.y && float32(my) < rx.y+rx.h {
				if g.selectedBuildingID >= 0 && g.selectedBuildingID < len(g.world.Buildings) {
					if !upgradeDock(g.world, g.world.Buildings[g.selectedBuildingID]) {
						g.flashCostTargets(missingCostTargets(g.world, dockL2WoodCost, dockL2WaterCost))
					}
				}
			}
			return
		}
	}

	// HUD clicks are consumed by EbitenUI.
	if g.hud.pointInHUD(mx, my, g.debug) {
		return
	}

	// Hit-test selectable buildings (dock, town hall). Building selection takes
	// priority over placement so clicking on a building opens its tray.
	wp := g.screenToWorld(mx, my)
	for i, b := range g.world.Buildings {
		if (b.Kind == KindDock || b.Kind == KindTownHall) && wp.Dist(b.Pos) <= 8.0 {
			if g.selectedBuildingID != i {
				g.closeBuildingTray()
			}
			g.selectedBuildingID = i
			return
		}
	}

	// In debug mode, placement requires g.placing to be explicitly enabled.
	if g.debug && !g.placing {
		g.closeBuildingTray()
		return
	}
	// Suppress placement during the locked pre-discovery camp period (non-debug only).
	if !g.debug && len(g.world.Buildings) > 0 && !g.world.ResourceDiscovered {
		g.closeBuildingTray()
		return
	}

	// Placement: try to place at cursor position near the rim.
	pv := g.placementPreviewAtCursor()
	if pv == nil {
		g.closeBuildingTray()
		return
	}
	isTownHall := len(g.world.Buildings) == 0
	if !pv.Affordable && g.world.ResourceDiscovered {
		g.flashCostTargets(missingPlacementCostTargets(g.world, pv))
	}
	if !pv.Valid {
		g.rejectTime = microPulseTime
		return
	}
	if placeBuildingWithFreePlacement(g.world, pv.Angle, g.freePlacing) {
		g.closeBuildingTray()
		if isTownHall || !g.debug {
			g.freePlacing = false
		}
	}
}

// placeBuilding attempts to place a building at the given rim angle. Returns
// true if the building was placed. The first placement is always a free Town
// Hall (requires a local free node within previewArc). Subsequent placements
// are paid logging camps that skip the local-node check.
func placeBuilding(w *World, angle float64) bool {
	return placeBuildingWithFreePlacement(w, angle, false)
}

func placeBuildingWithFreePlacement(w *World, angle float64, freePlacement bool) bool {
	pv := buildPreviewWithFreePlacement(w, angle, freePlacement)
	if pv.Hidden || !pv.Valid {
		return false
	}

	id := w.NextBuildingID
	w.NextBuildingID++

	if len(w.Buildings) == 0 {
		// Free Town Hall — does not consume cost progression.
		w.Buildings = append(w.Buildings, &Building{
			ID:    id,
			Kind:  KindTownHall,
			Angle: angle,
			Pos:   w.Planet.RimPoint(angle),
		})
		// Grant the founding capacity slot and spawn the first worker immediately.
		w.Economy.WorkerCapacity = 1
		w.Economy.TownGrowthCap = townGrowthInitialCap
		spawnWorkerAtTownHall(w)
		// Awaken all known fields: distribute starting nodes across them.
		foundStartingNodes(w, angle)
		return true
	}

	if pv.Kind == KindDock {
		firstDock := !dockExists(w)
		if !freePlacement {
			if pv.Extension {
				w.Economy.Wood -= dockExtWoodCost
				w.Economy.Water -= dockExtWaterCost
			} else {
				w.Economy.Wood -= dockShoreCost
			}
		}
		w.Buildings = append(w.Buildings, &Building{
			ID:        id,
			Kind:      KindDock,
			Level:     1,
			Angle:     angle,
			Pos:       w.Planet.RimPoint(angle),
			Extension: pv.Extension,
		})
		if firstDock {
			seedInitialDockSparkles(w, w.Buildings[len(w.Buildings)-1])
		}
		return true
	}

	// Paid logging camp.
	cost := CampCost(w)
	if !freePlacement && w.Economy.Wood < cost {
		return false
	}
	if !freePlacement {
		w.Economy.Wood -= cost
		w.Economy.CampsBought++
	}
	w.Buildings = append(w.Buildings, &Building{
		ID:    id,
		Kind:  KindLoggingCamp,
		Angle: angle,
		Pos:   w.Planet.RimPoint(angle),
	})
	return true
}

// curPlacementPreview returns the placement preview for the current cursor
// position, or nil when placement is inactive, locked, or the cursor is over
// a selectable building or too far from the rim.
func (g *Game) curPlacementPreview() *placementPreview {
	if !g.placing {
		return nil
	}
	// Suppress during the locked pre-discovery camp period (non-debug mode only).
	if !g.debug && len(g.world.Buildings) > 0 && !g.world.ResourceDiscovered {
		return nil
	}
	// Suppress when hovering over a selectable building so the click goes to selection.
	mx, my := ebiten.CursorPosition()
	if !g.hud.pointInHUD(mx, my, g.debug) {
		wp := g.screenToWorld(mx, my)
		for _, b := range g.world.Buildings {
			if (b.Kind == KindDock || b.Kind == KindTownHall) && wp.Dist(b.Pos) <= 8.0 {
				return nil
			}
		}
	}
	pv := g.placementPreviewAtCursor()
	if pv != nil && g.rejectTime > 0 {
		pv.Reject = g.rejectTime / microPulseTime
		if pv.Reject > 1 {
			pv.Reject = 1
		}
	}
	return pv
}

func (g *Game) placementPreviewAtCursor() *placementPreview {
	mx, my := ebiten.CursorPosition()
	wp := g.screenToWorld(mx, my)
	center := g.world.Planet.Center
	radius := g.world.Planet.Radius
	dist := wp.Dist(center)
	if dist < radius-rimSnapBand || dist > radius+rimSnapBand {
		return nil
	}
	angle := g.world.Planet.AngleOf(wp)
	pv := buildPreviewWithFreePlacement(g.world, angle, g.freePlacing)
	if pv.Hidden {
		return nil
	}
	return &pv
}
