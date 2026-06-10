package game

import (
	"math"
	"testing"
)

func qaPtr[T any](v T) *T { return &v }

func TestBuildQAWorld_NearCapStall(t *testing.T) {
	discovered := true
	p := QAPreset{
		Seed:            11,
		Discovered:      &discovered,
		PlaceTownHall:   true,
		Workers:         7,
		SettleSeconds:   1,
		FieldExpFromCap: qaPtr(-nurtureEXP),
		Wood:            qaPtr(100.0),
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}

	if w.Version != SaveVersion {
		t.Errorf("version = %d, want %d", w.Version, SaveVersion)
	}
	if len(w.Nodes) != startingNodes {
		t.Errorf("expected %d nodes, got %d", startingNodes, len(w.Nodes))
	}
	assertAtLeastOneIdleWorker(t, w)
	f := w.Planet.Fields[0]
	wantEXP := f.Cap - nurtureEXP
	if math.Abs(f.EXP-wantEXP) > 0.01 {
		t.Errorf("field EXP = %.2f, want %.2f (Cap - nurtureEXP)", f.EXP, wantEXP)
	}
	if w.Economy.Wood != 100 {
		t.Errorf("wood = %.1f, want 100", w.Economy.Wood)
	}
}

func TestBuildQAWorld_FarCapStall(t *testing.T) {
	discovered := true
	cycles := 2
	p := QAPreset{
		Seed:           11,
		Discovered:     &discovered,
		PlaceTownHall:  true,
		Workers:        7,
		SettleSeconds:  1,
		FieldCapCycles: &cycles,
		FieldExpAbs:    qaPtr(nurtureEXP),
		Wood:           qaPtr(100.0),
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		t.Fatalf("BuildQAWorld: %v", err)
	}

	if len(w.Nodes) != startingNodes {
		t.Errorf("expected %d nodes, got %d", startingNodes, len(w.Nodes))
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
