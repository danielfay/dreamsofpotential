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

// TestSaveLoadRoundTripSystem verifies that the System durable state — unlock
// flag, view mode, selected planet, and abstract rates — survives a save/load
// cycle unchanged.
func TestSaveLoadRoundTripSystem(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(100)
	w.System.Unlocked = true
	w.System.View = ViewSystem
	w.System.Selected = 0
	if len(w.System.Planets) > 0 {
		w.System.Planets[0].AbstractRate = 3.14
	}

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.System.Unlocked != w.System.Unlocked {
		t.Errorf("System.Unlocked: got %v, want %v", got.System.Unlocked, w.System.Unlocked)
	}
	if got.System.View != w.System.View {
		t.Errorf("System.View: got %v, want %v", got.System.View, w.System.View)
	}
	if got.System.Selected != w.System.Selected {
		t.Errorf("System.Selected: got %v, want %v", got.System.Selected, w.System.Selected)
	}
	if len(got.System.Planets) != len(w.System.Planets) {
		t.Fatalf("System.Planets len: got %d, want %d", len(got.System.Planets), len(w.System.Planets))
	}
	if len(got.System.Planets) > 0 {
		if got.System.Planets[0].AbstractRate != w.System.Planets[0].AbstractRate {
			t.Errorf("System.Planets[0].AbstractRate: got %v, want %v",
				got.System.Planets[0].AbstractRate, w.System.Planets[0].AbstractRate)
		}
	}
}

// TestSaveLoadRoundTripPlanetStates verifies that PlanetStates and Active
// survive a save/load cycle, and that Economy.Wood is not corrupted by
// the park/load helpers.
func TestSaveLoadRoundTripPlanetStates(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(100)
	// Manually park the starting planet so PlanetStates[0] is populated.
	parkActive(w)

	wantWood := 42.5
	w.Economy.Wood = wantWood
	wantCenter := w.Planet.Center
	wantBuildings := len(w.Buildings)
	wantWorkers := len(w.Workers)
	wantTownGrowthCap := w.Economy.TownGrowthCap

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Active != w.Active {
		t.Errorf("Active: got %d, want %d", got.Active, w.Active)
	}
	if len(got.PlanetStates) != len(w.PlanetStates) {
		t.Fatalf("PlanetStates len: got %d, want %d", len(got.PlanetStates), len(w.PlanetStates))
	}
	ps := got.PlanetStates[0]
	if ps == nil {
		t.Fatal("PlanetStates[0] is nil after load")
	}
	if ps.Planet.Center != wantCenter {
		t.Errorf("PlanetStates[0].Planet.Center: got %v, want %v", ps.Planet.Center, wantCenter)
	}
	if len(ps.Buildings) != wantBuildings {
		t.Errorf("PlanetStates[0].Buildings len: got %d, want %d", len(ps.Buildings), wantBuildings)
	}
	if len(ps.Workers) != wantWorkers {
		t.Errorf("PlanetStates[0].Workers len: got %d, want %d", len(ps.Workers), wantWorkers)
	}
	if ps.TownGrowthCap != wantTownGrowthCap {
		t.Errorf("PlanetStates[0].TownGrowthCap: got %v, want %v", ps.TownGrowthCap, wantTownGrowthCap)
	}
	if got.Economy.Wood != wantWood {
		t.Errorf("Economy.Wood: got %v, want %v", got.Economy.Wood, wantWood)
	}
}

// TestSaveLoadRoundTripEchoLifecycle verifies that awakened and completed echo
// state — including PlanetStates, Awakened, Completed, LayoutID, and the amplified
// AbstractRate — survives a save/load cycle unchanged.
func TestSaveLoadRoundTripEchoLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newMasteredWorld()
	addWorker(w)
	runSim(w, 2)
	Tick(w, dt) // unlock

	// Awaken echo 1 and mark it completed with a known amplified rate.
	w.Economy.Potential[PotentialForest] = 1
	awakenPlanet(w, 1)
	w.System.Planets[1].Completed = true
	w.System.Planets[1].AbstractRate = 3.75
	wantLayoutID := w.System.Planets[1].LayoutID
	w.System.Selected = 1
	w.Active = 0

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !got.System.Planets[1].Awakened {
		t.Error("echo 1 Awakened should survive round-trip")
	}
	if !got.System.Planets[1].Completed {
		t.Error("echo 1 Completed should survive round-trip")
	}
	if got.System.Planets[1].AbstractRate != 3.75 {
		t.Errorf("echo 1 AbstractRate: got %f, want 3.75", got.System.Planets[1].AbstractRate)
	}
	if got.System.Planets[1].LayoutID != wantLayoutID {
		t.Errorf("echo 1 LayoutID: got %d, want %d", got.System.Planets[1].LayoutID, wantLayoutID)
	}
	if got.PlanetStates[1] == nil {
		t.Error("PlanetStates[1] should be non-nil after awakening echo 1")
	}
	if got.System.Selected != 1 {
		t.Errorf("System.Selected: got %d, want 1", got.System.Selected)
	}
}

func TestSavePotentialRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(100)
	w.Economy.Potential[PotentialForest] = 2
	w.Economy.Potential[PotentialWater] = 1

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Economy.Potential[PotentialForest] != 2 {
		t.Errorf("PotentialForest: got %d, want 2", got.Economy.Potential[PotentialForest])
	}
	if got.Economy.Potential[PotentialWater] != 1 {
		t.Errorf("PotentialWater: got %d, want 1", got.Economy.Potential[PotentialWater])
	}
}

func TestFieldProgressRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(100)
	// Manually set non-default EXP/Cap on the wood field progress.
	fp := w.Planet.FieldProgress[KindWood]
	fp.EXP = 7.5
	fp.Cap = 40.0

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	gfp := got.Planet.FieldProgress[KindWood]
	if gfp == nil {
		t.Fatal("FieldProgress[KindWood] is nil after load")
	}
	if gfp.EXP != fp.EXP {
		t.Errorf("EXP: got %f, want %f", gfp.EXP, fp.EXP)
	}
	if gfp.Cap != fp.Cap {
		t.Errorf("Cap: got %f, want %f", gfp.Cap, fp.Cap)
	}
}

// TestSaveLoadRoundTripWaterEconomy verifies that Economy.Water and
// Economy.WaterDiscovered survive a save/load cycle unchanged.
func TestSaveLoadRoundTripWaterEconomy(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(100)
	w.Economy.Water = 12.5
	w.Economy.WaterDiscovered = true

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Economy.Water != w.Economy.Water {
		t.Errorf("Economy.Water: got %.4f, want %.4f", got.Economy.Water, w.Economy.Water)
	}
	if got.Economy.WaterDiscovered != w.Economy.WaterDiscovered {
		t.Errorf("Economy.WaterDiscovered: got %v, want %v", got.Economy.WaterDiscovered, w.Economy.WaterDiscovered)
	}
}

// TestSaveLoadRoundTripAbstractWaterRate verifies that SystemPlanet.AbstractWaterRate
// survives a save/load cycle.
func TestSaveLoadRoundTripAbstractWaterRate(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(100)
	w.System.Planets[1].AbstractWaterRate = 1.75
	w.System.Planets[2].AbstractWaterRate = 0.9

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.System.Planets[1].AbstractWaterRate != w.System.Planets[1].AbstractWaterRate {
		t.Errorf("Planets[1].AbstractWaterRate: got %f, want %f",
			got.System.Planets[1].AbstractWaterRate, w.System.Planets[1].AbstractWaterRate)
	}
	if got.System.Planets[2].AbstractWaterRate != w.System.Planets[2].AbstractWaterRate {
		t.Errorf("Planets[2].AbstractWaterRate: got %f, want %f",
			got.System.Planets[2].AbstractWaterRate, w.System.Planets[2].AbstractWaterRate)
	}
}

// TestSaveLoadRoundTripFrontierAwakened verifies that awakening the frontier
// (Planets[3], PlanetUnknown) and its PlanetState survive a save/load cycle.
func TestSaveLoadRoundTripFrontierAwakened(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newRevealedWorld()
	w.Economy.Potential[PotentialForest] = 1
	w.Economy.Potential[PotentialWater] = 1
	awakenPlanet(w, 3)

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !got.System.Planets[3].Awakened {
		t.Error("Planets[3].Awakened should survive round-trip")
	}
	if got.PlanetStates[3] == nil {
		t.Error("PlanetStates[3] should be non-nil after round-trip")
	}
	if got.Economy.Potential[PotentialForest] != 0 {
		t.Errorf("Forest Potential after frontier awaken: got %d, want 0", got.Economy.Potential[PotentialForest])
	}
	if got.Economy.Potential[PotentialWater] != 0 {
		t.Errorf("Water Potential after frontier awaken: got %d, want 0", got.Economy.Potential[PotentialWater])
	}
}

// TestSaveLoadRoundTripInteriorSparkle verifies that Interior and ServicingDockID
// survive a save/load cycle and that the sparkle's Pos is NOT expected to be on the rim.
func TestSaveLoadRoundTripInteriorSparkle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newWaterSparkleTestWorld()
	f := fieldForKind(w, KindWater)
	result := spawnSparkle(w, f)
	if result.Outcome != growthOutcomeSpawnedNode {
		t.Fatalf("setup: spawnSparkle failed: outcome=%v", result.Outcome)
	}
	sparkle := findNode(w, result.NodeID)
	if sparkle == nil {
		t.Fatal("setup: sparkle not found after spawn")
	}
	wantPos := sparkle.Pos
	wantAngle := sparkle.Angle

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var gotSparkle *ResourceNode
	for _, n := range got.Nodes {
		if n.ID == sparkle.ID {
			gotSparkle = n
			break
		}
	}
	if gotSparkle == nil {
		t.Fatal("sparkle not found after load")
	}
	if !gotSparkle.Interior {
		t.Error("Interior should be true after round-trip")
	}
	if gotSparkle.ServicingDockID != -1 {
		t.Errorf("ServicingDockID: got %d, want -1", gotSparkle.ServicingDockID)
	}
	if gotSparkle.Pos != wantPos {
		t.Errorf("Pos: got %v, want %v", gotSparkle.Pos, wantPos)
	}
	if gotSparkle.Angle != wantAngle {
		t.Errorf("Angle: got %v, want %v", gotSparkle.Angle, wantAngle)
	}
	// Interior sparkle must NOT be on the rim.
	dist := gotSparkle.Pos.Dist(got.Planet.Center)
	if math.Abs(dist-got.Planet.Radius) < 1e-6 {
		t.Errorf("interior sparkle Pos should not be on the rim (dist=%.4f, radius=%.4f)", dist, got.Planet.Radius)
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
