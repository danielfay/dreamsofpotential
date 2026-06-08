package game

import "math"

// Step advances the simulation by dt seconds (called at 60 TPS, dt = 1/60).
func Step(w *World, dt float64) {
	w.SimTime += dt
	tickPulses(w, dt)
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
			wk.Carried = loadAmount * node.Size
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
	if wk.Carried > 0 {
		w.ResourceDiscovered = true
	}
	w.Economy.Wood += wk.Carried
	if b := nearestCamp(w, node); b != nil {
		b.DeliveredWood += wk.Carried
		b.DeliveryCount++
	}
	depositToField(w, node.Kind, wk.Carried)
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

// depositToField increments the field counter for kind and spawns a new node
// each time the counter meets or exceeds the cap. Assignment happens
// automatically on the next tick via assignNodes.
func depositToField(w *World, kind ResourceKind, amount float64) {
	for _, f := range w.Planet.Fields {
		if f.Kind != kind {
			continue
		}
		f.Counter += amount
		for f.Counter >= f.Cap {
			f.Counter -= f.Cap
			f.Cap *= nodeCapGrowth
			spawnNode(w, f)
		}
		return
	}
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
		rate += (loadAmount * node.Size) / tripTime
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

func addFreeWorkerAtTownHall(w *World) bool {
	th := townHall(w)
	if th == nil {
		return false
	}
	id := w.NextWorkerID
	w.NextWorkerID++
	w.Workers = append(w.Workers, &Worker{
		ID:            id,
		Pos:           th.Pos,
		Angle:         th.Angle,
		State:         StateSettling,
		NodeID:        -1,
		TargetNodeID:  -1,
		PendingNodeID: -1,
		Timer:         settleDelay,
	})
	return true
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

func tickPulse(p *PulseState, dt float64) {
	if p.Remaining > 0 {
		p.Remaining -= dt
		if p.Remaining < 0 {
			p.Remaining = 0
		}
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
