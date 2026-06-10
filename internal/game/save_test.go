package game

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"strings"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(100)
	addWorker(w)
	runSim(w, 5)

	w.ResourceDiscovered = true // ensure the bool is tested in the round-trip
	if len(w.Planet.Fields) > 0 {
		w.Planet.Fields[0].NurtureCharges = 3 // ensure NurtureCharges survives round-trip
	}
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
	if got.Economy.WorkerCapacity != w.Economy.WorkerCapacity {
		t.Errorf("Economy.WorkerCapacity: got %d, want %d", got.Economy.WorkerCapacity, w.Economy.WorkerCapacity)
	}
	if got.Economy.CapacityBought != w.Economy.CapacityBought {
		t.Errorf("Economy.CapacityBought: got %d, want %d", got.Economy.CapacityBought, w.Economy.CapacityBought)
	}
	if got.Economy.TownGrowth != w.Economy.TownGrowth {
		t.Errorf("Economy.TownGrowth: got %.4f, want %.4f", got.Economy.TownGrowth, w.Economy.TownGrowth)
	}
	if got.Economy.TownGrowthCap != w.Economy.TownGrowthCap {
		t.Errorf("Economy.TownGrowthCap: got %.4f, want %.4f", got.Economy.TownGrowthCap, w.Economy.TownGrowthCap)
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
		if gb.Kind != b.Kind {
			t.Errorf("Buildings[%d].Kind: got %v, want %v", i, gb.Kind, b.Kind)
		}
	}

	if got.ResourceDiscovered != w.ResourceDiscovered {
		t.Errorf("ResourceDiscovered: got %v, want %v", got.ResourceDiscovered, w.ResourceDiscovered)
	}
	if len(got.Planet.Fields) > 0 && len(w.Planet.Fields) > 0 {
		if got.Planet.Fields[0].NurtureCharges != w.Planet.Fields[0].NurtureCharges {
			t.Errorf("Fields[0].NurtureCharges: got %d, want %d",
				got.Planet.Fields[0].NurtureCharges, w.Planet.Fields[0].NurtureCharges)
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
		if gwk.TargetNodeID != wk.TargetNodeID {
			t.Errorf("Workers[%d].TargetNodeID: got %v, want %v", i, gwk.TargetNodeID, wk.TargetNodeID)
		}
		if gwk.PendingNodeID != wk.PendingNodeID {
			t.Errorf("Workers[%d].PendingNodeID: got %v, want %v", i, gwk.PendingNodeID, wk.PendingNodeID)
		}
	}
}

func TestSaveLoadRoundTripWorkerReservationAndPulseState(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newWorldSingleNode(0, 0)
	addWorker(w)
	runUntilAssigned(w)

	wk := w.Workers[0]
	node := w.Nodes[0]
	node.ReservedByWorkerID = wk.ID
	wk.PendingNodeID = node.ID
	wk.Pulse = PulseState{Remaining: 0.2, LastActivated: 1.1, SteadyUntil: 1.6}
	node.Pulse = PulseState{Remaining: 0.1, LastActivated: 1.2, SteadyUntil: 1.7}
	w.Buildings[0].DeliveredWood = 3
	w.Buildings[0].DeliveryCount = 2
	w.Buildings[0].Pulse = PulseState{Remaining: 0.15, LastActivated: 1.3, SteadyUntil: 1.8}
	w.SimTime = 2.5

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.SimTime != w.SimTime {
		t.Fatalf("SimTime got %v, want %v", got.SimTime, w.SimTime)
	}
	gwk := got.Workers[0]
	if gwk.PendingNodeID != wk.PendingNodeID || gwk.Pulse != wk.Pulse {
		t.Fatalf("worker intent/pulse did not round-trip")
	}
	if got.Nodes[0].ReservedByWorkerID != node.ReservedByWorkerID || got.Nodes[0].Pulse != node.Pulse {
		t.Fatalf("node reservation/pulse did not round-trip")
	}
	gb := got.Buildings[0]
	if gb.DeliveredWood != 3 || gb.DeliveryCount != 2 || gb.Pulse != w.Buildings[0].Pulse {
		t.Fatalf("building delivery/pulse state did not round-trip")
	}
}

func TestGrowthCueStateIsTransientAndNotSaved(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newWorldSingleNode(0, 0)
	field := w.Planet.Fields[0]
	activateGrowthCue(w, growthResult{
		Outcome:     growthOutcomeUpgradedNode,
		Kind:        field.Kind,
		CenterAngle: field.CenterAngle,
		HalfArc:     field.HalfArc,
		NodeID:      w.Nodes[0].ID,
	})

	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "growthCue") ||
		strings.Contains(string(data), "GaugeRelease") ||
		strings.Contains(string(data), "FieldPulse") {
		t.Fatalf("transient growth cue leaked into save JSON: %s", data)
	}

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.growthCue.Outcome != growthOutcomeNone ||
		got.growthCue.GaugeRelease != 0 ||
		got.growthCue.FieldPulse != 0 ||
		got.growthCue.NodeCue != 0 {
		t.Fatalf("loaded world should not restore transient growth cue: %+v", got.growthCue)
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
