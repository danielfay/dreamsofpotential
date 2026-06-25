package game

// Step advances the simulation by dt seconds (called at 60 TPS, dt = 1/60).
func Step(w *World, dt float64) {
	w.SimTime += dt
	tickPulses(w, dt)
	tickGrowthCue(w, dt)
	tickOverflowGrowth(w)
	assignServicingDocks(w)
	assignNodes(w)
	for _, wk := range w.Workers {
		stepWorker(w, wk, dt)
		updateWorkerPos(w, wk)
	}
}

// assignNodes runs every tick. Eligible idle workers get first claim on free
// nodes. If all workers are busy, the longest-route active worker may reserve a
// better free node and switch only at its next unload checkpoint.
func assignNodes(w *World) {
	if len(w.Buildings) == 0 {
		return
	}
	releaseInvalidReservations(w)
	reconcileLaborFocus(w)

	assignedIdle := false
	for _, wk := range w.Workers {
		if wk.State != StateIdleWaiting {
			continue
		}
		assignFocusToIdleWorker(w, wk)
		switch wk.FocusedKind {
		case KindWood:
			if node := bestFreeNodeForKind(w, KindWood); node != nil {
				startReaction(wk, node)
				assignedIdle = true
			}
			// else: stay idle at Town Hall (focus-gated)
		case KindWater:
			if dock := bestFreeDock(w); dock != nil {
				startWaterDeparture(w, wk, dock)
				assignedIdle = true
			}
			// else: stay idle at Town Hall (focus-gated)
		default:
			if len(w.LaborFocus) > 0 {
				// Ratio is active but every eligible target is full or unavailable.
				continue
			}
			if node := bestFreeNode(w); node != nil {
				startReaction(wk, node)
				assignedIdle = true
			} else if dock := bestFreeDock(w); dock != nil {
				startWaterDeparture(w, wk, dock)
				assignedIdle = true
			}
		}
	}
	if assignedIdle || hasEligibleIdleWorker(w) {
		return
	}
	reserveDelayedRebalance(w)
}

// Tick is the per-frame world advance called by Game.Update instead of Step.
// It gates the live sim by view mode and detects first-planet completion.
// Returns true exactly once: on the tick that triggers the unlock reveal.
func Tick(w *World, dt float64) (justUnlocked bool) {
	if w.System.Unlocked && w.System.View == ViewSystem {
		// System view: live sim is frozen; abstract producers add wood and water.
		w.Economy.Wood += abstractIncome(w) * dt
		w.Economy.Water += abstractWaterIncome(w) * dt
		return false
	}
	// Planet view (or pre-unlock): run the live sim.
	Step(w, dt)
	if w.System.Unlocked {
		// Post-unlock planet view: abstract income + check for echo completion + rate ratchet.
		w.Economy.Wood += abstractIncome(w) * dt
		w.Economy.Water += abstractWaterIncome(w) * dt
		checkActivePlanetCompletion(w)
		updateActiveAbstractRate(w, dt)
		return false
	}
	// Pre-unlock: check mastery gate exactly once.
	if forestPlanetComplete(w) {
		triggerUnlock(w)
		return true
	}
	return false
}

// ── Shared lookup helpers ─────────────────────────────────────────────────────

// findNode looks up a node by ID.
func findNode(w *World, id int) *ResourceNode {
	for _, n := range w.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// findBuilding looks up a building by ID.
func findBuilding(w *World, id int) *Building {
	for _, b := range w.Buildings {
		if b.ID == id {
			return b
		}
	}
	return nil
}

func findWorker(w *World, id int) *Worker {
	for _, wk := range w.Workers {
		if wk.ID == id {
			return wk
		}
	}
	return nil
}

// ── Pulse helpers ─────────────────────────────────────────────────────────────

func activatePulse(w *World, p *PulseState) {
	if w.SimTime-p.LastActivated < pulseMinInterval {
		p.Remaining = 0
		p.SteadyUntil = w.SimTime + pulseMinInterval
	} else {
		p.Remaining = microPulseTime
	}
	p.LastActivated = w.SimTime
}

func tickPulses(w *World, dt float64) {
	for _, wk := range w.Workers {
		tickPulse(&wk.Pulse, dt)
	}
	for _, n := range w.Nodes {
		tickPulse(&n.Pulse, dt)
	}
	for _, b := range w.Buildings {
		tickPulse(&b.Pulse, dt)
	}
}

func tickPulse(p *PulseState, dt float64) {
	if p.Remaining > 0 {
		tickTimer(&p.Remaining, dt)
	}
}

func tickTimer(v *float64, dt float64) {
	if *v <= 0 {
		return
	}
	*v -= dt
	if *v < 0 {
		*v = 0
	}
}

func pulseActive(w *World, p PulseState) bool {
	return p.Remaining > 0 || p.SteadyUntil > w.SimTime
}
