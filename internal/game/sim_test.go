package game

import (
	"math"
	"testing"
)

// newTestWorld builds a minimal world with one camp and no workers.
// campDist is the arc distance along the rim from the camp to the forest.
func newTestWorld(campDist float64) *World {
	w := NewWorld()
	forestAngle := w.Planet.AngleOf(w.Forest.Pos)
	dTheta := campDist / w.Planet.Radius
	campAngle := normAngle(forestAngle + dTheta)
	camp := &Building{Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}
	w.Buildings = append(w.Buildings, camp)
	return w
}

// addWorker attaches a worker to the first building in w.
func addWorker(w *World) {
	camp := w.Buildings[0]
	camp.Workers = append(camp.Workers, &Worker{
		Pos:   camp.Pos,
		Angle: camp.Angle,
		State: StateToForest,
		Home:  camp,
	})
}

// runSim advances the simulation for the given duration (in seconds).
func runSim(w *World, seconds float64) {
	ticks := int(math.Round(seconds / dt))
	for i := 0; i < ticks; i++ {
		Step(w, dt)
	}
}

// TestWoodAccumulates verifies that wood actually rises when a worker is running.
func TestWoodAccumulates(t *testing.T) {
	w := newTestWorld(30)
	startWood := w.Economy.Wood
	addWorker(w)
	runSim(w, 10)
	if w.Economy.Wood <= startWood {
		t.Errorf("expected wood to increase; got %.2f (started at %.2f)", w.Economy.Wood, startWood)
	}
}

// TestCloserCampProducesMore asserts the core spatial mechanic:
// a camp placed closer to the forest yields more wood over equal time.
func TestCloserCampProducesMore(t *testing.T) {
	near := newTestWorld(10)
	addWorker(near)

	far := newTestWorld(60)
	addWorker(far)

	runSim(near, 30)
	runSim(far, 30)

	initialWood := NewWorld().Economy.Wood
	nearNet := near.Economy.Wood - initialWood
	farNet := far.Economy.Wood - initialWood

	if nearNet <= farNet {
		t.Errorf("expected near camp (%.2f wood) to out-produce far camp (%.2f wood)", nearNet, farNet)
	}
}

// TestAnalyticRateMatchesSim checks that EstimateRate is a reasonable predictor
// of actual throughput (within 20%, to allow for partial-trip rounding).
func TestAnalyticRateMatchesSim(t *testing.T) {
	const dist = 40.0
	const simSeconds = 60.0

	w := newTestWorld(dist)
	addWorker(w)
	addWorker(w)

	initialWood := w.Economy.Wood
	runSim(w, simSeconds)
	actualRate := (w.Economy.Wood - initialWood) / simSeconds

	analyticRate := EstimateRate(w)

	ratio := actualRate / analyticRate
	if ratio < 0.8 || ratio > 1.2 {
		t.Errorf("analytic rate %.4f/s differs from simulated %.4f/s by more than 20%% (ratio %.2f)",
			analyticRate, actualRate, ratio)
	}
}

// TestMoreWorkersProduceMoreWood verifies linear worker scaling.
func TestMoreWorkersProduceMoreWood(t *testing.T) {
	const dist = 20.0
	const seconds = 20.0

	one := newTestWorld(dist)
	addWorker(one)
	runSim(one, seconds)

	two := newTestWorld(dist)
	addWorker(two)
	addWorker(two)
	runSim(two, seconds)

	initial := NewWorld().Economy.Wood
	oneNet := one.Economy.Wood - initial
	twoNet := two.Economy.Wood - initial

	if twoNet <= oneNet {
		t.Errorf("two workers (%.2f) should produce more than one (%.2f)", twoNet, oneNet)
	}
}

// TestSnapToRim verifies Planet.RimPoint and Planet.AngleOf round-trip correctly.
func TestSnapToRim(t *testing.T) {
	w := NewWorld()
	p := w.Planet

	// Forest must be on the rim.
	forestDist := w.Forest.Pos.Dist(p.Center)
	if math.Abs(forestDist-p.Radius) > 1e-9 {
		t.Errorf("forest is %.6f from center, want %.6f (on the rim)", forestDist, p.Radius)
	}

	// RimPoint(AngleOf(p)) should return a point on the rim in the same direction.
	tests := []Vec{
		{X: 200, Y: 80},  // interior point
		{X: 300, Y: 200}, // exterior point
		{X: 160, Y: 30},  // already on the rim (top)
	}
	for _, pt := range tests {
		theta := p.AngleOf(pt)
		rim := p.RimPoint(theta)
		dist := rim.Dist(p.Center)
		if math.Abs(dist-p.Radius) > 1e-9 {
			t.Errorf("RimPoint(%v): distance from center %.9f, want %.9f", pt, dist, p.Radius)
		}
		// Same direction from center.
		wantTheta := p.AngleOf(rim)
		if math.Abs(normAngle(wantTheta-theta)) > 1e-9 {
			t.Errorf("RimPoint(%v): angle %.9f round-trips to %.9f", pt, theta, wantTheta)
		}
	}
}

// TestWorkerStaysOnRim runs the sim and asserts workers never leave the surface.
func TestWorkerStaysOnRim(t *testing.T) {
	w := newTestWorld(45)
	addWorker(w)

	p := w.Planet
	ticks := int(math.Round(10.0 / dt))
	for i := 0; i < ticks; i++ {
		Step(w, dt)
		for _, wk := range w.Buildings[0].Workers {
			dist := wk.Pos.Dist(p.Center)
			if math.Abs(dist-p.Radius) > 1e-6 {
				t.Errorf("tick %d: worker %.6f from center, want %.6f", i, dist, p.Radius)
				return
			}
		}
	}
}
