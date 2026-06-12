package game

// Sim trace: run with  go test -v -run TestSimTrace ./internal/game/...
//
// runSimTrace drives a headless simulation using a SimTraceRunner that supplies
// all planet-specific behaviour — world setup, player AI, columns to display,
// and events to detect. The infrastructure handles the tick loop, per-minute
// snapshots, table formatting, and completion detection.
//
// To add a trace for a new planet type, implement SimTraceRunner and call
// runSimTrace from a new TestSimTrace* function.

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// SimTraceRunner defines the planet-specific behaviour for a sim-trace run.
// Implement one per planet / scenario type and pass it to runSimTrace.
type SimTraceRunner interface {
	// Setup initialises the world for this scenario (place buildings, set flags, etc.).
	// Called once before the loop starts.
	Setup(w *World)

	// ColHeader returns a pre-formatted header string for the scenario-specific
	// columns that follow "time" and "workers" in each row.
	ColHeader() string

	// ColRow returns a pre-formatted value string for those same columns,
	// matched in width to ColHeader so the table stays aligned.
	ColRow(w *World) string

	// PlayerAI is called once per tick before Tick(). It executes player
	// actions (buy, build, press buttons) and returns any events worth logging
	// (e.g. "+camp 2 at 1.05"), or nil.
	PlayerAI(w *World) []string

	// Events is called once per tick after Tick(). It detects world-state
	// changes and returns log events (e.g. "+worker → 3"), or nil.
	Events(w *World) []string

	// Summary returns a completion message logged on the row where the planet
	// finishes. runSimTrace exits when w.System.Unlocked is true.
	Summary(w *World) string
}

// runSimTrace runs the simulation loop and prints a labelled trace table.
// It returns the final World so the caller can log extra summary stats.
func runSimTrace(t *testing.T, desc string, maxMinutes float64, runner SimTraceRunner) *World {
	t.Helper()
	simDuration := maxMinutes * 60.0

	w := NewWorld()
	runner.Setup(w)

	header := fmt.Sprintf("%-5s  %-7s  %s  notes", "time", "workers", runner.ColHeader())
	sep := strings.Repeat("─", len(header)+20)
	if desc != "" {
		t.Log(desc)
	}
	t.Log(sep)
	t.Log(header)
	t.Log(sep)

	logRow := func(simTime float64, note string) {
		mm := int(simTime) / 60
		ss := int(simTime) % 60
		line := fmt.Sprintf("%02d:%02d  %-7d  %s", mm, ss, len(w.Workers), runner.ColRow(w))
		if note != "" {
			line += "  " + note
		}
		t.Log(line)
	}

	logRow(0, "start")

	nextMinute := 60.0

	for simTime := dt; simTime <= simDuration; simTime += dt {
		for _, ev := range runner.PlayerAI(w) {
			logRow(simTime, ev)
		}

		Tick(w, dt)

		for _, ev := range runner.Events(w) {
			logRow(simTime, ev)
		}

		if w.System.Unlocked {
			logRow(simTime, runner.Summary(w))
			break
		}

		if simTime >= nextMinute {
			logRow(simTime, "")
			nextMinute += 60.0
		}
	}

	if !w.System.Unlocked {
		logRow(simDuration, "ceiling reached — planet NOT complete.  "+runner.Summary(w))
	}

	t.Log(sep)
	return w
}

// ── Starting planet ───────────────────────────────────────────────────────────

// TestSimTrace runs the starting (wood) planet with a simple player AI.
// Useful for checking that economy constants produce the intended play-session
// length without needing to run the game manually.
func TestSimTrace(t *testing.T) {
	runner := &startingPlanetRunner{}
	w := runSimTrace(t, "starting planet (wood)", 20, runner)
	fld := fieldForKind(w, KindWood)
	t.Logf("max town slots: %d  |  field cap at end: %.0f  |  trees: %d",
		maxTownSlots(w), fld.Cap, woodTreeCount(w))
}

// startingPlanetRunner implements SimTraceRunner for the single-resource
// starting planet. The player AI buys capacity greedily, places up to 4 camps
// at worker thresholds of 3/5/7/9, and presses Nurture once the attention
// condition is met.
type startingPlanetRunner struct {
	field          *ResourceField
	campTargets    []float64
	prevWorkers    int
	prevTrees      int
	nurturePressed int
	townFullLogged bool
	// first-lesson milestone flags
	houseAffordableLogged bool
	campAffordableLogged  bool
}

func (r *startingPlanetRunner) Setup(w *World) {
	r.field = fieldForKind(w, KindWood)
	if r.field == nil {
		panic("startingPlanetRunner: no wood field in world")
	}
	// Place Town Hall 90° from the forest centre — a typical player placement
	// that gives realistic ~6 s trip times.
	thAngle := normAngle(r.field.CenterAngle + math.Pi/2)
	if !placeBuilding(w, thAngle) {
		panic("startingPlanetRunner: could not place Town Hall")
	}
	w.ResourceDiscovered = true // skip first-delivery gate for trace purposes

	// Four target angles spread ±60° and ±120° from the forest centre.
	r.campTargets = []float64{
		normAngle(r.field.CenterAngle - math.Pi/3),
		normAngle(r.field.CenterAngle + math.Pi/3),
		normAngle(r.field.CenterAngle - 2*math.Pi/3),
		normAngle(r.field.CenterAngle + 2*math.Pi/3),
	}

	r.prevWorkers = len(w.Workers)
	r.prevTrees = woodTreeCount(w)
}

func (r *startingPlanetRunner) ColHeader() string {
	return fmt.Sprintf("%-6s  %-8s  %-14s", "trees", "wood", "field exp/cap")
}

func (r *startingPlanetRunner) ColRow(w *World) string {
	trees := woodTreeCount(w)
	fld := fieldForKind(w, KindWood)
	var expStr string
	if fld != nil {
		expStr = fmt.Sprintf("%.0f/%.0f", fld.EXP, fld.Cap)
	}
	return fmt.Sprintf("%-6d  %-8.0f  %-14s", trees, w.Economy.Wood, expStr)
}

func (r *startingPlanetRunner) PlayerAI(w *World) []string {
	var events []string

	// Buy capacity the moment it becomes affordable.
	if !townFieldFull(w) && w.Economy.Wood >= townCapacityCost(w) {
		buildTownCapacity(w)
	}

	// Place the next camp once the worker threshold and wood cost are met.
	campCount := len(w.Buildings) - 1 // excludes Town Hall
	if campCount < len(r.campTargets) &&
		len(w.Workers) >= campCount*2+3 &&
		w.Economy.Wood >= CampCost(w) {
		if a, ok := r.findCampAngle(w, r.campTargets[campCount]); ok {
			if placeBuilding(w, a) {
				events = append(events, fmt.Sprintf("+camp %d at %.2f", campCount+1, a))
			}
		}
	}

	// Press Nurture as soon as the attention condition lights up.
	if nurtureAttentionActive(w, KindWood) {
		if nurtureField(w, KindWood) {
			r.nurturePressed++
		}
	}

	return events
}

// findCampAngle searches outward from target within ±60° for a valid placement.
func (r *startingPlanetRunner) findCampAngle(w *World, target float64) (float64, bool) {
	const steps = 60
	const sweep = math.Pi / 3
	for i := 0; i < steps; i++ {
		offset := sweep * float64(i) / float64(steps-1)
		for _, sign := range []float64{1, -1} {
			a := normAngle(target + sign*offset)
			if buildPreview(w, a).Valid {
				return a, true
			}
		}
	}
	return 0, false
}

func (r *startingPlanetRunner) Events(w *World) []string {
	var events []string

	if cur := len(w.Workers); cur != r.prevWorkers {
		events = append(events, fmt.Sprintf("+worker → %d (growth %.0f/%.0f)",
			cur, w.Economy.TownGrowth, w.Economy.TownGrowthCap))
		r.prevWorkers = cur
	}

	if cur := woodTreeCount(w); cur != r.prevTrees {
		events = append(events, fmt.Sprintf("+tree → %d", cur))
		r.prevTrees = cur
	}

	// First-lesson milestones: log the moment each button first lights up.
	if !r.houseAffordableLogged && !townFieldFull(w) && w.Economy.Wood >= townCapacityCost(w) {
		events = append(events, fmt.Sprintf("-- house affordable (wood %.0f, growth %.0f/%.0f)",
			w.Economy.Wood, w.Economy.TownGrowth, w.Economy.TownGrowthCap))
		r.houseAffordableLogged = true
	}
	if !r.campAffordableLogged && w.Economy.Wood >= CampCost(w) {
		events = append(events, fmt.Sprintf("-- camp affordable (wood %.0f)", w.Economy.Wood))
		r.campAffordableLogged = true
	}

	if townFieldFull(w) && r.nurturePressed == 0 && !r.townFullLogged {
		events = append(events, "*** town full — Nurture unlocked ***")
		r.townFullLogged = true
	}

	return events
}

func (r *startingPlanetRunner) Summary(w *World) string {
	return fmt.Sprintf("*** PLANET COMPLETE (nurture presses: %d) ***", r.nurturePressed)
}

// ── Shared helpers ────────────────────────────────────────────────────────────

func woodTreeCount(w *World) int {
	n := 0
	for _, node := range w.Nodes {
		if node.Kind == KindWood {
			n++
		}
	}
	return n
}
