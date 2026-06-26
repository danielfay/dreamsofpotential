package game

import (
	"fmt"
	"math"
)

// QAPreset describes a reproducible mid-game world state for manual QA.
// Pointer fields are optional; nil means "use default or skip".
type QAPreset struct {
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Seed          int64     `json:"seed"`
	Discovered    *bool     `json:"discovered"`
	PlaceTownHall bool      `json:"placeTownHall"`
	TownHallAngle *float64  `json:"townHallAngle"`
	Camps         []float64 `json:"camps"`
	Workers       int       `json:"workers"`
	NoFreeNodes   bool      `json:"noFreeNodes"`
	SettleSeconds float64   `json:"settleSeconds"`
	// FieldCapCycles multiplies the field cap by woodFieldEXPGrowth N times before
	// setting EXP, simulating N completed growth cycles.
	FieldCapCycles  *int     `json:"fieldCapCycles"`
	FieldExpFromCap *float64 `json:"fieldExpFromCap"` // EXP = Cap + delta (use negative to go below cap)
	FieldExpFrac    *float64 `json:"fieldExpFrac"`    // EXP = frac * Cap
	FieldExpAbs     *float64 `json:"fieldExpAbs"`     // EXP = abs value
	Wood            *float64 `json:"wood"`
	// Town Growth overrides — applied after workers, before final Wood stamp.
	TownGrowth       *float64 `json:"townGrowth"`
	TownGrowthCap    *float64 `json:"townGrowthCap"`
	WorkerCapacity   *int     `json:"workerCapacity"`
	FillTownCapacity bool     `json:"fillTownCapacity"` // set WorkerCapacity to the geometry max

	// Wood field saturation — applied after settle and field EXP overrides.
	// NearWoodFieldSaturation fills the field leaving exactly one spawn slot.
	// SaturateWoodField fills the field until no new tree node can spawn.
	NearWoodFieldSaturation bool `json:"nearWoodFieldSaturation"`
	SaturateWoodField       bool `json:"saturateWoodField"`

	// Reveal — calls triggerUnlock after all other overrides.
	// Requires both mastery gates to be met; errors otherwise.
	Reveal bool `json:"reveal"`

	// Echo lifecycle — applied after Reveal in order: Awaken, Complete, Enter.
	AwakenEchoes   []int `json:"awakenEchoes"`   // planet indices to awaken (bypasses wood cost)
	CompleteEchoes []int `json:"completeEchoes"` // planet indices to fully complete (awaken + build + saturate)

	// SelectPlanet sets System.Selected after echo lifecycle processing.
	SelectPlanet *int `json:"selectPlanet"`

	// AwakenFrontier awakens the unknown frontier (Planets index 3) regardless of
	// current Potential balance — it grants the required Potential before awakening.
	// Applied after CompleteEchoes and before SelectPlanet/EnterPlanet.
	AwakenFrontier bool `json:"awakenFrontier"`

	// CompleteFrontier drives the frontier (Planets index 3) to completion:
	// places a Town Hall, fills town capacity, saturates both wood and water fields,
	// places and upgrades a dock to Level 2, then calls checkActivePlanetCompletion.
	// Requires AwakenFrontier (or frontier already awakened). Leaves the world in
	// system view on the starting planet. Applied after AwakenFrontier.
	CompleteFrontier bool `json:"completeFrontier"`

	// ForestPotential / WaterPotential stamp exact Potential balances after all
	// other overrides (including AwakenEchoes, CompleteEchoes, AwakenFrontier).
	// Use to set precise amounts for QA scenarios without relying on the growth path.
	ForestPotential *int `json:"forestPotential"`
	WaterPotential  *int `json:"waterPotential"`

	// EnterPlanet switches the active planet and enters planet view.
	// Applied after AwakenEchoes, CompleteEchoes, and AwakenFrontier.
	EnterPlanet *int `json:"enterPlanet"`

	// Echo setup — applied to the entered planet when EnterPlanet is set.
	EchoPlaceTownHall    bool      `json:"echoPlaceTownHall"`
	EchoFillTownCapacity bool      `json:"echoFillTownCapacity"`
	EchoNearSaturate     bool      `json:"echoNearSaturate"` // leave exactly one spawn slot
	EchoDocks            []float64 `json:"echoDocks"`        // dock angles placed via free placement after EchoPlaceTownHall

	// Water sparkle saturation — applied after echo setup.
	// SaturateWaterField fills all known water fields with sparkles until no more can spawn.
	SaturateWaterField bool `json:"saturateWaterField"`

	// LaborFocus stamps the labor focus ratio after all other overrides.
	// Keys are string resource kind names ("wood", "water").
	LaborFocusWood  *int `json:"laborFocusWood,omitempty"`
	LaborFocusWater *int `json:"laborFocusWater,omitempty"`
}

// BuildQAWorld constructs a *World by applying preset overrides on top of NewWorld.
// Overrides are applied in a deterministic order so the final state is reproducible.
func BuildQAWorld(p QAPreset) (*World, error) {
	seed := p.Seed
	if seed == 0 {
		seed = 11
	}
	w := newWorldWithSeed(seed)

	// Resource discovery (default true so HUD gauge and Nurture are active).
	if p.Discovered != nil {
		w.ResourceDiscovered = *p.Discovered
	} else {
		w.ResourceDiscovered = true
	}

	// Town Hall — required before buying workers.
	if p.PlaceTownHall {
		if p.TownHallAngle != nil {
			if !placeBuilding(w, *p.TownHallAngle) {
				return nil, fmt.Errorf("failed to place Town Hall at angle %.3f", *p.TownHallAngle)
			}
		} else {
			f := fieldForKind(w, KindWood)
			if f == nil || !placeBuilding(w, f.CenterAngle) {
				return nil, fmt.Errorf("placeTownHall: could not place Town Hall at wood field center")
			}
		}
	}

	// Logging camps.
	for _, angle := range p.Camps {
		if !placeBuilding(w, angle) {
			return nil, fmt.Errorf("failed to place camp at angle %.3f", angle)
		}
	}

	// Workers — spawn without wood cost; Town Hall placement already created one
	// founding worker, so start from the current count.
	if p.Workers > len(w.Workers) {
		if w.Economy.WorkerCapacity < p.Workers {
			w.Economy.WorkerCapacity = p.Workers
		}
		for len(w.Workers) < p.Workers {
			if spawnWorkerAtTownHall(w) == nil {
				return nil, fmt.Errorf("failed to spawn worker %d of %d (no Town Hall)", len(w.Workers)+1, p.Workers)
			}
		}
	}

	// Remove unclaimed nodes before settling so workers find nothing and go idle.
	if p.NoFreeNodes {
		clearFreeNodes(w)
	}

	// Settle: advance the sim so workers reach their steady state.
	for range int(p.SettleSeconds * 60) {
		Step(w, dt)
	}

	// Field EXP — applied after settling to preserve exact values.
	if len(w.Planet.Fields) > 0 {
		f := w.Planet.Fields[0]
		fp := w.Planet.FieldProgress[f.Kind]
		if fp != nil {
			if p.FieldCapCycles != nil {
				for range *p.FieldCapCycles {
					fp.Cap *= woodFieldEXPGrowth
				}
			}
			switch {
			case p.FieldExpFromCap != nil:
				fp.EXP = math.Max(0, math.Min(fp.Cap, fp.Cap+*p.FieldExpFromCap))
			case p.FieldExpFrac != nil:
				fp.EXP = math.Max(0, math.Min(fp.Cap, fp.Cap**p.FieldExpFrac))
			case p.FieldExpAbs != nil:
				fp.EXP = math.Max(0, math.Min(fp.Cap, *p.FieldExpAbs))
			}
		}
	}

	// Wood field saturation — fill nodes after settle to preserve exact saturation state.
	if p.SaturateWoodField || p.NearWoodFieldSaturation {
		fillWoodFieldNodes(w, p.NearWoodFieldSaturation)
	}

	// Town Growth overrides — applied after workers so capacity is already known.
	// Must run before Reveal so townFieldFull() sees the final WorkerCapacity.
	if p.FillTownCapacity {
		if max := maxTownSlots(w); max > w.Economy.WorkerCapacity {
			w.Economy.WorkerCapacity = max
		}
	} else if p.WorkerCapacity != nil && *p.WorkerCapacity > w.Economy.WorkerCapacity {
		w.Economy.WorkerCapacity = *p.WorkerCapacity
	}
	if p.TownGrowthCap != nil {
		w.Economy.TownGrowthCap = *p.TownGrowthCap
	}
	if p.TownGrowth != nil {
		// Clamp to the cap (possibly just updated) to honour the no-overflow rule.
		g := math.Min(*p.TownGrowth, w.Economy.TownGrowthCap)
		if g < 0 {
			g = 0
		}
		w.Economy.TownGrowth = g
	}

	// System reveal — call triggerUnlock if both mastery gates are met.
	if p.Reveal {
		if !forestPlanetComplete(w) {
			return nil, fmt.Errorf("Reveal: world not mastered (town full=%v, field saturated=%v)",
				townFieldFull(w), func() bool {
					f := fieldForKind(w, KindWood)
					return f != nil && !fieldCanSpawnNode(w, f)
				}())
		}
		triggerUnlock(w)
	}

	// Echo lifecycle — applied after Reveal.
	for _, idx := range p.AwakenEchoes {
		if idx < 0 || idx >= len(w.System.Planets) {
			return nil, fmt.Errorf("awakenEchoes: index %d out of range", idx)
		}
		if w.System.Planets[idx].Kind != PlanetEcho {
			return nil, fmt.Errorf("awakenEchoes: planet %d is not an echo", idx)
		}
		if !w.System.Planets[idx].Awakened {
			w.Economy.Potential[PotentialForest] += 1.0
			awakenPlanet(w, idx)
		}
	}

	for _, idx := range p.CompleteEchoes {
		if idx < 0 || idx >= len(w.System.Planets) {
			return nil, fmt.Errorf("completeEchoes: index %d out of range", idx)
		}
		if w.System.Planets[idx].Kind != PlanetEcho {
			return nil, fmt.Errorf("completeEchoes: planet %d is not an echo", idx)
		}
		if !w.System.Planets[idx].Awakened {
			w.Economy.Potential[PotentialForest] += 1.0
			awakenPlanet(w, idx)
		}
		// Build out the echo to meet the completion gate.
		switchToPlanet(w, idx)
		ef := fieldForKind(w, KindWood)
		if ef == nil {
			return nil, fmt.Errorf("completeEchoes: echo %d has no wood field", idx)
		}
		// Find a clear TH angle for the authored echo layout.
		thAngle, ok := findValidBuildingAngle(w)
		if !ok || !placeBuilding(w, thAngle) {
			return nil, fmt.Errorf("completeEchoes: failed to place Town Hall on echo %d", idx)
		}
		if max := maxTownSlots(w); max > w.Economy.WorkerCapacity {
			w.Economy.WorkerCapacity = max
		}
		fillWoodFieldNodes(w, false)
		checkActivePlanetCompletion(w)
		if !w.System.Planets[idx].Completed {
			return nil, fmt.Errorf("completeEchoes: echo %d did not complete", idx)
		}
		// Restore starting planet as active and return to system view.
		switchToPlanet(w, 0)
		enterSystemView(w)
	}

	// Frontier awakening — grant required Potential and awaken the frontier.
	if p.AwakenFrontier {
		const frontierIdx = 3
		if !w.System.Planets[frontierIdx].Awakened {
			for kind, cost := range planetAwakenCost(w, frontierIdx) {
				w.Economy.Potential[kind] += float64(cost)
			}
			awakenPlanet(w, frontierIdx)
		}
	}

	// Frontier completion — drive planet 3 to its water-planet completion gate.
	if p.CompleteFrontier {
		const frontierIdx = 3
		if !w.System.Planets[frontierIdx].Awakened {
			return nil, fmt.Errorf("completeFrontier: frontier not awakened (set AwakenFrontier first)")
		}
		switchToPlanet(w, frontierIdx)
		enterPlanetView(w)
		thAngle, ok := findValidBuildingAngle(w)
		if !ok || !placeBuilding(w, thAngle) {
			return nil, fmt.Errorf("completeFrontier: cannot place Town Hall on frontier")
		}
		if max := maxTownSlots(w); max > w.Economy.WorkerCapacity {
			w.Economy.WorkerCapacity = max
		}
		fillWoodFieldNodes(w, false)
		if !placeBuildingWithFreePlacement(w, waterFrontierLakeAngle, true) {
			return nil, fmt.Errorf("completeFrontier: cannot place dock at frontier lake angle")
		}
		fillWaterFieldSparkles(w)
		for _, b := range w.Buildings {
			if b.Kind == KindDock {
				b.Level = 2
			}
		}
		checkActivePlanetCompletion(w)
		if !w.System.Planets[frontierIdx].Completed {
			return nil, fmt.Errorf("completeFrontier: planet did not complete after full setup")
		}
		switchToPlanet(w, 0)
		enterSystemView(w)
	}

	// Potential stamps — applied after all lifecycle overrides so callers can set
	// exact balances regardless of what the growth path happened to award.
	if p.ForestPotential != nil {
		w.Economy.Potential[PotentialForest] = float64(*p.ForestPotential)
	}
	if p.WaterPotential != nil {
		w.Economy.Potential[PotentialWater] = float64(*p.WaterPotential)
	}

	// Select a specific planet in system view.
	if p.SelectPlanet != nil {
		w.System.Selected = *p.SelectPlanet
	}

	// Enter a specific planet (switch active + enter planet view).
	if p.EnterPlanet != nil {
		idx := *p.EnterPlanet
		if idx < 0 || idx >= len(w.System.Planets) {
			return nil, fmt.Errorf("enterPlanet: index %d out of range", idx)
		}
		switchToPlanet(w, idx)
		enterPlanetView(w)
		w.System.Selected = idx
		if p.EchoPlaceTownHall {
			thAngle, ok := findValidBuildingAngle(w)
			if !ok || !placeBuilding(w, thAngle) {
				return nil, fmt.Errorf("echoPlaceTownHall: failed to place Town Hall")
			}
		}
		if p.EchoFillTownCapacity {
			if max := maxTownSlots(w); max > w.Economy.WorkerCapacity {
				w.Economy.WorkerCapacity = max
			}
		}
		if p.EchoNearSaturate {
			fillWoodFieldNodes(w, true)
		}
		for _, a := range p.EchoDocks {
			if !placeBuildingWithFreePlacement(w, a, true) {
				return nil, fmt.Errorf("echoDocks: failed to place dock at angle %.3f", a)
			}
		}
	}

	// Water sparkle saturation — applied after echo setup so docks are placed first.
	if p.SaturateWaterField {
		fillWaterFieldSparkles(w)
	}

	// Wood — stamped last so it reflects the intended final balance exactly.
	if p.Wood != nil {
		w.Economy.Wood = *p.Wood
	}

	// Labor focus — applied after Wood so it can observe the full worker roster.
	if p.LaborFocusWood != nil || p.LaborFocusWater != nil {
		wood := 0
		water := 0
		if p.LaborFocusWood != nil {
			wood = *p.LaborFocusWood
		}
		if p.LaborFocusWater != nil {
			water = *p.LaborFocusWater
		}
		w.LaborFocus = map[ResourceKind]int{
			KindWood:  wood,
			KindWater: water,
		}
	}

	return w, nil
}

// findValidBuildingAngle searches the rim for the first angle where a building
// (TH or camp) can legally be placed. Used for authored echo layouts whose
// lakes or field geometry make simple angles invalid.
func findValidBuildingAngle(w *World) (float64, bool) {
	const steps = 120
	for i := 0; i < steps; i++ {
		a := normAngle(-math.Pi + float64(i)*2*math.Pi/steps)
		if buildPreview(w, a).Valid {
			return a, true
		}
	}
	return 0, false
}

// fillWoodFieldNodes spawns wood-field nodes until every known KindWood field is
// saturated. If leaveSpaceForOne is true, removes the last spawned node so
// exactly one more growth event will trigger saturation (and thus the mastery gate).
func fillWoodFieldNodes(w *World, leaveSpaceForOne bool) {
	startID := w.NextNodeID
	for _, f := range w.Planet.Fields {
		if f.Kind != KindWood || !f.Known {
			continue
		}
		for fieldCanSpawnNode(w, f) {
			spawnNode(w, f)
		}
	}
	if !leaveSpaceForOne {
		return
	}
	// Remove the last added node to re-open exactly one spawn slot.
	for i := len(w.Nodes) - 1; i >= 0; i-- {
		n := w.Nodes[i]
		if n.ID >= startID && n.Kind == KindWood {
			nid := n.ID
			for _, wk := range w.Workers {
				if wk.NodeID == nid {
					wk.NodeID = -1
				}
				if wk.TargetNodeID == nid {
					wk.TargetNodeID = -1
				}
				if wk.PendingNodeID == nid {
					wk.PendingNodeID = -1
				}
			}
			w.Nodes = append(w.Nodes[:i], w.Nodes[i+1:]...)
			break
		}
	}
}

// fillWaterFieldSparkles spawns interior sparkles in all known KindWater fields
// until no more can be placed (i.e., the field is saturated).
func fillWaterFieldSparkles(w *World) {
	innerR := w.Planet.Radius * sparkleInnerFrac
	outerR := w.Planet.Radius * sparkleOuterFrac
	const angularSteps = 16
	const radialSteps = 4
	for _, f := range w.Planet.Fields {
		if f.Kind != KindWater || !f.Known {
			continue
		}
		// Try exactly the 16×4 grid positions that waterFieldCanSpawnSparkle samples.
		// Placing sparkles at every valid grid position guarantees the gate check sees
		// no remaining valid positions and returns false (no golden-angle divergence).
		for ai := range angularSteps {
			angle := normAngle(f.CenterAngle - f.HalfArc + 2*f.HalfArc*float64(ai)/float64(angularSteps-1))
			for ri := range radialSteps {
				r := innerR + (outerR-innerR)*float64(ri)/float64(radialSteps-1)
				pos := Vec{
					X: w.Planet.Center.X + r*math.Cos(angle),
					Y: w.Planet.Center.Y + r*math.Sin(angle),
				}
				if sparkleSpawnPosValid(w, f, pos) {
					n := newSparkle(w, pos)
					w.Nodes = append(w.Nodes, n)
				}
			}
		}
	}
}

// clearFreeNodes removes nodes that are neither owned nor reserved by a worker.
func clearFreeNodes(w *World) {
	kept := w.Nodes[:0]
	for _, n := range w.Nodes {
		if n.OwnerID != -1 || n.ReservedByWorkerID != -1 {
			kept = append(kept, n)
		}
	}
	w.Nodes = kept
}
