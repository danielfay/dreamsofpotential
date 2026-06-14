package game

import (
	"math"
	"sort"
	"testing"
)

// physicalFields returns only KindWood and KindWater fields, excluding
// KindWaterInfluence which is explicitly allowed to overlap adjacent fields.
func physicalFields(fields []*ResourceField) []*ResourceField {
	var out []*ResourceField
	for _, f := range fields {
		if f.Kind != KindWaterInfluence {
			out = append(out, f)
		}
	}
	return out
}

func TestLakewoodFieldsTileFullCircle(t *testing.T) {
	state := newLakewoodState(Vec{X: 160, Y: 120})
	assertFieldsTileFullCircle(t, "Lakewood", physicalFields(state.Planet.Fields))
}

func TestTightGroveFieldsTileFullCircle(t *testing.T) {
	state := newTightGroveState(Vec{X: 160, Y: 120})
	assertFieldsTileFullCircle(t, "Tight Grove", physicalFields(state.Planet.Fields))
}

func TestWaterFrontierFieldsTileFullCircle(t *testing.T) {
	state := newWaterFrontierState()
	assertFieldsTileFullCircle(t, "Water Frontier", physicalFields(state.Planet.Fields))
}

// assertFieldsTileFullCircle checks two invariants for an authored planet layout:
//  1. The sum of all field arc spans equals 2π (full circle, no missing coverage).
//  2. When sorted by start angle, each field's end aligns exactly with the next
//     field's start (no gap and no overlap between adjacent pairs).
//
// KindWaterInfluence fields must be excluded before calling — they legitimately overlap.
func assertFieldsTileFullCircle(t *testing.T, planet string, fields []*ResourceField) {
	t.Helper()
	if len(fields) == 0 {
		t.Errorf("%s: no physical fields to check", planet)
		return
	}

	// 1. Arc sum must equal 2π.
	total := 0.0
	for _, f := range fields {
		total += 2 * f.HalfArc
	}
	if math.Abs(total-2*math.Pi) > 1e-9 {
		t.Errorf("%s: arc sum = %.9f rad, want 2π = %.9f rad (delta %.9f)",
			planet, total, 2*math.Pi, total-2*math.Pi)
	}

	// 2. Adjacent boundary alignment: sort by start angle, then each field's end
	// must equal the next field's start within floating-point tolerance.
	sorted := make([]*ResourceField, len(fields))
	copy(sorted, fields)
	sort.Slice(sorted, func(i, j int) bool {
		return normAngle(sorted[i].CenterAngle-sorted[i].HalfArc) <
			normAngle(sorted[j].CenterAngle-sorted[j].HalfArc)
	})

	for i, f := range sorted {
		end := normAngle(f.CenterAngle + f.HalfArc)
		next := sorted[(i+1)%len(sorted)]
		nextStart := normAngle(next.CenterAngle - next.HalfArc)
		delta := math.Abs(normAngle(end - nextStart))
		if delta > 1e-9 {
			t.Errorf("%s: field[%d] ends at %.6f rad (%.1f°) but field[%d] starts at %.6f rad (%.1f°); gap/overlap = %.9f rad",
				planet, i, end, end*180/math.Pi,
				(i+1)%len(sorted), nextStart, nextStart*180/math.Pi,
				delta)
		}
	}
}
