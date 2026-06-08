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
	camp := &Building{Kind: KindTownHall, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}
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

// newWorldSingleNode returns a world with one node of Size 1.0 at nodeAngle and
// one camp at campDist arc-units away. Used to test trip-distance effects
// independently of random node sizes and positions.
func newWorldSingleNode(nodeAngle, campDist float64) *World {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	node := newNode(w, KindWood, nodeAngle)
	node.Size = 1.0
	w.Nodes = []*ResourceNode{node}
	campAngle := normAngle(nodeAngle + campDist/w.Planet.Radius)
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}}
	return w
}

// TestCloserCampProducesMore asserts the core spatial mechanic: shorter node-to-camp
// arc distance yields more wood over equal time.
func TestCloserCampProducesMore(t *testing.T) {
	const nodeAngle = -math.Pi / 2 // fixed node position

	near := newWorldSingleNode(nodeAngle, 30)
	addWorker(near)

	far := newWorldSingleNode(nodeAngle, 120)
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

// TestMoreWorkersProduceMoreWood verifies that adding a second worker increases
// output. Uses a fixed two-node setup so random seeding can't skew the worlds.
func TestMoreWorkersProduceMoreWood(t *testing.T) {
	const seconds = 60.0

	buildWorld := func() *World {
		w := NewWorld()
		w.Nodes = nil
		w.NextNodeID = 0
		campAngle := 0.0
		for _, offset := range []float64{0.5, -0.5} {
			n := newNode(w, KindWood, normAngle(campAngle+offset))
			n.Size = 1.0
			w.Nodes = append(w.Nodes, n)
		}
		w.Buildings = []*Building{{Kind: KindTownHall, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}}
		return w
	}

	one := buildWorld()
	addWorker(one)
	runSim(one, seconds)

	two := buildWorld()
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
	// Use a fully deterministic setup: one camp and one node at the same angle
	// so trip time is ~0.8 s and Size=1.0 gives exactly 1 wood per delivery.
	// The spawn cap is nodeSpawnBaseCap (20), so 30 s is well above the minimum.
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	campAngle := 0.0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}}

	n := newNode(w, KindWood, campAngle)
	n.Size = 1.0
	w.Nodes = []*ResourceNode{n}

	addWorker(w)
	initialNodes := len(w.Nodes)
	field := w.Planet.Fields[0]

	runSim(w, 30)

	if len(w.Nodes) <= initialNodes {
		t.Errorf("expected new node after deliveries; still have %d nodes", len(w.Nodes))
	}
	if field.Counter >= field.Cap {
		t.Errorf("field counter should have reset after spawn, got %.2f / %.2f", field.Counter, field.Cap)
	}
}

// TestNewWorkerClaimsBestRouteNode verifies that when a worker is assigned it
// takes the free node with the shortest route to the nearest camp, not simply
// the node closest to its own position.
func TestNewWorkerClaimsBestRouteNode(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	campAngle := 0.0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}}

	// Far node: 2 rad from camp.
	farNode := newNode(w, KindWood, normAngle(campAngle+2.0))
	farNode.Size = 1.0
	// Close node: 0.2 rad from camp.
	closeNode := newNode(w, KindWood, normAngle(campAngle+0.2))
	closeNode.Size = 1.0
	w.Nodes = []*ResourceNode{farNode, closeNode}

	addWorker(w)
	Step(w, dt)

	if w.Workers[0].NodeID != closeNode.ID {
		t.Errorf("expected worker to claim close node (ID %d), got node ID %d",
			closeNode.ID, w.Workers[0].NodeID)
	}
}

// TestNewNodeTriggersReassignment verifies that when a new node with a shorter route
// appears, the active worker with the longest route switches to it on the next tick.
func TestNewNodeTriggersReassignment(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	campAngle := 0.0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}}

	farNode := newNode(w, KindWood, normAngle(campAngle+2.0))
	farNode.Size = 1.0
	w.Nodes = []*ResourceNode{farNode}

	addWorker(w)
	Step(w, dt) // assign worker to farNode

	if w.Workers[0].NodeID != farNode.ID {
		t.Fatalf("setup: expected worker on far node")
	}

	// Spawn a closer node; rebalance runs automatically on the next tick.
	closeNode := newNode(w, KindWood, normAngle(campAngle+0.2))
	closeNode.Size = 1.0
	w.Nodes = append(w.Nodes, closeNode)
	Step(w, dt)

	if w.Workers[0].NodeID != closeNode.ID {
		t.Errorf("expected worker to switch to close node")
	}
	if farNode.OwnerID != -1 {
		t.Errorf("expected far node to be freed after reassignment")
	}
}

// TestCampPlacementTriggersRebalance verifies that placing a new camp near a
// free node causes a worker on a farther node to switch to it on the next tick.
func TestCampPlacementTriggersRebalance(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	// Initial camp is far from both nodes.
	initialCampAngle := 0.0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: initialCampAngle, Pos: w.Planet.RimPoint(initialCampAngle)}}

	// Two nodes: both at similar arc distance from the initial camp.
	nodeA := newNode(w, KindWood, normAngle(initialCampAngle+0.5))
	nodeA.Size = 1.0
	nodeB := newNode(w, KindWood, normAngle(initialCampAngle+3.0))
	nodeB.Size = 1.0
	w.Nodes = []*ResourceNode{nodeA, nodeB}

	// One worker: should claim nodeA (shorter route from initial camp).
	addWorker(w)
	Step(w, dt)

	if w.Workers[0].NodeID != nodeA.ID {
		t.Fatalf("setup: expected worker on nodeA (closer to initial camp)")
	}
	// nodeB is free.

	// Place a new camp right next to nodeB — its route is now much shorter.
	newCampAngle := normAngle(nodeB.Angle + 0.05)
	w.Buildings = append(w.Buildings, &Building{
		Kind: KindLoggingCamp, Angle: newCampAngle, Pos: w.Planet.RimPoint(newCampAngle),
	})
	Step(w, dt)

	if w.Workers[0].NodeID != nodeB.ID {
		t.Errorf("expected worker to switch to nodeB after new camp placed nearby")
	}
	if nodeA.OwnerID != -1 {
		t.Errorf("expected nodeA to be freed after worker switched")
	}
}

// TestNearestCampDelivery verifies that nearestCamp picks the camp with the
// smallest arc distance to the node, not to the worker.
func TestNearestCampDelivery(t *testing.T) {
	w := NewWorld()

	// Create a single node at a known angle.
	nodeAngle := 0.0 // 3 o'clock
	node := newNode(w, KindWood, nodeAngle)
	node.Size = 1.0

	// Near camp: 10 arc-units from the node.
	nearAngle := normAngle(nodeAngle + 10.0/w.Planet.Radius)
	// Far camp: 150 arc-units from the node.
	farAngle := normAngle(nodeAngle + 150.0/w.Planet.Radius)

	nearCamp := &Building{Kind: KindTownHall, Angle: nearAngle, Pos: w.Planet.RimPoint(nearAngle)}
	farCamp := &Building{Kind: KindLoggingCamp, Angle: farAngle, Pos: w.Planet.RimPoint(farAngle)}
	w.Buildings = append(w.Buildings, nearCamp, farCamp)

	got := nearestCamp(w, node)
	if got != nearCamp {
		t.Errorf("expected nearestCamp to return nearCamp; got farCamp")
	}
}

// TestTownHallIsValidDeliveryPoint verifies that the Town Hall participates in
// delivery routing: nearestCamp returns it when it is closer to a node than
// any logging camp.
func TestTownHallIsValidDeliveryPoint(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	nodeAngle := 0.0
	node := newNode(w, KindWood, nodeAngle)
	node.Size = 1.0
	w.Nodes = []*ResourceNode{node}

	// Town Hall close to node; logging camp far away.
	thAngle := normAngle(nodeAngle + 0.1)
	campAngle := normAngle(nodeAngle + math.Pi/2)
	w.Buildings = []*Building{
		{Kind: KindTownHall, Angle: thAngle, Pos: w.Planet.RimPoint(thAngle)},
		{Kind: KindLoggingCamp, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)},
	}

	got := nearestCamp(w, node)
	if got.Kind != KindTownHall {
		t.Errorf("expected Town Hall to be nearest delivery point; got Kind %v", got.Kind)
	}
}
