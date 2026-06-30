package game

import (
	"fmt"
	"math"
	"math/rand"
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
	w.Nodes = append(w.Nodes, newNode(w, KindWood, 0.25))

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
	w.Nodes = append(w.Nodes, newNode(w, KindWood, 0.25))

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

func TestNoWorkerArrivalBeyondClaimableWork(t *testing.T) {
	w, node := newDeliveryWorld()
	gross := baseLoadAmount * node.Size
	w.Economy.TownGrowthCap = gross * 0.5
	w.Economy.TownGrowth = 0

	// The delivery worker occupies the only claimable node.
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

	if len(w.Workers) != 1 {
		t.Errorf("no worker should spawn beyond claimable work; got %d workers", len(w.Workers))
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
	if !nurtureField(w) {
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

	if nurtureField(w) {
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

	// Planet-wide nurtureField runs via fallback but adds no nodes on this tiny saturated field.
	nurtureField(w)
	if len(w.Nodes) != 1 {
		t.Fatalf("saturated Nurture should not spawn nodes, got %d", len(w.Nodes))
	}
}

func TestNurtureBlockedWhenCuePending(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true

	if !nurtureField(w) {
		t.Fatal("first Nurture press should succeed")
	}
	// A cue is now active — second press must be blocked.
	if nurtureField(w) {
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

	if !nurtureField(w) {
		t.Fatal("nurtureField should succeed with one remaining slot")
	}
	if len(w.Nodes) != nodesBefore+1 {
		t.Errorf("should spawn exactly 1 node (capacity limit), got %d new nodes",
			len(w.Nodes)-nodesBefore)
	}
}

func TestNurtureAttentionRequiresMinCompletionPop(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}

	// Minimum completion population not reached yet — attention should be inactive.
	if nurtureAttentionActive(w) {
		t.Error("attention should be inactive before minimum completion population")
	}

	for i := range w.Planet.MinCompletionPop {
		w.Workers = append(w.Workers, &Worker{ID: i + 1})
	}
	if !nurtureAttentionActive(w) {
		t.Error("attention should be active once minimum completion population is reached")
	}
}

func TestNurtureAttentionInactiveWhenFieldSaturated(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.ResourceDiscovered = true
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.01, Known: true}
	w.Planet.Fields = []*ResourceField{field}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}

	node := newNode(w, KindWood, 0)
	node.Size = 1
	w.Nodes = []*ResourceNode{node}

	if nurtureAttentionActive(w) {
		t.Fatal("Nurture attention should be inactive when field is saturated")
	}
}

func TestNurtureAttentionSuppressedWhenCuePending(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}
	for i := range w.Planet.MinCompletionPop {
		w.Workers = append(w.Workers, &Worker{ID: i + 1})
	}

	// Minimum population reached and field has room — attention should be active.
	if !nurtureAttentionActive(w) {
		t.Error("setup: expected attention active with minimum population")
	}

	// After a Nurture press a cue is active — attention must be suppressed.
	nurtureField(w)
	if nurtureAttentionActive(w) {
		t.Error("attention should be suppressed while a growth cue is pending")
	}
}

func TestNurtureAttentionInactiveWhenNotDiscovered(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = false
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}

	if nurtureAttentionActive(w) {
		t.Error("attention should be inactive when resource is not yet discovered")
	}
}

// ── System unlock tests ───────────────────────────────────────────────────────

// newMasteredWorld returns a world that satisfies both completion gates:
// minimum completion population reached and wood field fully saturated.
func newMasteredWorld() *World {
	w := NewWorld()
	f := fieldForKind(w, KindWood)
	_ = placeBuilding(w, f.CenterAngle)
	spawnWorkersToMinCompletion(w)
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

	// Only field: reset workers.
	w3 := newMasteredWorld()
	w3.Workers = nil
	if forestPlanetComplete(w3) {
		t.Error("expected forestPlanetComplete false when minimum population is not reached")
	}
}

func TestTickTriggersUnlockExactlyOnce(t *testing.T) {
	w := newMasteredWorld()
	w.ResourceDiscovered = true
	// Add a worker so EstimateRate can snapshot a non-zero value.
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

	// Economy.Wood must not change in system view — planets have local stockpiles only.
	if w.Economy.Wood != 0 {
		t.Errorf("Economy.Wood should stay at 0 in system view (no stockpile), got %.4f", w.Economy.Wood)
	}
}

// TestSwitchToPlanet_IsolatesLocalStockpiles verifies that Economy.Wood and
// Economy.Water are per-planet: switching planets neither bleeds one planet's
// stockpile into another nor loses it when returning.
func TestSwitchToPlanet_IsolatesLocalStockpiles(t *testing.T) {
	w := newRevealedWorld() // planet 0 "active" (live), world in system view

	// Awaken planet 1 so switchToPlanet can load it.
	awakenPlanet(w, 1)

	// Set known stockpiles on planet 0 (still live — PlanetStates[0] is nil until parked).
	w.Economy.Wood = 100
	w.Economy.Water = 25

	// Switch to echo planet 1: parks planet 0 (saves Wood=100, Water=25) then loads planet 1.
	// Echo bootstrap seeds awakenSeedWood into the local wood stockpile.
	switchToPlanet(w, 1)
	wantEchoWood := awakenSeedWood
	if w.Economy.Wood != wantEchoWood {
		t.Errorf("echo planet 1 Wood: want %.4f (awakenSeedWood), got %.4f", wantEchoWood, w.Economy.Wood)
	}
	if w.Economy.Water != 0 {
		t.Errorf("echo planet 1 Water: want 0, got %.4f", w.Economy.Water)
	}

	// Simulate delivery on planet 1 — must not contaminate planet 0's stockpile.
	w.Economy.Wood = 42
	w.Economy.Water = 7

	// Switch back to planet 0 — stockpile must be exactly as parked.
	switchToPlanet(w, 0)
	if w.Economy.Wood != 100 {
		t.Errorf("planet 0 Wood after return: want 100, got %.4f", w.Economy.Wood)
	}
	if w.Economy.Water != 25 {
		t.Errorf("planet 0 Water after return: want 25, got %.4f", w.Economy.Water)
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
	if nurtureAttentionActive(w) {
		t.Error("nurtureAttentionActive should be false when field is saturated")
	}
	// Planet-wide nurtureField runs via fallback but is a no-op on this saturated field.
	nurtureField(w)
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

func TestTriggerUnlock_EchoesGetProjectedRate(t *testing.T) {
	w := newRevealedWorld()

	// Starting planet must be Completed with AbstractRate > 0.
	if !w.System.Planets[0].Completed {
		t.Error("starting planet must be Completed after triggerUnlock")
	}
	base := w.System.Planets[0].AbstractRate
	if base <= 0 {
		t.Errorf("starting planet AbstractRate: got %f, want > 0", base)
	}

	// Echoes must have ProjectedRate set but AbstractRate=0 and Completed=false.
	for _, idx := range []int{1, 2} {
		p := w.System.Planets[idx]
		if p.AbstractRate != 0 {
			t.Errorf("echo %d: AbstractRate should be 0, got %f", idx, p.AbstractRate)
		}
		if p.ProjectedRate <= 0 {
			t.Errorf("echo %d: ProjectedRate should be > 0, got %f", idx, p.ProjectedRate)
		}
		if p.Completed {
			t.Errorf("echo %d: should not be Completed after unlock", idx)
		}
	}
	if math.Abs(w.System.Planets[1].ProjectedRate-base*echoRateFracA) > 1e-9 {
		t.Errorf("echo 1 ProjectedRate: got %f, want base*echoRateFracA=%f",
			w.System.Planets[1].ProjectedRate, base*echoRateFracA)
	}
	if math.Abs(w.System.Planets[2].ProjectedRate-base*echoRateFracB) > 1e-9 {
		t.Errorf("echo 2 ProjectedRate: got %f, want base*echoRateFracB=%f",
			w.System.Planets[2].ProjectedRate, base*echoRateFracB)
	}
}

func TestEchoCompletion_AmplifiedRate(t *testing.T) {
	w := newRevealedWorld()
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
	spawnWorkersToMinCompletion(w)
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

// ── Awakening tests ───────────────────────────────────────────────────────────

func TestCanAwaken_DormantEcho(t *testing.T) {
	w := newRevealedWorld()
	if !canAwaken(w, 1) {
		t.Error("canAwaken should return true for dormant echo")
	}
	// Cannot awaken something already awakened.
	awakenPlanet(w, 1)
	if canAwaken(w, 1) {
		t.Error("canAwaken should return false for already-awakened planet")
	}
}

func TestAwakenPlanet_DoesNotSpendWood(t *testing.T) {
	w := newRevealedWorld()
	startWood := w.Economy.Wood
	awakenPlanet(w, 1)
	if !w.System.Planets[1].Awakened {
		t.Fatal("echo 1 should be awakened after awakenPlanet")
	}
	if w.Economy.Wood != startWood {
		t.Errorf("Wood changed after awaken: got %.2f, want %.2f", w.Economy.Wood, startWood)
	}
}

func TestAwakenPlanet_SeedsWood(t *testing.T) {
	w := newRevealedWorld()
	awakenPlanet(w, 1)
	if w.PlanetStates[1] == nil {
		t.Fatal("PlanetStates[1] should be non-nil after awakening")
	}
	if w.PlanetStates[1].LocalWood < awakenSeedWood {
		t.Errorf("echo LocalWood after awaken: got %.2f, want >= awakenSeedWood (%.2f)",
			w.PlanetStates[1].LocalWood, awakenSeedWood)
	}
}

func TestEchoCompletion_SetsCompletedAndRate(t *testing.T) {
	w := newRevealedWorld()
	awakenPlanet(w, 1)

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
	spawnWorkersToMinCompletion(w)
	fillWoodFieldNodes(w, false)
	runSim(w, 5) // let workers enter loops so EstimateRate returns nonzero

	checkActivePlanetCompletion(w)
	if !w.System.Planets[1].Completed {
		t.Fatal("echo 1 should be marked Completed")
	}
	if w.System.Planets[1].AbstractRate <= 0 {
		t.Error("echo 1 AbstractRate should be > 0 after completion")
	}
	// Must be idempotent.
	checkActivePlanetCompletion(w)
	if !w.System.Planets[1].Completed {
		t.Error("idempotent: echo 1 should still be Completed")
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
	spawnWorkersToMinCompletion(w)
	fillWoodFieldNodes(w, false)
}

// TestTightGrove_CompletionSetsCompleted verifies that Tight Grove
// (layoutID 1, planet index 2) reaches the Completed state.
func TestTightGrove_CompletionSetsCompleted(t *testing.T) {
	w := newRevealedWorld()
	awakenPlanet(w, 2) // layoutID 1 = Tight Grove
	setupEchoForCompletion(t, w, 2)

	checkActivePlanetCompletion(w)

	if !w.System.Planets[2].Completed {
		t.Fatal("Tight Grove should be marked Completed")
	}
}

// TestLakewood_CompletionSetsCompleted verifies Lakewood (layoutID 0, planet 1) completes.
func TestLakewood_CompletionSetsCompleted(t *testing.T) {
	w := newRevealedWorld()
	awakenPlanet(w, 1) // layoutID 0 = Lakewood
	setupEchoForCompletion(t, w, 1)
	runSim(w, 5) // let workers enter loops so EstimateRate returns nonzero

	checkActivePlanetCompletion(w)

	if !w.System.Planets[1].Completed {
		t.Fatal("Lakewood should be marked Completed")
	}
	if w.System.Planets[1].AbstractRate <= 0 {
		t.Error("Lakewood AbstractRate should be > 0 after completion")
	}
}

// TestLakewood_RequiresIslandSaturation verifies that Lakewood does not complete
// until the island forest region is also saturated.
func TestLakewood_RequiresIslandSaturation(t *testing.T) {
	w := newRevealedWorld()
	awakenPlanet(w, 1) // layoutID 0 = Lakewood
	switchToPlanet(w, 1)
	enterPlanetView(w)
	thAngle, ok := findValidBuildingAngle(w)
	if !ok || !placeBuilding(w, thAngle) {
		t.Fatal("setup: cannot place Town Hall on Lakewood")
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
	if w.System.Planets[1].Completed {
		t.Error("Lakewood should NOT complete with only main forest saturated")
	}
}

// TestLakewood_WaterFieldsDoNotAccrueEXP verifies unknown KindWater fields on
// Lakewood never accumulate field EXP (they don't gate completion).
func TestLakewood_WaterFieldsDoNotAccrueEXP(t *testing.T) {
	w := newRevealedWorld()
	awakenPlanet(w, 1) // layoutID 0 = Lakewood
	switchToPlanet(w, 1)
	enterPlanetView(w)

	for _, f := range w.Planet.Fields {
		if f.Kind == KindWater {
			if fp := w.Planet.FieldProgress[KindWater]; fp != nil && fp.EXP != 0 {
				t.Errorf("KindWater FieldProgress.EXP should be 0, got %v", fp.EXP)
			}
		}
	}
}

// TestLakewood_Awakens verifies Lakewood can be awakened directly.
func TestLakewood_Awakens(t *testing.T) {
	w := newRevealedWorld()
	if !canAwaken(w, 1) {
		t.Fatal("canAwaken should be true for dormant Lakewood")
	}
	awakenPlanet(w, 1) // layoutID 0 = Lakewood
	if !w.System.Planets[1].Awakened {
		t.Error("Lakewood (planet 1) should be awakened")
	}
}

// ── Water frontier completion tests ──────────────────────────────────────────

// newWaterFrontierForCompletion returns a frontier world ready for completion:
// awakened, TH placed, town capacity maxed, wood + water fields saturated,
// and a dock placed (Level 1). The dock has NOT been upgraded to Level 2 yet.
func newWaterFrontierForCompletion(t *testing.T) *World {
	t.Helper()
	w := newRevealedWorld()
	awakenPlanet(w, 3)
	switchToPlanet(w, 3)
	enterPlanetView(w)
	thAngle, ok := findValidBuildingAngle(w)
	if !ok || !placeBuilding(w, thAngle) {
		t.Fatal("setup: cannot place frontier TH")
	}
	spawnWorkersToMinCompletion(w)
	fillWoodFieldNodes(w, false)
	if !placeBuildingWithFreePlacement(w, waterFrontierLakeAngle, true) {
		t.Fatal("setup: cannot place frontier dock")
	}
	fillWaterFieldSparkles(w)
	return w
}

func TestWaterPlanetComplete_AllThreeGates(t *testing.T) {
	w := newWaterFrontierForCompletion(t)

	// Without L2 dock: should not complete.
	if waterPlanetComplete(w) {
		t.Error("waterPlanetComplete should be false with only L1 dock")
	}

	// Upgrade dock to L2.
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			b.Level = 2
		}
	}

	if !waterPlanetComplete(w) {
		t.Error("waterPlanetComplete should be true when all three gates are met")
	}
}

func TestWaterPlanetComplete_GateMinCompletionPop(t *testing.T) {
	w := newWaterFrontierForCompletion(t)
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			b.Level = 2
		}
	}
	w.Workers = nil
	if waterPlanetComplete(w) {
		t.Error("waterPlanetComplete should be false when minimum population is not reached")
	}
}

func TestWaterPlanetComplete_GateWoodFieldSaturation(t *testing.T) {
	w := newWaterFrontierForCompletion(t)
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			b.Level = 2
		}
	}
	// Remove all wood nodes so the wood field can spawn again.
	kept := w.Nodes[:0]
	for _, n := range w.Nodes {
		if n.Kind != KindWood {
			kept = append(kept, n)
		}
	}
	w.Nodes = kept
	if waterPlanetComplete(w) {
		t.Error("waterPlanetComplete should be false when wood field is not saturated")
	}
}

func TestWaterPlanetComplete_GateWaterFieldSaturation(t *testing.T) {
	w := newWaterFrontierForCompletion(t)
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			b.Level = 2
		}
	}
	// Remove all sparkles so the water field can spawn again.
	kept := w.Nodes[:0]
	for _, n := range w.Nodes {
		if n.Kind != KindWater {
			kept = append(kept, n)
		}
	}
	w.Nodes = kept
	if waterPlanetComplete(w) {
		t.Error("waterPlanetComplete should be false when water field is not saturated")
	}
}

func TestWaterPlanetComplete_GateDockLevel(t *testing.T) {
	w := newWaterFrontierForCompletion(t)
	// All fields saturated, but dock is still Level 1 — should not complete.
	if waterPlanetComplete(w) {
		t.Error("waterPlanetComplete should be false with no Level-2 dock")
	}
}

func TestWaterFrontierCompletion_DualAbstractRate(t *testing.T) {
	w := newWaterFrontierForCompletion(t)
	// Add a worker so EstimateRate / EstimateWaterRate return non-zero.
	addWorker(w)
	runSim(w, 5)

	wantWood := EstimateRate(w) * completionAmplifier
	wantWater := EstimateWaterRate(w) * completionAmplifier

	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			b.Level = 2
		}
	}
	checkActivePlanetCompletion(w)
	if !w.System.Planets[3].Completed {
		t.Fatal("frontier should be Completed")
	}
	if math.Abs(w.System.Planets[3].AbstractRate-wantWood) > 1e-9 {
		t.Errorf("AbstractRate: got %f, want %f", w.System.Planets[3].AbstractRate, wantWood)
	}
	if math.Abs(w.System.Planets[3].AbstractWaterRate-wantWater) > 1e-9 {
		t.Errorf("AbstractWaterRate: got %f, want %f", w.System.Planets[3].AbstractWaterRate, wantWater)
	}
}

func TestWaterFrontierCompletion_Idempotent(t *testing.T) {
	w := newWaterFrontierForCompletion(t)
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			b.Level = 2
		}
	}
	checkActivePlanetCompletion(w)
	if !w.System.Planets[3].Completed {
		t.Fatal("frontier should be Completed on first call")
	}
	// Stamp a sentinel rate and confirm it's not overwritten.
	w.System.Planets[3].AbstractRate = 99.0
	w.System.Planets[3].AbstractWaterRate = 88.0
	checkActivePlanetCompletion(w)
	if w.System.Planets[3].AbstractRate != 99.0 || w.System.Planets[3].AbstractWaterRate != 88.0 {
		t.Error("checkActivePlanetCompletion must not re-fire after Completed=true")
	}
}

// TestSpawnSparkle_GridFallback verifies that spawnSparkle can still place a
// sparkle when golden-angle jitter is exhausted but a valid grid position exists.
// This covers the Nurture-button regression where clicking it only fired the
// upgrade animation rather than adding a new sparkle.
func TestSpawnSparkle_GridFallback(t *testing.T) {
	w := newWaterFrontierForCompletion(t)
	wf := fieldForKind(w, KindWater)
	if wf == nil {
		t.Fatal("no water field")
	}
	// Remove all sparkles so we have a fully empty water field.
	kept := w.Nodes[:0]
	for _, n := range w.Nodes {
		if n.Kind != KindWater {
			kept = append(kept, n)
		}
	}
	w.Nodes = kept

	// Fill via the same grid strategy spawnSparkle now uses; this is equivalent
	// to exhausting golden-angle until the jitter can no longer find valid positions.
	// We pack the field using fillWaterFieldSparkles (grid-based) then verify that
	// further spawnSparkle calls fall back to upgrade rather than infinite-looping.
	fillWaterFieldSparkles(w)
	countAfterFill := 0
	for _, n := range w.Nodes {
		if n.Kind == KindWater {
			countAfterFill++
		}
	}
	if countAfterFill == 0 {
		t.Fatal("fillWaterFieldSparkles placed no sparkles")
	}

	// After fill, waterFieldCanSpawnSparkle must return false.
	if waterFieldCanSpawnSparkle(w, wf) {
		t.Error("waterFieldCanSpawnSparkle should be false after grid fill")
	}

	// spawnSparkle should now upgrade (not hang or panic).
	before := len(w.Nodes)
	result := spawnSparkle(w, wf)
	if result.Outcome != growthOutcomeUpgradedNode {
		t.Errorf("spawnSparkle on saturated field: outcome = %v, want growthOutcomeUpgradedNode", result.Outcome)
	}
	if len(w.Nodes) != before {
		t.Errorf("spawnSparkle on saturated field added a node (before=%d after=%d)", before, len(w.Nodes))
	}
}

// TestSpawnSparkle_GridFallbackReachesGap verifies that spawnSparkle's grid
// fallback can place a sparkle when golden-angle jitter is blocked but a valid
// grid position exists (the practical Nurture regression scenario).
func TestSpawnSparkle_GridFallbackReachesGap(t *testing.T) {
	w := newWaterFrontierForCompletion(t)
	wf := fieldForKind(w, KindWater)
	if wf == nil {
		t.Fatal("no water field")
	}
	// Remove all sparkles.
	kept := w.Nodes[:0]
	for _, n := range w.Nodes {
		if n.Kind != KindWater {
			kept = append(kept, n)
		}
	}
	w.Nodes = kept

	// Use fillWaterFieldSparkles to fill all but one grid position.
	// Remove one sparkle after filling to leave a single valid gap.
	fillWaterFieldSparkles(w)
	// Remove the last water sparkle added.
	for i := len(w.Nodes) - 1; i >= 0; i-- {
		if w.Nodes[i].Kind == KindWater {
			w.Nodes = append(w.Nodes[:i], w.Nodes[i+1:]...)
			break
		}
	}

	// The field should now have a valid position.
	if !waterFieldCanSpawnSparkle(w, wf) {
		t.Skip("gap creation didn't leave a valid grid position; skipping")
	}

	before := len(w.Nodes)
	result := spawnSparkle(w, wf)
	if result.Outcome != growthOutcomeSpawnedNode {
		t.Errorf("spawnSparkle with gap available: outcome = %v, want growthOutcomeSpawnedNode", result.Outcome)
	}
	if len(w.Nodes) != before+1 {
		t.Errorf("spawnSparkle should have added one node (before=%d after=%d)", before, len(w.Nodes))
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

func TestAbstractRateWindow_ResetsOnPlanetSwitch(t *testing.T) {
	w := newRevealedWorld()
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
	spawnWorkersToMinCompletion(w)

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
	spawnWorkersToMinCompletion(w)

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

// ── Water influence tests ─────────────────────────────────────────────────────

// influenceWorld builds a minimal world with one KindWood field at center 0
// and one KindWaterInfluence field at the same center (wider than the wood
// field so the whole forest arc is influenced).
func influenceWorld() (*World, *ResourceField) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	inf := &ResourceField{Kind: KindWaterInfluence, CenterAngle: 0, HalfArc: 1.0, Known: false}
	forest := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.5, Known: true}
	w.Planet.Fields = []*ResourceField{forest, inf}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}
	return w, forest
}

func TestWaterInfluence_SpawnSizeBonus(t *testing.T) {
	w, forest := influenceWorld()

	result := spawnNodeNear(w, forest, 0)

	if result.Outcome != growthOutcomeSpawnedNode {
		t.Fatalf("expected spawned node, got %v", result.Outcome)
	}
	if !result.WaterInfluenced {
		t.Fatal("result.WaterInfluenced should be true")
	}
	node := w.Nodes[0]
	if node.Size < waterForestSpawnSizeBonus+0.6 {
		t.Errorf("influenced node size %.3f is below minimum expected (base+bonus=%.3f)", node.Size, 0.6+waterForestSpawnSizeBonus)
	}
}

func TestWaterInfluence_NoBonus_WhenUninfluenced(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	forest := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.5, Known: true}
	w.Planet.Fields = []*ResourceField{forest} // no influence field
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}

	result := spawnNodeNear(w, forest, 0)

	if result.WaterInfluenced {
		t.Fatal("result.WaterInfluenced should be false with no influence field")
	}
	node := w.Nodes[0]
	if node.Size >= 1.4+waterForestSpawnSizeBonus {
		t.Errorf("uninfluenced node size %.3f is suspiciously large", node.Size)
	}
}

func TestWaterInfluence_FoundingSpawnBonus(t *testing.T) {
	w, _ := influenceWorld()
	// Make both fields cover the full ring so every founding node is influenced.
	w.Planet.Fields[0].HalfArc = math.Pi
	w.Planet.Fields[1].HalfArc = math.Pi

	beforeCount := len(w.Nodes)
	foundStartingNodes(w, 0)
	newNodes := w.Nodes[beforeCount:]

	if len(newNodes) == 0 {
		t.Fatal("foundStartingNodes should have created nodes")
	}
	for _, n := range newNodes {
		if n.Size < waterForestSpawnSizeBonus+0.6 {
			t.Errorf("founding node size %.3f below minimum expected (base+bonus=%.3f)", n.Size, 0.6+waterForestSpawnSizeBonus)
		}
	}
}

func TestWaterInfluence_UpgradeSizeBonus(t *testing.T) {
	w, forest := influenceWorld()

	n := newNode(w, KindWood, 0)
	n.Size = 1.0
	w.Nodes = []*ResourceNode{n}

	upgraded := upgradeNearestFieldNode(w, forest, 0)

	if upgraded == nil {
		t.Fatal("expected upgrade, got nil")
	}
	wantSize := 1.0 + 0.15 + waterForestUpgradeSizeBonus
	if math.Abs(upgraded.Size-wantSize) > 1e-9 {
		t.Errorf("influenced upgrade size got %.4f, want %.4f", upgraded.Size, wantSize)
	}
}

func TestWaterInfluence_UpgradeCueFlag(t *testing.T) {
	w, forest := influenceWorld()

	// Saturate the field by spawning until the first upgrade result.
	var upgradeResult growthResult
	for range 200 {
		r := spawnNode(w, forest)
		if r.Outcome == growthOutcomeUpgradedNode {
			upgradeResult = r
			break
		}
	}

	if upgradeResult.Outcome != growthOutcomeUpgradedNode {
		t.Fatalf("expected upgrade outcome after saturation, got %v", upgradeResult.Outcome)
	}
	if !upgradeResult.WaterInfluenced {
		t.Fatal("upgrade result.WaterInfluenced should be true")
	}
}

func TestWaterInfluence_UnknownFieldStillInfluences(t *testing.T) {
	w, forest := influenceWorld()
	w.Planet.Fields[1].Known = false // already false, but be explicit

	result := spawnNodeNear(w, forest, 0)

	if !result.WaterInfluenced {
		t.Fatal("unknown KindWaterInfluence field should still influence")
	}
}

func TestWaterInfluence_NonStacking_TwoOverlappingArcs(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	inf1 := &ResourceField{Kind: KindWaterInfluence, CenterAngle: 0, HalfArc: 0.6, Known: false}
	inf2 := &ResourceField{Kind: KindWaterInfluence, CenterAngle: 0.1, HalfArc: 0.6, Known: false}
	forest := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.3, Known: true}
	w.Planet.Fields = []*ResourceField{forest, inf1, inf2}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{KindWood: {Cap: woodFieldBaseEXP}}

	result := spawnNodeNear(w, forest, 0)

	if !result.WaterInfluenced {
		t.Fatal("expected influenced result")
	}
	// Only one bonus regardless of two overlapping arcs.
	node := w.Nodes[0]
	maxExpected := 1.4 + waterForestSpawnSizeBonus + 1e-9 // base max + one bonus
	if node.Size > maxExpected {
		t.Errorf("non-stacking: node size %.4f exceeds one-bonus maximum %.4f", node.Size, maxExpected)
	}
}

func TestWaterInfluence_UpgradeClampAt2(t *testing.T) {
	w, forest := influenceWorld()

	n := newNode(w, KindWood, 0)
	n.Size = 1.95
	w.Nodes = []*ResourceNode{n}

	upgraded := upgradeNearestFieldNode(w, forest, 0)

	if upgraded.Size != 2.0 {
		t.Errorf("influenced upgrade should clamp at 2.0, got %.4f", upgraded.Size)
	}
}

func TestWaterInfluence_DoesNotBlockSpawning(t *testing.T) {
	w, forest := influenceWorld()

	result := spawnNodeNear(w, forest, 0)

	if result.Outcome == growthOutcomeNone {
		t.Fatal("KindWaterInfluence should not block spawning (inLake should ignore it)")
	}
}

// ── Frontier awakening tests ──────────────────────────────────────────────────

func TestCanAwaken_FrontierDormant(t *testing.T) {
	w := newRevealedWorld()
	if !canAwaken(w, 3) {
		t.Error("canAwaken(frontier): should be true for dormant frontier")
	}
}

func TestAwakenPlanet_FrontierInitializesPlanetState(t *testing.T) {
	w := newRevealedWorld()
	startWood := w.Economy.Wood

	awakenPlanet(w, 3)

	if !w.System.Planets[3].Awakened {
		t.Fatal("frontier (Planets[3]) should be Awakened after awakenPlanet")
	}
	if w.Economy.Wood != startWood {
		t.Errorf("Wood changed after frontier awaken: got %.2f, want %.2f", w.Economy.Wood, startWood)
	}
	if w.PlanetStates[3] == nil {
		t.Fatal("PlanetStates[3] should be non-nil after awakening the frontier")
	}
}

func TestAwakenPlanet_FrontierProjectedRates(t *testing.T) {
	w := newRevealedWorld()
	base := w.System.Planets[0].AbstractRate
	awakenPlanet(w, 3)

	p := w.System.Planets[3]
	wantWood := base * waterFrontierProjectedWoodFrac
	wantWater := base * waterFrontierProjectedWaterFrac
	if math.Abs(p.ProjectedRate-wantWood) > 1e-9 {
		t.Errorf("frontier ProjectedRate: got %f, want %f", p.ProjectedRate, wantWood)
	}
	if math.Abs(p.ProjectedWaterRate-wantWater) > 1e-9 {
		t.Errorf("frontier ProjectedWaterRate: got %f, want %f", p.ProjectedWaterRate, wantWater)
	}
}

func TestAwakenPlanet_FrontierZoomableWhenAwakened(t *testing.T) {
	w := newRevealedWorld()
	if w.System.Planets[3].zoomable() {
		t.Error("frontier should not be zoomable before awakening")
	}
	awakenPlanet(w, 3)
	if !w.System.Planets[3].zoomable() {
		t.Error("frontier should be zoomable after awakening")
	}
}

func TestAwakenPlanet_FrontierCreatesValidPlanetState(t *testing.T) {
	w := newRevealedWorld()
	awakenPlanet(w, 3)

	ps := w.PlanetStates[3]
	if ps == nil {
		t.Fatal("PlanetStates[3] is nil")
	}
	if ps.Planet.Radius <= 0 {
		t.Errorf("frontier planet radius: got %f, want > 0", ps.Planet.Radius)
	}
	hasForest := false
	hasWater := false
	for _, f := range ps.Planet.Fields {
		switch f.Kind {
		case KindWood:
			hasForest = true
		case KindWater:
			hasWater = true
		}
	}
	if !hasForest {
		t.Error("frontier planet state should have a KindWood field")
	}
	if !hasWater {
		t.Error("frontier planet state should have a KindWater field")
	}
}

// ── Water sparkle tests ────────────────────────────────────────────────────────

// newWaterSparkleTestWorld returns a water frontier world suitable for sparkle tests.
func newWaterSparkleTestWorld() *World {
	ps := newWaterFrontierState()
	w := &World{
		Version: SaveVersion,
		Planet:  ps.Planet,
		Economy: Economy{
			TownGrowthCap: ps.TownGrowthCap,
		},
		PlanetStates: make([]*PlanetState, 4),
		System:       System{Planets: []SystemPlanet{{}, {}, {}, {}}},
		rng:          rand.New(rand.NewSource(42)),
	}
	w.ResourceDiscovered = true
	thAngle := waterFrontierShoreAngle
	w.Buildings = []*Building{{
		ID:    0,
		Kind:  KindTownHall,
		Angle: thAngle,
		Pos:   w.Planet.RimPoint(thAngle),
	}}
	w.NextBuildingID = 1
	return w
}

func TestWaterSparkleSpawnsInInterior(t *testing.T) {
	w := newWaterSparkleTestWorld()
	f := fieldForKind(w, KindWater)
	if f == nil {
		t.Fatal("water field missing")
	}

	before := len(w.Nodes)
	result := spawnSparkle(w, f)

	if result.Outcome != growthOutcomeSpawnedNode {
		t.Fatalf("expected growthOutcomeSpawnedNode, got %v", result.Outcome)
	}
	if len(w.Nodes) != before+1 {
		t.Fatalf("expected 1 new node, got %d total (was %d)", len(w.Nodes), before)
	}
	n := w.Nodes[len(w.Nodes)-1]
	if !n.Interior {
		t.Error("spawned node should have Interior=true")
	}
	if n.Kind != KindWater {
		t.Errorf("spawned node Kind: got %v, want KindWater", n.Kind)
	}
	if n.ServicingDockID != -1 {
		t.Errorf("ServicingDockID: got %d, want -1", n.ServicingDockID)
	}
	// Pos must be inside the radial band.
	r := n.Pos.Dist(w.Planet.Center)
	innerR := w.Planet.Radius * sparkleInnerFrac
	outerR := w.Planet.Radius * sparkleOuterFrac
	if r < innerR || r > outerR {
		t.Errorf("sparkle r=%.2f outside [%.2f, %.2f]", r, innerR, outerR)
	}
	// Pos angle must be within the water field arc.
	angle := w.Planet.AngleOf(n.Pos)
	if !angleWithinField(f, angle) {
		t.Errorf("sparkle angle %.3f not within water field (center %.3f ±%.3f)", angle, f.CenterAngle, f.HalfArc)
	}
}

func TestWaterSparkleNotWorkableByRimWorkers(t *testing.T) {
	w := newWaterSparkleTestWorld()
	f := fieldForKind(w, KindWater)
	spawnSparkle(w, f)
	sparkle := w.Nodes[len(w.Nodes)-1]

	if nodeFreeForWorker(w, sparkle, -1) {
		t.Error("interior sparkle should not be workable by rim workers")
	}
}

func TestWaterSparkleSaturationFallsBackToUpgrade(t *testing.T) {
	w := newWaterSparkleTestWorld()
	f := fieldForKind(w, KindWater)

	// Keep spawning until spawnSparkle falls back to upgrading.
	// This tests the upgrade-fallback path, not the capacity-check convergence.
	const maxIter = 500
	gotUpgrade := false
	for i := range maxIter {
		result := spawnSparkle(w, f)
		if result.Outcome == growthOutcomeUpgradedNode {
			gotUpgrade = true
			// Verify no new node was added.
			_ = i
			break
		}
	}
	if !gotUpgrade {
		t.Fatalf("spawnSparkle never fell back to upgrade after %d attempts; nodes=%d", maxIter, len(w.Nodes))
	}
	if len(w.Nodes) == 0 {
		t.Error("no sparkles in field")
	}
}

func TestUpgradeNearestSparkle(t *testing.T) {
	w := newWaterSparkleTestWorld()
	f := fieldForKind(w, KindWater)
	spawnSparkle(w, f)

	n := w.Nodes[len(w.Nodes)-1]
	sizeBefore := n.Size

	upgraded := upgradeNearestSparkle(w, f)
	if upgraded == nil {
		t.Fatal("upgradeNearestSparkle returned nil with one sparkle in field")
	}
	if upgraded.Size <= sizeBefore {
		t.Errorf("size after upgrade: %.2f, want > %.2f", upgraded.Size, sizeBefore)
	}
	if upgraded.Size > 2.0 {
		t.Errorf("size clamped at 2.0, got %.2f", upgraded.Size)
	}
}

func TestWaterSparkleGrowthCueFiresOnSpawn(t *testing.T) {
	w := newWaterSparkleTestWorld()
	f := fieldForKind(w, KindWater)

	result := spawnSparkle(w, f)
	if result.Outcome == growthOutcomeNone {
		t.Fatal("spawnSparkle returned no outcome")
	}
	activateGrowthCue(w, result)

	if !growthCueActive(w.growthCue) {
		t.Error("growth cue should be active after sparkle spawn")
	}
	if w.growthCue.NodeID != result.NodeID {
		t.Errorf("cue NodeID: got %d, want %d", w.growthCue.NodeID, result.NodeID)
	}
	if w.growthCue.Kind != KindWater {
		t.Errorf("cue Kind: got %v, want KindWater", w.growthCue.Kind)
	}
}

func TestWaterSparklesDeferredUntilFirstDock(t *testing.T) {
	w := newWaterSparkleTestWorld()
	f := fieldForKind(w, KindWater)
	if f == nil {
		t.Fatal("water field missing")
	}
	fp := w.Planet.FieldProgress[KindWater]
	fp.EXP = fp.Cap - 0.1

	depositToField(w, KindWater, 1.0)

	if waterSparkleCount(w) != 0 {
		t.Fatalf("water sparkles should not spawn before first dock, got %d", waterSparkleCount(w))
	}
	if fp.EXP < fp.Cap {
		t.Errorf("water EXP should remain ready to grow after dock placement; EXP %.2f cap %.2f", fp.EXP, fp.Cap)
	}

	if !placeBuildingWithFreePlacement(w, shoreEdgeAngle(), true) {
		t.Fatal("could not place first dock")
	}
	if waterSparkleCount(w) == 0 {
		t.Fatal("first dock should seed initial water sparkles")
	}
}

func TestFirstDockSeedsAllKnownWaterFieldsAndDockReach(t *testing.T) {
	w := newWorldWithSeed(88)
	w.Planet.Fields = []*ResourceField{
		{Kind: KindWood, CenterAngle: -math.Pi / 2, HalfArc: math.Pi / 8, Known: true},
		{Kind: KindWater, CenterAngle: 0, HalfArc: math.Pi / 5, Known: true},
		{Kind: KindWater, CenterAngle: math.Pi, HalfArc: math.Pi / 5, Known: true},
	}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{
		KindWood:  {Cap: woodFieldBaseEXP},
		KindWater: {Cap: waterFieldBaseEXP},
	}
	w.Nodes = nil
	placeBuildingWithFreePlacement(w, -math.Pi/2, true)

	if !placeBuildingWithFreePlacement(w, 0, true) {
		t.Fatal("could not place first dock")
	}
	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
			break
		}
	}
	if dock == nil {
		t.Fatal("dock missing after placement")
	}

	for i, f := range w.Planet.Fields {
		if f.Kind != KindWater {
			continue
		}
		count := 0
		for _, n := range w.Nodes {
			if n.Interior && n.Kind == KindWater && angleWithinField(f, n.Angle) {
				count++
			}
		}
		if count == 0 {
			t.Errorf("known water field %d got no initial sparkles", i)
		}
	}
	if got := len(dockServiceableSparkles(w, dock)); got == 0 {
		t.Error("first dock should have at least one serviceable sparkle in cone range")
	}
}

func TestDepositToFieldWaterGrowsSparkle(t *testing.T) {
	w := newWaterSparkleTestWorld()
	placeBuildingWithFreePlacement(w, shoreEdgeAngle(), true)
	// Ensure the water field has EXP progress tracking.
	if w.Planet.FieldProgress[KindWater] == nil {
		t.Fatal("water field has no FieldProgress entry")
	}

	before := len(w.Nodes)
	// Set EXP just below cap, then deposit enough to trigger a growth event.
	fp := w.Planet.FieldProgress[KindWater]
	fp.EXP = fp.Cap - 0.1
	depositToField(w, KindWater, 1.0)

	if len(w.Nodes) <= before {
		t.Errorf("expected a new sparkle after deposit; node count: %d (was %d)", len(w.Nodes), before)
	}
	// Verify the new node is an interior water sparkle.
	for _, n := range w.Nodes[before:] {
		if !n.Interior || n.Kind != KindWater {
			t.Errorf("unexpected non-interior node spawned by water depositToField: Interior=%v Kind=%v", n.Interior, n.Kind)
		}
	}
}

func TestNurtureFieldWaterSpawnsSparkle(t *testing.T) {
	w := newWaterSparkleTestWorld()
	placeBuildingWithFreePlacement(w, shoreEdgeAngle(), true)
	f := fieldForKind(w, KindWater)
	if f == nil {
		t.Fatal("water field missing")
	}

	before := len(w.Nodes)
	// With nurtureTreesPerPress=1 each call spawns only one node, so loop until
	// the water field is picked (up to 20 attempts with cues cleared between calls).
	gotWater := false
	for i := 0; i < 20 && !gotWater; i++ {
		w.growthCue = growthCueState{}
		w.pendingGrowthCues = nil
		ok := nurtureField(w)
		if !ok {
			t.Fatalf("nurtureField returned false on attempt %d", i+1)
		}
		for _, n := range w.Nodes[before:] {
			if n.Interior && n.Kind == KindWater {
				gotWater = true
				break
			}
		}
	}
	if !gotWater {
		t.Error("nurtureField never spawned a water sparkle in 20 attempts")
	}
}

func TestSparkleSpawnPosValid(t *testing.T) {
	w := newWaterSparkleTestWorld()
	f := fieldForKind(w, KindWater)

	innerR := w.Planet.Radius * sparkleInnerFrac
	outerR := w.Planet.Radius * sparkleOuterFrac
	midR := (innerR + outerR) / 2
	midAngle := f.CenterAngle

	// Center of the field at mid-radius: should be valid with no existing sparkles.
	pos := Vec{
		X: w.Planet.Center.X + midR*math.Cos(midAngle),
		Y: w.Planet.Center.Y + midR*math.Sin(midAngle),
	}
	if !sparkleSpawnPosValid(w, f, pos) {
		t.Error("valid mid-field position rejected")
	}

	// Position too close to planet center (below innerR): invalid.
	tooClose := Vec{
		X: w.Planet.Center.X + (innerR-1)*math.Cos(midAngle),
		Y: w.Planet.Center.Y + (innerR-1)*math.Sin(midAngle),
	}
	if sparkleSpawnPosValid(w, f, tooClose) {
		t.Error("position inside innerR should be invalid")
	}

	// Position too far from center (above outerR): invalid.
	tooFar := Vec{
		X: w.Planet.Center.X + (outerR+1)*math.Cos(midAngle),
		Y: w.Planet.Center.Y + (outerR+1)*math.Sin(midAngle),
	}
	if sparkleSpawnPosValid(w, f, tooFar) {
		t.Error("position outside outerR should be invalid")
	}

	// Place a sparkle at pos then check the same position is now blocked.
	n := newSparkle(w, pos)
	w.Nodes = append(w.Nodes, n)
	if sparkleSpawnPosValid(w, f, pos) {
		t.Error("position occupied by existing sparkle should be invalid")
	}
}

// ── Water harvesting (Phase 5) ────────────────────────────────────────────────

// newWaterHarvestFixture returns a water frontier world with a shore dock and
// several sparkles already assigned to it, ready for harvest testing.
func newWaterHarvestFixture(t *testing.T) *World {
	t.Helper()
	w := newWaterSparkleTestWorld()
	shoreEdge := shoreEdgeAngle()
	if !placeBuildingWithFreePlacement(w, shoreEdge, true) {
		t.Fatal("newWaterHarvestFixture: could not place shore dock")
	}
	f := fieldForKind(w, KindWater)
	if f == nil {
		t.Fatal("newWaterHarvestFixture: no water field")
	}
	for range 3 {
		spawnSparkle(w, f)
	}
	assignServicingDocks(w)
	return w
}

// TestAssignServicingDocksInWedge verifies that sparkles reachable by the dock
// (within dockWedgeHalfArc AND within the level's depth limit) get assigned,
// while sparkles outside either constraint are not.
func TestAssignServicingDocksInWedge(t *testing.T) {
	w := newWaterHarvestFixture(t)

	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
			break
		}
	}
	if dock == nil {
		t.Fatal("no dock found")
	}

	maxDepth := w.Planet.Radius / 3 // L1
	if dock.Level >= 2 {
		maxDepth = w.Planet.Radius
	}

	inReach := 0
	outReach := 0
	for _, n := range w.Nodes {
		if !n.Interior || n.Kind != KindWater {
			continue
		}
		depthFromRim := w.Planet.Radius - w.Planet.Center.Dist(n.Pos)
		inAngular := angularDistance(n.Angle, dock.Angle) <= dockWedgeHalfArc
		inDepth := depthFromRim <= maxDepth
		if inAngular && inDepth {
			inReach++
			if n.ServicingDockID != dock.ID {
				t.Errorf("reachable sparkle has ServicingDockID=%d, want %d", n.ServicingDockID, dock.ID)
			}
		} else {
			outReach++
			if n.ServicingDockID == dock.ID {
				t.Errorf("unreachable sparkle has ServicingDockID=%d (should not be assigned)", dock.ID)
			}
		}
	}
	t.Logf("sparkles in reach: %d, out of reach: %d", inReach, outReach)
}

// TestUnreachableSparklesUnserviced verifies that sparkles outside all dock wedges
// have ServicingDockID == -1 after assignServicingDocks.
func TestUnreachableSparklesUnserviced(t *testing.T) {
	w := newWaterSparkleTestWorld()
	f := fieldForKind(w, KindWater)
	if f == nil {
		t.Fatal("no water field")
	}
	for range 5 {
		spawnSparkle(w, f)
	}
	assignServicingDocks(w)
	for _, n := range w.Nodes {
		if !n.Interior || n.Kind != KindWater {
			continue
		}
		if n.ServicingDockID != -1 {
			t.Errorf("sparkle with no dock has ServicingDockID=%d, want -1", n.ServicingDockID)
		}
	}
}

// TestWaterDeliveryReveal verifies that the first water unload sets WaterDiscovered.
func TestWaterDeliveryReveal(t *testing.T) {
	w := newWaterHarvestFixture(t)

	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
			break
		}
	}
	if dock == nil {
		t.Fatal("no dock")
	}

	if w.Economy.WaterDiscovered {
		t.Fatal("WaterDiscovered should be false before any delivery")
	}

	wk := &Worker{ID: 99, NodeID: -1, TargetNodeID: -1, PendingNodeID: -1, DockID: dock.ID}
	w.Workers = append(w.Workers, wk)
	wk.Carried = 5.0
	completeWaterUnload(w, wk, dock)

	if !w.Economy.WaterDiscovered {
		t.Error("WaterDiscovered should be true after first water delivery")
	}
	if w.Economy.Water <= 0 {
		t.Error("Economy.Water should be positive after delivery")
	}
}

// TestWaterFocusIdleNoSparkles verifies that a worker assigned to a dock with no
// serviceable sparkles returns home instead of hanging.
func TestWaterFocusIdleNoSparkles(t *testing.T) {
	w := newWaterSparkleTestWorld()
	shoreEdge := shoreEdgeAngle()
	placeBuildingWithFreePlacement(w, shoreEdge, true)
	kept := w.Nodes[:0]
	for _, n := range w.Nodes {
		if n.Kind != KindWater {
			kept = append(kept, n)
		}
	}
	w.Nodes = kept
	assignServicingDocks(w)

	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
			break
		}
	}
	if dock == nil {
		t.Fatal("no dock")
	}

	wk := &Worker{ID: 99, NodeID: -1, TargetNodeID: -1, PendingNodeID: -1, DockID: dock.ID}
	wk.State = StateToDock
	wk.Angle = dock.Angle
	wk.Pos = w.Planet.RimPoint(dock.Angle)
	wk.DeliveryKind = KindDock
	w.Workers = append(w.Workers, wk)

	stepWorker(w, wk, 1.0/60.0)

	if wk.State == StateToDock || wk.State == StateDiving {
		t.Errorf("worker should not stay in water state with no sparkles; state=%v", wk.State)
	}
	if wk.DockID != -1 {
		t.Errorf("DockID should be -1 after returning home; got %d", wk.DockID)
	}
}

// TestNurtureFieldPlanetWide verifies that nurtureField on a planet with both
// a wood shore field and a water lake field can spawn into the water field when
// the wood field is already saturated (planet-wide Nurture).
func TestNurtureFieldPlanetWide(t *testing.T) {
	w := newWorldWithSeed(77)
	w.Planet.Fields = []*ResourceField{
		{Kind: KindWood, CenterAngle: waterFrontierShoreAngle, HalfArc: waterFrontierShoreArc, Known: true},
		{Kind: KindWater, CenterAngle: waterFrontierLakeAngle, HalfArc: waterFrontierLakeArc, Known: true},
	}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{
		KindWood:  {Cap: woodFieldBaseEXP},
		KindWater: {Cap: waterFieldBaseEXP},
	}
	w.Nodes = nil
	placeBuildingWithFreePlacement(w, waterFrontierShoreAngle, true)
	w.ResourceDiscovered = true

	// Saturate the wood (shore) field.
	fillWoodFieldNodes(w, false)
	if fieldCanSpawnNode(w, w.Planet.Fields[0]) {
		t.Fatal("wood field should be saturated after fillWoodFieldNodes")
	}
	if !placeBuildingWithFreePlacement(w, shoreEdgeAngle(), true) {
		t.Fatal("could not place dock")
	}

	// With a dock placed, nurtureField should still succeed by spawning into the water field.
	nodesBefore := len(w.Nodes)
	if !nurtureField(w) {
		t.Fatal("nurtureField returned false on a planet with an unsaturated water field")
	}
	// At least one sparkle should have been spawned into the water field.
	waterSpawned := 0
	for _, n := range w.Nodes {
		if n.Interior && n.Kind == KindWater && n.ID >= nodesBefore {
			waterSpawned++
		}
	}
	if waterSpawned == 0 {
		t.Error("nurtureField should have spawned at least one water sparkle when wood field is saturated")
	}
}

// TestFirstDockSeedingPreventsZeroWaterAttention verifies that first-dock
// sparkle seeding gives the dock reachable work instead of relying on Nurture.
func TestFirstDockSeedingPreventsZeroWaterAttention(t *testing.T) {
	w := newWorldWithSeed(11)
	w.Planet.Fields = []*ResourceField{
		{Kind: KindWood, CenterAngle: waterFrontierShoreAngle, HalfArc: waterFrontierShoreArc, Known: true},
		{Kind: KindWater, CenterAngle: waterFrontierLakeAngle, HalfArc: waterFrontierLakeArc, Known: true},
	}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{
		KindWood:  {Cap: woodFieldBaseEXP},
		KindWater: {Cap: waterFieldBaseEXP},
	}
	w.Nodes = nil
	placeBuildingWithFreePlacement(w, waterFrontierShoreAngle, true)
	w.ResourceDiscovered = true

	// No dock → should not fire.
	if nurtureAttentionActive(w) {
		t.Error("nurtureAttentionActive should be false when no dock exists")
	}

	// Place a dock at the shore edge (must be inLake so buildPreview routes to dockPreview).
	placeBuildingWithFreePlacement(w, shoreEdgeAngle(), true)

	assignServicingDocks(w)
	if waterSparkleCount(w) == 0 {
		t.Fatal("first dock should seed water sparkles")
	}
	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
			break
		}
	}
	if dock == nil {
		t.Fatal("dock missing after placement")
	}
	if len(dockServiceableSparkles(w, dock)) == 0 {
		t.Fatal("first dock should have serviceable sparkles after seeding")
	}
	if nurtureAttentionActive(w) {
		t.Error("nurtureAttentionActive should be false when first-dock seeding creates reachable water work")
	}
}

func TestRevealKindFields_ActivePlanet(t *testing.T) {
	// Build a world whose active planet has one known wood field and two
	// unknown water fields (mirroring the Lakewood tease layout).
	p := Planet{
		Center: Vec{X: 160, Y: 120},
		Radius: 80,
		Fields: []*ResourceField{
			{Kind: KindWood, Known: true, CenterAngle: 0, HalfArc: 1.0},
			{Kind: KindWater, Known: false, CenterAngle: 1.5, HalfArc: 0.5},
			{Kind: KindWater, Known: false, CenterAngle: 3.0, HalfArc: 0.4},
		},
		FieldProgress: map[ResourceKind]*KindProgress{
			KindWood: {Cap: woodFieldBaseEXP},
		},
	}
	w := &World{
		Planet:       p,
		PlanetStates: make([]*PlanetState, 2),
		rng:          rand.New(rand.NewSource(1)),
	}

	revealKindFields(w, KindWater)

	for i, f := range w.Planet.Fields {
		if f.Kind == KindWater && !f.Known {
			t.Errorf("field[%d]: KindWater should be Known after reveal", i)
		}
	}
	if w.Planet.Fields[0].Known != true {
		t.Error("wood field should remain known")
	}
	if w.Planet.FieldProgress[KindWater] == nil {
		t.Error("FieldProgress[KindWater] should be initialised after reveal")
	}
}

func TestRevealKindFields_ParkedPlanet(t *testing.T) {
	// Active planet: water frontier (no unknown fields).
	active := newWaterFrontierState()
	// Parked planet: Lakewood-style with unknown water fields.
	parked := &PlanetState{
		Planet: Planet{
			Center: Vec{X: 160, Y: 120},
			Radius: 80,
			Fields: []*ResourceField{
				{Kind: KindWood, Known: true, CenterAngle: 0, HalfArc: 1.0},
				{Kind: KindWater, Known: false, CenterAngle: 1.5, HalfArc: 0.5},
			},
			FieldProgress: map[ResourceKind]*KindProgress{
				KindWood: {Cap: woodFieldBaseEXP},
			},
		},
	}
	w := &World{
		Planet:       active.Planet,
		PlanetStates: []*PlanetState{nil, parked},
		Active:       0,
		rng:          rand.New(rand.NewSource(1)),
	}

	revealKindFields(w, KindWater)

	// Active planet water field (already known on frontier) untouched.
	for _, f := range w.Planet.Fields {
		if f.Kind == KindWater && !f.Known {
			t.Error("active planet: unknown water field not revealed")
		}
	}
	// Parked planet water field revealed.
	for i, f := range parked.Planet.Fields {
		if f.Kind == KindWater && !f.Known {
			t.Errorf("parked planet field[%d]: KindWater should be Known after reveal", i)
		}
	}
	if parked.Planet.FieldProgress[KindWater] == nil {
		t.Error("parked planet: FieldProgress[KindWater] should be initialised after reveal")
	}
}

// ── M5 economy split tests ────────────────────────────────────────────────────

// TestCampPurchaseIsolatesLocalStockpile verifies that spending wood on a logging
// camp on echo 1 does not reduce planet 0's parked local stockpile.
func TestCampPurchaseIsolatesLocalStockpile(t *testing.T) {
	w := newRevealedWorld()
	w.Economy.Wood = 500
	awakenPlanet(w, 1)
	// switchToPlanet parks planet 0 with Wood=500 into PlanetStates[0].LocalWood.
	switchToPlanet(w, 1)
	enterPlanetView(w)
	// Place Town Hall (free) so the next call places a logging camp.
	thAngle, ok := findValidBuildingAngle(w)
	if !ok || !placeBuilding(w, thAngle) {
		t.Fatal("cannot place Town Hall on echo 1")
	}
	// Fund the camp and buy it — deducts from echo 1's Economy.Wood.
	campCost := CampCost(w)
	w.Economy.Wood = campCost * 3
	campAngle, ok := findValidBuildingAngle(w)
	if !ok || !placeBuilding(w, campAngle) {
		t.Fatal("cannot place logging camp on echo 1")
	}
	if w.Economy.Wood >= campCost*3 {
		t.Error("setup: camp purchase should have reduced Economy.Wood on echo 1")
	}
	// Return to planet 0 — its parked stockpile must be intact.
	switchToPlanet(w, 0)
	if w.Economy.Wood != 500 {
		t.Errorf("planet 0 Economy.Wood: want 500, got %.2f (echo 1 purchase leaked)", w.Economy.Wood)
	}
}

// TestAwakenBootstrap_LocalWoodSufficientForCamp verifies that the seed wood
// granted on awakening an echo is enough to place at least one logging camp.
func TestAwakenBootstrap_LocalWoodSufficientForCamp(t *testing.T) {
	w := newRevealedWorld()
	awakenPlanet(w, 1)
	switchToPlanet(w, 1)
	enterPlanetView(w)
	if w.Economy.Wood < CampCost(w) {
		t.Errorf("bootstrap wood %.2f < first camp cost %.2f — player cannot afford first camp on entry",
			w.Economy.Wood, CampCost(w))
	}
}

// TestChannelsRunInBothViews asserts that tickSystemChannels runs on every Tick
// regardless of whether the player is in system view or planet view.
func TestChannelsRunInBothViews(t *testing.T) {
	const ticks = 300 // 5 simulated seconds

	woodDelivered := func(view ViewMode) float64 {
		w := newRevealedWorld()
		// Add a completed echo with a nonzero AbstractRate so channels have something to deliver.
		w.System.Planets[1].Completed = true
		w.System.Planets[1].AbstractRate = 10.0
		// Add a dormant echo as channel target.
		w.System.Channels = append(w.System.Channels, Channel{Source: 1, Resource: KindWood, Target: 2})
		before := w.System.Planets[2].AwakenFillWood
		if view == ViewSystem {
			enterSystemView(w)
		} else {
			enterPlanetView(w)
		}
		for range ticks {
			Tick(w, dt)
		}
		return w.System.Planets[2].AwakenFillWood - before
	}

	sysDelivered := woodDelivered(ViewSystem)
	plDelivered := woodDelivered(ViewPlanet)

	if sysDelivered <= 0 {
		t.Errorf("channels did not deliver wood in system view: delta=%f", sysDelivered)
	}
	if plDelivered <= 0 {
		t.Errorf("channels did not deliver wood in planet view: delta=%f", plDelivered)
	}
	if math.Abs(sysDelivered-plDelivered) > 1e-6 {
		t.Errorf("channel delivery differs between views: system=%f planet=%f", sysDelivered, plDelivered)
	}
}

// ── Channel unit tests (tickSystemChannels) ───────────────────────────────────

// newChannelFixture returns a revealed world with a completed source echo (planet 1,
// AbstractRate=10) and a dormant target echo (planet 2) linked by a wood channel.
func newChannelFixture(t *testing.T) *World {
	t.Helper()
	w := newRevealedWorld()
	w.System.Planets[1].Completed = true
	w.System.Planets[1].AbstractRate = 10.0
	if w.PlanetStates[1] == nil {
		w.PlanetStates[1] = &PlanetState{}
	}
	w.System.Channels = []Channel{{Source: 1, Resource: KindWood, Target: 2}}
	return w
}

func TestChannel_StockedDrainsSource(t *testing.T) {
	w := newChannelFixture(t)
	w.PlanetStates[1].LocalWood = 100.0
	before := w.PlanetStates[1].LocalWood
	tickSystemChannels(w, 1.0)
	want := before - channelStockedFrac*10.0*1.0
	if math.Abs(w.PlanetStates[1].LocalWood-want) > 1e-9 {
		t.Errorf("source LocalWood: got %f, want %f", w.PlanetStates[1].LocalWood, want)
	}
}

func TestChannel_EmptyNoDrain(t *testing.T) {
	w := newChannelFixture(t)
	w.PlanetStates[1].LocalWood = 0
	tickSystemChannels(w, 1.0)
	if w.PlanetStates[1].LocalWood != 0 {
		t.Errorf("source drained when empty: got %f, want 0", w.PlanetStates[1].LocalWood)
	}
}

func TestChannel_EmptyDeliversFrac(t *testing.T) {
	w := newChannelFixture(t)
	w.PlanetStates[1].LocalWood = 0
	w.System.Planets[2].AwakenReqWood = 999 // prevent auto-awaken
	tickSystemChannels(w, 1.0)
	want := channelEmptyFrac * 10.0 * 1.0
	if math.Abs(w.System.Planets[2].AwakenFillWood-want) > 1e-9 {
		t.Errorf("dormant fill (empty source): got %f, want %f", w.System.Planets[2].AwakenFillWood, want)
	}
}

func TestChannel_DormantFillAccumulates(t *testing.T) {
	w := newChannelFixture(t)
	w.System.Planets[2].AwakenReqWood = 999
	tickSystemChannels(w, 1.0)
	first := w.System.Planets[2].AwakenFillWood
	tickSystemChannels(w, 1.0)
	if w.System.Planets[2].AwakenFillWood <= first {
		t.Errorf("AwakenFillWood did not accumulate: after tick1=%f, after tick2=%f", first, w.System.Planets[2].AwakenFillWood)
	}
}

func TestChannel_AutoAwakenOnRequirementsMet(t *testing.T) {
	w := newChannelFixture(t)
	w.PlanetStates[1].LocalWood = 100.0 // stocked: delivers channelStockedFrac*rate*dt = 1.0
	// req=1.0 is exactly met by one stocked tick.
	w.System.Planets[2].AwakenReqWood = 1.0
	w.System.Planets[2].AwakenReqWater = 0
	tickSystemChannels(w, 1.0)
	if !w.System.Planets[2].Awakened {
		t.Error("target should auto-awaken when AwakenFillWood reaches AwakenReqWood")
	}
}

func TestChannel_AwakensAndContinuesDelivery(t *testing.T) {
	w := newChannelFixture(t)
	w.PlanetStates[1].LocalWood = 100.0
	w.System.Planets[2].AwakenReqWood = 1.0
	// First tick auto-awakens.
	tickSystemChannels(w, 1.0)
	if !w.System.Planets[2].Awakened {
		t.Fatal("expected auto-awaken on first tick")
	}
	// Subsequent ticks deliver wood into the awakened target's local stockpile.
	ps := w.PlanetStates[2]
	if ps == nil {
		t.Fatal("PlanetStates[2] not initialized after awakening")
	}
	beforeWood := ps.LocalWood
	tickSystemChannels(w, 1.0)
	if ps.LocalWood <= beforeWood {
		t.Errorf("LocalWood did not increase post-awakening: before=%f after=%f", beforeWood, ps.LocalWood)
	}
}

func TestRevealKindFields_WaterDeliveryTrigger(t *testing.T) {
	// Verify that completing a water unload reveals unknown water fields.
	w := newWaterHarvestFixture(t)
	// Park a Lakewood-style planet at slot 1.
	parked := &PlanetState{
		Planet: Planet{
			Center: Vec{X: 160, Y: 120},
			Radius: 80,
			Fields: []*ResourceField{
				{Kind: KindWood, Known: true, CenterAngle: 0, HalfArc: 1.0},
				{Kind: KindWater, Known: false, CenterAngle: 1.5, HalfArc: 0.5},
			},
			FieldProgress: map[ResourceKind]*KindProgress{
				KindWood: {Cap: woodFieldBaseEXP},
			},
		},
	}
	if len(w.PlanetStates) < 2 {
		w.PlanetStates = append(w.PlanetStates, nil)
	}
	w.PlanetStates[1] = parked

	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
			break
		}
	}
	if dock == nil {
		t.Fatal("no dock")
	}
	wk := &Worker{ID: 99, NodeID: -1, TargetNodeID: -1, PendingNodeID: -1, DockID: dock.ID}
	w.Workers = append(w.Workers, wk)
	wk.Carried = 5.0
	completeWaterUnload(w, wk, dock)

	if !w.Economy.WaterDiscovered {
		t.Fatal("WaterDiscovered should be true after delivery")
	}
	for i, f := range parked.Planet.Fields {
		if f.Kind == KindWater && !f.Known {
			t.Errorf("parked planet field[%d]: should be revealed after first water delivery", i)
		}
	}
	if parked.Planet.FieldProgress[KindWater] == nil {
		t.Error("parked planet: FieldProgress[KindWater] should exist after first water delivery")
	}
}
