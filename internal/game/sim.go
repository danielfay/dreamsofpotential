package game

import "math"

// Step advances the simulation by dt seconds (called at 60 TPS, dt = 1/60).
func Step(w *World, dt float64) {
	w.SimTime += dt
	tickPulses(w, dt)
	tickGrowthCue(w, dt)
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
		if moveAlongArc(&wk.Angle, th.Angle, w.Planet.Radius, workerSpeed*dt) {
			wk.State = StateToForest
		}
	case StateToForest:
		node := findNode(w, wk.NodeID)
		if node == nil {
			startReturnHome(w, wk)
			return
		}
		if moveAlongArc(&wk.Angle, node.Angle, w.Planet.Radius, workerSpeed*dt) {
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
		if moveAlongArc(&wk.Angle, camp.Angle, w.Planet.Radius, workerSpeed*dt) {
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
		if moveAlongArc(&wk.Angle, th.Angle, w.Planet.Radius, workerSpeed*dt) {
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
	banked := gross * (1 - fieldReturnRatio)
	returned := gross * fieldReturnRatio
	if f := fieldForKind(w, node.Kind); f != nil && f.NurtureCharges > 0 {
		if fieldCanSpawnNode(w, f) {
			f.NurtureCharges--
			returned += fieldEXPToNextLevel(f)
		} else {
			f.NurtureCharges = 0
		}
	}
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

func fieldEXPToNextLevel(f *ResourceField) float64 {
	if f == nil || f.Cap <= 0 {
		return 0
	}
	needed := f.Cap - f.EXP
	if needed <= 0 {
		return f.Cap
	}
	return needed
}

func startReturnHome(w *World, wk *Worker) {
	releaseOwnedNode(w, wk)
	releaseWorkerReservations(w, wk.ID)
	wk.TargetNodeID = -1
	wk.PendingNodeID = -1
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

// routeLen returns the arc distance from node to its nearest camp.
// Returns math.MaxFloat64 if no camps exist.
func routeLen(w *World, node *ResourceNode) float64 {
	camp := nearestCamp(w, node)
	if camp == nil {
		return math.MaxFloat64
	}
	return math.Abs(normAngle(node.Angle-camp.Angle)) * w.Planet.Radius
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

// nearestCamp returns the camp with the smallest arc distance to node's angle.
// Returns nil if no camps exist.
func nearestCamp(w *World, node *ResourceNode) *Building {
	var best *Building
	bestDist := math.MaxFloat64
	for _, b := range w.Buildings {
		dist := math.Abs(normAngle(b.Angle-node.Angle)) * w.Planet.Radius
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

// depositToField increments the field EXP for kind and spawns a new node
// each time EXP meets or exceeds the cap. Assignment happens
// automatically on the next tick via assignNodes.
func depositToField(w *World, kind ResourceKind, amount float64) {
	for _, f := range w.Planet.Fields {
		if f.Kind != kind {
			continue
		}
		f.EXP += amount
		for f.EXP >= f.Cap {
			f.EXP -= f.Cap
			f.Cap *= fieldEXPGrowth
			activateGrowthCue(w, spawnNode(w, f))
		}
		return
	}
}

func activateGrowthCue(w *World, result growthResult) {
	if result.Outcome == growthOutcomeNone {
		return
	}
	cue := growthCueState{
		Outcome:        result.Outcome,
		Kind:           result.Kind,
		CenterAngle:    result.CenterAngle,
		HalfArc:        result.HalfArc,
		NodeID:         result.NodeID,
		GaugeRelease:   growthGaugeReleaseTime,
		GaugeAfterglow: growthGaugeAfterglowTime,
		FieldDelay:     growthFieldPulseDelay,
		FieldPulse:     growthFieldPulseTime,
		NodeDelay:      growthNodeCueDelay,
		NodeCue:        growthNodeCueTime,
	}
	if growthCueActive(w.growthCue) {
		w.pendingGrowthCues = append(w.pendingGrowthCues, cue)
		return
	}
	w.growthCue = cue
}

func upgradeFirstFieldForDebug(w *World) bool {
	if len(w.Planet.Fields) == 0 {
		return false
	}
	f := w.Planet.Fields[0]
	amount := f.Cap - f.EXP
	if amount <= 0 {
		amount = f.Cap
	}
	depositToField(w, f.Kind, amount)
	return true
}

func growFirstFieldUntilBlockedForDebug(w *World) bool {
	if len(w.Planet.Fields) == 0 {
		return false
	}
	const maxDebugGrowthSteps = 512
	for i := 0; i < maxDebugGrowthSteps; i++ {
		before := len(w.Nodes)
		if !upgradeFirstFieldForDebug(w) {
			return false
		}
		if len(w.Nodes) == before {
			return true
		}
	}
	return true
}

// nurtureField spends nurtureCost wood to arm the matching field with
// nurtureCharges level-completing delivery charges. Returns false if the
// resource is not yet discovered, the field is missing, charges are already
// active, the field cannot place more nodes, or the player cannot afford the
// cost.
func nurtureField(w *World, kind ResourceKind) bool {
	f := fieldForKind(w, kind)
	if !w.ResourceDiscovered || f == nil || f.NurtureCharges > 0 ||
		!fieldCanSpawnNode(w, f) || w.Economy.Wood < nurtureCost {
		return false
	}
	w.Economy.Wood -= nurtureCost
	f.NurtureCharges = nurtureCharges
	return true
}

// nurtureAttentionActive reports whether the resource square should show its
// attention pulse. True when Nurture is affordable, no charges are active, and
// either an idle worker has no free node to claim, or one Nurture charge would
// complete the current field level.
func nurtureAttentionActive(w *World, kind ResourceKind) bool {
	if !w.ResourceDiscovered || w.Economy.Wood < nurtureCost {
		return false
	}
	f := fieldForKind(w, kind)
	if f == nil || f.NurtureCharges > 0 || !fieldCanSpawnNode(w, f) {
		return false
	}
	// Condition 1: idle worker with no free (unclaimed, unreserved) node.
	hasIdle := false
	for _, wk := range w.Workers {
		if wk.State == StateIdleWaiting {
			hasIdle = true
			break
		}
	}
	if hasIdle {
		hasFreeNode := false
		for _, n := range w.Nodes {
			if n.Kind == kind && n.OwnerID == -1 && n.ReservedByWorkerID == -1 {
				hasFreeNode = true
				break
			}
		}
		if !hasFreeNode {
			return true
		}
	}
	return fieldEXPToNextLevel(f) > 0
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
		rate += (baseLoadAmount * node.Size * (1 - fieldReturnRatio)) / tripTime
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
// cap and a slot is free, then resets growth to 0 and raises the cap.
// When capacity-blocked and growth is full, clamps growth at the cap (no
// overflow accumulation). Returns true if a worker spawned.
func tryConsumeGrowth(w *World) bool {
	if w.Economy.TownGrowth < w.Economy.TownGrowthCap {
		return false
	}
	if availableCapacity(w) <= 0 {
		w.Economy.TownGrowth = w.Economy.TownGrowthCap
		return false
	}
	th := townHall(w)
	if spawnWorkerAtTownHall(w) == nil {
		return false
	}
	if th != nil {
		activatePulse(w, &th.Pulse)
	}
	w.Economy.TownGrowth = 0
	w.Economy.TownGrowthCap *= townGrowthCapGrowth
	return true
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
	}
	wk.Pos = w.Planet.RimPoint(wk.Angle)
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
