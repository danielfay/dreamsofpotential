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

// assignNodes pairs idle workers with the nearest unclaimed node.
// Workers are kept idle when no camps exist (nowhere to deliver).
func assignNodes(w *World) {
	if len(w.Buildings) == 0 {
		return
	}
	for _, wk := range w.Workers {
		if wk.NodeID != -1 {
			continue
		}
		node := nearestFreeNode(w, wk)
		if node == nil {
			continue
		}
		wk.NodeID = node.ID
		wk.State = StateToForest
		node.OwnerID = wk.ID
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
			wk.Carried = loadAmount
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

// nearestFreeNode returns the unclaimed node closest by arc distance to wk's angle.
func nearestFreeNode(w *World, wk *Worker) *ResourceNode {
	var best *ResourceNode
	bestDist := math.MaxFloat64
	for _, n := range w.Nodes {
		if n.OwnerID != -1 {
			continue
		}
		dist := math.Abs(normAngle(n.Angle-wk.Angle)) * w.Planet.Radius
		if dist < bestDist {
			bestDist = dist
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
// each time the counter meets or exceeds the cap.
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
		camp := nearestCamp(w, node)
		if camp == nil {
			continue
		}
		dist := math.Abs(normAngle(node.Angle-camp.Angle)) * w.Planet.Radius
		tripTime := loadTime + unloadTime + 2*dist/workerSpeed
		rate += loadAmount / tripTime
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
