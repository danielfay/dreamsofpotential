package game

import (
	"math"
	"testing"
)

// newTestWorld builds a minimal world with one camp and no workers.
// campDist is the Euclidean distance from the camp to the forest.
func newTestWorld(campDist float64) *World {
	w := NewWorld()
	campPos := Vec{
		X: w.Forest.Pos.X,
		Y: w.Forest.Pos.Y + campDist,
	}
	camp := &Building{Pos: campPos}
	w.Buildings = append(w.Buildings, camp)
	return w
}

// addWorker attaches a worker to the first building in w.
func addWorker(w *World) {
	camp := w.Buildings[0]
	camp.Workers = append(camp.Workers, &Worker{
		Pos:   camp.Pos,
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

	// Subtract starting wood to get net production.
	nearProduced := near.Economy.Wood - near.Economy.Wood // recalculate below
	_ = nearProduced
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
