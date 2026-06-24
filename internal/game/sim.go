package game

import "math"

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

	assignedIdle := false
	for _, wk := range w.Workers {
		if wk.State != StateIdleWaiting {
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
	if assignedIdle || hasEligibleIdleWorker(w) {
		return
	}
	reserveDelayedRebalance(w)
}

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

func nodeFreeForWorker(w *World, n *ResourceNode, workerID int) bool {
	if n.Interior {
		return false // interior sparkles can't be worked by rim workers until Phase 5
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

func findWorker(w *World, id int) *Worker {
	for _, wk := range w.Workers {
		if wk.ID == id {
			return wk
		}
	}
	return nil
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

// assignServicingDocks assigns each interior water sparkle to the nearest dock
// whose reach wedge (dockWedgeHalfArc) contains it. Sparkles outside all wedges
// get ServicingDockID = -1. Called every Step tick.
func assignServicingDocks(w *World) {
	for _, n := range w.Nodes {
		if !n.Interior || n.Kind != KindWater {
			continue
		}
		var best *Building
		bestDist := math.MaxFloat64
		for _, b := range w.Buildings {
			if b.Kind != KindDock {
				continue
			}
			dist := angularDistance(n.Angle, b.Angle)
			if dist <= dockWedgeHalfArc && dist < bestDist {
				bestDist = dist
				best = b
			}
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
	if gross > 0 {
		w.Economy.WaterDiscovered = true
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

	if dock != nil {
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

// depositToField increments the planet-level field EXP for kind and spawns a
// new node each time EXP meets or exceeds the cap. The spawn target region is
// chosen by pickGrowthRegion (random among eligible known regions of that kind).
func depositToField(w *World, kind ResourceKind, amount float64) {
	fp := w.Planet.FieldProgress[kind]
	if fp == nil {
		return
	}
	fp.EXP += amount
	for fp.EXP >= fp.Cap {
		fp.EXP -= fp.Cap
		// Capped geometric: grow the threshold exponentially while the step is
		// small, then switch to additive so late-game trees stay naturally reachable.
		if step := fp.Cap * (woodFieldEXPGrowth - 1); step < woodFieldEXPMaxStep {
			fp.Cap *= woodFieldEXPGrowth
		} else {
			fp.Cap += woodFieldEXPMaxStep
		}
		f := pickGrowthRegion(w, kind)
		if f == nil {
			break
		}
		var result growthResult
		if kind == KindWater {
			result = spawnSparkle(w, f)
		} else {
			result = spawnNode(w, f)
		}
		activateGrowthCue(w, result)
	}
}

// pickGrowthRegion selects a known region of the given kind to receive a new
// spawn. Prefers regions that can still accept a node/sparkle; falls back to
// any known region when all are saturated (spawnNode/spawnSparkle handles the
// upgrade path).
func pickGrowthRegion(w *World, kind ResourceKind) *ResourceField {
	var eligible []*ResourceField
	var fallback *ResourceField
	for _, f := range w.Planet.Fields {
		if f.Kind != kind || !f.Known {
			continue
		}
		if fallback == nil {
			fallback = f
		}
		var canSpawn bool
		if kind == KindWater {
			canSpawn = waterFieldCanSpawnSparkle(w, f)
		} else {
			canSpawn = fieldCanSpawnNode(w, f)
		}
		if canSpawn {
			eligible = append(eligible, f)
		}
	}
	if len(eligible) > 0 {
		return eligible[w.rng.Intn(len(eligible))]
	}
	return fallback
}

func activateGrowthCue(w *World, result growthResult) {
	if result.Outcome == growthOutcomeNone {
		return
	}
	cue := growthCueState{
		Outcome:         result.Outcome,
		Kind:            result.Kind,
		CenterAngle:     result.CenterAngle,
		HalfArc:         result.HalfArc,
		NodeID:          result.NodeID,
		WaterInfluenced: result.WaterInfluenced,
		GaugeRelease:    growthGaugeReleaseTime,
		GaugeAfterglow:  growthGaugeAfterglowTime,
		FieldDelay:      growthFieldPulseDelay,
		FieldPulse:      growthFieldPulseTime,
		NodeDelay:       growthNodeCueDelay,
		NodeCue:         growthNodeCueTime,
	}
	if growthCueActive(w.growthCue) {
		w.pendingGrowthCues = append(w.pendingGrowthCues, cue)
		return
	}
	w.growthCue = cue
}

func upgradeAllFieldsForDebug(w *World) bool {
	fp := w.Planet.FieldProgress[KindWood]
	if fp == nil {
		return false
	}
	amount := fp.Cap - fp.EXP
	if amount <= 0 {
		amount = fp.Cap
	}
	depositToField(w, KindWood, amount)
	return true
}

func growAllFieldsUntilBlockedForDebug(w *World) bool {
	const maxDebugGrowthSteps = 512
	for i := 0; i < maxDebugGrowthSteps; i++ {
		before := len(w.Nodes)
		if !upgradeAllFieldsForDebug(w) {
			return false
		}
		if len(w.Nodes) == before {
			return true
		}
	}
	return true
}

// nurtureGrowthCuePending reports whether a growth cue is currently playing or
// queued. Nurture is blocked while cues are pending so each press has
// unambiguous visual feedback before the next can fire.
func nurtureGrowthCuePending(w *World) bool {
	return growthCueActive(w.growthCue) || len(w.pendingGrowthCues) > 0
}

// nurtureField directly spawns up to nurtureTreesPerPress new nodes across all
// known regions of the given kind. Returns false if the resource is not yet
// discovered, no known region exists, or a growth cue is already playing.
// For KindWater, spawnSparkle handles interior placement; saturation upgrades.
func nurtureField(w *World, kind ResourceKind) bool {
	if !w.ResourceDiscovered || nurtureGrowthCuePending(w) {
		return false
	}
	f := pickGrowthRegion(w, kind)
	if f == nil {
		return false
	}
	if kind == KindWater {
		for range nurtureTreesPerPress {
			f = pickGrowthRegion(w, kind)
			if f == nil {
				break
			}
			activateGrowthCue(w, spawnSparkle(w, f))
		}
		return true
	}
	if !fieldCanSpawnNode(w, f) {
		return false
	}
	for range nurtureTreesPerPress {
		f = pickGrowthRegion(w, kind)
		if f == nil || !fieldCanSpawnNode(w, f) {
			break
		}
		activateGrowthCue(w, spawnNode(w, f))
	}
	return true
}

// nurtureAttentionActive reports whether the Nurture button should show its
// attention pulse. Fires once all worker slots are both purchased AND filled
// with physical workers, any known region of that kind can still accept a tree,
// and no growth cue is pending.
func nurtureAttentionActive(w *World, kind ResourceKind) bool {
	if !w.ResourceDiscovered || nurtureGrowthCuePending(w) {
		return false
	}
	f := pickGrowthRegion(w, kind)
	if f == nil || !fieldCanSpawnNode(w, f) {
		return false
	}
	slots := maxTownSlots(w)
	return slots > 0 && townFieldFull(w) && len(w.Workers) >= slots
}

// EstimateRate returns the analytic resource/sec for all active workers.
func EstimateRate(w *World) float64 {
	var rate float64
	for _, wk := range w.Workers {
		if !workerInLoop(wk) {
			continue
		}
		node := findNode(w, wk.NodeID)
		if node == nil {
			continue
		}
		dist := routeLen(w, node)
		if dist == math.MaxFloat64 {
			continue
		}
		tripTime := loadTime + unloadTime + 2*dist/workerSpeed
		rate += (baseLoadAmount * node.Size * (1 - woodFieldReturnRatio)) / tripTime
	}
	return rate
}

// EstimateWaterRate returns the analytic water/sec for active water workers.
func EstimateWaterRate(w *World) float64 {
	var rate float64
	th := townHall(w)
	if th == nil {
		return 0
	}
	for _, wk := range w.Workers {
		if !workerInWaterLoop(wk) {
			continue
		}
		dock := findBuilding(w, wk.DockID)
		if dock == nil {
			continue
		}
		sparkles := dockServiceableSparkles(w, dock)
		if len(sparkles) == 0 {
			continue
		}
		var totalCarry float64
		for _, s := range sparkles {
			totalCarry += baseLoadAmount * s.Size
		}
		rimDist := effectiveArc(w, th.Angle, dock.Angle) * 2
		diveDist := math.Sqrt(float64(len(sparkles))) * 15.0
		tripTime := (rimDist+diveDist)/workerSpeed + unloadTime
		if tripTime > 0 {
			rate += totalCarry * (1 - woodFieldReturnRatio) / tripTime
		}
	}
	return rate
}

func activeWorkerCount(w *World) int {
	active := 0
	for _, wk := range w.Workers {
		if workerInLoop(wk) {
			active++
		}
	}
	return active
}

// availableCapacity returns the number of unused worker slots (capacity minus
// current worker count). Clamped to 0 so debug free-spawns past capacity
// do not yield a negative value that could trigger spurious growth spawns.
func availableCapacity(w *World) int {
	avail := w.Economy.WorkerCapacity - len(w.Workers)
	if avail < 0 {
		return 0
	}
	return avail
}

// townFieldSlots returns the world positions of every potential dwelling slot
// inside the town field, anchored to th.Angle and stepping inward by rows.
// len() of the result is the planet's max worker capacity.
// Returns nil if th is nil.
func townFieldSlots(p Planet, th *Building) []Vec {
	if th == nil {
		return nil
	}
	cos := math.Cos(th.Angle)
	sin := math.Sin(th.Angle)
	inx, iny := -cos, -sin // inward unit vector (toward planet center)
	tx, ty := -sin, cos    // tangent unit vector (counterclockwise along rim)

	rim := p.RimPoint(th.Angle)
	maxDepth := townFieldDepthFrac * p.Radius

	var slots []Vec
	for d := townFieldRimInset; d <= maxDepth; d += townFieldSlotSpacing {
		arcLen := 2 * (p.Radius - d) * townFieldHalfArc
		cols := int(arcLen / townFieldSlotSpacing)
		if cols == 0 {
			continue
		}
		for order := 0; order < cols; order++ {
			col := townFieldColumnIndex(order, cols)
			tangOffset := (float64(col) - float64(cols-1)/2) * townFieldSlotSpacing
			slots = append(slots, Vec{
				X: rim.X + inx*d + tx*tangOffset,
				Y: rim.Y + iny*d + ty*tangOffset,
			})
		}
	}
	return slots
}

func townFieldColumnIndex(order, cols int) int {
	centerLeft := (cols - 1) / 2
	if order == 0 {
		return centerLeft
	}
	step := (order + 1) / 2
	if order%2 == 1 {
		return centerLeft + step
	}
	return centerLeft - step
}

// maxTownSlots returns the geometry-derived maximum worker capacity for the
// current planet. Returns 0 if no Town Hall exists.
func maxTownSlots(w *World) int {
	return len(townFieldSlots(w.Planet, townHall(w)))
}

// townFieldFull reports whether the town field has no remaining dwelling slots
// to build — capacity purchases are blocked when this returns true.
func townFieldFull(w *World) bool {
	return townHall(w) != nil && w.Economy.WorkerCapacity >= maxTownSlots(w)
}

// townCapacityCost returns the wood cost of the next paid capacity slot.
func townCapacityCost(w *World) float64 {
	return townCapacityBaseCost * math.Pow(townCapacityCostGrowth, float64(w.Economy.CapacityBought))
}

// addFreeCapacity unlocks one worker slot without spending wood. Debug-only.
func addFreeCapacity(w *World) bool {
	if townHall(w) == nil {
		return false
	}
	if w.Economy.WorkerCapacity >= maxTownSlots(w) {
		return false
	}
	w.Economy.WorkerCapacity++
	w.Economy.CapacityBought++
	tryConsumeGrowth(w)
	return true
}

// buildTownCapacity spends wood to unlock one worker slot. Calls
// tryConsumeGrowth so a worker arrives immediately if growth is already full.
func buildTownCapacity(w *World) bool {
	if townHall(w) == nil {
		return false
	}
	if w.Economy.WorkerCapacity >= maxTownSlots(w) {
		return false
	}
	cost := townCapacityCost(w)
	if w.Economy.Wood < cost {
		return false
	}
	w.Economy.Wood -= cost
	w.Economy.WorkerCapacity++
	w.Economy.CapacityBought++
	tryConsumeGrowth(w)
	return true
}

// tryConsumeGrowth spawns at most one worker when Town Growth has reached its
// cap and a slot is free, then resets growth and raises the cap.
//
// When all slots are filled but more can be purchased, excess growth is banked
// in TownGrowthOverflow instead of discarded; draining happens in
// tickOverflowGrowth and after each spawn.
//
// When capacity is permanently full (townFieldFull && no available slots),
// growth is clamped at cap and overflow is cleared.
//
// A workerSpawnCooldown prevents overflow from triggering rapid-fire spawns.
func tryConsumeGrowth(w *World) bool {
	if w.Economy.TownGrowth < w.Economy.TownGrowthCap {
		return false
	}
	if availableCapacity(w) <= 0 {
		if townFieldFull(w) {
			// All slots bought and all workers spawned: clear overflow, stop tracking.
			w.Economy.TownGrowth = w.Economy.TownGrowthCap
			w.Economy.TownGrowthOverflow = 0
		} else {
			// Capacity full but more houses can still be purchased: bank overflow.
			if excess := w.Economy.TownGrowth - w.Economy.TownGrowthCap; excess > 0 {
				w.Economy.TownGrowthOverflow += excess
			}
			w.Economy.TownGrowth = w.Economy.TownGrowthCap
		}
		return false
	}
	// Enforce minimum gap between spawns so overflow doesn't burst all at once.
	if w.Economy.LastWorkerSpawnTime > 0 && w.SimTime-w.Economy.LastWorkerSpawnTime < workerSpawnCooldown {
		w.Economy.TownGrowth = w.Economy.TownGrowthCap
		return false
	}
	// Bank any excess above the cap before resetting.
	if excess := w.Economy.TownGrowth - w.Economy.TownGrowthCap; excess > 0 {
		w.Economy.TownGrowthOverflow += excess
	}
	th := townHall(w)
	if spawnWorkerAtTownHall(w) == nil {
		return false
	}
	w.Economy.LastWorkerSpawnTime = w.SimTime
	if th != nil {
		activatePulse(w, &th.Pulse)
	}
	// Transition from the scripted first-lesson cap to the normal-play ramp.
	// After that, grow geometrically like every other fill.
	if w.Economy.TownGrowthCap < townGrowthBaseCap {
		w.Economy.TownGrowthCap = townGrowthBaseCap
	} else {
		w.Economy.TownGrowthCap *= townGrowthCapGrowth
	}
	// Immediately drain overflow into the fresh gauge so the bar visually
	// refills and the next spawn is ready once the cooldown expires.
	w.Economy.TownGrowth = 0
	if w.Economy.TownGrowthOverflow > 0 {
		drain := math.Min(w.Economy.TownGrowthOverflow, w.Economy.TownGrowthCap)
		w.Economy.TownGrowth = drain
		w.Economy.TownGrowthOverflow -= drain
	}
	return true
}

// tickOverflowGrowth drains banked overflow into the growth gauge each tick
// and tries to spawn once the cooldown has expired. This allows overflow-driven
// spawns to proceed even when no delivery is incoming.
func tickOverflowGrowth(w *World) {
	if availableCapacity(w) <= 0 {
		return
	}
	if w.Economy.TownGrowthOverflow > 0 {
		if needed := w.Economy.TownGrowthCap - w.Economy.TownGrowth; needed > 0 {
			drain := math.Min(w.Economy.TownGrowthOverflow, needed)
			w.Economy.TownGrowth += drain
			w.Economy.TownGrowthOverflow -= drain
		}
	}
	tryConsumeGrowth(w)
}

func addFreeWorkerAtTownHall(w *World) bool {
	return spawnWorkerAtTownHall(w) != nil
}

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

func tickGrowthCue(w *World, dt float64) {
	if !growthCueActive(w.growthCue) {
		startNextGrowthCue(w)
	}
	tickTimer(&w.growthCue.GaugeRelease, dt)
	tickTimer(&w.growthCue.GaugeAfterglow, dt)
	if w.growthCue.FieldDelay > 0 {
		tickTimer(&w.growthCue.FieldDelay, dt)
	} else {
		tickTimer(&w.growthCue.FieldPulse, dt)
	}
	if w.growthCue.NodeDelay > 0 {
		tickTimer(&w.growthCue.NodeDelay, dt)
	} else {
		tickTimer(&w.growthCue.NodeCue, dt)
	}
	if growthCueTimersDone(w.growthCue) {
		w.growthCue = growthCueState{NodeID: -1}
		startNextGrowthCue(w)
	}
}

func growthCueActive(c growthCueState) bool {
	return c.Outcome != growthOutcomeNone || !growthCueTimersDone(c)
}

func growthCueTimersDone(c growthCueState) bool {
	return c.GaugeRelease == 0 &&
		c.GaugeAfterglow == 0 &&
		c.FieldDelay == 0 &&
		c.FieldPulse == 0 &&
		c.NodeDelay == 0 &&
		c.NodeCue == 0
}

func startNextGrowthCue(w *World) {
	if len(w.pendingGrowthCues) == 0 {
		return
	}
	w.growthCue = w.pendingGrowthCues[0]
	copy(w.pendingGrowthCues, w.pendingGrowthCues[1:])
	w.pendingGrowthCues = w.pendingGrowthCues[:len(w.pendingGrowthCues)-1]
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

// ── System-view / unlock helpers ─────────────────────────────────────────────

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

// updateActiveAbstractRate samples EstimateRate into a rolling bucket-min window and
// ratchets AbstractRate upward (raise-only) when the sustained floor exceeds the stored
// value. The window resets on planet change so pre-filled samples can't carry across
// enter/exit cycles (anti-fishing). Call only from the post-unlock planet-view branch of Tick.
func updateActiveAbstractRate(w *World, dt float64) {
	win := &w.abstractRateWin
	bucketSpan := abstractRateWindowSec / abstractRateBuckets

	// Reset when the active planet has changed (or on first call).
	if win.planet != w.Active || len(win.buckets) == 0 {
		win.buckets = make([]float64, abstractRateBuckets)
		for i := range win.buckets {
			win.buckets[i] = 1e18 // sentinel: unwritten bucket never constrains min
		}
		win.idx = 0
		win.filled = 0
		win.elapsed = 0
		win.planet = w.Active
	}

	rate := EstimateRate(w)

	// Advance the bucket pointer when the current bucket's span has elapsed.
	win.elapsed += dt
	for win.elapsed >= bucketSpan {
		win.elapsed -= bucketSpan
		win.idx = (win.idx + 1) % abstractRateBuckets
		// Overwrite the oldest bucket with a fresh sentinel before accumulating.
		win.buckets[win.idx] = 1e18
		if win.filled < abstractRateBuckets {
			win.filled++
		}
	}

	// Fold the current rate into the active bucket's running minimum.
	if rate < win.buckets[win.idx] {
		win.buckets[win.idx] = rate
	}

	// Only update AbstractRate once every bucket has been written at least once.
	if win.filled < abstractRateBuckets {
		return
	}

	// Window minimum = sustained floor over the full window.
	windowMin := win.buckets[0]
	for _, b := range win.buckets[1:] {
		if b < windowMin {
			windowMin = b
		}
	}

	p := &w.System.Planets[w.Active]
	if windowMin > p.AbstractRate {
		p.AbstractRate = windowMin
	}
}

// abstractIncome returns total abstract wood/sec from all non-active producing
// planets. The active planet runs live (or is frozen in system view), so it is
// excluded when in planet view to avoid double-counting. Unknown never produces.
func abstractIncome(w *World) float64 {
	var total float64
	for i, p := range w.System.Planets {
		if p.Kind == PlanetUnknown {
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
		if p.Kind == PlanetUnknown {
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
