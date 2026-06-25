package game

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

// abstractIncome returns total abstract wood/sec from all non-active producing
// planets. The active planet runs live (or is frozen in system view), so it is
// excluded when in planet view to avoid double-counting. Unawakened unknowns
// produce nothing; awakened unknowns (water frontier) can contribute AbstractRate.
func abstractIncome(w *World) float64 {
	var total float64
	for i, p := range w.System.Planets {
		if p.Kind == PlanetUnknown && !p.Awakened {
			continue
		}
		if w.System.View == ViewPlanet && i == w.Active {
			continue // active planet runs its live sim; skip abstract contribution
		}
		total += p.AbstractRate
	}
	return total
}

// abstractWaterIncome returns total abstract water/sec from all non-active producing planets.
// Mirrors abstractIncome but sums AbstractWaterRate instead of AbstractRate.
func abstractWaterIncome(w *World) float64 {
	var total float64
	for i, p := range w.System.Planets {
		if p.Kind == PlanetUnknown && !p.Awakened {
			continue
		}
		if w.System.View == ViewPlanet && i == w.Active {
			continue
		}
		total += p.AbstractWaterRate
	}
	return total
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

// checkActivePlanetCompletion detects when the active echo planet finishes and
// snapshots its amplified abstract rate, then fires a lightweight Town Hall pulse.
func checkActivePlanetCompletion(w *World) {
	p := &w.System.Planets[w.Active]
	if p.Kind != PlanetEcho || !p.Awakened || p.Completed {
		return
	}
	if !forestPlanetComplete(w) {
		return
	}
	awardCompletionPotential(w)
	p.AbstractRate = EstimateRate(w) * completionAmplifier
	p.Completed = true
	p.CompletedAt = w.SimTime
	if th := townHall(w); th != nil {
		activatePulse(w, &th.Pulse)
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
		if w.Economy.Potential[kind] < cost {
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
		w.Economy.Potential[kind] -= cost
	}
	p := &w.System.Planets[idx]
	p.Awakened = true
	if p.Kind == PlanetUnknown {
		w.PlanetStates[idx] = newWaterFrontierState()
	} else {
		p.LayoutID = p.RingColorIdx
		w.PlanetStates[idx] = newEchoPlanetState(p.LayoutID)
	}
}

// awardCompletionPotential grants 1 Potential token per distinct resource kind
// present on the active planet's fields. Called once on starting-planet unlock
// and once on each echo completion; the caller's one-shot flags prevent re-fire.
func awardCompletionPotential(w *World) {
	if w.Economy.Potential == nil {
		w.Economy.Potential = make(map[PotentialKind]int)
	}
	seen := make(map[ResourceKind]bool)
	for _, f := range w.Planet.Fields {
		if seen[f.Kind] {
			continue
		}
		seen[f.Kind] = true
		switch f.Kind {
		case KindWood:
			w.Economy.Potential[PotentialForest]++
		case KindWater:
			w.Economy.Potential[PotentialWater]++
		}
	}
}

// triggerUnlock snapshots the starting planet's analytic rate once, marks the
// system as unlocked, switches to system view, and selects the starting planet.
// Echo planet rates are also snapshotted as fractions of the starting rate with
// slight per-planet variance so they feel related but distinct.
// Must only be called when startingPlanetComplete is true.
func triggerUnlock(w *World) {
	awardCompletionPotential(w)
	base := EstimateRate(w)
	w.System.Planets[0].AbstractRate = base
	// Echoes are dormant — produce at a fraction of the completed planet's rate.
	// The two seeds give stable but different offsets: +5% and -5%.
	if len(w.System.Planets) > 1 {
		w.System.Planets[1].AbstractRate = base * echoRateFracA
	}
	if len(w.System.Planets) > 2 {
		w.System.Planets[2].AbstractRate = base * echoRateFracB
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
