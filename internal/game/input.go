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
	fmx, fmy := float32(mx), float32(my)

	// Allocation pip strips: left-click increments, right-click decrements by one step.
	if g.world.System.Unlocked {
		stepAlloc := func(fam *resourceFamily, delta int) bool {
			if fam == nil {
				return false
			}
			r := g.sysAllocRect[fam.Potential]
			if !r.contains(fmx, fmy) {
				return false
			}
			cur := int(math.Round(*fam.AllocPotential(&g.world.SystemEconomy) * 4))
			next := cur + delta
			if next < 0 {
				next = 0
			} else if next > 4 {
				next = 4
			}
			*fam.AllocPotential(&g.world.SystemEconomy) = float64(next) / 4
			return true
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			for i := range resourceFamilies {
				if stepAlloc(&resourceFamilies[i], +1) {
					return
				}
			}
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
			for i := range resourceFamilies {
				if stepAlloc(&resourceFamilies[i], -1) {
					return
				}
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
	if g.sysAwakenRect.contains(fmx, fmy) {
		awakenPlanet(g.world, g.world.System.Selected)
		return
	}

	// Check enter-planet tray button (sysEnterRect set in drawOverlay previous frame).
	if g.sysEnterRect.contains(fmx, fmy) {
		idx := g.world.System.Selected
		switchToPlanet(g.world, idx)
		enterPlanetView(g.world)
		clearTransientUI(g)
		return
	}

	// Circle inject buttons in the top HUD.
	if g.world.System.Unlocked && g.world.System.Selected >= 0 {
		for i := range resourceFamilies {
			fam := &resourceFamilies[i]
			if !g.sysInjectRect[fam.Potential].contains(fmx, fmy) {
				continue
			}
			if injectCirclePacket(g.world, fam.Potential) {
				g.spawnInjectDots(fam.Potential)
			} else {
				g.flashCostTargets(fam.InjectCostPulse)
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
		if g.sysReturnRect.contains(float32(mx), float32(my)) {
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

	// Right-click: cancel pending destructive placement, deselect building, or (debug) cancel placing.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		if g.pendingDestructive {
			g.pendingDestructive = false
			return
		}
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
	fmx, fmy := float32(mx), float32(my)

	// Dock tray: any click inside is consumed. The tray stays open so future
	// multi-level upgrades can be bought repeatedly while resources allow.
	if g.dockTrayRect.contains(fmx, fmy) {
		if g.dockUpgradeRect.contains(fmx, fmy) {
			if g.selectedBuildingID >= 0 && g.selectedBuildingID < len(g.world.Buildings) {
				if !upgradeDock(g.world, g.world.Buildings[g.selectedBuildingID]) {
					g.flashCostTargets(missingCostTargets(g.world, dockL2WoodCost, dockL2WaterCost))
				}
			}
		}
		return
	}

	// HUD clicks are consumed by EbitenUI.
	if g.hud.pointInHUD(mx, my, g.debug) {
		return
	}

	// Hit-test selectable buildings. Building selection takes
	// priority over placement so clicking on a building opens its tray.
	wp := g.screenToWorld(mx, my)
	for i, b := range g.world.Buildings {
		if b.Kind == KindDock && wp.Dist(b.Pos) <= 8.0 {
			if g.selectedBuildingID != i {
				g.closeBuildingTray()
			}
			g.selectedBuildingID = i
			return
		}
	}

	// Pending destructive placement: second click confirms if near the ghost, cancels otherwise.
	if g.pendingDestructive {
		confirmed := false
		wp := g.screenToWorld(mx, my)
		dist := wp.Dist(g.world.Planet.Center)
		if dist >= g.world.Planet.Radius-rimSnapBand && dist <= g.world.Planet.Radius+rimSnapBand {
			angle := g.world.Planet.AngleOf(wp)
			half := buildingHardHalfArc(KindLoggingCamp, g.world.Planet.Radius)
			if math.Abs(normAngle(angle-g.pendingPreview.Angle)) <= half*2 {
				placeBuildingWithFreePlacement(g.world, g.pendingPreview.Angle, g.pendingDestructiveFreePlacing)
				if !g.debug {
					g.freePlacing = false
				}
				confirmed = true
			}
		}
		g.pendingDestructive = false
		_ = confirmed
		return
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
	if len(pv.Blocked) > 0 {
		// Destructive: trees in the footprint — require a second confirming click.
		pv.Locked = true
		g.pendingPreview = *pv
		g.pendingDestructiveFreePlacing = g.freePlacing
		g.pendingDestructive = true
		return
	}
	if placeBuildingWithFreePlacement(g.world, pv.Angle, g.freePlacing) {
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
		// Spawn the founding worker immediately.
		w.Economy.TownGrowthCap = townGrowthInitialCap
		spawnWorkerAtTownHall(w)
		// Awaken all known fields: distribute starting nodes across them.
		foundStartingNodes(w, angle)
		return true
	}

	if pv.Kind == KindDock {
		firstDock := !dockExists(w)
		if !freePlacement {
			w.Economy.Wood -= dockExtWoodCost
			w.Economy.Water -= dockExtWaterCost
		}
		w.Buildings = append(w.Buildings, &Building{
			ID:           id,
			Kind:         KindDock,
			Level:        1,
			Angle:        angle,
			Pos:          w.Planet.RimPoint(angle),
			WorkCapacity: true,
			Extension:    pv.Extension,
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
	clearOverlappingNodes(w, pv.Blocked)
	return true
}

// clearOverlappingNodes removes nodes whose footprint overlapped a newly placed
// camp. Workers whose NodeID referenced a cleared node will get nil from
// findNode on their next tick and safely return home.
func clearOverlappingNodes(w *World, blocked []*ResourceNode) {
	if len(blocked) == 0 {
		return
	}
	ids := make(map[int]bool, len(blocked))
	for _, n := range blocked {
		ids[n.ID] = true
	}
	kept := w.Nodes[:0]
	for _, n := range w.Nodes {
		if !ids[n.ID] {
			kept = append(kept, n)
		}
	}
	w.Nodes = kept
}

// curPlacementPreview returns the placement preview for the current cursor
// position, or nil when placement is inactive, locked, or the cursor is over
// a selectable building or too far from the rim.
func (g *Game) curPlacementPreview() *placementPreview {
	if g.pendingDestructive {
		return &g.pendingPreview
	}
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
			if b.Kind == KindDock && wp.Dist(b.Pos) <= 8.0 {
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
