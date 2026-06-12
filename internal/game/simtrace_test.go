package game

// SimTrace: run with  go test -v -run TestSimTrace ./internal/game/...
// Prints a per-minute snapshot table and an event log so you can judge
// whether the pacing hits a target play session length.

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestSimTrace(t *testing.T) {
	const simDuration = 1200.0 // 20 min ceiling — stops early on planet completion
	const ticksPerSecond = 60

	w := NewWorld()

	// Found the world: place Town Hall 90° from the forest centre — a typical
	// player placement, giving realistic ~6s trip times.
	f := fieldForKind(w, KindWood)
	if f == nil {
		t.Fatal("no wood field")
	}
	thAngle := normAngle(f.CenterAngle + math.Pi/2)
	if !placeBuilding(w, thAngle) {
		t.Fatal("could not place Town Hall")
	}
	w.ResourceDiscovered = true // skip first-delivery gate for trace purposes

	header := fmt.Sprintf("%-9s  %-7s  %-6s  %-8s  %-14s  %s",
		"time", "workers", "trees", "wood", "field exp/cap", "notes")
	t.Log(strings.Repeat("─", len(header)))
	t.Log(header)
	t.Log(strings.Repeat("─", len(header)))

	logRow := func(simTime float64, notes string) {
		mm := int(simTime) / 60
		ss := int(simTime) % 60
		trees := woodTreeCount(w)
		fld := fieldForKind(w, KindWood)
		var expStr string
		if fld != nil {
			expStr = fmt.Sprintf("%.0f/%.0f", fld.EXP, fld.Cap)
		}
		t.Logf("%02d:%02d      %-7d  %-6d  %-8.0f  %-14s  %s",
			mm, ss, len(w.Workers), trees, w.Economy.Wood, expStr, notes)
	}

	// Candidate camp angles: spread across the forest at ±60° and ±120° from
	// the forest centre, giving reasonable coverage of where trees will grow.
	campAngles := []float64{
		normAngle(f.CenterAngle - math.Pi/3),
		normAngle(f.CenterAngle + math.Pi/3),
		normAngle(f.CenterAngle - 2*math.Pi/3),
	}

	prevWorkers := len(w.Workers)
	prevTrees := woodTreeCount(w)
	nextMinute := 60.0
	nurturePressed := 0

	logRow(0, "start")

	for simTime := dt; simTime <= simDuration; simTime += dt {
		// ── Player AI ────────────────────────────────────────────────────────
		// Buy capacity the moment it's affordable (optimal player).
		if !townFieldFull(w) && w.Economy.Wood >= townCapacityCost(w) {
			buildTownCapacity(w)
		}
		// Place the next camp once we have enough workers and can afford it.
		// Threshold: 4 workers for the 1st camp, 7 for the 2nd, 10 for the 3rd.
		campCount := len(w.Buildings) - 1 // excludes Town Hall
		if campCount < len(campAngles) &&
			len(w.Workers) >= campCount*3+4 &&
			w.Economy.Wood >= CampCost(w) {
			if placeBuilding(w, campAngles[campCount]) {
				logRow(simTime, fmt.Sprintf("+camp %d at angle %.2f", campCount+1, campAngles[campCount]))
			}
		}
		// Press Nurture as soon as the attention condition is met.
		if nurtureAttentionActive(w, KindWood) {
			if nurtureField(w, KindWood) {
				nurturePressed++
			}
		}

		Tick(w, dt)

		// ── Event detection ───────────────────────────────────────────────────
		curWorkers := len(w.Workers)
		curTrees := woodTreeCount(w)

		if curWorkers != prevWorkers {
			logRow(simTime, fmt.Sprintf("+worker → %d (growth %.0f/%.0f)",
				curWorkers, w.Economy.TownGrowth, w.Economy.TownGrowthCap))
			prevWorkers = curWorkers
		}
		if curTrees != prevTrees {
			logRow(simTime, fmt.Sprintf("+tree → %d", curTrees))
			prevTrees = curTrees
		}
		if townFieldFull(w) && nurturePressed == 0 {
			logRow(simTime, "*** town full — Nurture unlocked ***")
		}
		if w.System.Unlocked {
			logRow(simTime, fmt.Sprintf("*** PLANET COMPLETE (nurture presses: %d) ***", nurturePressed))
			break
		}

		// ── Per-minute snapshot ───────────────────────────────────────────────
		if simTime >= nextMinute {
			logRow(simTime, "")
			nextMinute += 60.0
		}
	}

	if !w.System.Unlocked {
		logRow(simDuration, fmt.Sprintf("ceiling reached — planet NOT complete (nurture presses: %d)", nurturePressed))
	}

	t.Log(strings.Repeat("─", len(header)))
	fld := fieldForKind(w, KindWood)
	t.Logf("max town slots: %d  |  field cap at end: %.0f  |  trees: %d",
		maxTownSlots(w), fld.Cap, woodTreeCount(w))
}

func woodTreeCount(w *World) int {
	n := 0
	for _, node := range w.Nodes {
		if node.Kind == KindWood {
			n++
		}
	}
	return n
}
