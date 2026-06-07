package game

import "math"

// Step advances the simulation by dt seconds (called at 60 TPS, dt = 1/60).
func Step(w *World, dt float64) {
	assignNodes(w)
	for _, wk := range w.Workers {
		stepWorker(w, wk, dt)
		wk.Pos = w.Planet.RimPoint(wk.Angle)
	}
}

// assignNodes runs every tick. It first rebalances active workers (swapping
// any active worker whose route is longer than the best free node's route),
// then assigns remaining idle workers to the best available free node.
func assignNodes(w *World) {
	if len(w.Buildings) == 0 {
		return
	}
	rebalance(w)
	for _, wk := range w.Workers {
		if wk.NodeID != -1 {
			continue
		}
		if node := bestFreeNode(w); node != nil {
			wk.NodeID = node.ID
			wk.State = StateToForest
			node.OwnerID = wk.ID
		}
	}
}

// rebalance iteratively swaps the active worker with the longest route onto the
// free node with the shortest route, until no further improvement is possible.
// This converges in O(swaps) iterations and stays stable once balanced.
func rebalance(w *World) {
	for {
		free := bestFreeNode(w)
		if free == nil {
			return
		}
		freeRoute := routeLen(w, free)

		var worst *Worker
		worstRoute := -1.0
		for _, wk := range w.Workers {
			if wk.NodeID == -1 {
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

		if old := findNode(w, worst.NodeID); old != nil {
			old.OwnerID = -1
		}
		free.OwnerID = worst.ID
		worst.NodeID = free.ID
		worst.State = StateToForest
	}
}

// stepWorker advances one worker's state machine by dt seconds.
func stepWorker(w *World, wk *Worker, dt float64) {
	if wk.NodeID == -1 {
		return
	}
	node := findNode(w, wk.NodeID)
	if node == nil {
		return
	}
	switch wk.State {
	case StateToForest:
		if moveAlongArc(&wk.Angle, node.Angle, w.Planet.Radius, workerSpeed*dt) {
			wk.State = StateLoading
			wk.Timer = loadTime
		}
	case StateLoading:
		wk.Timer -= dt
		if wk.Timer <= 0 {
			wk.Carried = loadAmount * node.Size
			wk.State = StateToBuilding
		}
	case StateToBuilding:
		camp := nearestCamp(w, node)
		if camp == nil {
			return
		}
		if moveAlongArc(&wk.Angle, camp.Angle, w.Planet.Radius, workerSpeed*dt) {
			wk.State = StateUnloading
			wk.Timer = unloadTime
		}
	case StateUnloading:
		wk.Timer -= dt
		if wk.Timer <= 0 {
			w.Economy.Wood += wk.Carried
			depositToField(w, node.Kind, wk.Carried)
			wk.Carried = 0
			wk.State = StateToForest
		}
	}
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

// bestFreeNode returns the unclaimed node with the shortest route to its nearest camp.
func bestFreeNode(w *World) *ResourceNode {
	var best *ResourceNode
	bestRoute := math.MaxFloat64
	for _, n := range w.Nodes {
		if n.OwnerID != -1 {
			continue
		}
		if r := routeLen(w, n); r < bestRoute {
			bestRoute = r
			best = n
		}
	}
	return best
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
// each time the counter meets or exceeds the cap. Rebalancing happens
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
		if wk.NodeID == -1 {
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
