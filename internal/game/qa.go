package game

import (
	"fmt"
	"math"
	"math/rand"
)

// QAPreset describes a reproducible mid-game world state for manual QA.
// Pointer fields are optional; nil means "use default or skip".
type QAPreset struct {
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Seed          int64     `json:"seed"`
	Discovered    *bool     `json:"discovered"`
	PlaceTownHall bool      `json:"placeTownHall"`
	TownHallAngle *float64  `json:"townHallAngle"`
	Camps         []float64 `json:"camps"`
	Workers       int       `json:"workers"`
	NoFreeNodes   bool      `json:"noFreeNodes"`
	SettleSeconds float64   `json:"settleSeconds"`
	// FieldCapCycles multiplies the field cap by fieldEXPGrowth N times before
	// setting EXP, simulating N completed growth cycles.
	FieldCapCycles  *int     `json:"fieldCapCycles"`
	FieldExpFromCap *float64 `json:"fieldExpFromCap"` // EXP = Cap + delta (use negative to go below cap)
	FieldExpFrac    *float64 `json:"fieldExpFrac"`    // EXP = frac * Cap
	FieldExpAbs     *float64 `json:"fieldExpAbs"`     // EXP = abs value
	NurtureCharges  *int     `json:"nurtureCharges"`  // pre-arm the field with N level-completing delivery charges
	Wood            *float64 `json:"wood"`
	// Town Growth overrides — applied after workers, before final Wood stamp.
	TownGrowth       *float64 `json:"townGrowth"`
	TownGrowthCap    *float64 `json:"townGrowthCap"`
	WorkerCapacity   *int     `json:"workerCapacity"`
	FillTownCapacity bool     `json:"fillTownCapacity"` // set WorkerCapacity to the geometry max
}

// BuildQAWorld constructs a *World by applying preset overrides on top of NewWorld.
// Overrides are applied in a deterministic order so the final state is reproducible.
func BuildQAWorld(p QAPreset) (*World, error) {
	seed := p.Seed
	if seed == 0 {
		seed = 11
	}
	rand.Seed(seed)

	w := NewWorld()

	// Resource discovery (default true so HUD gauge and Nurture are active).
	if p.Discovered != nil {
		w.ResourceDiscovered = *p.Discovered
	} else {
		w.ResourceDiscovered = true
	}

	// Town Hall — required before buying workers.
	if p.PlaceTownHall {
		if p.TownHallAngle != nil {
			if !placeBuilding(w, *p.TownHallAngle) {
				return nil, fmt.Errorf("failed to place Town Hall at angle %.3f", *p.TownHallAngle)
			}
		} else {
			f := fieldForKind(w, KindWood)
			if f == nil || !placeBuilding(w, f.CenterAngle) {
				return nil, fmt.Errorf("placeTownHall: could not place Town Hall at wood field center")
			}
		}
	}

	// Logging camps.
	for _, angle := range p.Camps {
		if !placeBuilding(w, angle) {
			return nil, fmt.Errorf("failed to place camp at angle %.3f", angle)
		}
	}

	// Workers — spawn without wood cost; Town Hall placement already created one
	// founding worker, so start from the current count.
	if p.Workers > len(w.Workers) {
		if w.Economy.WorkerCapacity < p.Workers {
			w.Economy.WorkerCapacity = p.Workers
		}
		for len(w.Workers) < p.Workers {
			if spawnWorkerAtTownHall(w) == nil {
				return nil, fmt.Errorf("failed to spawn worker %d of %d (no Town Hall)", len(w.Workers)+1, p.Workers)
			}
		}
	}

	// Remove unclaimed nodes before settling so workers find nothing and go idle.
	if p.NoFreeNodes {
		clearFreeNodes(w)
	}

	// Settle: advance the sim so workers reach their steady state.
	for range int(p.SettleSeconds * 60) {
		Step(w, dt)
	}

	// Field EXP — applied after settling to preserve exact values.
	if len(w.Planet.Fields) > 0 {
		f := w.Planet.Fields[0]
		if p.FieldCapCycles != nil {
			for range *p.FieldCapCycles {
				f.Cap *= fieldEXPGrowth
			}
		}
		switch {
		case p.FieldExpFromCap != nil:
			f.EXP = math.Max(0, math.Min(f.Cap, f.Cap+*p.FieldExpFromCap))
		case p.FieldExpFrac != nil:
			f.EXP = math.Max(0, math.Min(f.Cap, f.Cap**p.FieldExpFrac))
		case p.FieldExpAbs != nil:
			f.EXP = math.Max(0, math.Min(f.Cap, *p.FieldExpAbs))
		}
	}

	// Nurture charges — set directly on the field after EXP is finalised.
	if len(w.Planet.Fields) > 0 && p.NurtureCharges != nil {
		w.Planet.Fields[0].NurtureCharges = *p.NurtureCharges
	}

	// Town Growth overrides — applied after workers so capacity is already known.
	if p.FillTownCapacity {
		if max := maxTownSlots(w); max > w.Economy.WorkerCapacity {
			w.Economy.WorkerCapacity = max
		}
	} else if p.WorkerCapacity != nil && *p.WorkerCapacity > w.Economy.WorkerCapacity {
		w.Economy.WorkerCapacity = *p.WorkerCapacity
	}
	if p.TownGrowthCap != nil {
		w.Economy.TownGrowthCap = *p.TownGrowthCap
	}
	if p.TownGrowth != nil {
		// Clamp to the cap (possibly just updated) to honour the no-overflow rule.
		g := math.Min(*p.TownGrowth, w.Economy.TownGrowthCap)
		if g < 0 {
			g = 0
		}
		w.Economy.TownGrowth = g
	}

	// Wood — stamped last so it reflects the intended final balance exactly.
	if p.Wood != nil {
		w.Economy.Wood = *p.Wood
	}

	return w, nil
}

// clearFreeNodes removes nodes that are neither owned nor reserved by a worker.
func clearFreeNodes(w *World) {
	kept := w.Nodes[:0]
	for _, n := range w.Nodes {
		if n.OwnerID != -1 || n.ReservedByWorkerID != -1 {
			kept = append(kept, n)
		}
	}
	w.Nodes = kept
}
