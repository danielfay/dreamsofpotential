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
	// fieldBaseEXP (10) is well below what 30 s of deliveries produces.
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
	if field.EXP >= field.Cap {
		t.Errorf("field EXP should have reset after spawn, got %.2f / %.2f", field.EXP, field.Cap)
	}
}

func TestFieldEXPAdvanceBelowCapRecordsNoGrowthCue(t *testing.T) {
	w := NewWorld()
	field := w.Planet.Fields[0]
	field.EXP = 3
	field.Cap = 20

	depositToField(w, KindWood, 2)

	if field.EXP != 5 {
		t.Fatalf("field EXP got %.2f, want 5", field.EXP)
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
	field := w.Planet.Fields[0]
	field.EXP = 19
	field.Cap = 20

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
	activateGrowthCue(w, growthResult{
		Outcome:     growthOutcomeSpawnedNode,
		Kind:        field.Kind,
		CenterAngle: field.CenterAngle,
		HalfArc:     field.HalfArc,
		NodeID:      w.Nodes[0].ID,
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

	candidate := newNode(w, KindWood, 5/w.Planet.Radius)
	candidate.Size = 1
	if !nodeSpawnAngleValid(w, field, candidate, candidate.Angle) {
		t.Fatal("candidate outside soft spacing but inside larger visual/blocking width should be valid")
	}
}

func TestSpawnNodeSaturatedFieldUpgradesNearestNode(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.01, Cap: fieldBaseEXP}
	w.Planet.Fields = []*ResourceField{field}

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
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.01, Cap: fieldBaseEXP}
	w.Planet.Fields = []*ResourceField{field}

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
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.01, EXP: 19, Cap: 20}
	w.Planet.Fields = []*ResourceField{field}

	n := newNode(w, KindWood, 0)
	n.Size = 1
	w.Nodes = []*ResourceNode{n}

	depositToField(w, KindWood, 2)

	if len(w.Nodes) != 1 {
		t.Fatalf("expected upgrade fallback without append, got %d nodes", len(w.Nodes))
	}
	if math.Abs(field.EXP-1) > 1e-9 {
		t.Fatalf("field EXP got %.2f, want 1", field.EXP)
	}
	if math.Abs(field.Cap-40) > 1e-9 {
		t.Fatalf("cap got %.2f, want 40", field.Cap)
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
	field := w.Planet.Fields[0]
	field.EXP = 7
	field.Cap = 20

	if !upgradeFirstFieldForDebug(w) {
		t.Fatal("expected debug field upgrade to run")
	}
	if len(w.Nodes) != 1 {
		t.Fatalf("expected one spawned node, got %d", len(w.Nodes))
	}
	if field.EXP != 0 {
		t.Fatalf("field EXP got %.2f, want 0", field.EXP)
	}
	if math.Abs(field.Cap-40) > 1e-9 {
		t.Fatalf("field cap got %.2f, want 40", field.Cap)
	}
}

func TestGrowFirstFieldUntilBlockedForDebugStopsOnUpgrade(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	field := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: 0.01, Cap: fieldBaseEXP}
	w.Planet.Fields = []*ResourceField{field}

	n := newNode(w, KindWood, 0)
	n.Size = 1
	w.Nodes = []*ResourceNode{n}

	if !growFirstFieldUntilBlockedForDebug(w) {
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
	if !buyWorker(w) {
		t.Fatal("expected first worker purchase to succeed")
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

	wantBanked := gross * (1 - fieldReturnRatio)
	wantReturned := gross * fieldReturnRatio

	if math.Abs(w.Economy.Wood-wantBanked) > 1e-9 {
		t.Errorf("Economy.Wood got %.4f, want %.4f (80%% of gross)", w.Economy.Wood, wantBanked)
	}
	if math.Abs(w.Planet.Fields[0].EXP-wantReturned) > 1e-9 {
		t.Errorf("field EXP got %.4f, want %.4f (20%% of gross)", w.Planet.Fields[0].EXP, wantReturned)
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

	field := w.Planet.Fields[0]
	initialCap := fieldBaseEXP

	depositToField(w, KindWood, fieldBaseEXP)

	if len(w.Nodes) != 1 {
		t.Fatalf("expected one spawned node after crossing fieldBaseEXP, got %d", len(w.Nodes))
	}
	wantCap := initialCap * fieldEXPGrowth
	if math.Abs(field.Cap-wantCap) > 1e-9 {
		t.Fatalf("cap after first cycle got %.2f, want %.2f (×fieldEXPGrowth)", field.Cap, wantCap)
	}
	if field.EXP != 0 {
		t.Fatalf("field EXP after exact cap deposit got %.2f, want 0", field.EXP)
	}
}

func TestNurtureFieldSuccess(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true
	w.Economy.Wood = nurtureCost + 10
	initialEXP := w.Planet.Fields[0].EXP

	if !nurtureField(w, KindWood) {
		t.Fatal("nurtureField should succeed with enough wood")
	}
	if math.Abs(w.Economy.Wood-10) > 1e-9 {
		t.Errorf("wood got %.2f, want 10 after nurtureCost spent", w.Economy.Wood)
	}
	if math.Abs(w.Planet.Fields[0].EXP-(initialEXP+nurtureEXP)) > 1e-9 {
		t.Errorf("field EXP got %.2f, want %.2f", w.Planet.Fields[0].EXP, initialEXP+nurtureEXP)
	}
}

func TestNurtureFieldUnaffordable(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true
	w.Economy.Wood = nurtureCost - 1
	initialEXP := w.Planet.Fields[0].EXP
	initialWood := w.Economy.Wood

	if nurtureField(w, KindWood) {
		t.Fatal("nurtureField should fail without enough wood")
	}
	if w.Economy.Wood != initialWood {
		t.Errorf("wood should not change on failed nurture")
	}
	if w.Planet.Fields[0].EXP != initialEXP {
		t.Errorf("field EXP should not change on failed nurture")
	}
}

func TestNurtureFieldNotDiscovered(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = false
	w.Economy.Wood = nurtureCost * 10

	if nurtureField(w, KindWood) {
		t.Fatal("nurtureField should fail when resource not yet discovered")
	}
}

func TestNurtureFieldCapCrossing(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.ResourceDiscovered = true
	w.Economy.Wood = nurtureCost * 10

	field := w.Planet.Fields[0]
	field.EXP = field.Cap - (nurtureEXP / 2) // just below cap

	nurtureField(w, KindWood)

	if w.growthCue.Outcome == growthOutcomeNone {
		t.Fatal("nurture crossing cap should trigger field growth cue")
	}
}

func TestNurtureFieldRepeatClicks(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true
	w.Economy.Wood = nurtureCost * 3

	for i := 0; i < 3; i++ {
		if !nurtureField(w, KindWood) {
			t.Fatalf("nurture %d should succeed", i+1)
		}
	}
	if math.Abs(w.Economy.Wood) > 1e-9 {
		t.Errorf("after 3 nurtures, wood got %.2f, want 0", w.Economy.Wood)
	}
	if nurtureField(w, KindWood) {
		t.Error("4th nurture should fail (no wood left)")
	}
}

func TestNurtureAttentionActiveIdleWorkerNoFreeNode(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true
	w.Economy.Wood = nurtureCost

	// All nodes are owned.
	for _, n := range w.Nodes {
		n.OwnerID = 99
	}
	// One idle worker.
	w.Workers = []*Worker{{ID: 0, State: StateIdleWaiting, NodeID: -1, TargetNodeID: -1, PendingNodeID: -1}}

	if !nurtureAttentionActive(w, KindWood) {
		t.Error("expected attention active: idle worker with no free node")
	}
}

func TestNurtureAttentionActiveCapProximity(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true
	w.Economy.Wood = nurtureCost

	// Set EXP so one click would cross the cap.
	field := w.Planet.Fields[0]
	field.EXP = field.Cap - nurtureEXP

	if !nurtureAttentionActive(w, KindWood) {
		t.Error("expected attention active: one click would complete the cap")
	}
}

func TestNurtureAttentionInactiveWhenUnaffordable(t *testing.T) {
	w := NewWorld()
	w.ResourceDiscovered = true
	w.Economy.Wood = nurtureCost - 1

	for _, n := range w.Nodes {
		n.OwnerID = 99
	}
	w.Workers = []*Worker{{ID: 0, State: StateIdleWaiting, NodeID: -1, TargetNodeID: -1, PendingNodeID: -1}}

	if nurtureAttentionActive(w, KindWood) {
		t.Error("expected attention inactive: cannot afford nurture")
	}
}
