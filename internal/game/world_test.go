package game

import (
	"math"
	"testing"
)

func TestWorkerCostFirstFree(t *testing.T) {
	w := NewWorld()
	if got := WorkerCost(w); got != 0 {
		t.Errorf("WorkerCost with WorkersBought=0: got %.2f, want 0", got)
	}
	w.Economy.WorkersBought = 1
	want := workerBaseCost * math.Pow(workerCostGrowth, 1)
	if got := WorkerCost(w); math.Abs(got-want) > 1e-9 {
		t.Errorf("WorkerCost with WorkersBought=1: got %.4f, want %.4f", got, want)
	}
	w.Economy.WorkersBought = 2
	want2 := workerBaseCost * math.Pow(workerCostGrowth, 2)
	if got := WorkerCost(w); math.Abs(got-want2) > 1e-9 {
		t.Errorf("WorkerCost with WorkersBought=2: got %.4f, want %.4f", got, want2)
	}
}

func TestCampCostProgression(t *testing.T) {
	w := NewWorld()
	// First logging camp (CampsBought==0) costs campBaseCost.
	want0 := campBaseCost * math.Pow(campCostGrowth, 0) // 30
	if got := CampCost(w); math.Abs(got-want0) > 1e-9 {
		t.Errorf("CampCost with CampsBought=0: got %.4f, want %.4f", got, want0)
	}
	w.Economy.CampsBought = 1
	want1 := campBaseCost * math.Pow(campCostGrowth, 1)
	if got := CampCost(w); math.Abs(got-want1) > 1e-9 {
		t.Errorf("CampCost with CampsBought=1: got %.4f, want %.4f", got, want1)
	}
}

func TestBuyWorkerNoCamp(t *testing.T) {
	w := NewWorld()
	if buyWorker(w) {
		t.Error("buyWorker with no camps should return false")
	}
	if len(w.Workers) != 0 {
		t.Error("buyWorker with no camps should not add a worker")
	}
}

func TestBuyWorkerFirstFree(t *testing.T) {
	w := NewWorld()
	// Add a Town Hall manually so we test buyWorker in isolation.
	w.Buildings = append(w.Buildings, &Building{
		Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0),
	})
	w.Economy.Wood = 0

	if !buyWorker(w) {
		t.Fatal("first buyWorker should succeed even with Wood=0 (free)")
	}
	if len(w.Workers) != 1 {
		t.Errorf("expected 1 worker after first buy, got %d", len(w.Workers))
	}
	if w.Economy.WorkersBought != 1 {
		t.Errorf("WorkersBought: got %d, want 1", w.Economy.WorkersBought)
	}
	if w.Economy.Wood != 0 {
		t.Errorf("Wood after free buy: got %.2f, want 0", w.Economy.Wood)
	}
}

func TestBuyWorkerSecondCostsWood(t *testing.T) {
	w := NewWorld()
	w.Buildings = append(w.Buildings, &Building{
		Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0),
	})
	w.Economy.WorkersBought = 1 // first already bought
	w.Economy.Wood = 0          // no wood for the second

	if buyWorker(w) {
		t.Error("second buyWorker with Wood=0 should return false")
	}
	if len(w.Workers) != 0 {
		t.Error("no worker should be added when purchase fails")
	}

	// Give enough wood and it should succeed.
	w.Economy.Wood = WorkerCost(w)
	if !buyWorker(w) {
		t.Error("second buyWorker with enough wood should succeed")
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
	free, _ := localNodes(w, nodeAngle)
	if len(free) != 1 {
		t.Errorf("expected 1 free node when query == node angle, got %d", len(free))
	}
	// Just inside the arc.
	free, _ = localNodes(w, nodeAngle+previewArc*0.99)
	if len(free) != 1 {
		t.Errorf("expected 1 free node just inside arc, got %d", len(free))
	}
	// Just outside the arc.
	free, _ = localNodes(w, nodeAngle+previewArc*1.01)
	if len(free) != 0 {
		t.Errorf("expected 0 free nodes just outside arc, got %d", len(free))
	}

	// Wraparound: node near +π, query near -π.
	w.Nodes = nil
	n2 := newNode(w, KindWood, math.Pi-0.1)
	n2.OwnerID = -1
	w.Nodes = []*ResourceNode{n2}
	free, _ = localNodes(w, -math.Pi+0.1)
	if len(free) != 1 {
		t.Errorf("expected 1 free node for wraparound query, got %d", len(free))
	}
}

func TestPlaceBuildingFirstIsTownHall(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	// Single node at a known angle.
	nodeAngle := 0.0
	w.Nodes = []*ResourceNode{newNode(w, KindWood, nodeAngle)}

	// Far from the node — should fail (Town Hall needs a local free node).
	farAngle := normAngle(nodeAngle + math.Pi)
	if placeBuilding(w, farAngle) {
		t.Error("first placeBuilding far from all nodes should return false")
	}
	if len(w.Buildings) != 0 {
		t.Error("no building should be added when first placeBuilding fails local check")
	}

	// Near the node — should succeed and place a free Town Hall.
	if !placeBuilding(w, nodeAngle) {
		t.Error("first placeBuilding near a node should succeed")
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
