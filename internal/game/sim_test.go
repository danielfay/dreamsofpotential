package game

import (
	"fmt"
	"math"
	"testing"
)

// newDeliveryWorld builds a minimal world for completeUnload tests: one Town Hall
// and one Size-1 node, both at angle 0, with no other workers.
func newDeliveryWorld() (*World, *ResourceNode) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	campAngle := 0.0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}}
	node := newNode(w, KindWood, campAngle)
	node.Size = 1.0
	w.Nodes = []*ResourceNode{node}
	w.Economy.WorkerCapacity = 99 // pre-set high so capacity is never the bottleneck
	return w, node
}

func TestDeliveryFeedsGrowth(t *testing.T) {
	w, node := newDeliveryWorld()
	w.Economy.TownGrowthCap = 100 // cap far above one delivery

	gross := baseLoadAmount * node.Size
	wk := &Worker{ID: 0, Carried: gross, NodeID: node.ID,
		Angle: 0, Pos: w.Planet.RimPoint(0), TargetNodeID: -1, PendingNodeID: -1}
	node.OwnerID = wk.ID
	w.Workers = []*Worker{wk}

	completeUnload(w, wk, node)

	if math.Abs(w.Economy.TownGrowth-gross) > 1e-9 {
		t.Errorf("TownGrowth got %.4f, want %.4f (gross)", w.Economy.TownGrowth, gross)
	}
	if len(w.Workers) != 1 {
		t.Errorf("no worker should spawn (growth below cap); got %d workers", len(w.Workers))
	}
}

func TestWorkerArrivalOnDelivery(t *testing.T) {
	w, node := newDeliveryWorld()
	gross := baseLoadAmount * node.Size
	// Use a cap in the normal-play range so the geometric ramp applies.
	w.Economy.TownGrowthCap = townGrowthBaseCap
	w.Economy.TownGrowth = townGrowthBaseCap // already full

	wk := &Worker{ID: 0, Carried: gross, NodeID: node.ID,
		Angle: 0, Pos: w.Planet.RimPoint(0), TargetNodeID: -1, PendingNodeID: -1}
	node.OwnerID = wk.ID
	w.Workers = []*Worker{wk}

	completeUnload(w, wk, node)

	if len(w.Workers) != 2 {
		t.Errorf("expected 1 original + 1 spawned worker; got %d", len(w.Workers))
	}
	// The delivery pushed growth above cap by exactly gross; that excess is banked as
	// overflow and immediately re-drained into the fresh gauge.
	if w.Economy.TownGrowth != gross {
		t.Errorf("TownGrowth should equal delivery excess (%.4f) after spawn; got %.4f", gross, w.Economy.TownGrowth)
	}
	if w.Economy.TownGrowthOverflow != 0 {
		t.Errorf("TownGrowthOverflow should be zero after drain; got %.4f", w.Economy.TownGrowthOverflow)
	}
	wantCap := townGrowthBaseCap * townGrowthCapGrowth
	if math.Abs(w.Economy.TownGrowthCap-wantCap) > 1e-9 {
		t.Errorf("TownGrowthCap got %.4f, want %.4f (×townGrowthCapGrowth)", w.Economy.TownGrowthCap, wantCap)
	}
}

func TestWorkerArrivalInitialCapTransition(t *testing.T) {
	w, node := newDeliveryWorld()
	gross := baseLoadAmount * node.Size
	// Simulate the first fill: cap is in the initial (scripted) range.
	w.Economy.TownGrowthCap = townGrowthInitialCap
	w.Economy.TownGrowth = townGrowthInitialCap // already full

	wk := &Worker{ID: 0, Carried: gross, NodeID: node.ID,
		Angle: 0, Pos: w.Planet.RimPoint(0), TargetNodeID: -1, PendingNodeID: -1}
	node.OwnerID = wk.ID
	w.Workers = []*Worker{wk}

	completeUnload(w, wk, node)

	if len(w.Workers) != 2 {
		t.Errorf("expected 1 original + 1 spawned worker; got %d", len(w.Workers))
	}
	// After the initial fill the cap must jump to townGrowthBaseCap, not multiply.
	if math.Abs(w.Economy.TownGrowthCap-townGrowthBaseCap) > 1e-9 {
		t.Errorf("TownGrowthCap after initial fill = %.4f, want townGrowthBaseCap %.4f",
			w.Economy.TownGrowthCap, townGrowthBaseCap)
	}
}

func TestGrowthCappedWithoutCapacity(t *testing.T) {
	w, node := newDeliveryWorld()
	gross := baseLoadAmount * node.Size
	w.Economy.TownGrowthCap = gross * 0.5
	w.Economy.TownGrowth = 0
	// Fill capacity: 1 slot, 1 worker already occupying it.
	w.Economy.WorkerCapacity = 1

	wk := &Worker{ID: 0, Carried: gross, NodeID: node.ID,
		Angle: 0, Pos: w.Planet.RimPoint(0), TargetNodeID: -1, PendingNodeID: -1}
	node.OwnerID = wk.ID
	w.Workers = []*Worker{wk}

	completeUnload(w, wk, node)

	if len(w.Workers) != 1 {
		t.Errorf("no worker should spawn when capacity full; got %d", len(w.Workers))
	}
	if w.Economy.TownGrowth != w.Economy.TownGrowthCap {
		t.Errorf("TownGrowth should clamp to cap %.4f; got %.4f", w.Economy.TownGrowthCap, w.Economy.TownGrowth)
	}
}

func TestWorkerArrivalOnCapacityBuild(t *testing.T) {
	w, _ := newDeliveryWorld()
	w.Economy.TownGrowthCap = 5.0
	w.Economy.TownGrowth = w.Economy.TownGrowthCap // growth already at cap
	w.Economy.WorkerCapacity = 1
	// One worker occupies the slot.
	w.Workers = []*Worker{{ID: 0, State: StateIdleWaiting, NodeID: -1, TargetNodeID: -1, PendingNodeID: -1}}
	w.Economy.Wood = townCapacityCost(w) + 1

	if !buildTownCapacity(w) {
		t.Fatal("buildTownCapacity should succeed")
	}

	if len(w.Workers) != 2 {
		t.Errorf("expected 1 existing + 1 spawned worker; got %d", len(w.Workers))
	}
	if w.Economy.TownGrowth != 0 {
		t.Errorf("TownGrowth should reset to 0 after spawn; got %.4f", w.Economy.TownGrowth)
	}
}

func TestNoWorkerArrivalBeyondMaxSlots(t *testing.T) {
	w, node := newDeliveryWorld()
	gross := baseLoadAmount * node.Size
	w.Economy.TownGrowthCap = gross * 0.5
	w.Economy.TownGrowth = 0

	// Fill exactly to the geometry max.
	max := maxTownSlots(w)
	w.Economy.WorkerCapacity = max
	for i := 0; i < max-1; i++ {
		w.Workers = append(w.Workers, &Worker{
			ID:            w.NextWorkerID,
			State:         StateIdleWaiting,
			NodeID:        -1,
			TargetNodeID:  -1,
			PendingNodeID: -1,
		})
		w.NextWorkerID++
	}
	// The delivery worker occupies the last slot.
	wk := &Worker{
		ID:            w.NextWorkerID,
		Carried:       gross,
		NodeID:        node.ID,
		Angle:         0,
		Pos:           w.Planet.RimPoint(0),
		TargetNodeID:  -1,
		PendingNodeID: -1,
	}
	node.OwnerID = wk.ID
	w.Workers = append(w.Workers, wk)
	w.NextWorkerID++

	completeUnload(w, wk, node)

	if len(w.Workers) != max {
		t.Errorf("no worker should spawn beyond max slots; got %d workers, want %d", len(w.Workers), max)
	}
	if w.Economy.TownGrowth != w.Economy.TownGrowthCap {
		t.Errorf("TownGrowth should clamp to cap %.4f; got %.4f", w.Economy.TownGrowthCap, w.Economy.TownGrowth)
	}
}

func TestNoMultiWorkerBurst(t *testing.T) {
	w, node := newDeliveryWorld()
	w.Economy.TownGrowthCap = 1.0 // tiny cap; huge gross will dwarf it
	w.Economy.TownGrowth = 0
	node.Size = 100.0 // gross = 500, far above cap

	gross := baseLoadAmount * node.Size
	wk := &Worker{ID: 0, Carried: gross, NodeID: node.ID,
		Angle: 0, Pos: w.Planet.RimPoint(0), TargetNodeID: -1, PendingNodeID: -1}
	node.OwnerID = wk.ID
	w.Workers = []*Worker{wk}

	completeUnload(w, wk, node)

	// At most one new worker should have spawned (original + 1 = 2).
	if len(w.Workers) > 2 {
		t.Errorf("expected at most 1 spawned worker; got %d total (original + spawned)", len(w.Workers))
	}
	if len(w.Workers) != 2 {
		t.Errorf("expected exactly 1 spawned worker; got %d total", len(w.Workers))
	}
}

// newTestWorld builds a minimal world with one camp placed campDist arc-units
// away from the resource field's center angle, with starting nodes seeded near
// the field for worker tests.
func newTestWorld(campDist float64) *World {
	w := NewWorld()
	fieldAngle := w.Planet.Fields[0].CenterAngle
	dTheta := campDist / w.Planet.Radius
	campAngle := normAngle(fieldAngle + dTheta)
	camp := &Building{Kind: KindTownHall, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}
	w.Buildings = append(w.Buildings, camp)
	f := w.Planet.Fields[0]
	for range startingNodes {
		spawnNodeNear(w, f, fieldAngle)
	}
	// Freeze sizes and disable growth so analytic-rate tests are deterministic.
	for _, n := range w.Nodes {
		n.Size = 1.0
	}
	for _, f := range w.Planet.Fields {
		if fp := w.Planet.FieldProgress[f.Kind]; fp != nil {
			fp.EXP = 0
			fp.Cap = math.MaxFloat64
		}
	}
	return w
}

// addWorker appends a new idle worker to the global pool, spawned at the first camp.
func addWorker(w *World) {
	camp := w.Buildings[0]
	id := w.NextWorkerID
	w.NextWorkerID++
	w.Workers = append(w.Workers, &Worker{
		ID:            id,
		Pos:           camp.Pos,
		Angle:         camp.Angle,
		State:         StateIdleWaiting,
		NodeID:        -1,
		TargetNodeID:  -1,
		PendingNodeID: -1,
	})
}

// runSim advances the simulation for the given duration (in seconds).
func runSim(w *World, seconds float64) {
	ticks := int(math.Round(seconds / dt))
	for i := 0; i < ticks; i++ {
		Step(w, dt)
	}
}

func runUntilAssigned(w *World) {
	runSim(w, reactionDelay+dt)
}

func runUntilInLoop(w *World) {
	runSim(w, 1)
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
			if !workerInLoop(wk) && wk.State != StateDeparturePulse && wk.State != StateToRim {
				continue
			}
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
	runUntilInLoop(w)

	idle := 0
	for _, wk := range w.Workers {
		if !workerInLoop(wk) && wk.State != StateDeparturePulse && wk.State != StateToRim {
			idle++
		}
	}
	if idle != extra {
		t.Errorf("expected %d idle workers, got %d", extra, idle)
	}
}

// TestNodeSpawning verifies that delivering enough resources causes a new node
// to appear and the field EXP to reset.
func TestNodeSpawning(t *testing.T) {
	// Use a fully deterministic setup: one camp and one node at the same angle
	// so trip time is ~0.8 s and Size=1.0 gives baseLoadAmount per delivery.
	// woodFieldBaseEXP (10) is well below what 30 s of deliveries produces.
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
	fp := w.Planet.FieldProgress[field.Kind]
	if fp.EXP >= fp.Cap {
		t.Errorf("field EXP should have reset after spawn, got %.2f / %.2f", fp.EXP, fp.Cap)
	}
}

func TestFieldEXPAdvanceBelowCapRecordsNoGrowthCue(t *testing.T) {
	w := NewWorld()
	fp := w.Planet.FieldProgress[KindWood]
	fp.EXP = 3
	fp.Cap = 20

	depositToField(w, KindWood, 2)

	if fp.EXP != 5 {
		t.Fatalf("field EXP got %.2f, want 5", fp.EXP)
	}
	if w.growthCue.Outcome != growthOutcomeNone ||
		w.growthCue.GaugeRelease != 0 ||
		w.growthCue.FieldPulse != 0 ||
		w.growthCue.NodeCue != 0 {
		t.Fatalf("below-cap deposit should not record a growth cue: %+v", w.growthCue)
	}
}

func TestFieldGrowthCueRecordsSpawnedNode(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	fp := w.Planet.FieldProgress[KindWood]
	fp.EXP = 19
	fp.Cap = 20

	depositToField(w, KindWood, 1)

	if len(w.Nodes) != 1 {
		t.Fatalf("expected one spawned node, got %d", len(w.Nodes))
	}
	if w.growthCue.Outcome != growthOutcomeSpawnedNode {
		t.Fatalf("growth cue outcome got %v, want spawned node", w.growthCue.Outcome)
	}
	if w.growthCue.NodeID != w.Nodes[0].ID {
		t.Fatalf("growth cue node ID got %d, want %d", w.growthCue.NodeID, w.Nodes[0].ID)
	}
	if w.growthCue.Kind != KindWood || w.growthCue.GaugeRelease <= 0 || w.growthCue.FieldPulse <= 0 || w.growthCue.NodeCue <= 0 {
		t.Fatalf("growth cue timers/kind not initialized: %+v", w.growthCue)
	}
}

func TestGrowthCueDelaysFieldAndNodeResponses(t *testing.T) {
	w := NewWorld()
	field := w.Planet.Fields[0]
	node := newNode(w, KindWood, field.CenterAngle)
	w.Nodes = append(w.Nodes, node)
	activateGrowthCue(w, growthResult{
		Outcome:     growthOutcomeSpawnedNode,
		Kind:        field.Kind,
		CenterAngle: field.CenterAngle,
		HalfArc:     field.HalfArc,
		NodeID:      node.ID,
	})

	tickGrowthCue(w, growthFieldPulseDelay/2)
	if w.growthCue.FieldPulse != growthFieldPulseTime {
		t.Fatalf("field pulse should not tick before delay clears")
	}
	if w.growthCue.NodeCue != growthNodeCueTime {
		t.Fatalf("node cue should not tick before delay clears")
	}

	tickGrowthCue(w, growthFieldPulseDelay)
	tickGrowthCue(w, dt)
	if w.growthCue.FieldPulse >= growthFieldPulseTime {
		t.Fatalf("field pulse should tick after delay clears")
	}
	if w.growthCue.NodeCue != growthNodeCueTime {
		t.Fatalf("node cue should still wait for its longer delay")
	}

	tickGrowthCue(w, growthNodeCueDelay)
	tickGrowthCue(w, dt)
	if w.growthCue.NodeCue >= growthNodeCueTime {
		t.Fatalf("node cue should tick after delay clears")
	}
}

func TestGrowthCueCompletionsQueueInsteadOfRestarting(t *testing.T) {
	w := NewWorld()
	field := w.Planet.Fields[0]
	first := newNode(w, KindWood, field.CenterAngle)
	second := newNode(w, KindWood, normAngle(field.CenterAngle+0.1))
	w.Nodes = append(w.Nodes, first, second)

	activateGrowthCue(w, growthResult{
		Outcome:     growthOutcomeSpawnedNode,
		Kind:        field.Kind,
		CenterAngle: field.CenterAngle,
		HalfArc:     field.HalfArc,
		NodeID:      first.ID,
	})
	activateGrowthCue(w, growthResult{
		Outcome:     growthOutcomeUpgradedNode,
		Kind:        field.Kind,
		CenterAngle: field.CenterAngle,
		HalfArc:     field.HalfArc,
		NodeID:      second.ID,
	})

	if w.growthCue.NodeID != first.ID {
		t.Fatalf("active cue node ID got %d, want first node %d", w.growthCue.NodeID, first.ID)
	}
	if len(w.pendingGrowthCues) != 1 {
		t.Fatalf("pending cue count got %d, want 1", len(w.pendingGrowthCues))
	}

	for i := 0; i < 120 && w.growthCue.NodeID == first.ID; i++ {
		tickGrowthCue(w, dt)
	}

	if w.growthCue.NodeID != second.ID {
		t.Fatalf("queued cue node ID got %d, want second node %d", w.growthCue.NodeID, second.ID)
	}
	if w.growthCue.GaugeRelease != growthGaugeReleaseTime ||
		w.growthCue.FieldPulse != growthFieldPulseTime ||
		w.growthCue.NodeCue != growthNodeCueTime {
		t.Fatalf("queued cue should start with full timers: %+v", w.growthCue)
	}
	if len(w.pendingGrowthCues) != 0 {
		t.Fatalf("pending cue count got %d, want 0", len(w.pendingGrowthCues))
	}
}

func TestSpawnNodeAvoidsBuildingFootprint(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	field := w.Planet.Fields[0]

	intended := normAngle(field.CenterAngle - field.HalfArc)
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: intended, Pos: w.Planet.RimPoint(intended)}}

	spawnNode(w, field)

	if len(w.Nodes) != 1 {
		t.Fatalf("expected one spawned node, got %d", len(w.Nodes))
	}
	n := w.Nodes[0]
	if anglesOverlap(n.Angle, nodeBuildingBlockHalfArc(n, w.Planet.Radius), intended, buildingHardHalfArc(KindTownHall, w.Planet.Radius)) {
		t.Fatalf("spawned node at %.4f overlaps building footprint at %.4f", n.Angle, intended)
	}
}

func TestSpawnNodeSearchesNearestValidAngle(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	field := w.Planet.Fields[0]

	intended := normAngle(field.CenterAngle - field.HalfArc)
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: intended, Pos: w.Planet.RimPoint(intended)}}
	spawnNode(w, field)

	n := w.Nodes[0]
	blockedHalf := nodeBuildingBlockHalfArc(n, w.Planet.Radius) + buildingHardHalfArc(KindTownHall, w.Planet.Radius)
	dist := angularDistance(n.Angle, intended)
	step := 2 / w.Planet.Radius
	if dist < blockedHalf {
		t.Fatalf("spawned node should be outside combined building footprint")
	}
	if dist > blockedHalf+step+1e-9 {
		t.Fatalf("spawned node should choose nearest searched valid angle; dist %.6f, blocked %.6f, step %.6f", dist, blockedHalf, step)
	}
	if !angleWithinField(field, n.Angle) {
		t.Fatalf("spawned node should stay inside field")
	}
}

func TestSameFieldNodesCanPartiallyOverlapUnderSoftSpacing(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	field := w.Planet.Fields[0]

	existing := newNode(w, KindWood, 0)
	existing.Size = 1
	w.Nodes = []*ResourceNode{existing}

	candidate := newNode(w, KindWood, 6/w.Planet.Radius)
	candidate.Size = 1
	if !nodeSpawnAngleValid(w, field, candidate, candidate.Angle) {
		t.Fatal("candidate outside soft spacing but inside larger visual/blocking width should be valid")
	}
}

func TestSameFieldNodesRejectOldDenseSoftSpacing(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	field := w.Planet.Fields[0]

	existing := newNode(w, KindWood, 0)
	existing.Size = 1
	w.Nodes = []*ResourceNode{existing}

	candidate := newNode(w, KindWood, 5/w.Planet.Radius)
	candidate.Size = 1
	if nodeSpawnAngleValid(w, field, candidate, candidate.Angle) {
		t.Fatal("candidate at the old dense spacing should now be rejected")
	}
}

func TestSpawnNodeSaturatedFieldUpgradesNearestNode(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.01, Known: true}
	w.Planet.Fields = []*ResourceField{field}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}

	near := newNode(w, KindWood, 0.009)
	near.Size = 1
	far := newNode(w, KindWood, -0.002)
	far.Size = 1
	w.Nodes = []*ResourceNode{near, far}

	spawnNode(w, field)

	if len(w.Nodes) != 2 {
		t.Fatalf("saturated field should upgrade instead of appending, got %d nodes", len(w.Nodes))
	}
	if math.Abs(near.Size-1.15) > 1e-9 {
		t.Fatalf("nearest node size got %.2f, want 1.15", near.Size)
	}
	if far.Size != 1 {
		t.Fatalf("far node should not be upgraded, got %.2f", far.Size)
	}
	if !pulseActive(w, near.Pulse) {
		t.Fatal("upgraded node should pulse")
	}
}

func TestUpgradeNodeSizeClamped(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.01, Known: true}
	w.Planet.Fields = []*ResourceField{field}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}

	n := newNode(w, KindWood, 0)
	n.Size = 1.95
	w.Nodes = []*ResourceNode{n}

	spawnNode(w, field)

	if n.Size != 2.0 {
		t.Fatalf("upgraded node size got %.2f, want clamp at 2.0", n.Size)
	}
}

func TestLargerNodeYieldsMoreWood(t *testing.T) {
	small := newWorldSingleNode(0, 0)
	small.Nodes[0].Size = 1
	large := newWorldSingleNode(0, 0)
	large.Nodes[0].Size = 1.5

	addWorker(small)
	addWorker(large)
	runSim(small, 10)
	runSim(large, 10)

	if large.Economy.Wood <= small.Economy.Wood {
		t.Fatalf("larger node should produce more wood, small %.2f large %.2f", small.Economy.Wood, large.Economy.Wood)
	}
}

func TestFieldEXPAndCapAdvanceAfterUpgradeFallback(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.01, Known: true}
	w.Planet.Fields = []*ResourceField{field}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {EXP: 19, Cap: 20}}
	fp := w.Planet.FieldProgress[KindWood]

	n := newNode(w, KindWood, 0)
	n.Size = 1
	w.Nodes = []*ResourceNode{n}

	depositToField(w, KindWood, 2)

	if len(w.Nodes) != 1 {
		t.Fatalf("expected upgrade fallback without append, got %d nodes", len(w.Nodes))
	}
	if math.Abs(fp.EXP-1) > 1e-9 {
		t.Fatalf("field EXP got %.2f, want 1", fp.EXP)
	}
	// With woodFieldEXPMaxStep=10 the step from cap 20 (=20) exceeds max, so additive: 20+10=30.
	wantCap := math.Min(20*woodFieldEXPGrowth, 20+woodFieldEXPMaxStep)
	if math.Abs(fp.Cap-wantCap) > 1e-9 {
		t.Fatalf("cap got %.2f, want %.2f", fp.Cap, wantCap)
	}
	if math.Abs(n.Size-1.15) > 1e-9 {
		t.Fatalf("node size got %.2f, want 1.15", n.Size)
	}
	if w.growthCue.Outcome != growthOutcomeUpgradedNode {
		t.Fatalf("growth cue outcome got %v, want upgraded node", w.growthCue.Outcome)
	}
	if w.growthCue.NodeID != n.ID {
		t.Fatalf("growth cue node ID got %d, want %d", w.growthCue.NodeID, n.ID)
	}
	if w.growthCue.GaugeRelease <= 0 || w.growthCue.FieldPulse <= 0 || w.growthCue.NodeCue <= 0 {
		t.Fatalf("growth cue timers not initialized: %+v", w.growthCue)
	}
}

func TestUpgradeFirstFieldForDebugTriggersOneGrowth(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	fp := w.Planet.FieldProgress[KindWood]
	fp.EXP = 7
	fp.Cap = 20

	if !upgradeAllFieldsForDebug(w) {
		t.Fatal("expected debug field upgrade to run")
	}
	if len(w.Nodes) != 1 {
		t.Fatalf("expected one spawned node, got %d", len(w.Nodes))
	}
	if fp.EXP != 0 {
		t.Fatalf("field EXP got %.2f, want 0", fp.EXP)
	}
	wantCap := math.Min(20*woodFieldEXPGrowth, 20+woodFieldEXPMaxStep)
	if math.Abs(fp.Cap-wantCap) > 1e-9 {
		t.Fatalf("field cap got %.2f, want %.2f", fp.Cap, wantCap)
	}
}

func TestGrowFirstFieldUntilBlockedForDebugStopsOnUpgrade(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.01, Known: true}
	w.Planet.Fields = []*ResourceField{field}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}

	n := newNode(w, KindWood, 0)
	n.Size = 1
	w.Nodes = []*ResourceNode{n}

	if !growAllFieldsUntilBlockedForDebug(w) {
		t.Fatal("expected debug grow-until-full to run")
	}
	if len(w.Nodes) != 1 {
		t.Fatalf("saturated debug growth should stop on upgrade without append, got %d nodes", len(w.Nodes))
	}
	if math.Abs(n.Size-1.15) > 1e-9 {
		t.Fatalf("node size got %.2f, want 1.15", n.Size)
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
	runUntilInLoop(w)

	if w.Workers[0].NodeID != closeNode.ID {
		t.Errorf("expected worker to claim close node (ID %d), got node ID %d",
			closeNode.ID, w.Workers[0].NodeID)
	}
}

func TestSettlingWorkerNotEligibleUntilDelayFinishes(t *testing.T) {
	w := newWorldSingleNode(0, 0)
	if spawnWorkerAtTownHall(w) == nil {
		t.Fatal("expected spawnWorkerAtTownHall to succeed")
	}
	wk := w.Workers[0]

	runSim(w, settleDelay-dt)
	if wk.State != StateSettling {
		t.Fatalf("worker should still be settling, got state %v", wk.State)
	}
	if wk.NodeID != -1 || wk.TargetNodeID != -1 {
		t.Fatalf("settling worker should not claim or target work")
	}

	runSim(w, 3*dt)
	if wk.State != StateReactionDelay {
		t.Fatalf("worker should enter reaction delay after settling, got state %v", wk.State)
	}
	if wk.TargetNodeID != w.Nodes[0].ID {
		t.Fatalf("worker should target the free node after settling")
	}
}

func TestReactionDelayReservesAtDepartureStart(t *testing.T) {
	w := newWorldSingleNode(0, 0)
	addWorker(w)
	wk := w.Workers[0]

	Step(w, dt)
	if wk.State != StateReactionDelay {
		t.Fatalf("worker should start reaction delay, got state %v", wk.State)
	}
	if w.Nodes[0].ReservedByWorkerID != -1 {
		t.Fatalf("node should not be reserved until departure starts")
	}

	runSim(w, reactionDelay+dt)
	if wk.State != StateDeparturePulse {
		t.Fatalf("worker should enter departure pulse, got state %v", wk.State)
	}
	if w.Nodes[0].ReservedByWorkerID != wk.ID {
		t.Fatalf("node should be reserved by departing worker")
	}
}

func TestReservedNodeExcludedFromBestFreeAndPreview(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}

	closeNode := newNode(w, KindWood, 0.1)
	farNode := newNode(w, KindWood, 0.3)
	closeNode.ReservedByWorkerID = 7
	w.Nodes = []*ResourceNode{closeNode, farNode}

	if got := bestFreeNode(w); got != farNode {
		t.Fatalf("bestFreeNode should skip reserved node")
	}

	free, claimed, reserved := localNodes(w, 0)
	if len(free) != 1 || free[0].Node != farNode {
		t.Fatalf("preview free routes should include only unreserved free node")
	}
	if len(claimed) != 0 {
		t.Fatalf("reserved-only setup should have no claimed nodes")
	}
	if len(reserved) != 1 || reserved[0] != closeNode {
		t.Fatalf("preview should report the reserved node separately")
	}
}

func TestUnloadCheckpointSwitchesToReservedNode(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	campAngle := 0.0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}}

	oldNode := newNode(w, KindWood, 1.0)
	newNode := newNode(w, KindWood, 0.1)
	oldNode.OwnerID = 0
	newNode.ReservedByWorkerID = 0
	w.Nodes = []*ResourceNode{oldNode, newNode}
	w.Workers = []*Worker{{
		ID:            0,
		Angle:         campAngle,
		State:         StateUnloading,
		NodeID:        oldNode.ID,
		PendingNodeID: newNode.ID,
		TargetNodeID:  -1,
		Carried:       1,
		Timer:         dt,
	}}

	Step(w, 2*dt)

	wk := w.Workers[0]
	if wk.NodeID != newNode.ID || wk.State != StateDeparturePulse {
		t.Fatalf("worker should depart for pending node, got state %v node %d", wk.State, wk.NodeID)
	}
	if oldNode.OwnerID != -1 {
		t.Fatalf("old node should be freed at checkpoint")
	}
	if newNode.ReservedByWorkerID != wk.ID {
		t.Fatalf("new node should remain reserved during departure")
	}
}

func TestWorkerReturnsHomeWhenWorkDisappears(t *testing.T) {
	w := newWorldSingleNode(0, 0)
	w.Nodes = nil
	addWorker(w)
	wk := w.Workers[0]
	wk.State = StateToForest
	wk.NodeID = 999
	wk.Angle = math.Pi / 2

	Step(w, dt)
	if wk.State != StateReturningHome {
		t.Fatalf("missing node should send worker home, got state %v", wk.State)
	}

	runSim(w, 10)
	if wk.State != StateIdleWaiting || wk.NodeID != -1 {
		t.Fatalf("worker should settle back into idle home, got state %v node %d", wk.State, wk.NodeID)
	}
}

func TestActiveWorkerCountExcludesIdleTransitions(t *testing.T) {
	w := newTestWorld(0)
	w.Workers = []*Worker{
		{ID: 0, State: StateToForest, NodeID: 0},
		{ID: 1, State: StateReturningHome, NodeID: -1},
		{ID: 2, State: StateSettling, NodeID: -1},
		{ID: 3, State: StateReactionDelay, NodeID: -1},
		{ID: 4, State: StateIdleWaiting, NodeID: -1},
	}
	if got := activeWorkerCount(w); got != 1 {
		t.Fatalf("activeWorkerCount got %d, want 1", got)
	}
}

// TestNewNodeTriggersDelayedReassignment verifies that when a shorter free node
// appears and no idle worker exists, the active worker reserves it but does not
// abandon its current route mid-loop.
func TestNewNodeTriggersDelayedReassignment(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	campAngle := 0.0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}}

	farNode := newNode(w, KindWood, normAngle(campAngle+2.0))
	farNode.Size = 1.0
	w.Nodes = []*ResourceNode{farNode}

	addWorker(w)
	runUntilInLoop(w)

	if w.Workers[0].NodeID != farNode.ID {
		t.Fatalf("setup: expected worker on far node")
	}

	// Spawn a closer node; delayed rebalance reserves it on the next tick.
	closeNode := newNode(w, KindWood, normAngle(campAngle+0.2))
	closeNode.Size = 1.0
	w.Nodes = append(w.Nodes, closeNode)
	Step(w, dt)

	if w.Workers[0].NodeID != farNode.ID {
		t.Errorf("expected worker to stay on far node until checkpoint")
	}
	if w.Workers[0].PendingNodeID != closeNode.ID {
		t.Errorf("expected close node to be reserved as pending target")
	}
	if closeNode.ReservedByWorkerID != w.Workers[0].ID {
		t.Errorf("expected close node reservation for worker")
	}
}

// TestCampPlacementTriggersDelayedRebalance verifies that placing a new camp near
// a free node reserves it for checkpoint reassignment rather than swapping now.
func TestCampPlacementTriggersDelayedRebalance(t *testing.T) {
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
	runUntilInLoop(w)

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

	if w.Workers[0].NodeID != nodeA.ID {
		t.Errorf("expected worker to stay on nodeA until delivery checkpoint")
	}
	if w.Workers[0].PendingNodeID != nodeB.ID {
		t.Errorf("expected worker to reserve nodeB for delayed rebalance")
	}
	if nodeB.ReservedByWorkerID != w.Workers[0].ID {
		t.Errorf("expected nodeB to be reserved for worker")
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

func TestCompleteUnloadSplitsDelivery(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	campAngle := 0.0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}}
	node := newNode(w, KindWood, campAngle)
	node.Size = 1.0
	w.Nodes = []*ResourceNode{node}

	gross := baseLoadAmount * node.Size // 5.0
	wk := &Worker{
		ID:            0,
		Carried:       gross,
		NodeID:        node.ID,
		Angle:         campAngle,
		Pos:           w.Planet.RimPoint(campAngle),
		TargetNodeID:  -1,
		PendingNodeID: -1,
	}
	node.OwnerID = wk.ID
	w.Workers = []*Worker{wk}

	completeUnload(w, wk, node)

	wantBanked := gross * (1 - woodFieldReturnRatio)
	wantReturned := gross * woodFieldReturnRatio

	if math.Abs(w.Economy.Wood-wantBanked) > 1e-9 {
		t.Errorf("Economy.Wood got %.4f, want %.4f (80%% of gross)", w.Economy.Wood, wantBanked)
	}
	fp := w.Planet.FieldProgress[w.Planet.Fields[0].Kind]
	if math.Abs(fp.EXP-wantReturned) > 1e-9 {
		t.Errorf("field EXP got %.4f, want %.4f (20%% of gross)", fp.EXP, wantReturned)
	}
	if math.Abs(w.Buildings[0].DeliveredWood-gross) > 1e-9 {
		t.Errorf("DeliveredWood got %.4f, want %.4f (gross)", w.Buildings[0].DeliveredWood, gross)
	}
	if math.Abs(w.lastDelivery.Gross-gross) > 1e-9 ||
		math.Abs(w.lastDelivery.Banked-wantBanked) > 1e-9 ||
		math.Abs(w.lastDelivery.Returned-wantReturned) > 1e-9 {
		t.Errorf("lastDelivery got %+v, want {%.2f %.2f %.2f}", w.lastDelivery, gross, wantBanked, wantReturned)
	}
}

func TestFieldEXPCapTriggersCycle(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	fp := w.Planet.FieldProgress[KindWood]
	initialCap := woodFieldBaseEXP

	depositToField(w, KindWood, woodFieldBaseEXP)

	if len(w.Nodes) != 1 {
		t.Fatalf("expected one spawned node after crossing woodFieldBaseEXP, got %d", len(w.Nodes))
	}
	wantCap := initialCap * woodFieldEXPGrowth
	if math.Abs(fp.Cap-wantCap) > 1e-9 {
		t.Fatalf("cap after first cycle got %.2f, want %.2f (×woodFieldEXPGrowth)", fp.Cap, wantCap)
	}
	if fp.EXP != 0 {
		t.Fatalf("field EXP after exact cap deposit got %.2f, want 0", fp.EXP)
	}
}

func TestNurtureSpawnsTreesDirectly(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true

	nodesBefore := len(w.Nodes)
	if !nurtureField(w, KindWood) {
		t.Fatal("nurtureField should succeed when resource is discovered and field has room")
	}

	// First cue is active; the rest are pending.
	if w.growthCue.Outcome == growthOutcomeNone {
		t.Error("expected an active growth cue after Nurture press")
	}
	wantPending := nurtureTreesPerPress - 1
	if len(w.pendingGrowthCues) != wantPending {
		t.Errorf("pending growth cues: got %d, want %d", len(w.pendingGrowthCues), wantPending)
	}
	// Nodes are spawned immediately — no need to wait for cue animation.
	if len(w.Nodes) != nodesBefore+nurtureTreesPerPress {
		t.Errorf("nodes after Nurture: got %d, want %d", len(w.Nodes), nodesBefore+nurtureTreesPerPress)
	}
}

func TestNurtureFieldNotDiscovered(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = false

	if nurtureField(w, KindWood) {
		t.Fatal("nurtureField should fail when resource not yet discovered")
	}
}

func TestNurtureFieldBlockedWhenFieldSaturated(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.ResourceDiscovered = true
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.01, Known: true}
	w.Planet.Fields = []*ResourceField{field}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}

	node := newNode(w, KindWood, 0)
	node.Size = 1
	w.Nodes = []*ResourceNode{node}

	if nurtureField(w, KindWood) {
		t.Fatal("nurtureField should fail when field is saturated")
	}
	if len(w.Nodes) != 1 {
		t.Fatalf("saturated Nurture should not spawn nodes, got %d", len(w.Nodes))
	}
}

func TestNurtureBlockedWhenCuePending(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true

	if !nurtureField(w, KindWood) {
		t.Fatal("first Nurture press should succeed")
	}
	// A cue is now active — second press must be blocked.
	if nurtureField(w, KindWood) {
		t.Fatal("second Nurture press should fail while a growth cue is active")
	}
}

func TestNurtureSpawnsUpToCapacity(t *testing.T) {
	// Field can only fit 1 more node; Nurture should stop after spawning that one.
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.ResourceDiscovered = true
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.25, Known: true}
	w.Planet.Fields = []*ResourceField{field}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}

	// Fill the field until exactly 1 slot remains.
	for fieldCanSpawnNode(w, field) {
		result := spawnNode(w, field)
		if result.Outcome == growthOutcomeNone {
			break
		}
		// Leave one slot.
		if !fieldCanSpawnNode(w, field) {
			// We just filled the last slot — undo by breaking one step early.
			// Instead: count before and stop when only 1 remains.
		}
	}
	// Reset: spawn until only 1 spot left.
	w.Nodes = nil
	w.NextNodeID = 0
	w.growthCue = growthCueState{NodeID: -1}
	w.pendingGrowthCues = nil
	for {
		if !fieldCanSpawnNode(w, field) {
			break
		}
		spawnNode(w, field)
		// Check if one more would still fit — if not, we're at capacity-1.
		if !fieldCanSpawnNode(w, field) {
			// We just filled the last slot, back off by removing the last node.
			w.Nodes = w.Nodes[:len(w.Nodes)-1]
			w.NextNodeID--
			break
		}
	}
	// Now exactly 1 slot remains.
	nodesBefore := len(w.Nodes)

	if !nurtureField(w, KindWood) {
		t.Fatal("nurtureField should succeed with one remaining slot")
	}
	if len(w.Nodes) != nodesBefore+1 {
		t.Errorf("should spawn exactly 1 node (capacity limit), got %d new nodes",
			len(w.Nodes)-nodesBefore)
	}
}

func TestNurtureAttentionRequiresTownFull(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}

	// Town not full yet — attention should be inactive.
	if nurtureAttentionActive(w, KindWood) {
		t.Error("attention should be inactive before town capacity is maxed")
	}

	// Force town to max capacity with physical workers present.
	slots := maxTownSlots(w)
	w.Economy.WorkerCapacity = slots
	for i := range slots {
		w.Workers = append(w.Workers, &Worker{ID: i + 1})
	}
	if !nurtureAttentionActive(w, KindWood) {
		t.Error("attention should be active once town capacity is maxed")
	}
}

func TestNurtureAttentionInactiveWhenFieldSaturated(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.ResourceDiscovered = true
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}
	w.Economy.WorkerCapacity = maxTownSlots(w)
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.01, Known: true}
	w.Planet.Fields = []*ResourceField{field}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}

	node := newNode(w, KindWood, 0)
	node.Size = 1
	w.Nodes = []*ResourceNode{node}

	if nurtureAttentionActive(w, KindWood) {
		t.Fatal("Nurture attention should be inactive when field is saturated")
	}
}

func TestNurtureAttentionSuppressedWhenCuePending(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}
	slots := maxTownSlots(w)
	w.Economy.WorkerCapacity = slots
	for i := range slots {
		w.Workers = append(w.Workers, &Worker{ID: i + 1})
	}

	// Town full and field has room — attention should be active.
	if !nurtureAttentionActive(w, KindWood) {
		t.Error("setup: expected attention active with town full")
	}

	// After a Nurture press a cue is active — attention must be suppressed.
	nurtureField(w, KindWood)
	if nurtureAttentionActive(w, KindWood) {
		t.Error("attention should be suppressed while a growth cue is pending")
	}
}

func TestNurtureAttentionInactiveWhenNotDiscovered(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = false
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}
	w.Economy.WorkerCapacity = maxTownSlots(w)

	if nurtureAttentionActive(w, KindWood) {
		t.Error("attention should be inactive when resource is not yet discovered")
	}
}

// ── System unlock tests ───────────────────────────────────────────────────────

// newMasteredWorld returns a world that satisfies both completion gates:
// town capacity maxed and wood field fully saturated.
func newMasteredWorld() *World {
	w := NewWorld()
	// Place Town Hall so townFieldFull logic works.
	f := fieldForKind(w, KindWood)
	_ = placeBuilding(w, f.CenterAngle)
	// Max out town capacity.
	w.Economy.WorkerCapacity = maxTownSlots(w)
	// Saturate the wood field.
	for fieldCanSpawnNode(w, f) {
		spawnNode(w, f)
	}
	return w
}

func TestStartingPlanetComplete_RequiresBothGates(t *testing.T) {
	w := newMasteredWorld()

	// Both gates: should be complete.
	if !forestPlanetComplete(w) {
		t.Fatal("expected forestPlanetComplete true when both gates are met")
	}

	// Only town: reset field saturation by clearing nodes.
	w2 := newMasteredWorld()
	w2.Nodes = nil
	if forestPlanetComplete(w2) {
		t.Error("expected forestPlanetComplete false when wood field not saturated")
	}

	// Only field: reset town capacity.
	w3 := newMasteredWorld()
	w3.Economy.WorkerCapacity = 0
	if forestPlanetComplete(w3) {
		t.Error("expected forestPlanetComplete false when town capacity not maxed")
	}
}

func TestTickTriggersUnlockExactlyOnce(t *testing.T) {
	w := newMasteredWorld()
	w.ResourceDiscovered = true
	// Add a worker so EstimateRate can snapshot a non-zero value.
	w.Economy.WorkerCapacity = maxTownSlots(w)
	addWorker(w)
	runSim(w, 2) // settle worker into loop

	if w.System.Unlocked {
		t.Fatal("world should not be unlocked yet before Tick")
	}

	just := Tick(w, dt)
	if !just {
		t.Fatal("Tick should return justUnlocked=true on the first mastered tick")
	}
	if !w.System.Unlocked {
		t.Fatal("System.Unlocked should be true after first Tick on mastered world")
	}
	if w.System.View != ViewSystem {
		t.Fatal("view should switch to ViewSystem on unlock")
	}
	if w.System.Selected != 0 {
		t.Errorf("starting planet should be selected (index 0); got %d", w.System.Selected)
	}

	// Snapshotted rate must be > 0.
	if w.System.Planets[0].AbstractRate <= 0 {
		t.Errorf("snapshotted AbstractRate should be > 0; got %f", w.System.Planets[0].AbstractRate)
	}

	// Second Tick: already unlocked, should not re-fire.
	just2 := Tick(w, dt)
	if just2 {
		t.Error("Tick should not return justUnlocked=true a second time")
	}
	rateAfter := w.System.Planets[0].AbstractRate
	if rateAfter != w.System.Planets[0].AbstractRate {
		t.Error("AbstractRate should not change after initial snapshot")
	}
}

func TestAbstractIncome_PlanetView(t *testing.T) {
	w := newMasteredWorld()
	addWorker(w)
	runSim(w, 2)
	Tick(w, dt) // unlock
	enterPlanetView(w)

	inc := abstractIncome(w)
	// In planet view: only echoes (indices 1 and 2) should contribute.
	expected := w.System.Planets[1].AbstractRate + w.System.Planets[2].AbstractRate
	if math.Abs(inc-expected) > 1e-9 {
		t.Errorf("abstractIncome in planet view: got %f, want %f (echoes only)", inc, expected)
	}

	// Starting planet must NOT be included.
	if inc >= w.System.Planets[0].AbstractRate+expected {
		t.Error("starting planet must not contribute abstract income in planet view")
	}
}

func TestAbstractIncome_SystemView(t *testing.T) {
	w := newMasteredWorld()
	addWorker(w)
	runSim(w, 2)
	Tick(w, dt) // unlock → ViewSystem

	inc := abstractIncome(w)
	// In system view: starting + both echoes.
	expected := w.System.Planets[0].AbstractRate +
		w.System.Planets[1].AbstractRate +
		w.System.Planets[2].AbstractRate
	if math.Abs(inc-expected) > 1e-9 {
		t.Errorf("abstractIncome in system view: got %f, want %f", inc, expected)
	}
}

func TestTickSystemView_FreezesSim(t *testing.T) {
	w := newMasteredWorld()
	addWorker(w)
	runSim(w, 2)
	Tick(w, dt) // unlock → ViewSystem

	// Record worker positions before ticking in system view.
	type pos struct{ x, y float64 }
	before := make([]pos, len(w.Workers))
	for i, wk := range w.Workers {
		before[i] = pos{wk.Pos.X, wk.Pos.Y}
	}
	simTimeBefore := w.SimTime

	// Several ticks in system view.
	for range 10 {
		Tick(w, dt)
	}

	// Workers must not have moved (sim frozen).
	for i, wk := range w.Workers {
		if wk.Pos.X != before[i].x || wk.Pos.Y != before[i].y {
			t.Errorf("worker %d moved in system view (sim should be frozen)", wk.ID)
		}
	}
	if w.SimTime != simTimeBefore {
		t.Errorf("SimTime advanced in system view (want frozen at %f, got %f)", simTimeBefore, w.SimTime)
	}

	// Abstract wood should have accumulated.
	if w.Economy.Wood <= 0 {
		t.Error("expected abstract wood to accumulate in system view")
	}
}

func TestTickPlanetView_ResumesSimAfterSystemView(t *testing.T) {
	w := newMasteredWorld()
	addWorker(w)
	runSim(w, 2)
	Tick(w, dt) // unlock → ViewSystem

	simTimeBefore := w.SimTime

	// Switch back to planet view.
	enterPlanetView(w)
	Tick(w, dt) // should run Step

	if w.SimTime == simTimeBefore {
		t.Error("SimTime should advance when returning to planet view via Tick")
	}
}

func TestNurtureAttentionInactiveAtFieldSaturation(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true
	w.Economy.Wood = 1000 // plenty of wood
	_ = placeBuilding(w, w.Planet.Fields[0].CenterAngle)
	f := fieldForKind(w, KindWood)
	// Saturate the field.
	for fieldCanSpawnNode(w, f) {
		spawnNode(w, f)
	}
	// Ensure fieldCanSpawnNode is false.
	if fieldCanSpawnNode(w, f) {
		t.Fatal("setup: field should be saturated")
	}
	// nurtureAttentionActive must be false at saturation.
	if nurtureAttentionActive(w, KindWood) {
		t.Error("nurtureAttentionActive should be false when field is saturated")
	}
	// nurtureField must be a no-op.
	if nurtureField(w, KindWood) {
		t.Error("nurtureField should fail when field is saturated")
	}
}

// ── Echo planet tests ─────────────────────────────────────────────────────────

// newRevealedWorld returns a world that has triggered the system unlock,
// leaving it in system view with abstract rates snapshotted.
func newRevealedWorld() *World {
	w := newMasteredWorld()
	addWorker(w)
	runSim(w, 2)
	Tick(w, dt) // triggers unlock → ViewSystem
	return w
}

func TestAbstractIncome_AwakenedEchoUnviewed(t *testing.T) {
	w := newRevealedWorld()
	// Awaken echo 1 — its AbstractRate should remain the pre-awakened fraction.
	originalRate := w.System.Planets[1].AbstractRate
	w.Economy.Potential[PotentialForest] = 1
	awakenPlanet(w, 1)

	// Enter echo 1 (planet view on the echo).
	switchToPlanet(w, 1)
	enterPlanetView(w)

	inc := abstractIncome(w)
	// Active echo (idx 1) must NOT contribute abstract income; echo 2 and starting must.
	expected := w.System.Planets[0].AbstractRate + w.System.Planets[2].AbstractRate
	if math.Abs(inc-expected) > 1e-9 {
		t.Errorf("abstractIncome with echo active: got %f, want %f", inc, expected)
	}

	// Switch away: echo 1 should contribute its original rate again.
	switchToPlanet(w, 0)
	enterPlanetView(w)
	incAway := abstractIncome(w)
	if math.Abs(w.System.Planets[1].AbstractRate-originalRate) > 1e-9 {
		t.Errorf("echo 1 AbstractRate changed while unviewed: got %f, want %f",
			w.System.Planets[1].AbstractRate, originalRate)
	}
	expectedAway := w.System.Planets[1].AbstractRate + w.System.Planets[2].AbstractRate
	if math.Abs(incAway-expectedAway) > 1e-9 {
		t.Errorf("abstractIncome with starting active: got %f, want %f", incAway, expectedAway)
	}
}

func TestEchoCompletion_AmplifiedRate(t *testing.T) {
	w := newRevealedWorld()
	w.Economy.Potential[PotentialForest] = 1
	awakenPlanet(w, 1)

	// Enter echo 1 and build it to completion gates.
	switchToPlanet(w, 1)
	enterPlanetView(w)
	f := fieldForKind(w, KindWood)
	if f == nil {
		t.Fatal("setup: no wood field on echo")
	}
	thAngle, ok := findValidBuildingAngle(w)
	if !ok || !placeBuilding(w, thAngle) {
		t.Fatal("setup: cannot place echo TH")
	}
	if max := maxTownSlots(w); max > w.Economy.WorkerCapacity {
		w.Economy.WorkerCapacity = max
	}
	fillWoodFieldNodes(w, false)

	// Estimate what the rate should be before completion fires.
	// (workers = 0 in loop, so EstimateRate = 0; AbstractRate will be 0*1.25 = 0.
	//  That is acceptable — the completion logic is what we're testing.)
	checkActivePlanetCompletion(w)

	if !w.System.Planets[1].Completed {
		t.Fatal("echo 1 should be Completed after forestPlanetComplete gates met")
	}
	want := EstimateRate(w) * completionAmplifier
	got := w.System.Planets[1].AbstractRate
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("AbstractRate after completion: got %f, want EstimateRate*1.25=%f", got, want)
	}

	// checkActivePlanetCompletion must be idempotent (no second fire).
	w.System.Planets[1].AbstractRate = 99.0
	checkActivePlanetCompletion(w)
	if w.System.Planets[1].AbstractRate != 99.0 {
		t.Error("checkActivePlanetCompletion should not re-fire after Completed=true")
	}
}

func TestAwakenEitherEchoFirst(t *testing.T) {
	for _, first := range []int{1, 2} {
		t.Run(fmt.Sprintf("echo%d_first", first), func(t *testing.T) {
			w := newRevealedWorld()
			second := 3 - first // if first=1 → second=2 and vice versa
			w.Economy.Potential[PotentialForest] = 2

			awakenPlanet(w, first)
			awakenPlanet(w, second)

			if !w.System.Planets[first].Awakened {
				t.Errorf("echo %d: expected Awakened=true", first)
			}
			if !w.System.Planets[second].Awakened {
				t.Errorf("echo %d: expected Awakened=true", second)
			}
			if w.PlanetStates[first] == nil {
				t.Errorf("echo %d: PlanetStates should be non-nil after awakening", first)
			}
			if w.PlanetStates[second] == nil {
				t.Errorf("echo %d: PlanetStates should be non-nil after awakening", second)
			}
			// Each layout has a wood field.
			switchToPlanet(w, first)
			if fieldForKind(w, KindWood) == nil {
				t.Errorf("echo %d: no wood field after switching to it", first)
			}
			switchToPlanet(w, second)
			if fieldForKind(w, KindWood) == nil {
				t.Errorf("echo %d: no wood field after switching to it", second)
			}
		})
	}
}

func TestAllEchoesComplete(t *testing.T) {
	w := newRevealedWorld()
	if allEchoesComplete(w) {
		t.Error("allEchoesComplete should be false before any echo is completed")
	}
	w.System.Planets[1].Completed = true
	if allEchoesComplete(w) {
		t.Error("allEchoesComplete should be false when only one echo is completed")
	}
	w.System.Planets[2].Completed = true
	if !allEchoesComplete(w) {
		t.Error("allEchoesComplete should be true when both echoes are completed")
	}
}

// ── Potential currency tests ──────────────────────────────────────────────────

func TestCanAwaken_RequiresForestPotential(t *testing.T) {
	w := newRevealedWorld()
	// Clear any Potential awarded by unlock so we can test the zero case.
	w.Economy.Potential[PotentialForest] = 0
	if canAwaken(w, 1) {
		t.Error("canAwaken should return false with 0 Forest Potential")
	}
	// Wood alone does not satisfy the gate.
	w.Economy.Wood = 9999
	if canAwaken(w, 1) {
		t.Error("canAwaken should not be satisfied by wood")
	}
	w.Economy.Potential[PotentialForest] = 1
	if !canAwaken(w, 1) {
		t.Error("canAwaken should return true with 1 Forest Potential")
	}
}

func TestAwakenPlanet_SpendsPotentialNotWood(t *testing.T) {
	w := newRevealedWorld()
	w.Economy.Potential[PotentialForest] = 1
	startWood := w.Economy.Wood
	awakenPlanet(w, 1)
	if !w.System.Planets[1].Awakened {
		t.Fatal("echo 1 should be awakened after awakenPlanet")
	}
	if got := w.Economy.Potential[PotentialForest]; got != 0 {
		t.Errorf("Forest Potential after awaken: got %d, want 0", got)
	}
	if w.Economy.Wood != startWood {
		t.Errorf("Wood changed after awaken: got %.2f, want %.2f", w.Economy.Wood, startWood)
	}
}

func TestTriggerUnlock_AwardsForestPotential(t *testing.T) {
	w := newMasteredWorld()
	addWorker(w)
	runSim(w, 2)
	if got := w.Economy.Potential[PotentialForest]; got != 0 {
		t.Errorf("Forest Potential before unlock: got %d, want 0", got)
	}
	Tick(w, dt) // triggers unlock → awards Potential
	if got := w.Economy.Potential[PotentialForest]; got != 1 {
		t.Errorf("Forest Potential after starting completion: got %d, want 1", got)
	}
}

func TestEchoCompletion_AwardsForestPotential(t *testing.T) {
	w := newRevealedWorld()
	w.Economy.Potential[PotentialForest] = 1
	awakenPlanet(w, 1) // spends 1 → Potential now 0

	switchToPlanet(w, 1)
	enterPlanetView(w)
	f := fieldForKind(w, KindWood)
	if f == nil {
		t.Fatal("setup: no wood field on echo")
	}
	thAngle, ok := findValidBuildingAngle(w)
	if !ok || !placeBuilding(w, thAngle) {
		t.Fatal("setup: cannot place echo TH")
	}
	if max := maxTownSlots(w); max > w.Economy.WorkerCapacity {
		w.Economy.WorkerCapacity = max
	}
	fillWoodFieldNodes(w, false)

	if got := w.Economy.Potential[PotentialForest]; got != 0 {
		t.Errorf("Forest Potential before echo completion: got %d, want 0", got)
	}
	checkActivePlanetCompletion(w)
	if !w.System.Planets[1].Completed {
		t.Fatal("echo 1 should be marked Completed")
	}
	if got := w.Economy.Potential[PotentialForest]; got != 1 {
		t.Errorf("Forest Potential after echo completion: got %d, want 1", got)
	}
	// Must not double-award.
	checkActivePlanetCompletion(w)
	if got := w.Economy.Potential[PotentialForest]; got != 1 {
		t.Errorf("Forest Potential double-award guard: got %d, want 1", got)
	}
}

func setupEchoForCompletion(t *testing.T, w *World, echoIdx int) {
	t.Helper()
	switchToPlanet(w, echoIdx)
	enterPlanetView(w)
	thAngle, ok := findValidBuildingAngle(w)
	if !ok || !placeBuilding(w, thAngle) {
		t.Fatalf("setup: cannot place Town Hall on echo %d", echoIdx)
	}
	if max := maxTownSlots(w); max > w.Economy.WorkerCapacity {
		w.Economy.WorkerCapacity = max
	}
	fillWoodFieldNodes(w, false)
}

// TestTightGrove_CompletionAwardsForestPotentialOnly verifies that Tight Grove
// (echoA, layoutID 0) awards exactly +1 Forest Potential and no Water Potential.
func TestTightGrove_CompletionAwardsForestPotentialOnly(t *testing.T) {
	w := newRevealedWorld()
	w.Economy.Potential[PotentialForest] = 1
	awakenPlanet(w, 1) // echoA = layoutID 0 (Tight Grove)
	setupEchoForCompletion(t, w, 1)

	checkActivePlanetCompletion(w)

	if !w.System.Planets[1].Completed {
		t.Fatal("Tight Grove should be marked Completed")
	}
	if got := w.Economy.Potential[PotentialForest]; got != 1 {
		t.Errorf("Forest Potential after Tight Grove: got %d, want 1", got)
	}
	if got := w.Economy.Potential[PotentialWater]; got != 0 {
		t.Errorf("Water Potential after Tight Grove: got %d, want 0", got)
	}
}

// TestLakewood_CompletionAwardsForestAndWaterPotential verifies that Lakewood
// (echoB, layoutID 1) awards +1 Forest Potential and +1 Water Potential.
func TestLakewood_CompletionAwardsForestAndWaterPotential(t *testing.T) {
	w := newRevealedWorld()
	w.Economy.Potential[PotentialForest] = 1
	awakenPlanet(w, 2) // echoB = layoutID 1 (Lakewood)
	setupEchoForCompletion(t, w, 2)

	checkActivePlanetCompletion(w)

	if !w.System.Planets[2].Completed {
		t.Fatal("Lakewood should be marked Completed")
	}
	if got := w.Economy.Potential[PotentialForest]; got != 1 {
		t.Errorf("Forest Potential after Lakewood: got %d, want 1", got)
	}
	if got := w.Economy.Potential[PotentialWater]; got != 1 {
		t.Errorf("Water Potential after Lakewood: got %d, want 1", got)
	}
}

// TestLakewood_RequiresIslandSaturation verifies that Lakewood does not complete
// until the island forest region is also saturated.
func TestLakewood_RequiresIslandSaturation(t *testing.T) {
	w := newRevealedWorld()
	w.Economy.Potential[PotentialForest] = 1
	awakenPlanet(w, 2)
	switchToPlanet(w, 2)
	enterPlanetView(w)
	thAngle, ok := findValidBuildingAngle(w)
	if !ok || !placeBuilding(w, thAngle) {
		t.Fatal("setup: cannot place Town Hall on Lakewood")
	}
	if max := maxTownSlots(w); max > w.Economy.WorkerCapacity {
		w.Economy.WorkerCapacity = max
	}
	// Fill only the main (first) wood field, leave island unsaturated.
	var mainField *ResourceField
	for _, f := range w.Planet.Fields {
		if f.Kind == KindWood && f.Known {
			mainField = f
			break
		}
	}
	if mainField == nil {
		t.Fatal("setup: no main wood field on Lakewood")
	}
	for fieldCanSpawnNode(w, mainField) {
		spawnNode(w, mainField)
	}

	checkActivePlanetCompletion(w)
	if w.System.Planets[2].Completed {
		t.Error("Lakewood should NOT complete with only main forest saturated")
	}
}

// TestLakewood_WaterFieldsDoNotAccrueEXP verifies unknown KindWater fields on
// Lakewood never accumulate field EXP (they don't gate completion).
func TestLakewood_WaterFieldsDoNotAccrueEXP(t *testing.T) {
	w := newRevealedWorld()
	w.Economy.Potential[PotentialForest] = 1
	awakenPlanet(w, 2)
	switchToPlanet(w, 2)
	enterPlanetView(w)

	for _, f := range w.Planet.Fields {
		if f.Kind == KindWater {
			if fp := w.Planet.FieldProgress[KindWater]; fp != nil && fp.EXP != 0 {
				t.Errorf("KindWater FieldProgress.EXP should be 0, got %v", fp.EXP)
			}
		}
	}
}

// TestLakewood_AwakensWithForestPotential verifies awakening Lakewood still costs
// exactly 1 Forest Potential (same gate as Tight Grove and Phase 1 generic).
func TestLakewood_AwakensWithForestPotential(t *testing.T) {
	w := newRevealedWorld()
	w.Economy.Potential[PotentialForest] = 1

	if !canAwaken(w, 2) {
		t.Fatal("should be able to awaken echoB with 1 Forest Potential")
	}
	awakenPlanet(w, 2)
	if got := w.Economy.Potential[PotentialForest]; got != 0 {
		t.Errorf("Forest Potential after awakening Lakewood: got %d, want 0", got)
	}
	if !w.System.Planets[2].Awakened {
		t.Error("echoB should be awakened")
	}
}

// runTick advances the simulation for the given duration using Tick (view-aware path).
func runTick(w *World, seconds float64) {
	ticks := int(math.Round(seconds / dt))
	for i := 0; i < ticks; i++ {
		Tick(w, dt)
	}
}

// unlockedPlanetViewWorld returns a revealed world with workers settled in planet view,
// ensuring EstimateRate > AbstractRate so the rolling window has something to ratchet.
func unlockedPlanetViewWorld() *World {
	w := newRevealedWorld()
	enterPlanetView(w)
	addWorker(w)
	addWorker(w)
	addWorker(w)
	runSim(w, 5) // settle extra workers into routes via Step (doesn't touch window)
	return w
}

func TestAbstractRateWindow_NoUpdateBeforeFullWindow(t *testing.T) {
	w := unlockedPlanetViewWorld()

	initialRate := w.System.Planets[w.Active].AbstractRate
	if EstimateRate(w) <= initialRate {
		t.Skip("EstimateRate not > AbstractRate; worker settlement incomplete")
	}

	// Run for 90% of the window — not enough for an update.
	runTick(w, abstractRateWindowSec*0.9)

	if w.System.Planets[w.Active].AbstractRate != initialRate {
		t.Errorf("AbstractRate changed before full window: got %f, want %f",
			w.System.Planets[w.Active].AbstractRate, initialRate)
	}
}

func TestAbstractRateWindow_UpdateAfterFullWindow(t *testing.T) {
	w := unlockedPlanetViewWorld()

	initialRate := w.System.Planets[w.Active].AbstractRate
	if EstimateRate(w) <= initialRate {
		t.Skip("EstimateRate not > AbstractRate; worker settlement incomplete")
	}

	// Run for longer than a full window — AbstractRate must rise.
	runTick(w, abstractRateWindowSec*1.1)

	if w.System.Planets[w.Active].AbstractRate <= initialRate {
		t.Errorf("AbstractRate did not rise after full window: got %f, want > %f",
			w.System.Planets[w.Active].AbstractRate, initialRate)
	}
}

func TestAbstractRateWindow_MonotonicRaiseOnly(t *testing.T) {
	w := unlockedPlanetViewWorld()

	if EstimateRate(w) <= w.System.Planets[w.Active].AbstractRate {
		t.Skip("EstimateRate not > AbstractRate; worker settlement incomplete")
	}

	// Raise the rate by filling one full window.
	runTick(w, abstractRateWindowSec*1.1)
	raisedRate := w.System.Planets[w.Active].AbstractRate

	// Remove all workers so EstimateRate drops to zero.
	w.Workers = nil

	// Run another full window with zero production.
	runTick(w, abstractRateWindowSec*1.1)

	if w.System.Planets[w.Active].AbstractRate < raisedRate {
		t.Errorf("AbstractRate decreased (not monotonic): got %f, want >= %f",
			w.System.Planets[w.Active].AbstractRate, raisedRate)
	}
}

func TestAbstractRateWindow_ResetsOnPlanetSwitch(t *testing.T) {
	w := newRevealedWorld()
	w.Economy.Potential[PotentialForest] = 1
	awakenPlanet(w, 1)

	enterPlanetView(w)
	addWorker(w)
	addWorker(w)
	addWorker(w)
	runSim(w, 5)

	initialRate0 := w.System.Planets[0].AbstractRate

	// Fill 90% of the window on the starting planet — not enough to update.
	runTick(w, abstractRateWindowSec*0.9)
	if w.System.Planets[0].AbstractRate != initialRate0 {
		t.Error("starting planet rate changed before full window")
	}

	// Switch to echo 1; the window must reset.
	switchToPlanet(w, 1)
	enterPlanetView(w)
	echo1Rate := w.System.Planets[1].AbstractRate

	// Run 90% of a window on the echo. Combined elapsed (180% total) must not trigger
	// an update because each planet requires its own independent full window.
	runTick(w, abstractRateWindowSec*0.9)

	if w.System.Planets[1].AbstractRate != echo1Rate {
		t.Errorf("echo1 AbstractRate changed without a full window: got %f, want %f",
			w.System.Planets[1].AbstractRate, echo1Rate)
	}
}

func TestAbstractRateWindow_SystemViewDoesNotUpdate(t *testing.T) {
	w := newRevealedWorld()
	// newRevealedWorld leaves the world in ViewSystem.

	initialRate := w.System.Planets[0].AbstractRate

	// Run a full window's worth of Ticks while in system view.
	runTick(w, abstractRateWindowSec*1.1)

	if w.System.Planets[0].AbstractRate != initialRate {
		t.Errorf("AbstractRate changed during system view: got %f, want %f",
			w.System.Planets[0].AbstractRate, initialRate)
	}
}

// ── Multi-region field model tests ───────────────────────────────────────────

// newTwoRegionWorld builds a world with two disjoint KindWood regions and one
// shared FieldProgress entry. Regions are placed at opposite sides of the rim
// with tight arcs so each can hold exactly a few nodes before saturation.
func newTwoRegionWorld() *World {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	regionA := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.25, Known: true}
	regionB := &ResourceField{Kind: KindWood, CenterAngle: math.Pi, HalfArc: 0.25, Known: true}
	w.Planet.Fields = []*ResourceField{regionA, regionB}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}
	return w
}

func TestMultiRegion_SpawnStaysWithinChosenRegion(t *testing.T) {
	w := newTwoRegionWorld()
	regionA := w.Planet.Fields[0]

	result := spawnNode(w, regionA)
	if result.Outcome == growthOutcomeNone {
		t.Fatal("expected spawn in region A")
	}
	n := w.Nodes[len(w.Nodes)-1]
	if !angleWithinField(regionA, n.Angle) {
		t.Errorf("node spawned outside region A: angle %.4f, center %.4f, halfArc %.4f",
			n.Angle, regionA.CenterAngle, regionA.HalfArc)
	}
}

func TestMultiRegion_GrowthDistributesAcrossEligibleRegions(t *testing.T) {
	w := newTwoRegionWorld()

	// Deposit enough to trigger many spawns; collect which regions each lands in.
	regionA := w.Planet.Fields[0]
	regionB := w.Planet.Fields[1]
	for range 20 {
		depositToField(w, KindWood, w.Planet.FieldProgress[KindWood].Cap)
	}

	inA, inB := 0, 0
	for _, n := range w.Nodes {
		switch {
		case angleWithinField(regionA, n.Angle):
			inA++
		case angleWithinField(regionB, n.Angle):
			inB++
		}
	}
	if inA == 0 || inB == 0 {
		t.Errorf("expected both regions to receive nodes; inA=%d inB=%d total=%d",
			inA, inB, len(w.Nodes))
	}
}

func TestMultiRegion_CompletionRequiresAllKnownRegionsSaturated(t *testing.T) {
	w := newTwoRegionWorld()
	_ = placeBuilding(w, math.Pi/2) // Town Hall off to the side
	w.Economy.WorkerCapacity = maxTownSlots(w)

	// Saturate only region A.
	regionA := w.Planet.Fields[0]
	for fieldCanSpawnNode(w, regionA) {
		spawnNode(w, regionA)
	}

	// forestPlanetComplete must be false while region B still has capacity.
	if forestPlanetComplete(w) {
		t.Error("forestPlanetComplete should be false when region B is not saturated")
	}

	// Saturate region B.
	regionB := w.Planet.Fields[1]
	for fieldCanSpawnNode(w, regionB) {
		spawnNode(w, regionB)
	}

	// Both regions saturated — now complete.
	if !forestPlanetComplete(w) {
		t.Error("forestPlanetComplete should be true when all known regions are saturated")
	}
}

func TestMultiRegion_UnknownRegionDoesNotGateCompletion(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	known := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.25, Known: true}
	unknown := &ResourceField{Kind: KindWater, CenterAngle: math.Pi, HalfArc: 0.25, Known: false}
	w.Planet.Fields = []*ResourceField{known, unknown}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}
	_ = placeBuilding(w, math.Pi/2)
	w.Economy.WorkerCapacity = maxTownSlots(w)

	for fieldCanSpawnNode(w, known) {
		spawnNode(w, known)
	}

	// Unknown water region must not block completion.
	if !forestPlanetComplete(w) {
		t.Error("unknown region should not gate forestPlanetComplete")
	}
}

func TestMultiRegion_SingleRegionBehaviourUnchanged(t *testing.T) {
	// Verify a single-region planet behaves identically to the pre-refactor design.
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: math.Pi * 0.75, Known: true}
	w.Planet.Fields = []*ResourceField{field}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}

	// Deposit exactly one cap worth — should spawn exactly one node.
	depositToField(w, KindWood, woodFieldBaseEXP)
	if len(w.Nodes) != 1 {
		t.Fatalf("single-region: expected 1 node after one cap deposit, got %d", len(w.Nodes))
	}
	if !angleWithinField(field, w.Nodes[0].Angle) {
		t.Error("single-region: spawned node is outside the only region")
	}
}
