package game

import (
	"math"
	"testing"
)

func qaPtr[T any](v T) *T { return &v }

func TestBuildQAWorld_NearCapLevelCharge(t *testing.T) {
	discovered := true
	p := QAPreset{
		Seed:            11,
		Discovered:      &discovered,
		PlaceTownHall:   true,
		Workers:         7,
		SettleSeconds:   1,
		FieldExpFromCap: qaPtr(-2.0), // near cap for a level-completing Nurture charge
		Wood:            qaPtr(100.0),
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}

	if w.Version != SaveVersion {
		t.Errorf("version = %d, want %d", w.Version, SaveVersion)
	}
	if len(w.Nodes) < startingNodes {
		t.Errorf("expected at least %d nodes after founding, got %d", startingNodes, len(w.Nodes))
	}
	assertAtLeastOneIdleWorker(t, w)
	f := w.Planet.Fields[0]
	wantEXP := f.Cap - 2.0
	if math.Abs(f.EXP-wantEXP) > 0.01 {
		t.Errorf("field EXP = %.2f, want %.2f (Cap - 2)", f.EXP, wantEXP)
	}
	if w.Economy.Wood != 100 {
		t.Errorf("wood = %.1f, want 100", w.Economy.Wood)
	}
}

func TestBuildQAWorld_FarCapLevelCharge(t *testing.T) {
	discovered := true
	cycles := 2
	p := QAPreset{
		Seed:           11,
		Discovered:     &discovered,
		PlaceTownHall:  true,
		Workers:        7,
		SettleSeconds:  1,
		FieldCapCycles: &cycles,
		FieldExpAbs:    qaPtr(5.0),
		Wood:           qaPtr(100.0),
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}

	if len(w.Nodes) < startingNodes {
		t.Errorf("expected at least %d nodes after founding, got %d", startingNodes, len(w.Nodes))
	}
	assertAtLeastOneIdleWorker(t, w)
	f := w.Planet.Fields[0]
	if f.EXP >= f.Cap/2 {
		t.Errorf("field EXP = %.2f should be clearly below Cap/2 = %.2f for far-cap scenario", f.EXP, f.Cap/2)
	}
	if w.Economy.Wood != 100 {
		t.Errorf("wood = %.1f, want 100", w.Economy.Wood)
	}
}

func TestBuildQAWorld_WoodOverride(t *testing.T) {
	discovered := true
	p := QAPreset{
		Seed:          11,
		Discovered:    &discovered,
		PlaceTownHall: true,
		Workers:       4,
		SettleSeconds: 1,
		Wood:          qaPtr(80.0),
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}

	if w.Version != SaveVersion {
		t.Errorf("version = %d, want %d", w.Version, SaveVersion)
	}
	if w.Economy.Wood != 80 {
		t.Errorf("wood = %.1f, want 80", w.Economy.Wood)
	}
}

func TestBuildQAWorld_PoorCoverage(t *testing.T) {
	discovered := true
	p := QAPreset{
		Seed:          11,
		Discovered:    &discovered,
		PlaceTownHall: true,
		Workers:       2,
		NoFreeNodes:   false,
		SettleSeconds: 10,
		Wood:          qaPtr(120.0),
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}

	hasFreeNode := false
	for _, n := range w.Nodes {
		if n.OwnerID == -1 && n.ReservedByWorkerID == -1 {
			hasFreeNode = true
			break
		}
	}
	if !hasFreeNode {
		t.Error("expected at least one free node for poor-coverage scenario")
	}
	if w.Economy.Wood != 120 {
		t.Errorf("wood = %.1f, want 120", w.Economy.Wood)
	}
}

func TestBuildQAWorld_Fresh(t *testing.T) {
	discovered := false
	p := QAPreset{
		Seed:       11,
		Discovered: &discovered,
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}

	if w.ResourceDiscovered {
		t.Error("expected ResourceDiscovered = false for fresh preset")
	}
	if len(w.Buildings) != 0 {
		t.Errorf("expected no buildings, got %d", len(w.Buildings))
	}
	if w.Version != SaveVersion {
		t.Errorf("version = %d, want %d", w.Version, SaveVersion)
	}
}

func assertAtLeastOneIdleWorker(t *testing.T, w *World) {
	t.Helper()
	for _, wk := range w.Workers {
		if wk.State == StateIdleWaiting {
			return
		}
	}
	t.Error("expected at least one StateIdleWaiting worker")
}

func TestFoundingWorkerOnTownHallPlacement(t *testing.T) {
	p := QAPreset{
		Seed:          11,
		PlaceTownHall: true,
		SettleSeconds: 1,
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}
	if w.Economy.WorkerCapacity != 1 {
		t.Errorf("WorkerCapacity: got %d, want 1 (founding slot)", w.Economy.WorkerCapacity)
	}
	if len(w.Workers) != 1 {
		t.Errorf("expected exactly 1 founding worker, got %d", len(w.Workers))
	}
	if len(w.Nodes) != startingNodes {
		t.Errorf("expected %d founded nodes, got %d", startingNodes, len(w.Nodes))
	}
}

func TestBuildQAWorld_TownGrowthArrival(t *testing.T) {
	p := QAPreset{
		Seed:           11,
		PlaceTownHall:  true,
		Workers:        2,
		SettleSeconds:  1,
		WorkerCapacity: qaPtr(3),
		TownGrowthCap:  qaPtr(10.0),
		TownGrowth:     qaPtr(8.0),
		Wood:           qaPtr(60.0),
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}
	if w.Version != SaveVersion {
		t.Errorf("version = %d, want %d", w.Version, SaveVersion)
	}
	if len(w.Workers) != 2 {
		t.Errorf("expected 2 workers, got %d", len(w.Workers))
	}
	if w.Economy.WorkerCapacity != 3 {
		t.Errorf("WorkerCapacity: got %d, want 3", w.Economy.WorkerCapacity)
	}
	if w.Economy.TownGrowthCap != 10 {
		t.Errorf("TownGrowthCap: got %.2f, want 10", w.Economy.TownGrowthCap)
	}
	if w.Economy.TownGrowth != 8 {
		t.Errorf("TownGrowth: got %.2f, want 8", w.Economy.TownGrowth)
	}
	if w.Economy.Wood != 60 {
		t.Errorf("Wood: got %.1f, want 60", w.Economy.Wood)
	}
}

func TestBuildQAWorld_TownGrowthCapacityBlocked(t *testing.T) {
	p := QAPreset{
		Seed:          11,
		PlaceTownHall: true,
		Workers:       2,
		SettleSeconds: 1,
		TownGrowthCap: qaPtr(10.0),
		TownGrowth:    qaPtr(10.0), // exactly at cap
		Wood:          qaPtr(80.0),
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}
	if len(w.Workers) != 2 {
		t.Errorf("expected 2 workers (not a burst spawn), got %d", len(w.Workers))
	}
	if w.Economy.TownGrowth != w.Economy.TownGrowthCap {
		t.Errorf("TownGrowth %.2f should equal TownGrowthCap %.2f", w.Economy.TownGrowth, w.Economy.TownGrowthCap)
	}
}

func TestBuildQAWorld_FillTownCapacity(t *testing.T) {
	p := QAPreset{
		Seed:             11,
		PlaceTownHall:    true,
		FillTownCapacity: true,
		Wood:             qaPtr(200.0),
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}
	max := maxTownSlots(w)
	if max == 0 {
		t.Fatal("expected maxTownSlots > 0 after Town Hall placement")
	}
	if w.Economy.WorkerCapacity != max {
		t.Errorf("WorkerCapacity: got %d, want %d (geometry max)", w.Economy.WorkerCapacity, max)
	}
	if !townFieldFull(w) {
		t.Error("townFieldFull should return true when WorkerCapacity == maxTownSlots")
	}
}

func TestTownGrowthClampedToCapOnPreset(t *testing.T) {
	p := QAPreset{
		Seed:          11,
		PlaceTownHall: true,
		TownGrowthCap: qaPtr(5.0),
		TownGrowth:    qaPtr(999.0), // over cap — should clamp
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}
	if w.Economy.TownGrowth != 5.0 {
		t.Errorf("TownGrowth should be clamped to cap 5.0; got %.2f", w.Economy.TownGrowth)
	}
}

func TestBuildQAWorld_SystemRevealPretrigger(t *testing.T) {
	p := QAPreset{
		Seed:                    11,
		PlaceTownHall:           true,
		Workers:                 5,
		SettleSeconds:           2,
		FillTownCapacity:        true,
		NearWoodFieldSaturation: true,
		Wood:                    qaPtr(50.0),
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}
	if w.System.Unlocked {
		t.Error("System should not be unlocked yet (one spawn slot remains)")
	}
	f := fieldForKind(w, KindWood)
	if f == nil {
		t.Fatal("no wood field found")
	}
	if !fieldCanSpawnNode(w, f) {
		t.Error("field should still have one spawn slot remaining")
	}
	if !townFieldFull(w) {
		t.Error("town should be at max capacity")
	}
}

func TestBuildQAWorld_SystemViewPostreveal(t *testing.T) {
	p := QAPreset{
		Seed:              11,
		PlaceTownHall:     true,
		Workers:           5,
		SettleSeconds:     2,
		FillTownCapacity:  true,
		SaturateWoodField: true,
		Reveal:            true,
		Wood:              qaPtr(50.0),
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}
	if !w.System.Unlocked {
		t.Error("System should be unlocked after reveal preset")
	}
	if w.System.View != ViewSystem {
		t.Errorf("System.View = %v, want ViewSystem", w.System.View)
	}
	if w.System.Selected != 0 {
		t.Errorf("System.Selected = %d, want 0 (starting planet)", w.System.Selected)
	}
	if len(w.System.Planets) == 0 {
		t.Fatal("no planets in system")
	}
	if w.System.Planets[0].AbstractRate <= 0 {
		t.Errorf("starting planet AbstractRate = %.4f, want > 0", w.System.Planets[0].AbstractRate)
	}
}
