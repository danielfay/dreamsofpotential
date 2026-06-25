package game

import (
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

	// Mouse wheel up: not applicable in system view (already here).

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.world.System.Selected = -1
		return
	}

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := ebiten.CursorPosition()

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

	// Esc: cancel placement if placing, otherwise toggle the settings menu.
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.placing {
			g.placing = false
			g.freePlacing = false
		} else {
			g.showMenu = !g.showMenu
		}
		return
	}
}

// handleInput processes build-placement input. It must be called after
// g.ui.Update() so that EbitenUI has already consumed any widget clicks this
// frame, preventing a HUD button click from simultaneously placing a camp on
// the world.
func (g *Game) handleInput() {
	// Menu is open — swallow all further input so nothing behind it fires.
	if g.showMenu {
		return
	}

	// Post-unlock: wheel-down and return button navigate to system view.
	g.handlePlanetViewSystemReturn()

	if !g.placing {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			mx, my := ebiten.CursorPosition()
			// Dock tray: any click inside the tray is consumed (upgrade if on button; dismiss otherwise).
			if g.dockTrayRect.w > 0 {
				tr := g.dockTrayRect
				if float32(mx) >= tr.x && float32(mx) < tr.x+tr.w &&
					float32(my) >= tr.y && float32(my) < tr.y+tr.h {
					rx := g.dockUpgradeRect
					if rx.w > 0 && float32(mx) >= rx.x && float32(mx) < rx.x+rx.w &&
						float32(my) >= rx.y && float32(my) < rx.y+rx.h {
						if g.selectedBuildingID >= 0 && g.selectedBuildingID < len(g.world.Buildings) {
							upgradeDock(g.world, g.world.Buildings[g.selectedBuildingID])
						}
					}
					g.selectedBuildingID = -1
					return
				}
			}
			// Hit-test dock buildings for selection tray.
			if !g.hud.pointInHUD(mx, my, g.debug) {
				wp := g.screenToWorld(mx, my)
				newSel := -1
				for i, b := range g.world.Buildings {
					if b.Kind == KindDock && wp.Dist(b.Pos) <= 6.0 {
						newSel = i
						break
					}
				}
				g.selectedBuildingID = newSel
			}
		}
		return
	}

	// Cancel placement with right-click.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		g.placing = false
		g.freePlacing = false
		return
	}

	// Confirm placement on left-click outside the HUD.
	// Any direction snaps to the rim. If invalid we stay in placement mode so the
	// player can retry at a better angle. After a Town Hall is placed, the Camp
	// tool is sticky — placement mode stays on so the player can keep placing
	// camps until they cancel.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if g.hud.pointInHUD(mx, my, g.debug) {
			return // click was on the HUD panel; ignore
		}
		pv := g.placementPreviewAtCursor()
		if pv == nil {
			return
		}
		isTownHall := len(g.world.Buildings) == 0
		if !pv.Affordable && g.world.ResourceDiscovered {
			g.pulseTime = pulseDuration
			g.pulseTarget = 1
		}
		if !pv.Valid {
			g.rejectTime = microPulseTime
			return
		}
		if placeBuildingWithFreePlacement(g.world, pv.Angle, g.freePlacing) {
			if isTownHall {
				g.placing = false
			}
			if isTownHall || !g.debug {
				g.freePlacing = false
			}
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
	if !pv.Valid {
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
// position, or nil when not placing or the cursor is too far from the rim.
func (g *Game) curPlacementPreview() *placementPreview {
	if !g.placing {
		return nil
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
	return &pv
}
