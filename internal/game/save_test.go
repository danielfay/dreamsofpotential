package game

import (
	"errors"
	"math"
	"os"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(100)
	addWorker(w)
	runSim(w, 5)

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Economy.Wood != w.Economy.Wood {
		t.Errorf("Economy.Wood: got %.4f, want %.4f", got.Economy.Wood, w.Economy.Wood)
	}
	if got.Economy.WorkersBought != w.Economy.WorkersBought {
		t.Errorf("Economy.WorkersBought: got %d, want %d", got.Economy.WorkersBought, w.Economy.WorkersBought)
	}
	if got.Planet.Radius != w.Planet.Radius {
		t.Errorf("Planet.Radius: got %v, want %v", got.Planet.Radius, w.Planet.Radius)
	}

	// All nodes must still be on the rim after a round-trip.
	if len(got.Nodes) != len(w.Nodes) {
		t.Fatalf("Nodes count: got %d, want %d", len(got.Nodes), len(w.Nodes))
	}
	p := got.Planet
	for i, n := range got.Nodes {
		dist := n.Pos.Dist(p.Center)
		if math.Abs(dist-p.Radius) > 1e-6 {
			t.Errorf("Nodes[%d].Pos is %.6f from center, want %.6f (on rim)", i, dist, p.Radius)
		}
	}

	if len(got.Buildings) != len(w.Buildings) {
		t.Fatalf("Buildings count: got %d, want %d", len(got.Buildings), len(w.Buildings))
	}
	for i, b := range w.Buildings {
		gb := got.Buildings[i]
		if gb.Angle != b.Angle {
			t.Errorf("Buildings[%d].Angle: got %v, want %v", i, gb.Angle, b.Angle)
		}
	}

	if len(got.Workers) != len(w.Workers) {
		t.Fatalf("Workers count: got %d, want %d", len(got.Workers), len(w.Workers))
	}
	for i, wk := range w.Workers {
		gwk := got.Workers[i]
		if gwk.State != wk.State {
			t.Errorf("Workers[%d].State: got %v, want %v", i, gwk.State, wk.State)
		}
		if gwk.Angle != wk.Angle {
			t.Errorf("Workers[%d].Angle: got %v, want %v", i, gwk.Angle, wk.Angle)
		}
		if gwk.Carried != wk.Carried {
			t.Errorf("Workers[%d].Carried: got %v, want %v", i, gwk.Carried, wk.Carried)
		}
		if gwk.NodeID != wk.NodeID {
			t.Errorf("Workers[%d].NodeID: got %v, want %v", i, gwk.NodeID, wk.NodeID)
		}
	}
}

// TestLoadIDConsistency verifies that after a save round-trip, every active
// worker's NodeID matches a node whose OwnerID equals that worker's ID.
func TestLoadIDConsistency(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(100)
	addWorker(w)
	runSim(w, 5) // let the assignment pass run

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	nodesByID := make(map[int]*ResourceNode, len(got.Nodes))
	for _, n := range got.Nodes {
		nodesByID[n.ID] = n
	}

	for _, wk := range got.Workers {
		if wk.NodeID == -1 {
			continue // idle worker; no ownership to check
		}
		n, ok := nodesByID[wk.NodeID]
		if !ok {
			t.Errorf("worker %d has NodeID %d which doesn't exist in Nodes", wk.ID, wk.NodeID)
			continue
		}
		if n.OwnerID != wk.ID {
			t.Errorf("worker %d → node %d: node.OwnerID is %d, want %d", wk.ID, n.ID, n.OwnerID, wk.ID)
		}
	}
}

func TestLoadMissingFileReturnsError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing save file, got nil")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestSaveNoMarshalCycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(100)
	addWorker(w)

	if err := Save(w); err != nil {
		t.Errorf("Save with workers: %v", err)
	}
}

// TestLoadVersionMismatch verifies that a save with a different version is
// treated as missing (returns os.ErrNotExist) so the caller starts fresh.
func TestLoadVersionMismatch(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(100)
	w.Version = SaveVersion - 1 // deliberately stale
	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, err := Load()
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist for version mismatch, got %v", err)
	}
}
