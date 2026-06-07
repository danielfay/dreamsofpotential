package game

import (
	"math"
	"testing"
)

// newTestWorld builds a minimal world with one camp placed campDist arc-units
// away from the resource field's center angle. campDist values above the field's
// half-arc width (54 world units for the default wood field) put the camp clearly
// outside the node cluster, which keeps the near/far comparison tests reliable.
func newTestWorld(campDist float64) *World {
	w := NewWorld()
	fieldAngle := w.Planet.Fields[0].CenterAngle
	dTheta := campDist / w.Planet.Radius
	campAngle := normAngle(fieldAngle + dTheta)
	camp := &Building{Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}
	w.Buildings = append(w.Buildings, camp)
	return w
}

// addWorker appends a new idle worker to the global pool, spawned at the first camp.
func addWorker(w *World) {
	camp := w.Buildings[0]
	id := w.NextWorkerID
	w.NextWorkerID++
	w.Workers = append(w.Workers, &Worker{
		ID:     id,
		Pos:    camp.Pos,
		Angle:  camp.Angle,
		State:  StateToForest,
		NodeID: -1,
	})
}

// runSim advances the simulation for the given duration (in seconds).
func runSim(w *World, seconds float64) {
	ticks := int(math.Round(seconds / dt))
	for i := 0; i < ticks; i++ {
		Step(w, dt)
	}
}

// TestWoodAccumulates verifies that wood rises when a worker is running.
func TestWoodAccumulates(t *testing.T) {
	w := newTestWorld(100)
	startWood := w.Economy.Wood
	addWorker(w)
	runSim(w, 10)
	if w.Economy.Wood <= startWood {
		t.Errorf("expected wood to increase; got %.2f (started at %.2f)", w.Economy.Wood, startWood)
	}
}

// TestCloserCampProducesMore asserts the core spatial mechanic: a camp closer
// to the nodes yields more wood over equal time. Both camps are placed outside
// the forest arc so their distances from nodes are clearly differentiated.
func TestCloserCampProducesMore(t *testing.T) {
	near := newTestWorld(100)
	addWorker(near)

	far := newTestWorld(200)
	addWorker(far)

	runSim(near, 60)
	runSim(far, 60)

	initial := NewWorld().Economy.Wood
	nearNet := near.Economy.Wood - initial
	farNet := far.Economy.Wood - initial

	if nearNet <= farNet {
		t.Errorf("expected near camp (%.2f wood) to out-produce far camp (%.2f wood)", nearNet, farNet)
	}
}

// TestAnalyticRateMatchesSim checks that EstimateRate is a reasonable predictor
// of actual throughput (within 20%).
func TestAnalyticRateMatchesSim(t *testing.T) {
	const dist = 100.0
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

// TestMoreWorkersProduceMoreWood verifies that adding a second worker increases output.
func TestMoreWorkersProduceMoreWood(t *testing.T) {
	const dist = 100.0
	const seconds = 60.0

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

// TestSnapToRim verifies Planet.RimPoint and Planet.AngleOf round-trip correctly,
// and that all starting nodes land on the rim.
func TestSnapToRim(t *testing.T) {
	w := NewWorld()
	p := w.Planet

	// All starting nodes must be on the rim.
	for i, n := range w.Nodes {
		dist := n.Pos.Dist(p.Center)
		if math.Abs(dist-p.Radius) > 1e-9 {
			t.Errorf("node[%d] is %.6f from center, want %.6f (on the rim)", i, dist, p.Radius)
		}
	}

	// RimPoint(AngleOf(p)) should return a point on the rim in the same direction.
	tests := []Vec{
		{X: 200, Y: 80},
		{X: 300, Y: 200},
		{X: 160, Y: 30},
	}
	for _, pt := range tests {
		theta := p.AngleOf(pt)
		rim := p.RimPoint(theta)
		dist := rim.Dist(p.Center)
		if math.Abs(dist-p.Radius) > 1e-9 {
			t.Errorf("RimPoint(%v): distance from center %.9f, want %.9f", pt, dist, p.Radius)
		}
		wantTheta := p.AngleOf(rim)
		if math.Abs(normAngle(wantTheta-theta)) > 1e-9 {
			t.Errorf("RimPoint(%v): angle %.9f round-trips to %.9f", pt, theta, wantTheta)
		}
	}
}

// TestWorkerStaysOnRim runs the sim and asserts workers never leave the surface.
func TestWorkerStaysOnRim(t *testing.T) {
	w := newTestWorld(100)
	addWorker(w)

	p := w.Planet
	ticks := int(math.Round(10.0 / dt))
	for i := 0; i < ticks; i++ {
		Step(w, dt)
		for _, wk := range w.Workers {
			dist := wk.Pos.Dist(p.Center)
			if math.Abs(dist-p.Radius) > 1e-6 {
				t.Errorf("tick %d: worker %.6f from center, want %.6f", i, dist, p.Radius)
				return
			}
		}
	}
}

// TestOneWorkerPerNode verifies that surplus workers remain idle when all nodes
// are claimed: with 5 nodes and 7 workers, exactly 2 should be idle.
func TestOneWorkerPerNode(t *testing.T) {
	w := newTestWorld(100)
	nodeCount := len(w.Nodes)
	extra := 2
	for i := 0; i < nodeCount+extra; i++ {
		addWorker(w)
	}
	// One tick triggers the assignment pass.
	Step(w, dt)

	idle := 0
	for _, wk := range w.Workers {
		if wk.NodeID == -1 {
			idle++
		}
	}
	if idle != extra {
		t.Errorf("expected %d idle workers, got %d", extra, idle)
	}
}

// TestNodeSpawning verifies that delivering enough resources causes a new node
// to appear and the field counter to reset.
func TestNodeSpawning(t *testing.T) {
	w := newTestWorld(100)
	addWorker(w)
	initialNodes := len(w.Nodes)
	field := w.Planet.Fields[0]

	// Cap is nodeSpawnBaseCap (20). Run long enough for at least one spawn.
	runSim(w, 120)

	if len(w.Nodes) <= initialNodes {
		t.Errorf("expected new node after deliveries; still have %d nodes", len(w.Nodes))
	}
	if field.Counter >= field.Cap {
		t.Errorf("field counter should have reset after spawn, got %.2f / %.2f", field.Counter, field.Cap)
	}
}

// TestNearestCampDelivery verifies that nearestCamp picks the camp closest to
// a given node's rim angle, not the one closest to the worker's current position.
func TestNearestCampDelivery(t *testing.T) {
	w := NewWorld()

	field := w.Planet.Fields[0]
	// Place two camps: one just outside the right edge of the node arc, one on
	// the opposite side of the planet.
	nearAngle := normAngle(field.CenterAngle + field.HalfArc + 0.1)
	farAngle := normAngle(field.CenterAngle + math.Pi)

	nearCamp := &Building{Angle: nearAngle, Pos: w.Planet.RimPoint(nearAngle)}
	farCamp := &Building{Angle: farAngle, Pos: w.Planet.RimPoint(farAngle)}
	w.Buildings = append(w.Buildings, nearCamp, farCamp)

	// The rightmost node (at CenterAngle+HalfArc) should be closest to nearCamp.
	rightNode := w.Nodes[len(w.Nodes)-1] // seeded at CenterAngle+HalfArc
	got := nearestCamp(w, rightNode)
	if got != nearCamp {
		t.Errorf("expected nearestCamp to return nearCamp; got farCamp")
	}
}
