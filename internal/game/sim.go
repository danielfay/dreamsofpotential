package game

import "math"

// Step advances the simulation by dt seconds (called at 60 TPS, dt = 1/60).
// Each worker moves toward its current target, dwells to load/unload, and
// deposits wood into the economy when it completes an unload cycle.
func Step(w *World, dt float64) {
	for _, b := range w.Buildings {
		for _, wk := range b.Workers {
			switch wk.State {
			case StateToForest:
				if moveToward(&wk.Pos, w.Forest.Pos, workerSpeed*dt) {
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
				if moveToward(&wk.Pos, b.Pos, workerSpeed*dt) {
					wk.State = StateUnloading
					wk.Timer = unloadTime
				}
			case StateUnloading:
				wk.Timer -= dt
				if wk.Timer <= 0 {
					w.Economy.Wood += wk.Carried
					wk.Carried = 0
					wk.State = StateToForest
				}
			}
		}
	}
}

// moveToward advances pos toward target by at most step world-units.
// Returns true and snaps pos to target when the remaining distance is ≤ step.
func moveToward(pos *Vec, target Vec, step float64) bool {
	dx, dy := target.X-pos.X, target.Y-pos.Y
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist <= step {
		*pos = target
		return true
	}
	pos.X += dx / dist * step
	pos.Y += dy / dist * step
	return false
}
