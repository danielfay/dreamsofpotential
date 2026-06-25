package game

import "math"

// stepWorker advances one worker's state machine by dt seconds.
func stepWorker(w *World, wk *Worker, dt float64) {
	switch wk.State {
	case StateIdleWaiting:
		return
	case StateSettling:
		wk.Timer -= dt
		if wk.Timer <= 0 {
			wk.State = StateIdleWaiting
			wk.Timer = 0
		}
	case StateReactionDelay:
		wk.Timer -= dt
		if wk.Timer <= 0 {
			if workerShouldAbortKind(w, wk, KindWood) {
				clearTarget(wk)
				wk.State = StateIdleWaiting
				return
			}
			node := findNode(w, wk.TargetNodeID)
			if node == nil || !nodeFreeForWorker(w, node, wk.ID) {
				clearTarget(wk)
				wk.State = StateIdleWaiting
				return
			}
			startDeparture(w, wk, node)
		}
	case StateDeparturePulse:
		wk.Timer -= dt
		if wk.Timer <= 0 {
			wk.State = StateToRim
			wk.Timer = 0
		}
	case StateToRim:
		th := townHall(w)
		if th == nil {
			wk.State = StateToForest
			return
		}
		speed := workerSpeed
		if inLake(w, wk.Angle) {
			speed *= lakeSpeedFactor
		}
		if moveAlongArc(&wk.Angle, th.Angle, w.Planet.Radius, speed*dt) {
			wk.State = StateToForest
		}
	case StateToForest:
		if workerShouldAbortKind(w, wk, KindWood) {
			startReturnHome(w, wk)
			return
		}
		node := findNode(w, wk.NodeID)
		if node == nil {
			startReturnHome(w, wk)
			return
		}
		speed := workerSpeed
		if inLake(w, wk.Angle) {
			speed *= lakeSpeedFactor
		}
		if moveAlongArc(&wk.Angle, node.Angle, w.Planet.Radius, speed*dt) {
			node.OwnerID = wk.ID
			node.ReservedByWorkerID = -1
			wk.State = StateLoading
			wk.Timer = loadTime
			activatePulse(w, &wk.Pulse)
			activatePulse(w, &node.Pulse)
		}
	case StateLoading:
		node := findNode(w, wk.NodeID)
		if node == nil {
			startReturnHome(w, wk)
			return
		}
		wk.Timer -= dt
		if wk.Timer <= 0 {
			wk.Carried = baseLoadAmount * node.Size
			wk.State = StateToBuilding
		}
	case StateToBuilding:
		node := findNode(w, wk.NodeID)
		if node == nil {
			startReturnHome(w, wk)
			return
		}
		camp := nearestCamp(w, node)
		if camp == nil {
			startReturnHome(w, wk)
			return
		}
		speed := workerSpeed
		if inLake(w, wk.Angle) {
			speed *= lakeSpeedFactor
		}
		if moveAlongArc(&wk.Angle, camp.Angle, w.Planet.Radius, speed*dt) {
			wk.State = StateUnloading
			wk.Timer = unloadTime
			wk.DeliveryKind = camp.Kind
			activatePulse(w, &wk.Pulse)
			activatePulse(w, &camp.Pulse)
		}
	case StateUnloading:
		node := findNode(w, wk.NodeID)
		if node == nil {
			startReturnHome(w, wk)
			return
		}
		wk.Timer -= dt
		if wk.Timer <= 0 {
			completeUnload(w, wk, node)
		}
	case StateReturningHome:
		th := townHall(w)
		if th == nil {
			wk.State = StateIdleWaiting
			return
		}
		speed := workerSpeed
		if inLake(w, wk.Angle) {
			speed *= lakeSpeedFactor
		}
		if moveAlongArc(&wk.Angle, th.Angle, w.Planet.Radius, speed*dt) {
			wk.State = StateToIdleSpot
		}
	case StateToIdleSpot:
		wk.Timer -= dt
		if wk.Timer <= 0 {
			th := townHall(w)
			if th != nil {
				wk.Angle = th.Angle
			}
			wk.State = StateIdleWaiting
			wk.Timer = 0
		}
	case StateToDock:
		if workerShouldAbortKind(w, wk, KindWater) {
			wk.DockID = -1
			startReturnHome(w, wk)
			return
		}
		dock := findBuilding(w, wk.DockID)
		if dock == nil {
			wk.DockID = -1
			startReturnHome(w, wk)
			return
		}
		speed := workerSpeed
		if inLake(w, wk.Angle) {
			speed *= lakeSpeedFactor
		}
		if moveAlongArc(&wk.Angle, dock.Angle, w.Planet.Radius, speed*dt) {
			next := nextDiveSparkle(w, dock, wk)
			if next == nil {
				wk.DockID = -1
				startReturnHome(w, wk)
				return
			}
			wk.NodeID = next.ID
			wk.Pos = w.Planet.RimPoint(dock.Angle)
			wk.State = StateDiving
			activatePulse(w, &dock.Pulse)
		}
	case StateDiving:
		dock := findBuilding(w, wk.DockID)
		if dock == nil {
			releaseInteriorNodes(w, wk.ID)
			wk.NodeID = -1
			wk.DockID = -1
			wk.Carried = 0
			startReturnHome(w, wk)
			return
		}
		target := findNode(w, wk.NodeID)
		if target == nil || (target.OwnerID != -1 && target.OwnerID != wk.ID) {
			next := nextDiveSparkle(w, dock, wk)
			if next == nil {
				returnToDockFromDive(w, wk, dock)
				return
			}
			wk.NodeID = next.ID
			return
		}
		if moveStraightLine(&wk.Pos, target.Pos, workerSpeed*diveSpeedFactor*dt) {
			target.OwnerID = wk.ID
			target.ReservedByWorkerID = -1
			wk.State = StateDiveLoading
			wk.Timer = loadTime
			activatePulse(w, &wk.Pulse)
			activatePulse(w, &target.Pulse)
		}
	case StateDiveLoading:
		dock := findBuilding(w, wk.DockID)
		if dock == nil {
			releaseInteriorNodes(w, wk.ID)
			wk.NodeID = -1
			wk.DockID = -1
			wk.Carried = 0
			startReturnHome(w, wk)
			return
		}
		target := findNode(w, wk.NodeID)
		wk.Timer -= dt
		if wk.Timer <= 0 {
			if target != nil {
				wk.Carried += baseLoadAmount * target.Size
			}
			next := nextDiveSparkle(w, dock, wk)
			if next == nil {
				returnToDockFromDive(w, wk, dock)
				return
			}
			wk.NodeID = next.ID
			wk.State = StateDiving
		}
	case StateSwimmingToDock:
		dock := findBuilding(w, wk.DockID)
		if dock == nil {
			releaseInteriorNodes(w, wk.ID)
			wk.NodeID = -1
			wk.DockID = -1
			wk.Carried = 0
			startReturnHome(w, wk)
			return
		}
		if moveStraightLine(&wk.Pos, dock.Pos, workerSpeed*diveSpeedFactor*dt) {
			wk.Angle = dock.Angle
			wk.State = StateDockUnloading
			wk.Timer = unloadTime
			activatePulse(w, &dock.Pulse)
			activatePulse(w, &wk.Pulse)
		}
	case StateDockUnloading:
		wk.Timer -= dt
		if wk.Timer <= 0 {
			dock := findBuilding(w, wk.DockID)
			completeWaterUnload(w, wk, dock)
		}
	}
}

func startReaction(wk *Worker, node *ResourceNode) {
	wk.TargetNodeID = node.ID
	wk.State = StateReactionDelay
	wk.Timer = reactionDelay
}

func startDeparture(w *World, wk *Worker, node *ResourceNode) {
	node.ReservedByWorkerID = wk.ID
	wk.NodeID = node.ID
	wk.TargetNodeID = -1
	wk.State = StateDeparturePulse
	wk.Timer = microPulseTime
	activatePulse(w, &wk.Pulse)
}

func completeUnload(w *World, wk *Worker, node *ResourceNode) {
	gross := wk.Carried
	if gross > 0 {
		w.ResourceDiscovered = true
	}
	banked := gross * (1 - woodFieldReturnRatio)
	returned := gross * woodFieldReturnRatio
	w.Economy.Wood += banked
	w.lastDelivery = deliverySplit{Gross: gross, Banked: banked, Returned: returned}
	if b := nearestCamp(w, node); b != nil {
		b.DeliveredWood += gross
		b.DeliveryCount++
	}
	depositToField(w, node.Kind, returned)
	w.Economy.TownGrowth += gross
	tryConsumeGrowth(w)
	wk.Carried = 0

	if pending := findNode(w, wk.PendingNodeID); pending != nil && pending.ReservedByWorkerID == wk.ID {
		releaseOwnedNode(w, wk)
		wk.NodeID = -1
		wk.PendingNodeID = -1
		wk.TargetNodeID = pending.ID
		startDeparture(w, wk, pending)
		return
	}
	wk.PendingNodeID = -1
	if node.OwnerID == wk.ID && nearestCamp(w, node) != nil {
		wk.State = StateToForest
		wk.Timer = 0
		return
	}
	startReturnHome(w, wk)
}

func startReturnHome(w *World, wk *Worker) {
	releaseOwnedNode(w, wk)
	releaseInteriorNodes(w, wk.ID)
	releaseWorkerReservations(w, wk.ID)
	wk.TargetNodeID = -1
	wk.PendingNodeID = -1
	wk.DockID = -1
	wk.Carried = 0
	wk.State = StateReturningHome
	wk.Timer = 0.15
}

func releaseOwnedNode(w *World, wk *Worker) {
	if node := findNode(w, wk.NodeID); node != nil && node.OwnerID == wk.ID {
		node.OwnerID = -1
	}
	wk.NodeID = -1
}

func clearTarget(wk *Worker) {
	wk.TargetNodeID = -1
	wk.PendingNodeID = -1
}

// routeLen returns the effective arc distance from node to its nearest camp.
// Returns math.MaxFloat64 if no camps exist.
func routeLen(w *World, node *ResourceNode) float64 {
	camp := nearestCamp(w, node)
	if camp == nil {
		return math.MaxFloat64
	}
	return effectiveArc(w, node.Angle, camp.Angle)
}

// bestFreeNode returns the unclaimed/unreserved node with the shortest route to
// its nearest camp, excluding nodes currently targeted during reaction delay.
func bestFreeNode(w *World) *ResourceNode {
	var best *ResourceNode
	bestRoute := math.MaxFloat64
	for _, n := range w.Nodes {
		if !nodeFreeForWorker(w, n, -1) {
			continue
		}
		if r := routeLen(w, n); r < bestRoute {
			bestRoute = r
			best = n
		}
	}
	return best
}

func bestFreeNodeForKind(w *World, kind ResourceKind) *ResourceNode {
	var best *ResourceNode
	bestRoute := math.MaxFloat64
	for _, n := range w.Nodes {
		if n.Kind != kind {
			continue
		}
		if !nodeFreeForWorker(w, n, -1) {
			continue
		}
		if r := routeLen(w, n); r < bestRoute {
			bestRoute = r
			best = n
		}
	}
	return best
}

// assignFocusToIdleWorker updates wk.FocusedKind to match LaborFocus before
// assigning the worker a job. If LaborFocus is empty, the worker is unfocused.
// Counts are based on actual work states, not FocusedKind labels, so that
// multiple idle workers dispatched in the same tick each see accurate counts:
// a worker dispatched earlier in the loop is in an active state (StateToDock,
// StateToForest, …) and gets counted; an idle worker waiting to be processed
// does not, preventing all idle workers from seeing the same baseline.
func assignFocusToIdleWorker(w *World, wk *Worker) {
	if len(w.LaborFocus) == 0 {
		wk.FocusedKind = focusKindNone
		return
	}
	counts := activeWorkerCountsByKind(w)
	var bestKind ResourceKind = focusKindNone
	bestDeficit := 0
	for kind, target := range w.LaborFocus {
		if !focusKindHasAvailableWork(w, kind) {
			continue
		}
		deficit := target - counts[kind]
		if deficit > bestDeficit || (deficit == bestDeficit && bestKind != focusKindNone && kind > bestKind) {
			bestDeficit = deficit
			bestKind = kind
		}
	}
	// Overflow: all targets are met but there are more workers than the focus total
	// covers (worker spawned after the ratio was last saved). The UI always sets the
	// focus sum to the current worker count, so workers > focus_sum is unambiguous
	// evidence of a post-ratio spawn rather than intentional idle. Spill to the kind
	// with the largest positive target that has available work.
	if bestKind == focusKindNone {
		totalFocused := 0
		for _, t := range w.LaborFocus {
			totalFocused += t
		}
		if len(w.Workers) > totalFocused {
			bestTarget := 0
			for kind, target := range w.LaborFocus {
				if target <= 0 || !focusKindHasAvailableWork(w, kind) {
					continue
				}
				if target > bestTarget || (target == bestTarget && kind > bestKind) {
					bestTarget = target
					bestKind = kind
				}
			}
		}
	}
	wk.FocusedKind = bestKind
}

func focusKindHasAvailableWork(w *World, kind ResourceKind) bool {
	switch kind {
	case KindWood:
		return bestFreeNodeForKind(w, KindWood) != nil
	case KindWater:
		return bestFreeDock(w) != nil
	default:
		return false
	}
}

func activeWorkerCountsByKind(w *World) map[ResourceKind]int {
	counts := map[ResourceKind]int{}
	for _, wk := range w.Workers {
		if kind := activeWorkerKind(wk); kind != focusKindNone {
			counts[kind]++
		}
	}
	return counts
}

func activeWorkerKind(wk *Worker) ResourceKind {
	switch {
	case workerInWaterLoop(wk):
		return KindWater
	case workerInWoodAssignment(wk):
		return KindWood
	default:
		return focusKindNone
	}
}

func workerInWoodAssignment(wk *Worker) bool {
	switch wk.State {
	case StateReactionDelay, StateDeparturePulse, StateToRim:
		return wk.TargetNodeID != -1 || wk.NodeID != -1
	default:
		return workerInLoop(wk)
	}
}

func activeWorkerHUDCounts(w *World) (wood, water, idle int) {
	for _, wk := range w.Workers {
		switch activeWorkerKind(wk) {
		case KindWood:
			wood++
		case KindWater:
			water++
		default:
			idle++
		}
	}
	return wood, water, idle
}

// effectiveFocusTarget returns the worker-count ceiling for kind, accounting
// for post-ratio-save overflow. When workers were added after the ratio was
// last set (len(w.Workers) > focusSum), the dominant kind (largest positive
// target) absorbs the extras so they are not recalled or aborted mid-journey.
func effectiveFocusTarget(w *World, kind ResourceKind) int {
	target := w.LaborFocus[kind]
	focusSum := 0
	for _, t := range w.LaborFocus {
		focusSum += t
	}
	overflow := len(w.Workers) - focusSum
	if overflow <= 0 {
		return target
	}
	dominantKind := focusKindNone
	bestTarget := 0
	for k, t := range w.LaborFocus {
		if t > bestTarget || (t == bestTarget && k > dominantKind) {
			bestTarget = t
			dominantKind = k
		}
	}
	if kind == dominantKind {
		return target + overflow
	}
	return target
}

func reconcileLaborFocus(w *World) {
	if len(w.LaborFocus) == 0 {
		return
	}
	counts := activeWorkerCountsByKind(w)
	for _, wk := range w.Workers {
		kind := activeWorkerKind(wk)
		if kind == focusKindNone {
			continue
		}
		if counts[kind] <= effectiveFocusTarget(w, kind) {
			continue
		}
		startReturnHome(w, wk)
		wk.FocusedKind = focusKindNone
		counts[kind]--
	}
}

// assignFocusToNewWorker assigns a FocusedKind to a freshly spawned worker
// based on the current LaborFocus targets and existing worker counts.
func assignFocusToNewWorker(w *World, wk *Worker) {
	if len(w.LaborFocus) == 0 {
		return
	}
	counts := map[ResourceKind]int{}
	for _, other := range w.Workers {
		if other.ID == wk.ID || other.FocusedKind == focusKindNone {
			continue
		}
		counts[other.FocusedKind]++
	}
	var bestKind ResourceKind = focusKindNone
	bestDeficit := 0
	for kind, target := range w.LaborFocus {
		deficit := target - counts[kind]
		// Use > for strict improvement; tiebreak by preferring higher ResourceKind
		// value (KindWater > KindWood) so water workers are filled first on ties.
		if deficit > bestDeficit || (deficit == bestDeficit && bestKind != focusKindNone && kind > bestKind) {
			bestDeficit = deficit
			bestKind = kind
		}
	}
	// Overflow: all targets met. Pick the kind that best maintains the saved ratio
	// and extend LaborFocus so the sum stays equal to len(w.Workers).
	if bestKind == focusKindNone {
		bestKind = ratioBalancedKind(w)
		if bestKind != focusKindNone {
			w.LaborFocus[bestKind]++
		}
	}
	if bestKind != focusKindNone {
		wk.FocusedKind = bestKind
	}
}

// ratioBalancedKind returns the resource kind a new overflow worker should be
// assigned to in order to best maintain the player's saved ratio. It compares
// the ideal per-kind count (total workers × saved weight / total weight) against
// the current LaborFocus values and picks the most underrepresented kind.
// Ties resolve toward the lower ResourceKind value (KindWood = left side of the
// slider). Falls back to the dominant-target kind if no saved ratio exists.
func ratioBalancedKind(w *World) ResourceKind {
	src := w.SavedLaborRatio
	if len(src) == 0 {
		src = w.LaborFocus
	}
	ratioSum := 0
	for _, t := range src {
		ratioSum += t
	}
	if ratioSum == 0 {
		return focusKindNone
	}
	total := len(w.Workers)
	var bestKind ResourceKind = focusKindNone
	bestDeficit := -math.MaxFloat64
	for kind, weight := range src {
		if weight <= 0 {
			continue
		}
		ideal := float64(total) * float64(weight) / float64(ratioSum)
		deficit := ideal - float64(w.LaborFocus[kind])
		if deficit > bestDeficit || (deficit == bestDeficit && (bestKind == focusKindNone || kind < bestKind)) {
			bestDeficit = deficit
			bestKind = kind
		}
	}
	return bestKind
}

func nodeFreeForWorker(w *World, n *ResourceNode, workerID int) bool {
	if n.Interior {
		return false
	}
	if n.OwnerID != -1 {
		return false
	}
	if n.ReservedByWorkerID != -1 && n.ReservedByWorkerID != workerID {
		return false
	}
	for _, wk := range w.Workers {
		if wk.ID == workerID {
			continue
		}
		if wk.TargetNodeID == n.ID && (wk.State == StateReactionDelay || wk.State == StateDeparturePulse) {
			return false
		}
	}
	return true
}

func hasEligibleIdleWorker(w *World) bool {
	for _, wk := range w.Workers {
		if wk.State == StateIdleWaiting {
			return true
		}
	}
	return false
}

func reserveDelayedRebalance(w *World) {
	free := bestFreeNode(w)
	if free == nil {
		return
	}
	freeRoute := routeLen(w, free)
	var worst *Worker
	worstRoute := -1.0
	for _, wk := range w.Workers {
		if wk.PendingNodeID != -1 || !workerInLoop(wk) {
			continue
		}
		// Don't redirect a worker whose focus is incompatible with this node's kind.
		if wk.FocusedKind != focusKindNone && wk.FocusedKind != free.Kind {
			continue
		}
		node := findNode(w, wk.NodeID)
		if node == nil {
			continue
		}
		if r := routeLen(w, node); r > worstRoute {
			worstRoute = r
			worst = wk
		}
	}
	if worst == nil || freeRoute >= worstRoute {
		return
	}
	free.ReservedByWorkerID = worst.ID
	worst.PendingNodeID = free.ID
}

func workerInLoop(wk *Worker) bool {
	switch wk.State {
	case StateToForest, StateLoading, StateToBuilding, StateUnloading:
		return wk.NodeID != -1
	default:
		return false
	}
}

func releaseInvalidReservations(w *World) {
	for _, n := range w.Nodes {
		if n.ReservedByWorkerID == -1 {
			continue
		}
		wk := findWorker(w, n.ReservedByWorkerID)
		if wk == nil || (wk.PendingNodeID != n.ID && wk.NodeID != n.ID) {
			n.ReservedByWorkerID = -1
		}
	}
}

func releaseWorkerReservations(w *World, workerID int) {
	for _, n := range w.Nodes {
		if n.ReservedByWorkerID == workerID {
			n.ReservedByWorkerID = -1
		}
	}
}

// buildingAcceptsResource reports whether a building kind can receive deliveries
// for the given resource kind. Each camp type is paired to one resource;
// TownHall is the early-game fallback for wood.
func buildingAcceptsResource(bKind BuildingKind, rKind ResourceKind) bool {
	switch rKind {
	case KindWood:
		return bKind == KindTownHall || bKind == KindLoggingCamp
	case KindWater:
		return bKind == KindDock
	}
	return false
}

// nearestCamp returns the delivery building of the matching kind with the lowest
// effective arc cost to node's angle. Returns nil if no suitable building exists.
func nearestCamp(w *World, node *ResourceNode) *Building {
	var best *Building
	bestDist := math.MaxFloat64
	for _, b := range w.Buildings {
		if !buildingAcceptsResource(b.Kind, node.Kind) {
			continue
		}
		dist := effectiveArc(w, node.Angle, b.Angle)
		if dist < bestDist {
			bestDist = dist
			best = b
		}
	}
	return best
}

// moveStraightLine advances pos toward target by at most step world-units.
// Returns true and snaps when within reach.
func moveStraightLine(pos *Vec, target Vec, step float64) bool {
	dx := target.X - pos.X
	dy := target.Y - pos.Y
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist <= step {
		*pos = target
		return true
	}
	pos.X += dx / dist * step
	pos.Y += dy / dist * step
	return false
}

// moveAlongArc advances *angle toward targetAngle along the rim by at most
// step world-units of arc length. Returns true and snaps when within reach.
func moveAlongArc(angle *float64, targetAngle, radius, step float64) bool {
	delta := normAngle(targetAngle - *angle)
	arcRemaining := math.Abs(delta) * radius
	if arcRemaining <= step {
		*angle = targetAngle
		return true
	}
	stepAngle := step / radius
	if delta < 0 {
		stepAngle = -stepAngle
	}
	*angle = normAngle(*angle + stepAngle)
	return false
}

func updateWorkerPos(w *World, wk *Worker) {
	th := townHall(w)
	switch wk.State {
	case StateIdleWaiting, StateSettling, StateReactionDelay:
		// Idle-home presentation assigns visual slots in render.
		if th != nil {
			wk.Angle = th.Angle
		}
	case StateToIdleSpot:
		if th != nil {
			wk.Pos = insetPoint(w.Planet, th.Angle, 9)
			wk.Angle = th.Angle
			return
		}
	case StateDiving, StateDiveLoading, StateSwimmingToDock:
		return // interior position managed directly in stepWorker
	}
	wk.Pos = w.Planet.RimPoint(wk.Angle)
}

// ── Water harvesting helpers ──────────────────────────────────────────────────

// assignServicingDocks assigns each interior water sparkle to the nearest dock
// whose reach wedge contains it. Wedge membership requires both angular proximity
// (dockWedgeHalfArc) and radial depth: L1 docks reach 1/3 of the planet radius
// from the rim; L2 docks reach all the way to the center. Sparkles outside all
// wedges get ServicingDockID = -1. Called every Step tick.
func assignServicingDocks(w *World) {
	for _, n := range w.Nodes {
		if !n.Interior || n.Kind != KindWater {
			continue
		}
		depthFromRim := w.Planet.Radius - w.Planet.Center.Dist(n.Pos)
		var best *Building
		bestDist := math.MaxFloat64
		for _, b := range w.Buildings {
			if b.Kind != KindDock {
				continue
			}
			dist := angularDistance(n.Angle, b.Angle)
			if dist > dockWedgeHalfArc || dist >= bestDist {
				continue
			}
			maxDepth := w.Planet.Radius / 3
			if b.Level >= 2 {
				maxDepth = w.Planet.Radius
			}
			if depthFromRim > maxDepth {
				continue
			}
			bestDist = dist
			best = b
		}
		if best != nil {
			n.ServicingDockID = best.ID
		} else {
			n.ServicingDockID = -1
		}
	}
}

// dockServiceableSparkles returns all interior water sparkles assigned to dock.
func dockServiceableSparkles(w *World, dock *Building) []*ResourceNode {
	var result []*ResourceNode
	for _, n := range w.Nodes {
		if n.Interior && n.Kind == KindWater && n.ServicingDockID == dock.ID {
			result = append(result, n)
		}
	}
	return result
}

// workerShouldAbortKind returns true when LaborFocus says the worker has
// exceeded the quota for kind — enough other workers are already doing that
// resource's loop. Safe to call mid-trip; relies on sequential stepWorker
// processing so workers that already aborted this tick aren't counted.
func workerShouldAbortKind(w *World, wk *Worker, kind ResourceKind) bool {
	if len(w.LaborFocus) == 0 {
		return false
	}
	target := effectiveFocusTarget(w, kind)
	if target == 0 {
		return true
	}
	count := 0
	for _, other := range w.Workers {
		if other.ID == wk.ID {
			continue
		}
		switch kind {
		case KindWater:
			if workerInWaterLoop(other) {
				count++
			}
		case KindWood:
			if workerInLoop(other) {
				count++
			}
		}
	}
	return count >= target
}

// workerInWaterLoop reports whether a worker is in the dock→dive→unload cycle.
func workerInWaterLoop(wk *Worker) bool {
	switch wk.State {
	case StateToDock, StateDiving, StateDiveLoading, StateSwimmingToDock, StateDockUnloading:
		return true
	}
	return false
}

// dockFreeForWorker reports whether no water worker currently owns this dock.
func dockFreeForWorker(w *World, dock *Building) bool {
	for _, wk := range w.Workers {
		if wk.DockID == dock.ID && workerInWaterLoop(wk) {
			return false
		}
	}
	return true
}

// bestFreeDock returns a dock that has free serviceable sparkles and is not
// already claimed by another water worker, or nil if none qualifies.
func bestFreeDock(w *World) *Building {
	for _, b := range w.Buildings {
		if b.Kind != KindDock {
			continue
		}
		if !dockFreeForWorker(w, b) {
			continue
		}
		for _, n := range w.Nodes {
			if n.Interior && n.Kind == KindWater && n.ServicingDockID == b.ID && n.OwnerID == -1 {
				return b
			}
		}
	}
	return nil
}

// nextDiveSparkle returns the nearest unclaimed sparkle assigned to dock
// relative to the worker's current position; nil if none remain.
func nextDiveSparkle(w *World, dock *Building, wk *Worker) *ResourceNode {
	var best *ResourceNode
	bestDist := math.MaxFloat64
	for _, n := range w.Nodes {
		if !n.Interior || n.Kind != KindWater {
			continue
		}
		if n.ServicingDockID != dock.ID {
			continue
		}
		if n.OwnerID != -1 {
			continue
		}
		if d := wk.Pos.Dist(n.Pos); d < bestDist {
			bestDist = d
			best = n
		}
	}
	return best
}

// releaseInteriorNodes releases all interior sparkles owned by workerID.
func releaseInteriorNodes(w *World, workerID int) {
	for _, n := range w.Nodes {
		if n.Interior && n.OwnerID == workerID {
			n.OwnerID = -1
		}
	}
}

// startWaterDeparture transitions an idle worker to StateToDock for dock.
func startWaterDeparture(w *World, wk *Worker, dock *Building) {
	wk.DockID = dock.ID
	wk.DeliveryKind = KindDock
	wk.State = StateToDock
	wk.Timer = 0
	activatePulse(w, &wk.Pulse)
}

// returnToDockFromDive starts the worker swimming back to the dock from wherever
// they are in the interior. The worker's Pos is preserved; movement happens in
// StateSwimmingToDock.
func returnToDockFromDive(w *World, wk *Worker, dock *Building) {
	wk.NodeID = -1
	wk.State = StateSwimmingToDock
	activatePulse(w, &wk.Pulse)
}

// completeWaterUnload deposits carried water, reveals Water on first delivery,
// releases owned sparkles, then either starts another dive or returns home.
func completeWaterUnload(w *World, wk *Worker, dock *Building) {
	gross := wk.Carried
	wasDiscovered := w.Economy.WaterDiscovered
	if gross > 0 {
		w.Economy.WaterDiscovered = true
	}
	// On first water delivery: reveal unknown water fields on all planets so
	// previously-teased lakes become real (enabling nurture and progression).
	// Also auto-set a labor split if the player hasn't configured one yet.
	if !wasDiscovered && w.Economy.WaterDiscovered {
		revealKindFields(w, KindWater)
		if len(w.LaborFocus) == 0 && dockHasServiceableSparkles(w) {
			setAutoProofSplit(w)
		}
	}
	banked := gross * (1 - woodFieldReturnRatio)
	returned := gross * woodFieldReturnRatio
	w.Economy.Water += banked
	if dock != nil {
		dock.DeliveredWood += gross
		dock.DeliveryCount++
		activatePulse(w, &dock.Pulse)
	}
	depositToField(w, KindWater, returned)
	wk.Carried = 0
	releaseInteriorNodes(w, wk.ID)

	if dock != nil && !workerShouldAbortKind(w, wk, KindWater) {
		if next := nextDiveSparkle(w, dock, wk); next != nil {
			wk.NodeID = next.ID
			wk.Pos = dock.Pos
			wk.State = StateDiving
			return
		}
	}
	wk.DockID = -1
	startReturnHome(w, wk)
}

// dockHasServiceableSparkles reports whether any dock has at least one
// sparkle that can be assigned to it (reachable dock work exists).
func dockHasServiceableSparkles(w *World) bool {
	for _, b := range w.Buildings {
		if b.Kind != KindDock {
			continue
		}
		if len(dockServiceableSparkles(w, b)) > 0 {
			return true
		}
	}
	return false
}

// setAutoProofSplit initialises LaborFocus to 1 water + rest wood worker,
// preserving any subsequent manual overrides (only called when LaborFocus is nil).
func setAutoProofSplit(w *World) {
	total := len(w.Workers)
	if total == 0 {
		return
	}
	wood := total - 1
	if wood < 0 {
		wood = 0
	}
	w.LaborFocus = map[ResourceKind]int{
		KindWater: 1,
		KindWood:  wood,
	}
}
