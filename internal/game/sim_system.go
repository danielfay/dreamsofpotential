package game

import "math"

// forestPlanetComplete reports the mastery gate for forest-kind planets:
// town capacity is maxed AND every known KindWood region is saturated.
func forestPlanetComplete(w *World) bool {
	if !townFieldFull(w) {
		return false
	}
	hasKnownForest := false
	for _, f := range w.Planet.Fields {
		if f.Kind == KindWood && f.Known {
			hasKnownForest = true
			if fieldCanSpawnNode(w, f) {
				return false
			}
		}
	}
	return hasKnownForest
}

// waterPlanetComplete reports the mastery gate for the water frontier:
// town capacity is maxed AND every known production field is saturated
// (KindWood nodes + KindWater sparkles) AND at least one dock is Level 2.
func waterPlanetComplete(w *World) bool {
	if !townFieldFull(w) {
		return false
	}
	hasKnownField := false
	for _, f := range w.Planet.Fields {
		if !f.Known || f.Kind == KindWaterInfluence {
			continue
		}
		hasKnownField = true
		if f.Kind == KindWater {
			if waterFieldCanSpawnSparkle(w, f) {
				return false
			}
		} else if fieldCanSpawnNode(w, f) {
			return false
		}
	}
	if !hasKnownField {
		return false
	}
	for _, b := range w.Buildings {
		if b.Kind == KindDock && b.Level >= 2 {
			return true
		}
	}
	return false
}

// abstractRateSpec pairs a live rate estimator with the PlanetState field it updates.
// Add an entry here to track any new resource — the rolling-window update handles the rest.
type abstractRateSpec struct {
	estimate func(*World) float64
	getField func(*SystemPlanet) *float64
}

var activeAbstractRateSpecs = []abstractRateSpec{
	{EstimateRate, func(p *SystemPlanet) *float64 { return &p.AbstractRate }},
	{EstimateWaterRate, func(p *SystemPlanet) *float64 { return &p.AbstractWaterRate }},
}

// updateActiveAbstractRate samples each resource estimator into its own rolling
// bucket-min window and ratchets the planet's abstract rate upward (raise-only)
// when the sustained floor exceeds the stored value. Windows reset on planet
// change so pre-filled samples can't carry across enter/exit cycles (anti-fishing).
// Call only from the post-unlock planet-view branch of Tick.
func updateActiveAbstractRate(w *World, dt float64) {
	if len(w.abstractRateWins) != len(activeAbstractRateSpecs) {
		w.abstractRateWins = make([]abstractRateWindow, len(activeAbstractRateSpecs))
	}

	bucketSpan := abstractRateWindowSec / abstractRateBuckets
	p := &w.System.Planets[w.Active]

	for i, spec := range activeAbstractRateSpecs {
		win := &w.abstractRateWins[i]

		// Reset when the active planet has changed (or on first call).
		if win.planet != w.Active || len(win.buckets) == 0 {
			win.buckets = make([]float64, abstractRateBuckets)
			for j := range win.buckets {
				win.buckets[j] = 1e18 // sentinel: unwritten bucket never constrains min
			}
			win.idx = 0
			win.filled = 0
			win.elapsed = 0
			win.planet = w.Active
		}

		rate := spec.estimate(w)

		// Advance the bucket pointer when the current bucket's span has elapsed.
		win.elapsed += dt
		for win.elapsed >= bucketSpan {
			win.elapsed -= bucketSpan
			win.idx = (win.idx + 1) % abstractRateBuckets
			win.buckets[win.idx] = 1e18
			if win.filled < abstractRateBuckets {
				win.filled++
			}
		}

		// Fold the current rate into the active bucket's running minimum.
		if rate < win.buckets[win.idx] {
			win.buckets[win.idx] = rate
		}

		// Only update once every bucket has been written at least once.
		if win.filled < abstractRateBuckets {
			continue
		}

		// Window minimum = sustained floor over the full window.
		windowMin := win.buckets[0]
		for _, b := range win.buckets[1:] {
			if b < windowMin {
				windowMin = b
			}
		}

		field := spec.getField(p)
		if windowMin > *field {
			*field = windowMin
		}
	}
}

// systemWoodRate returns total wood/sec from all completed planets.
func systemWoodRate(w *World) float64 {
	var total float64
	for _, p := range w.System.Planets {
		if !p.Completed {
			continue
		}
		total += p.AbstractRate
	}
	return total
}

// systemWaterRate returns total water/sec from all completed planets.
func systemWaterRate(w *World) float64 {
	var total float64
	for _, p := range w.System.Planets {
		if !p.Completed {
			continue
		}
		total += p.AbstractWaterRate
	}
	return total
}

// tickSystemEconomy computes system-wide rates from completed planets and
// allocates them to Potential and Research accumulators.
func tickSystemEconomy(w *World, dt float64) {
	woodRate := systemWoodRate(w)
	waterRate := systemWaterRate(w)
	w.SystemEconomy.WoodRate = woodRate
	w.SystemEconomy.WaterRate = waterRate
	w.Economy.Potential[PotentialForest] += woodRate * w.SystemEconomy.WoodAllocPotential * dt
	w.Economy.Potential[PotentialWater] += waterRate * w.SystemEconomy.WaterAllocPotential * dt
	w.SystemEconomy.WoodResearch += woodRate * (1 - w.SystemEconomy.WoodAllocPotential) * dt
	w.SystemEconomy.WaterResearch += waterRate * (1 - w.SystemEconomy.WaterAllocPotential) * dt
}

// allEchoesComplete reports whether every echo planet in the system is completed.
func allEchoesComplete(w *World) bool {
	for _, p := range w.System.Planets {
		if p.Kind == PlanetEcho && !p.Completed {
			return false
		}
	}
	return true
}

// checkActivePlanetCompletion detects when the active planet finishes and
// snapshots its amplified abstract rate, then fires a lightweight Town Hall pulse.
// Handles both echo planets (PlanetEcho) and the water frontier (PlanetUnknown).
func checkActivePlanetCompletion(w *World) {
	p := &w.System.Planets[w.Active]
	if (p.Kind != PlanetEcho && p.Kind != PlanetUnknown) || !p.Awakened || p.Completed {
		return
	}
	isWaterFrontier := p.Kind == PlanetUnknown
	if isWaterFrontier {
		if !waterPlanetComplete(w) {
			return
		}
	} else {
		if !forestPlanetComplete(w) {
			return
		}
	}
	awardCompletionPotential(w)
	p.AbstractRate = EstimateRate(w) * completionAmplifier
	if isWaterFrontier {
		p.AbstractWaterRate = EstimateWaterRate(w) * completionAmplifier
	}
	p.Completed = true
	p.CompletedAt = w.SimTime
	if th := townHall(w); th != nil {
		activatePulse(w, &th.Pulse)
	}
}

// injectCirclePacket spends 1 whole Potential circle of kind on the active planet:
// deducts 1.0 from the fractional pool, activates the matching field family if
// not yet active, and grants a flat local resource packet. Returns false if the
// player cannot afford the circle.
func injectCirclePacket(w *World, kind PotentialKind) bool {
	if math.Floor(w.Economy.Potential[kind]) < 1 {
		return false
	}
	w.Economy.Potential[kind] -= 1.0
	switch kind {
	case PotentialForest:
		activateFieldFamily(w, KindWood)
		w.Economy.Wood += circlePacketWood
	case PotentialWater:
		activateFieldFamily(w, KindWater)
		w.Economy.Water += circlePacketWater
	}
	return true
}

// activateFieldFamily marks the first field of kind as Known on the active planet
// if none is already known. No-op if the family is already active.
func activateFieldFamily(w *World, kind ResourceKind) {
	for _, f := range w.Planet.Fields {
		if f.Kind == kind && !f.Known {
			f.Known = true
			return
		}
	}
}

// planetAwakenCost returns the Potential cost to awaken the planet at idx.
// Echo planets cost 1 Forest Potential; the unknown frontier costs 1 Forest + 1 Water Potential.
func planetAwakenCost(w *World, idx int) map[PotentialKind]int {
	if idx >= 0 && idx < len(w.System.Planets) && w.System.Planets[idx].Kind == PlanetUnknown {
		return map[PotentialKind]int{PotentialForest: 1, PotentialWater: 1}
	}
	return map[PotentialKind]int{PotentialForest: 1}
}

// canAwaken reports whether the planet at idx can be awakened right now.
// Echoes require 1 Forest Potential; the unknown frontier requires 1 Forest + 1 Water Potential.
func canAwaken(w *World, idx int) bool {
	if idx < 0 || idx >= len(w.System.Planets) {
		return false
	}
	p := w.System.Planets[idx]
	if p.Awakened {
		return false
	}
	if p.Kind != PlanetEcho && p.Kind != PlanetUnknown {
		return false
	}
	for kind, cost := range planetAwakenCost(w, idx) {
		if math.Floor(w.Economy.Potential[kind]) < float64(cost) {
			return false
		}
	}
	return true
}

// awakenPlanet spends the required Potential to awaken the planet at idx,
// creating its durable live state. The player stays in system view (no auto-zoom).
func awakenPlanet(w *World, idx int) {
	if !canAwaken(w, idx) {
		return
	}
	for kind, cost := range planetAwakenCost(w, idx) {
		w.Economy.Potential[kind] -= float64(cost)
	}
	p := &w.System.Planets[idx]
	p.Awakened = true
	if p.Kind == PlanetUnknown {
		w.PlanetStates[idx] = newWaterFrontierState()
		base := w.System.Planets[0].AbstractRate
		p.ProjectedRate = base * waterFrontierProjectedWoodFrac
		p.ProjectedWaterRate = base * waterFrontierProjectedWaterFrac
	} else {
		p.LayoutID = p.RingColorIdx
		w.PlanetStates[idx] = newEchoPlanetState(p.LayoutID)
	}
	// Bootstrap: seed baseline wood plus a packet for each circle spent on awakening.
	// Lives in parked state; materialises into Economy.Wood/Water when player enters.
	w.PlanetStates[idx].LocalWood += awakenBaselineWood
	for kind, cost := range planetAwakenCost(w, idx) {
		for range cost {
			switch kind {
			case PotentialForest:
				w.PlanetStates[idx].LocalWood += circlePacketWood
			case PotentialWater:
				w.PlanetStates[idx].LocalWater += circlePacketWater
			}
		}
	}
}

// awardCompletionPotential grants 1 Potential token per distinct resource kind
// present on the active planet's fields. Called once on starting-planet unlock
// and once on each echo completion; the caller's one-shot flags prevent re-fire.
func awardCompletionPotential(w *World) {
	if w.Economy.Potential == nil {
		w.Economy.Potential = make(map[PotentialKind]float64)
	}
	seen := make(map[ResourceKind]bool)
	for _, f := range w.Planet.Fields {
		if seen[f.Kind] {
			continue
		}
		seen[f.Kind] = true
		switch f.Kind {
		case KindWood:
			w.Economy.Potential[PotentialForest] += 1.0
		case KindWater:
			w.Economy.Potential[PotentialWater] += 1.0
		}
	}
}

// triggerUnlock snapshots the starting planet's analytic rate once, marks the
// system as unlocked, switches to system view, and selects the starting planet.
// Echo planets get a ProjectedRate (not AbstractRate) as fraction of the starting
// rate; they only gain real AbstractRate after checkActivePlanetCompletion fires.
// Must only be called when startingPlanetComplete is true.
func triggerUnlock(w *World) {
	awardCompletionPotential(w)
	base := EstimateRate(w)
	w.System.Planets[0].AbstractRate = base
	w.System.Planets[0].Completed = true
	// Echoes are dormant — show a projected rate; AbstractRate stays 0 until completion.
	if len(w.System.Planets) > 1 {
		w.System.Planets[1].ProjectedRate = base * echoRateFracA
	}
	if len(w.System.Planets) > 2 {
		w.System.Planets[2].ProjectedRate = base * echoRateFracB
	}
	w.System.Planets[0].CompletedAt = w.SimTime
	w.System.Unlocked = true
	w.System.View = ViewSystem
	w.System.Selected = 0
}

// enterSystemView switches to system view (freezes the live sim).
func enterSystemView(w *World) {
	w.System.View = ViewSystem
}

// enterPlanetView switches to planet view (resumes the live sim on next Tick).
func enterPlanetView(w *World) {
	w.System.View = ViewPlanet
}
