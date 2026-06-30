package game

// forestPlanetComplete reports the mastery gate for forest-kind planets:
// minimum completion population is reached AND every known KindWood region is saturated.
func forestPlanetComplete(w *World) bool {
	if !planetPopComplete(w) {
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
// minimum completion population is reached AND every known production field is saturated
// (KindWood nodes + KindWater sparkles) AND at least one dock is Level 2.
func waterPlanetComplete(w *World) bool {
	if !planetPopComplete(w) {
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

// updateActiveAbstractRate samples each resource estimator into its own rolling
// bucket-min window and updates the planet's abstract rate to the sustained floor.
// Rates can rise or fall, reflecting worker-ratio changes. Windows reset on planet
// change so pre-filled samples can't carry across enter/exit cycles (anti-fishing).
// Call only from the post-unlock planet-view branch of Tick.
func updateActiveAbstractRate(w *World, dt float64) {
	if len(w.abstractRateWins) != len(resourceFamilies) {
		w.abstractRateWins = make([]abstractRateWindow, len(resourceFamilies))
	}

	bucketSpan := abstractRateWindowSec / abstractRateBuckets
	p := &w.System.Planets[w.Active]

	for i, fam := range resourceFamilies {
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

		rate := fam.Estimate(w)

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

		*fam.AbstractRate(p) = windowMin
	}
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

// canAwaken reports whether the planet at idx can be manually awakened right now.
// A planet can be awakened if it is dormant and is an echo or frontier kind.
// Requirement-fill-based auto-awakening happens inside tickSystemChannels.
func canAwaken(w *World, idx int) bool {
	if idx < 0 || idx >= len(w.System.Planets) {
		return false
	}
	p := w.System.Planets[idx]
	return !p.Awakened && (p.Kind == PlanetEcho || p.Kind == PlanetUnknown)
}

// awakenPlanet creates the durable live state for the planet at idx.
// Does not require Potential; seeds awakenSeedWood into the new planet's stockpile.
func awakenPlanet(w *World, idx int) {
	if idx < 0 || idx >= len(w.System.Planets) {
		return
	}
	p := &w.System.Planets[idx]
	if p.Awakened {
		return
	}
	if p.Kind != PlanetEcho && p.Kind != PlanetUnknown {
		return
	}
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
	// Seed starting wood so the player can afford a camp immediately on entry.
	w.PlanetStates[idx].LocalWood += awakenSeedWood
}

// findChannel returns a pointer to the first channel with the given source and
// resource, or nil if none exists.
func findChannel(w *World, source int, resource ResourceKind) *Channel {
	for i := range w.System.Channels {
		c := &w.System.Channels[i]
		if c.Source == source && c.Resource == resource {
			return c
		}
	}
	return nil
}

// tickSystemChannels delivers resources along each channel and auto-awakens
// dormant targets whose requirements are fully met.
// Called unconditionally on every tick when the system is unlocked (same path as
// the old tickSystemEconomy) to preserve the invariant from TestSystemEconomyRunsInBothViews.
func tickSystemChannels(w *World, dt float64) {
	for i := range w.System.Channels {
		ch := &w.System.Channels[i]
		src := ch.Source
		tgt := ch.Target
		if src < 0 || src >= len(w.System.Planets) || tgt < 0 || tgt >= len(w.System.Planets) {
			continue
		}
		fam := familyForResource(ch.Resource)
		if fam == nil {
			continue
		}
		srcPlanet := &w.System.Planets[src]

		// Resolve source rate and stockpile.
		rate := *fam.AbstractRate(srcPlanet)
		if rate <= 0 {
			continue
		}
		var srcStockpile *float64
		if src == w.Active {
			srcStockpile = fam.Stockpile(&w.Economy)
		} else if ps := w.PlanetStates[src]; ps != nil {
			srcStockpile = fam.LocalStockpile(ps)
		}

		// Compute delivery; drain source only when stocked.
		var delivered float64
		if srcStockpile != nil && *srcStockpile > 0 {
			delivered = channelStockedFrac * rate * dt
			if delivered > *srcStockpile {
				delivered = *srcStockpile
			}
			*srcStockpile -= delivered
		} else {
			delivered = channelEmptyFrac * rate * dt
		}

		tgtPlanet := &w.System.Planets[tgt]

		// Apply delivery to target.
		if !tgtPlanet.Awakened {
			// Dormant: accumulate fill toward awakening requirement.
			fill := fam.AwakenFill(tgtPlanet)
			req := *fam.AwakenReq(tgtPlanet)
			*fill += delivered
			if req > 0 && *fill > req {
				*fill = req
			}
		} else {
			// Awakened or completed: deliver into local stockpile.
			// Skip water delivery to a target that hasn't discovered water yet.
			if ch.Resource == KindWater {
				var discovered bool
				if tgt == w.Active {
					discovered = w.Economy.WaterDiscovered
				} else if ps := w.PlanetStates[tgt]; ps != nil {
					discovered = ps.ResourceDiscovered
				}
				if !discovered {
					continue
				}
			}
			if tgt == w.Active {
				*fam.Stockpile(&w.Economy) += delivered
			} else if ps := w.PlanetStates[tgt]; ps != nil {
				*fam.LocalStockpile(ps) += delivered
			}
		}

		// Auto-awaken dormant target if all nonzero requirements are filled.
		if !tgtPlanet.Awakened {
			allMet := true
			for j := range resourceFamilies {
				f := &resourceFamilies[j]
				req := *f.AwakenReq(tgtPlanet)
				if req > 0 && *f.AwakenFill(tgtPlanet) < req {
					allMet = false
					break
				}
			}
			if allMet {
				awakenPlanet(w, tgt)
			}
		}
	}
}

// triggerUnlock snapshots the starting planet's analytic rate once, marks the
// system as unlocked, switches to system view, and selects the starting planet.
// Echo planets get a ProjectedRate (not AbstractRate) as fraction of the starting
// rate; they only gain real AbstractRate after checkActivePlanetCompletion fires.
// Must only be called when startingPlanetComplete is true.
func triggerUnlock(w *World) {
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
