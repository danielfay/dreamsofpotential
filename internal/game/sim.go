package game

import "math"

// Step advances the simulation by dt seconds (called at 60 TPS, dt = 1/60).
// Each worker moves along the planet rim toward its current target, dwells to
// load/unload, and deposits wood into the economy when it completes an unload cycle.
func Step(w *World, dt float64) {
	forestAngle := w.Planet.AngleOf(w.Forest.Pos)
	for _, b := range w.Buildings {
		for _, wk := range b.Workers {
			switch wk.State {
			case StateToForest:
				if moveAlongArc(&wk.Angle, forestAngle, w.Planet.Radius, workerSpeed*dt) {
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
				if moveAlongArc(&wk.Angle, b.Angle, w.Planet.Radius, workerSpeed*dt) {
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
			wk.Pos = w.Planet.RimPoint(wk.Angle)
		}
	}
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
