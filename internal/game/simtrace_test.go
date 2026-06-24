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

	// Complete reports whether the scenario's terminal condition has been met.
	// runSimTrace exits when this returns true.
	Complete(w *World) bool

	// Summary returns a completion message logged on the row where the planet
	// finishes.
	Summary(w *World) string
}

// runSimTrace runs the simulation loop and prints a labelled trace table.
// It returns the final World so the caller can log extra summary stats.
func runSimTrace(t *testing.T, desc string, maxMinutes float64, runner SimTraceRunner) *World {
	t.Helper()
	simDuration := maxMinutes * 60.0

	w := newWorldWithSeed(42)
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

		if runner.Complete(w) {
			logRow(simTime, runner.Summary(w))
			break
		}

		if simTime >= nextMinute {
			logRow(simTime, "")
			nextMinute += 60.0
		}
	}

	if !runner.Complete(w) {
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
	if testing.Short() {
		t.Skip("sim trace: skipped in short mode")
	}
	runner := &startingPlanetRunner{}
	w := runSimTrace(t, "starting planet (wood)", 20, runner)
	fp := w.Planet.FieldProgress[KindWood]
	t.Logf("max town slots: %d  |  field cap at end: %.0f  |  trees: %d",
		maxTownSlots(w), fp.Cap, woodTreeCount(w))
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
	var expStr string
	if fp := w.Planet.FieldProgress[KindWood]; fp != nil {
		expStr = fmt.Sprintf("%.0f/%.0f", fp.EXP, fp.Cap)
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

func (r *startingPlanetRunner) Complete(w *World) bool { return w.System.Unlocked }

func (r *startingPlanetRunner) Summary(w *World) string {
	return fmt.Sprintf("*** PLANET COMPLETE (nurture presses: %d) ***", r.nurturePressed)
}

// ── Echo completion ───────────────────────────────────────────────────────────

// TestSimTraceEchoCompletion runs an echo planet from fresh entry to completion,
// verifying that the completion gate fires and the amplified rate is snapshotted.
func TestSimTraceEchoCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("sim trace: skipped in short mode")
	}
	const echoIdx = 1
	runner := &echoCompletionRunner{echoIdx: echoIdx}
	w := runSimTrace(t, "echo planet completion (layout 0)", 25, runner)
	if !w.System.Planets[echoIdx].Completed {
		t.Errorf("echo %d: expected Completed=true after trace", echoIdx)
	}
	if w.System.Planets[echoIdx].AbstractRate <= 0 {
		t.Errorf("echo %d: AbstractRate should be > 0 after completion, got %f",
			echoIdx, w.System.Planets[echoIdx].AbstractRate)
	}
	t.Logf("echo %d completed — AbstractRate: %.4f wood/sec", echoIdx, w.System.Planets[echoIdx].AbstractRate)
}

// echoCompletionRunner implements SimTraceRunner for an awakened echo planet.
// Setup establishes the starting planet unlock, awakens echo at echoIdx, enters it.
type echoCompletionRunner struct {
	echoIdx        int
	field          *ResourceField
	campTargets    []float64
	prevWorkers    int
	prevTrees      int
	nurturePressed int
	townFullLogged bool
}

func (r *echoCompletionRunner) Setup(w *World) {
	// Fully set up starting planet to allow triggerUnlock.
	f0 := fieldForKind(w, KindWood)
	if f0 == nil || !placeBuilding(w, f0.CenterAngle) {
		panic("echoCompletionRunner: cannot place starting TH")
	}
	w.Economy.WorkerCapacity = maxTownSlots(w)
	addWorker(w)
	fillWoodFieldNodes(w, false)
	w.ResourceDiscovered = true
	triggerUnlock(w)

	// Awaken the target echo and enter it.
	w.Economy.Potential[PotentialForest] = 1
	awakenPlanet(w, r.echoIdx)
	switchToPlanet(w, r.echoIdx)
	enterPlanetView(w)

	// Find a valid TH angle for the authored echo layout.
	r.field = fieldForKind(w, KindWood)
	if r.field == nil {
		panic("echoCompletionRunner: no wood field on echo")
	}
	thAngle, ok := findValidBuildingAngle(w)
	if !ok || !placeBuilding(w, thAngle) {
		panic("echoCompletionRunner: cannot place echo TH")
	}
	w.ResourceDiscovered = true

	r.campTargets = []float64{
		normAngle(r.field.CenterAngle - math.Pi/3),
		normAngle(r.field.CenterAngle + math.Pi/3),
		normAngle(r.field.CenterAngle - 2*math.Pi/3),
		normAngle(r.field.CenterAngle + 2*math.Pi/3),
	}
	r.prevWorkers = len(w.Workers)
	r.prevTrees = woodTreeCount(w)
}

func (r *echoCompletionRunner) ColHeader() string {
	return fmt.Sprintf("%-6s  %-8s  %-14s", "trees", "wood", "field exp/cap")
}

func (r *echoCompletionRunner) ColRow(w *World) string {
	var expStr string
	if fp := w.Planet.FieldProgress[KindWood]; fp != nil {
		expStr = fmt.Sprintf("%.0f/%.0f", fp.EXP, fp.Cap)
	}
	return fmt.Sprintf("%-6d  %-8.0f  %-14s", woodTreeCount(w), w.Economy.Wood, expStr)
}

func (r *echoCompletionRunner) PlayerAI(w *World) []string {
	var events []string
	if !townFieldFull(w) && w.Economy.Wood >= townCapacityCost(w) {
		buildTownCapacity(w)
	}
	campCount := len(w.Buildings) - 1
	if campCount < len(r.campTargets) &&
		len(w.Workers) >= campCount*2+3 &&
		w.Economy.Wood >= CampCost(w) {
		if a, ok := r.findCampAngle(w, r.campTargets[campCount]); ok {
			if placeBuilding(w, a) {
				events = append(events, fmt.Sprintf("+camp %d at %.2f", campCount+1, a))
			}
		}
	}
	if nurtureAttentionActive(w, KindWood) {
		if nurtureField(w, KindWood) {
			r.nurturePressed++
		}
	}
	return events
}

func (r *echoCompletionRunner) findCampAngle(w *World, target float64) (float64, bool) {
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

func (r *echoCompletionRunner) Events(w *World) []string {
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
	if townFieldFull(w) && !r.townFullLogged {
		events = append(events, "*** town full ***")
		r.townFullLogged = true
	}
	return events
}

func (r *echoCompletionRunner) Complete(w *World) bool {
	return w.System.Planets[r.echoIdx].Completed
}

func (r *echoCompletionRunner) Summary(w *World) string {
	rate := w.System.Planets[r.echoIdx].AbstractRate
	return fmt.Sprintf("*** ECHO COMPLETE (rate %.4f wood/sec, nurture presses: %d) ***",
		rate, r.nurturePressed)
}

// ── Tight Grove completion ────────────────────────────────────────────────────

// TestSimTraceTightGroveCompletion runs Tight Grove (layoutID 1, planet 2)
// from awakening to completion, verifying Forest Potential is awarded and
// Water Potential is not (Tight Grove has no water field).
func TestSimTraceTightGroveCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("sim trace: skipped in short mode")
	}
	const echoIdx = 2
	runner := &echoCompletionRunner{echoIdx: echoIdx}
	w := runSimTrace(t, "Tight Grove completion (layoutID 1)", 30, runner)
	if !w.System.Planets[echoIdx].Completed {
		t.Errorf("Tight Grove: expected Completed=true after trace")
	}
	if got := w.Economy.Potential[PotentialForest]; got != 1 {
		t.Errorf("Forest Potential after Tight Grove: got %d, want 1", got)
	}
	if got := w.Economy.Potential[PotentialWater]; got != 0 {
		t.Errorf("Water Potential after Tight Grove: got %d, want 0 (no water field)", got)
	}
	t.Logf("Tight Grove completed — AbstractRate: %.4f wood/sec", w.System.Planets[echoIdx].AbstractRate)
}

// ── Lakewood completion ───────────────────────────────────────────────────────

// TestSimTraceLakewoodCompletion runs Lakewood (layoutID 0, planet 1) from
// awakening to completion, asserting Water Potential is earned and that workers
// reached island forest nodes via the lake-aware router.
func TestSimTraceLakewoodCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("sim trace: skipped in short mode")
	}
	const echoIdx = 1
	runner := &echoCompletionRunner{echoIdx: echoIdx}
	w := runSimTrace(t, "Lakewood completion (layoutID 0)", 45, runner)
	if !w.System.Planets[echoIdx].Completed {
		t.Errorf("Lakewood: expected Completed=true after trace")
	}
	if got := w.Economy.Potential[PotentialWater]; got != 1 {
		t.Errorf("Water Potential after Lakewood: got %d, want 1", got)
	}
	// Verify lake-aware routing enabled workers to reach the island forest.
	var islandField *ResourceField
	for _, f := range w.Planet.Fields {
		if f.Kind == KindWood && math.Abs(normAngle(f.CenterAngle-lakewoodIslandForestAngle)) < 0.01 {
			islandField = f
			break
		}
	}
	if islandField == nil {
		t.Fatal("island field not found on Lakewood after completion")
	}
	islandCount := 0
	for _, n := range w.Nodes {
		if n.Kind == KindWood && angleWithinField(islandField, n.Angle) {
			islandCount++
		}
	}
	if islandCount == 0 {
		t.Error("no island forest nodes found — lake-aware routing may be broken")
	}
	t.Logf("Lakewood completed — island nodes: %d, AbstractRate: %.4f wood/sec",
		islandCount, w.System.Planets[echoIdx].AbstractRate)
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

func waterSparkleCount(w *World) int {
	n := 0
	for _, node := range w.Nodes {
		if node.Interior && node.Kind == KindWater {
			n++
		}
	}
	return n
}

// ── Water frontier first delivery ────────────────────────────────────────────

// TestSimTraceWaterFirstDelivery runs the water frontier from awakening through
// the first water delivery, confirming that WaterDiscovered is set.
func TestSimTraceWaterFirstDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("sim trace: skipped in short mode")
	}
	runner := &waterFrontierRunner{}
	w := runSimTrace(t, "water frontier — first delivery", 30, runner)
	if !w.Economy.WaterDiscovered {
		t.Error("WaterDiscovered should be true after sim trace completes")
	}
	t.Logf("water earned: %.2f  |  sparkles: %d", w.Economy.Water, waterSparkleCount(w))
}

type waterFrontierRunner struct {
	prevWorkers     int
	prevSparkles    int
	dockPlaced      bool
	waterDiscovered bool
}

func (r *waterFrontierRunner) Setup(w *World) {
	f0 := fieldForKind(w, KindWood)
	if f0 == nil || !placeBuilding(w, f0.CenterAngle) {
		panic("waterFrontierRunner: cannot place starting TH")
	}
	w.Economy.WorkerCapacity = maxTownSlots(w)
	addWorker(w)
	fillWoodFieldNodes(w, false)
	w.ResourceDiscovered = true
	triggerUnlock(w)

	w.Economy.Potential[PotentialForest] = 1
	w.Economy.Potential[PotentialWater] = 1
	awakenPlanet(w, 3)
	switchToPlanet(w, 3)
	enterPlanetView(w)

	if !placeBuilding(w, waterFrontierShoreAngle) {
		panic("waterFrontierRunner: cannot place frontier TH")
	}
	w.ResourceDiscovered = true
	w.Economy.Wood = 10000
	r.prevWorkers = len(w.Workers)
	r.prevSparkles = waterSparkleCount(w)
}

func (r *waterFrontierRunner) ColHeader() string {
	return fmt.Sprintf("%-8s  %-8s  %-6s", "wood", "water", "sparkle")
}

func (r *waterFrontierRunner) ColRow(w *World) string {
	return fmt.Sprintf("%-8.0f  %-8.2f  %-6d", w.Economy.Wood, w.Economy.Water, waterSparkleCount(w))
}

func (r *waterFrontierRunner) PlayerAI(w *World) []string {
	var events []string
	if !townFieldFull(w) && w.Economy.Wood >= townCapacityCost(w) {
		buildTownCapacity(w)
	}
	if !r.dockPlaced {
		hasDock := false
		for _, b := range w.Buildings {
			if b.Kind == KindDock {
				hasDock = true
				break
			}
		}
		if !hasDock && w.Economy.Wood >= dockShoreCost {
			angle := shoreEdgeAngle()
			if placeBuildingWithFreePlacement(w, angle, false) {
				r.dockPlaced = true
				events = append(events, fmt.Sprintf("+shore dock at %.2f", angle))
			}
		}
	}
	return events
}

func (r *waterFrontierRunner) Events(w *World) []string {
	var events []string
	if cur := len(w.Workers); cur != r.prevWorkers {
		events = append(events, fmt.Sprintf("+worker → %d", cur))
		r.prevWorkers = cur
	}
	if cur := waterSparkleCount(w); cur != r.prevSparkles {
		events = append(events, fmt.Sprintf("+sparkle → %d", cur))
		r.prevSparkles = cur
	}
	if !r.waterDiscovered && w.Economy.WaterDiscovered {
		events = append(events, fmt.Sprintf("*** WATER DISCOVERED (water=%.2f) ***", w.Economy.Water))
		r.waterDiscovered = true
	}
	return events
}

func (r *waterFrontierRunner) Complete(w *World) bool {
	return w.Economy.WaterDiscovered
}

func (r *waterFrontierRunner) Summary(w *World) string {
	return fmt.Sprintf("water=%.2f sparkles=%d", w.Economy.Water, waterSparkleCount(w))
}
