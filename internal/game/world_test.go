package game

import (
	"math"
	"testing"
)

func TestCampCostProgression(t *testing.T) {
	w := NewWorld()
	// First logging camp (CampsBought==0) costs campBaseCost.
	want0 := campBaseCost * math.Pow(campCostGrowth, 0) // 120
	if got := CampCost(w); math.Abs(got-want0) > 1e-9 {
		t.Errorf("CampCost with CampsBought=0: got %.4f, want %.4f", got, want0)
	}
	w.Economy.CampsBought = 1
	want1 := campBaseCost * math.Pow(campCostGrowth, 1)
	if got := CampCost(w); math.Abs(got-want1) > 1e-9 {
		t.Errorf("CampCost with CampsBought=1: got %.4f, want %.4f", got, want1)
	}
}

func TestMissingCostTargetsOnlyShortResources(t *testing.T) {
	w := NewWorld()
	w.Economy.Wood = 100
	w.Economy.Water = 5

	if got := missingCostTargets(w, 80, 5); got != 0 {
		t.Fatalf("affordable costs should not pulse; got mask %d", got)
	}
	if got := missingCostTargets(w, 120, 5); got != costPulseWood {
		t.Fatalf("wood-only shortage mask = %d, want %d", got, costPulseWood)
	}
	if got := missingCostTargets(w, 80, 10); got != costPulseWater {
		t.Fatalf("water-only shortage mask = %d, want %d", got, costPulseWater)
	}
	if got := missingCostTargets(w, 120, 10); got != costPulseWood|costPulseWater {
		t.Fatalf("wood+water shortage mask = %d, want %d", got, costPulseWood|costPulseWater)
	}
}

func TestSpawnWorkerAtTownHall(t *testing.T) {
	w := NewWorld()
	if spawnWorkerAtTownHall(w) != nil {
		t.Error("spawnWorkerAtTownHall with no Town Hall should return nil")
	}
	w.Buildings = append(w.Buildings, &Building{
		Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0),
	})
	wk := spawnWorkerAtTownHall(w)
	if wk == nil {
		t.Fatal("spawnWorkerAtTownHall should succeed when Town Hall exists")
	}
	if len(w.Workers) != 1 {
		t.Errorf("expected 1 worker, got %d", len(w.Workers))
	}
	if wk.State != StateSettling {
		t.Errorf("new worker state: got %v, want StateSettling", wk.State)
	}
	if math.Abs(wk.Timer-settleDelay) > 1e-9 {
		t.Errorf("new worker Timer: got %v, want settleDelay", wk.Timer)
	}
}

func TestLocalNodesPartitioning(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	nodeAngle := 1.0
	n := newNode(w, KindWood, nodeAngle)
	n.OwnerID = -1
	w.Nodes = []*ResourceNode{n}

	// Exactly at the node: inside.
	free, _, _ := localNodes(w, nodeAngle)
	if len(free) != 1 {
		t.Errorf("expected 1 free node when query == node angle, got %d", len(free))
	}
	// Just inside the arc.
	free, _, _ = localNodes(w, nodeAngle+previewArc*0.99)
	if len(free) != 1 {
		t.Errorf("expected 1 free node just inside arc, got %d", len(free))
	}
	// Just outside the arc.
	free, _, _ = localNodes(w, nodeAngle+previewArc*1.01)
	if len(free) != 0 {
		t.Errorf("expected 0 free nodes just outside arc, got %d", len(free))
	}

	// Wraparound: node near +π, query near -π.
	w.Nodes = nil
	n2 := newNode(w, KindWood, math.Pi-0.1)
	n2.OwnerID = -1
	w.Nodes = []*ResourceNode{n2}
	free, _, _ = localNodes(w, -math.Pi+0.1)
	if len(free) != 1 {
		t.Errorf("expected 1 free node for wraparound query, got %d", len(free))
	}
}

func TestFootprintHelpers(t *testing.T) {
	radius := 90.0
	node := &ResourceNode{Size: 1.25}

	if got, want := buildingHardHalfArc(KindLoggingCamp, radius), 6/radius; math.Abs(got-want) > 1e-9 {
		t.Fatalf("logging camp hard footprint got %.9f, want %.9f", got, want)
	}
	if got, want := buildingHardHalfArc(KindTownHall, radius), 10/radius; math.Abs(got-want) > 1e-9 {
		t.Fatalf("Town Hall hard footprint got %.9f, want %.9f", got, want)
	}
	if got, want := nodeBuildingBlockHalfArc(node, radius), (4*node.Size+2)/radius; math.Abs(got-want) > 1e-9 {
		t.Fatalf("node building blocker footprint got %.9f, want %.9f", got, want)
	}
	if got, want := nodeSoftHalfArc(node, radius), (2.5*node.Size)/radius; math.Abs(got-want) > 1e-9 {
		t.Fatalf("node soft footprint got %.9f, want %.9f", got, want)
	}
}

func TestAnglesOverlapWraparound(t *testing.T) {
	if !anglesOverlap(math.Pi-0.02, 0.03, -math.Pi+0.02, 0.03) {
		t.Fatal("expected overlap across ±pi boundary")
	}
	if anglesOverlap(math.Pi-0.2, 0.03, -math.Pi+0.2, 0.03) {
		t.Fatal("did not expect distant wraparound angles to overlap")
	}
}

func TestPlaceBuildingFirstIsTownHall(t *testing.T) {
	w := NewWorld()

	// First placement is valid on bare ground — no nearby tree required.
	if !placeBuilding(w, 0) {
		t.Error("first placeBuilding on empty planet should succeed")
	}
	if len(w.Buildings) != 1 {
		t.Fatalf("expected 1 building, got %d", len(w.Buildings))
	}
	if w.Buildings[0].Kind != KindTownHall {
		t.Errorf("first building Kind: got %v, want KindTownHall", w.Buildings[0].Kind)
	}
	if w.Economy.CampsBought != 0 {
		t.Errorf("CampsBought should stay 0 after Town Hall, got %d", w.Economy.CampsBought)
	}
	if w.Economy.Wood != 0 {
		t.Errorf("Wood after free Town Hall: got %.2f, want 0", w.Economy.Wood)
	}
}

func TestPlaceBuildingRefusesNodeFootprintOverlap(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	n := newNode(w, KindWood, 0)
	n.Size = 1
	w.Nodes = []*ResourceNode{n}

	if placeBuilding(w, 0) {
		t.Fatal("Town Hall placement overlapping a node should fail")
	}

	// Logging camp placement succeeds and clears the overlapping node.
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: math.Pi, Pos: w.Planet.RimPoint(math.Pi)}}
	w.Economy.Wood = CampCost(w)
	if !placeBuilding(w, 0) {
		t.Fatal("logging camp placement overlapping a node should succeed (node is cleared)")
	}
	if len(w.Nodes) != 0 {
		t.Fatalf("overlapping node should be removed after camp placement, got %d nodes", len(w.Nodes))
	}
}

func TestPlaceBuildingSecondIsLoggingCamp(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	// Pre-place a Town Hall.
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}
	w.Economy.Wood = CampCost(w) // 30

	// Second placement with a Town Hall present skips the local-node check.
	farAngle := math.Pi
	if !placeBuilding(w, farAngle) {
		t.Error("second placeBuilding should succeed even with no local nodes")
	}
	if len(w.Buildings) != 2 {
		t.Fatalf("expected 2 buildings, got %d", len(w.Buildings))
	}
	if w.Buildings[1].Kind != KindLoggingCamp {
		t.Errorf("second building Kind: got %v, want KindLoggingCamp", w.Buildings[1].Kind)
	}
	if w.Economy.CampsBought != 1 {
		t.Errorf("CampsBought: got %d, want 1", w.Economy.CampsBought)
	}
}

func TestFreePlacementIgnoresCampCost(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: math.Pi, Pos: w.Planet.RimPoint(math.Pi)}}
	w.Economy.Wood = 0

	if !placeBuildingWithFreePlacement(w, 0, true) {
		t.Fatal("free camp placement should succeed without wood")
	}
	if len(w.Buildings) != 2 {
		t.Fatalf("expected 2 buildings, got %d", len(w.Buildings))
	}
	if w.Buildings[1].Kind != KindLoggingCamp {
		t.Fatalf("free placement should add logging camp, got kind %v", w.Buildings[1].Kind)
	}
	if w.Economy.Wood != 0 {
		t.Fatalf("free placement should not change wood, got %.2f", w.Economy.Wood)
	}
	if w.Economy.CampsBought != 0 {
		t.Fatalf("free placement should not advance camp cost progression, got %d", w.Economy.CampsBought)
	}
}

func TestFreePlacementClearsOverlappingNodes(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: math.Pi, Pos: w.Planet.RimPoint(math.Pi)}}

	n := newNode(w, KindWood, 0)
	n.Size = 1
	w.Nodes = []*ResourceNode{n}

	if !placeBuildingWithFreePlacement(w, 0, true) {
		t.Fatal("free camp placement overlapping a node should succeed (node is cleared)")
	}
	if len(w.Nodes) != 0 {
		t.Fatalf("overlapping node should be removed, got %d nodes", len(w.Nodes))
	}
}

func TestPlanetMinCompletionPopFromGeometry(t *testing.T) {
	w := NewWorld()
	slots := planetMinCompletionPop(w.Planet)
	if slots != 19 {
		t.Errorf("expected 19 slots at radius 72, got %d", slots)
	}
	// Smaller planet should yield fewer slots.
	w2 := NewWorld()
	w2.Planet.Radius = 50
	slots2 := planetMinCompletionPop(w2.Planet)
	if slots2 >= slots {
		t.Errorf("smaller planet (radius 50, slots %d) should yield fewer slots than radius 72 (%d)", slots2, slots)
	}
}

func TestTownFieldSlotsStartBelowTownHall(t *testing.T) {
	w := NewWorld()
	th := &Building{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}
	w.Buildings = append(w.Buildings, th)

	slots := townFieldSlots(w.Planet, th)
	if len(slots) == 0 {
		t.Fatal("expected town field slots")
	}
	rim := w.Planet.RimPoint(th.Angle)
	firstDepth := rim.X - slots[0].X
	if math.Abs(firstDepth-townFieldRimInset) > 1e-9 {
		t.Errorf("first slot depth: got %.2f, want %.2f", firstDepth, townFieldRimInset)
	}
	townHallHeight := 2 * float64(townHallBldHalfH)
	if firstDepth < townHallHeight {
		t.Errorf("first slot depth %.2f should start below Town Hall art height %.2f", firstDepth, townHallHeight)
	}
	firstTangentialOffset := math.Abs(slots[0].Y - rim.Y)
	if firstTangentialOffset > townFieldSlotSpacing/2 {
		t.Errorf("first slot tangential offset: got %.2f, want <= %.2f", firstTangentialOffset, townFieldSlotSpacing/2)
	}
}

func TestTownHallHelper(t *testing.T) {
	w := NewWorld()

	if got := townHall(w); got != nil {
		t.Error("townHall with no buildings should return nil")
	}

	th := &Building{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}
	w.Buildings = append(w.Buildings, th)
	if got := townHall(w); got != th {
		t.Error("townHall should return the KindTownHall building")
	}

	// Adding a logging camp should not affect townHall result.
	w.Buildings = append(w.Buildings, &Building{Kind: KindLoggingCamp, Angle: 1})
	if got := townHall(w); got != th {
		t.Error("townHall should still return the Town Hall with mixed buildings")
	}
}

func TestDiscoveryFlag(t *testing.T) {
	w := newTestWorld(100)
	addWorker(w)

	if w.ResourceDiscovered {
		t.Error("ResourceDiscovered should be false before any delivery")
	}

	// Run long enough for at least one full trip (walk + load + walk + unload).
	// With workerSpeed=40, radius=90, campDist=100 arc-units, one trip takes
	// roughly 2*(100/40) + 0.5 + 0.3 ≈ 5.8 s — use 30 s to be safe.
	runSim(w, 30)

	if !w.ResourceDiscovered {
		t.Error("ResourceDiscovered should be true after a delivery")
	}
}

func TestNewWorldStartsEmpty(t *testing.T) {
	w := NewWorld()
	if len(w.Nodes) != 0 {
		t.Errorf("NewWorld: expected 0 nodes before founding, got %d", len(w.Nodes))
	}
	if w.Planet.Radius != 72.0 {
		t.Errorf("NewWorld: expected radius 72, got %.1f", w.Planet.Radius)
	}
}

func TestFoundingSpawnsStartingNodes(t *testing.T) {
	w := NewWorld()
	if !placeBuilding(w, 0) {
		t.Fatal("failed to place Town Hall on empty planet")
	}
	if len(w.Nodes) != startingNodes {
		t.Errorf("after founding: expected %d nodes, got %d", startingNodes, len(w.Nodes))
	}
}

func TestFoundingNodesDontOverlapTownHall(t *testing.T) {
	w := NewWorld()
	thAngle := 0.0
	if !placeBuilding(w, thAngle) {
		t.Fatal("failed to place Town Hall on empty planet")
	}
	th := w.Buildings[0]
	thHalf := buildingHardHalfArc(KindTownHall, w.Planet.Radius)
	for _, n := range w.Nodes {
		nodeHalf := nodeBuildingBlockHalfArc(n, w.Planet.Radius)
		if anglesOverlap(n.Angle, nodeHalf, th.Angle, thHalf) {
			t.Errorf("founded node at %.3f overlaps Town Hall at %.3f", n.Angle, thAngle)
		}
	}
}
