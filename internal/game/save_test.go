package game

import (
	"errors"
	"os"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(30)
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
	if got.Forest.Pos != w.Forest.Pos {
		t.Errorf("Forest.Pos: got %v, want %v", got.Forest.Pos, w.Forest.Pos)
	}
	if len(got.Buildings) != len(w.Buildings) {
		t.Fatalf("Buildings count: got %d, want %d", len(got.Buildings), len(w.Buildings))
	}
	for i, b := range w.Buildings {
		gb := got.Buildings[i]
		if gb.Angle != b.Angle {
			t.Errorf("Buildings[%d].Angle: got %v, want %v", i, gb.Angle, b.Angle)
		}
		if len(gb.Workers) != len(b.Workers) {
			t.Fatalf("Buildings[%d] worker count: got %d, want %d", i, len(gb.Workers), len(b.Workers))
		}
		for j, wk := range b.Workers {
			gwk := gb.Workers[j]
			if gwk.State != wk.State {
				t.Errorf("Workers[%d][%d].State: got %v, want %v", i, j, gwk.State, wk.State)
			}
			if gwk.Angle != wk.Angle {
				t.Errorf("Workers[%d][%d].Angle: got %v, want %v", i, j, gwk.Angle, wk.Angle)
			}
			if gwk.Carried != wk.Carried {
				t.Errorf("Workers[%d][%d].Carried: got %v, want %v", i, j, gwk.Carried, wk.Carried)
			}
			if gwk.Timer != wk.Timer {
				t.Errorf("Workers[%d][%d].Timer: got %v, want %v", i, j, gwk.Timer, wk.Timer)
			}
		}
	}
}

func TestLoadRebuildsHomePointers(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newTestWorld(30)
	addWorker(w)

	if err := Save(w); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for i, b := range got.Buildings {
		for j, wk := range b.Workers {
			if wk.Home != b {
				t.Errorf("Buildings[%d].Workers[%d].Home not rebuilt (got %p, want %p)", i, j, wk.Home, b)
			}
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

	w := newTestWorld(30)
	addWorker(w)

	if err := Save(w); err != nil {
		t.Errorf("Save with workers (cycle-prone): %v", err)
	}
}
