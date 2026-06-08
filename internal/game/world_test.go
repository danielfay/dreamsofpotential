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

func TestCampCostFirstFree(t *testing.T) {
	w := NewWorld()
	if got := CampCost(w); got != 0 {
		t.Errorf("CampCost with CampsBought=0: got %.2f, want 0", got)
	}
	w.Economy.CampsBought = 1
	want := campBaseCost * math.Pow(campCostGrowth, 1)
	if got := CampCost(w); math.Abs(got-want) > 1e-9 {
		t.Errorf("CampCost with CampsBought=1: got %.4f, want %.4f", got, want)
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
	// Add a camp manually (bypassing buyCamp) so we test buyWorker in isolation.
	w.Buildings = append(w.Buildings, &Building{
		Angle: 0, Pos: w.Planet.RimPoint(0),
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
		Angle: 0, Pos: w.Planet.RimPoint(0),
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

func TestHasLocalNode(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	nodeAngle := 1.0
	n := newNode(w, KindWood, nodeAngle)
	w.Nodes = []*ResourceNode{n}

	arc := 0.6

	// Exactly at the node: inside.
	if !hasLocalNode(w, nodeAngle, arc) {
		t.Error("expected true when query angle == node angle")
	}
	// Just inside the arc.
	if !hasLocalNode(w, nodeAngle+arc*0.99, arc) {
		t.Error("expected true just inside arc")
	}
	// Just outside the arc.
	if hasLocalNode(w, nodeAngle+arc*1.01, arc) {
		t.Error("expected false just outside arc")
	}

	// Wraparound: node near +π, query near -π.
	w.Nodes = nil
	n2 := newNode(w, KindWood, math.Pi-0.1)
	w.Nodes = []*ResourceNode{n2}
	if !hasLocalNode(w, -math.Pi+0.1, arc) {
		t.Error("expected true for wraparound query near ±π boundary")
	}
}

func TestBuyCampFirstFreeRequiresLocalNode(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	// Single node at a known angle.
	nodeAngle := 0.0
	w.Nodes = []*ResourceNode{newNode(w, KindWood, nodeAngle)}

	// Angle far from the node — should fail for the first camp.
	farAngle := normAngle(nodeAngle + math.Pi)
	if buyCamp(w, farAngle) {
		t.Error("first buyCamp far from all nodes should return false")
	}
	if len(w.Buildings) != 0 {
		t.Error("no building should be added when first buyCamp fails local check")
	}

	// Angle near the node — should succeed.
	if !buyCamp(w, nodeAngle) {
		t.Error("first buyCamp near a node should succeed")
	}
	if len(w.Buildings) != 1 {
		t.Errorf("expected 1 building, got %d", len(w.Buildings))
	}
	if w.Economy.CampsBought != 1 {
		t.Errorf("CampsBought: got %d, want 1", w.Economy.CampsBought)
	}
	if w.Economy.Wood != 0 {
		t.Errorf("Wood after free first camp: got %.2f, want 0", w.Economy.Wood)
	}
}

func TestBuyCampLaterSkipsLocalCheck(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.Economy.CampsBought = 1 // simulate first already placed
	w.Economy.Wood = CampCost(w)

	// No nodes anywhere — but later camps skip the check, so it should succeed.
	farAngle := math.Pi
	if !buyCamp(w, farAngle) {
		t.Error("second+ buyCamp should succeed even with no local nodes")
	}
	if len(w.Buildings) != 1 {
		t.Errorf("expected 1 building, got %d", len(w.Buildings))
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
