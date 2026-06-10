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
	NurtureCharges  *int     `json:"nurtureCharges"`  // pre-arm the field with N boosted-delivery charges
	Wood            *float64 `json:"wood"`
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
		} else if len(w.Nodes) > 0 {
			mustPlaceNearNode(w, w.Nodes[0])
		} else {
			return nil, fmt.Errorf("placeTownHall: no nodes to place near and townHallAngle not set")
		}
	}

	// Logging camps.
	for _, angle := range p.Camps {
		if !placeBuilding(w, angle) {
			return nil, fmt.Errorf("failed to place camp at angle %.3f", angle)
		}
	}

	// Workers — use a large temporary wood balance; the final balance is stamped last.
	if p.Workers > 0 {
		w.Economy.Wood = 1e9
		for i := range p.Workers {
			if !buyWorker(w) {
				return nil, fmt.Errorf("failed to buy worker %d of %d", i+1, p.Workers)
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
