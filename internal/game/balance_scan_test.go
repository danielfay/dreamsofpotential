package game

// Balance scan: headless scenario runner for tuning questions.
//
// Architecture:
//   - BotAI interface        — player decision logic (swappable)
//   - DefaultBot             — the standard player AI
//   - balanceScanRunner      — SimTraceRunner that wraps any BotAI + collects metrics
//   - runForestBalanceScan   — shared helper: runs camp-cap variants and writes log
//
// Scenarios live in balance_scan_scenarios_test.go.
//
// Run all scenarios:
//   go test -v -run TestSimTraceBalanceScan ./internal/game/
// Run a specific scenario:
//   go test -v -run TestSimTraceBalanceScanTightGrove ./internal/game/

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

// ── BotAI interface ───────────────────────────────────────────────────────────

// BotAI encapsulates player decision-making for a sim scenario.
// Implement a new BotAI to model a different play style without changing
// the scenario setup or the metrics collected.
type BotAI interface {
	// Act is called once per tick before Tick(). It executes player purchases
	// and returns events to log (e.g. "+house→cap3", "+camp1@1.57rad").
	Act(w *World) []string
}

// ── DefaultBot ────────────────────────────────────────────────────────────────

// DefaultBot is the standard player AI for wood-planet scenarios.
//
// Priority each tick:
//  1. Buy a house immediately when affordable and at worker cap.
//  2. Hold (save wood) when supply-blocked on a house AND growth bar > 50%
//     (a worker is incoming soon — don't drain wood on camps).
//  3. Otherwise buy a logging camp (coverage-first placement) until CampCap.
//  4. Nurture whenever the attention condition fires.
type DefaultBot struct {
	CampCap int

	// Metrics — read after the run via printMetrics.
	CampsPlaced  int
	SpaceBlocked bool
	Blocks       []blockPeriod // house supply-block durations
	HoldPeriods  int

	field      *ResourceField
	inBlock    bool
	blockStart float64
	inHold     bool
}

type blockPeriod struct {
	startTime float64
	duration  float64
}

// metricsSnapshot is a flat capture of all balance-scan metrics from one run.
// Collected after each sub-run and passed to writeBalanceScanLog for file output.
type metricsSnapshot struct {
	label        string
	campCap      int
	campsPlaced  int
	dockCap      int
	docksPlaced  int
	spaceBlocked bool
	holdPeriods  int
	blocks       []blockPeriod

	startingWood  float64
	startingWater float64

	firstCampSeen bool
	firstCampTime float64
	firstDockSeen bool
	firstDockTime float64

	workerEvents [][2]float64
	endSimTime   float64

	popMaxed      bool
	popMaxTime    float64
	treesAtPopMax int
	woodAtPopMax  float64
	waterAtPopMax float64
	endWood       float64
	endWater      float64

	stallDetected bool
	stallTime     float64

	dockUpgradeTimes []float64
}

func (b *DefaultBot) Act(w *World) []string {
	if b.field == nil {
		b.field = fieldForKind(w, KindWood)
	}

	var events []string

	atCap := len(w.Workers) >= w.Economy.WorkerCapacity
	popFull := townFieldFull(w)
	canAffordHouse := !popFull && w.Economy.Wood >= townCapacityCost(w)

	growthFrac := 0.0
	if w.Economy.TownGrowthCap > 0 {
		growthFrac = w.Economy.TownGrowth / w.Economy.TownGrowthCap
	}

	// Priority 1: buy house immediately if at worker cap and affordable.
	if atCap && canAffordHouse {
		b.endBlock(w.SimTime)
		buildTownCapacity(w)
		events = append(events, fmt.Sprintf("+house→cap%d (wood=%.0f)", w.Economy.WorkerCapacity, w.Economy.Wood))
		return events
	}

	// Track supply-block: at cap, pop not maxed, can't yet afford house.
	supplyBlocked := atCap && !popFull && !canAffordHouse
	if supplyBlocked && !b.inBlock {
		b.inBlock = true
		b.blockStart = w.SimTime
	} else if !supplyBlocked {
		b.endBlock(w.SimTime)
	}

	// Priority 2: hold when supply-blocked AND growth bar > 50%.
	holdForHouse := supplyBlocked && growthFrac > 0.5
	if holdForHouse && !b.inHold {
		b.HoldPeriods++
		b.inHold = true
	} else if !holdForHouse {
		b.inHold = false
	}

	// Priority 3: buy camp if not holding and below cap.
	if !holdForHouse && b.CampsPlaced < b.CampCap && !b.SpaceBlocked {
		if w.Economy.Wood >= CampCost(w) {
			a, ok := b.bestCampAngle(w)
			if !ok {
				b.SpaceBlocked = true
				events = append(events, fmt.Sprintf("space-blocked after %d camps", b.CampsPlaced))
			} else if placeBuilding(w, a) {
				b.CampsPlaced++
				events = append(events, fmt.Sprintf("+camp%d@%.2frad (wood=%.0f)", b.CampsPlaced, a, w.Economy.Wood))
			}
		}
	}

	// Nurture whenever the attention condition fires.
	if nurtureAttentionActive(w) {
		nurtureField(w)
	}

	return events
}

func (b *DefaultBot) endBlock(simTime float64) {
	if b.inBlock {
		b.Blocks = append(b.Blocks, blockPeriod{b.blockStart, simTime - b.blockStart})
		b.inBlock = false
	}
}

// bestCampAngle returns the valid rim angle that maximises coverage:
//   - first camp: closest valid angle to the wood field center
//   - subsequent camps: valid angle with the greatest min-distance to any existing camp
func (b *DefaultBot) bestCampAngle(w *World) (float64, bool) {
	const steps = 360

	hasCamps := false
	for _, bld := range w.Buildings {
		if bld.Kind == KindLoggingCamp {
			hasCamps = true
			break
		}
	}

	bestAngle := 0.0
	bestScore := -math.MaxFloat64
	found := false

	for i := 0; i < steps; i++ {
		a := float64(i) * 2 * math.Pi / float64(steps)
		if !buildPreview(w, a).Valid {
			continue
		}

		var score float64
		if !hasCamps {
			// First camp: prefer the wood field center.
			score = -angularDistance(a, b.field.CenterAngle)
		} else {
			// Coverage: maximise minimum distance to any existing camp.
			score = math.Pi
			for _, bld := range w.Buildings {
				if bld.Kind != KindLoggingCamp {
					continue
				}
				if d := angularDistance(a, bld.Angle); d < score {
					score = d
				}
			}
		}

		if score > bestScore {
			bestScore = score
			bestAngle = a
			found = true
		}
	}
	return bestAngle, found
}

// ── balanceScanRunner ─────────────────────────────────────────────────────────

// balanceScanRunner implements SimTraceRunner for the starting wood planet
// with a configurable BotAI. It collects balance metrics: supply-block
// durations, time at each worker count, and steady-state wood rate after
// population maxes.
type balanceScanRunner struct {
	bot      BotAI
	field    *ResourceField
	preSetup func(w *World) // called in Setup before world detection; use for planet switching

	prevWorkers int

	// worker spawn timeline: [[simTime, workerCount], ...]
	workerEvents [][2]float64

	// first camp
	firstCampTime float64
	firstCampSeen bool

	// pop-max tracking
	popMaxed      bool
	popMaxTime    float64
	woodAtPopMax  float64
	treesAtPopMax int

	// economy stall detection (post pop-max)
	prevTickWood   float64
	lastWoodUpdate float64
	stallTime      float64
	stallDetected  bool

	// starting resources (captured at end of Setup, before any ticks)
	startingWood  float64
	startingWater float64

	// end-of-run snapshot (updated every tick so printMetrics can read it)
	endWood    float64
	endSimTime float64

	// first dock
	firstDockSeen bool
	firstDockTime float64

	// water economy
	waterAtPopMax float64
	endWater      float64

	// maxScanTime caps the run if pop-max is never reached (0 = no cap).
	maxScanTime float64
}

func newBalanceScanRunner(bot BotAI) *balanceScanRunner {
	return &balanceScanRunner{bot: bot}
}

func (r *balanceScanRunner) Setup(w *World) {
	if r.preSetup != nil {
		r.preSetup(w)
	}
	r.field = fieldForKind(w, KindWood)
	if r.field == nil {
		panic("balanceScanRunner: no wood field")
	}
	// Prefer TH 90° from forest centre; fall back to first valid angle for
	// planets where that position is blocked (lake arc, terrain, etc.).
	thAngle := normAngle(r.field.CenterAngle + math.Pi/2)
	if !buildPreview(w, thAngle).Valid {
		var ok bool
		thAngle, ok = findValidBuildingAngle(w)
		if !ok {
			panic("balanceScanRunner: no valid TH angle")
		}
	}
	if !placeBuilding(w, thAngle) {
		panic("balanceScanRunner: could not place Town Hall")
	}
	w.ResourceDiscovered = true
	r.startingWood = w.Economy.Wood
	r.startingWater = w.Economy.Water
	r.prevWorkers = len(w.Workers)
	r.workerEvents = [][2]float64{{0, float64(len(w.Workers))}}
}

func (r *balanceScanRunner) ColHeader() string {
	return fmt.Sprintf("%-6s  %-8s  %-7s  %-5s", "trees", "wood", "growth%", "camps")
}

func (r *balanceScanRunner) ColRow(w *World) string {
	trees := 0
	for _, n := range w.Nodes {
		if n.Kind == KindWood {
			trees++
		}
	}
	pct := 0.0
	if w.Economy.TownGrowthCap > 0 {
		pct = 100 * w.Economy.TownGrowth / w.Economy.TownGrowthCap
	}
	camps := 0
	for _, b := range w.Buildings {
		if b.Kind == KindLoggingCamp {
			camps++
		}
	}
	return fmt.Sprintf("%-6d  %-8.0f  %-7.0f  %-5d", trees, w.Economy.Wood, pct, camps)
}

func (r *balanceScanRunner) PlayerAI(w *World) []string {
	return r.bot.Act(w)
}

func (r *balanceScanRunner) Events(w *World) []string {
	var events []string

	if !r.firstCampSeen {
		for _, b := range w.Buildings {
			if b.Kind == KindLoggingCamp {
				r.firstCampTime = w.SimTime
				r.firstCampSeen = true
				break
			}
		}
	}

	if !r.firstDockSeen {
		for _, b := range w.Buildings {
			if b.Kind == KindDock {
				r.firstDockTime = w.SimTime
				r.firstDockSeen = true
				break
			}
		}
	}

	if cur := len(w.Workers); cur != r.prevWorkers {
		events = append(events, fmt.Sprintf("+worker→%d (growth=%.0f/%.0f)",
			cur, w.Economy.TownGrowth, w.Economy.TownGrowthCap))
		r.workerEvents = append(r.workerEvents, [2]float64{w.SimTime, float64(cur)})
		r.prevWorkers = cur
	}

	if !r.popMaxed && townFieldFull(w) {
		r.popMaxed = true
		r.popMaxTime = w.SimTime
		r.woodAtPopMax = w.Economy.Wood
		r.waterAtPopMax = w.Economy.Water
		r.treesAtPopMax = woodTreeCount(w)
		r.lastWoodUpdate = w.SimTime
		r.prevTickWood = w.Economy.Wood
		events = append(events, fmt.Sprintf("*** pop maxed — %d workers ***", len(w.Workers)))
	}

	if r.popMaxed && !r.stallDetected {
		if w.Economy.Wood != r.prevTickWood {
			r.lastWoodUpdate = w.SimTime
		}
		r.prevTickWood = w.Economy.Wood
		if w.SimTime-r.lastWoodUpdate >= 30 {
			r.stallDetected = true
			r.stallTime = r.lastWoodUpdate
		}
	}

	r.endWood = w.Economy.Wood
	r.endWater = w.Economy.Water
	r.endSimTime = w.SimTime

	return events
}

func (r *balanceScanRunner) Complete(w *World) bool {
	if r.maxScanTime > 0 && w.SimTime >= r.maxScanTime {
		return true
	}
	return r.popMaxed && w.SimTime >= r.popMaxTime+120
}

func (r *balanceScanRunner) Summary(w *World) string {
	rate := 0.0
	if elapsed := w.SimTime - r.popMaxTime; elapsed > 0 {
		rate = (w.Economy.Wood - r.woodAtPopMax) / elapsed
	}
	return fmt.Sprintf("steady-state wood rate ~%.2f/s", rate)
}

// printMetrics logs a formatted balance summary after a run.
func (r *balanceScanRunner) printMetrics(t *testing.T) {
	t.Helper()
	t.Log("─── metrics ────────────────────────────────────────────────")
	t.Logf("starting:      wood=%.0f  water=%.0f", r.startingWood, r.startingWater)

	if r.firstCampSeen {
		t.Logf("first camp:    t=%.0fs", r.firstCampTime)
	} else {
		t.Log("first camp:    never placed")
	}

	if db, ok := r.bot.(*DefaultBot); ok {
		label := ""
		if db.SpaceBlocked {
			label = "  (space-blocked)"
		}
		t.Logf("camps:         %d / %d%s", db.CampsPlaced, db.CampCap, label)
		t.Logf("hold events:   %d periods", db.HoldPeriods)

		if len(db.Blocks) == 0 {
			t.Log("supply blocks: none")
		} else {
			total, longest := 0.0, blockPeriod{}
			for _, blk := range db.Blocks {
				total += blk.duration
				if blk.duration > longest.duration {
					longest = blk
				}
			}
			t.Logf("supply blocks: %d periods  total=%.1fs  longest=%.1fs (t=%.0fs)",
				len(db.Blocks), total, longest.duration, longest.startTime)
		}
	}

	t.Log("worker timeline:")
	for i, ev := range r.workerEvents {
		endTime := r.endSimTime
		if i+1 < len(r.workerEvents) {
			endTime = r.workerEvents[i+1][0]
		}
		dur := endTime - ev[0]
		if dur < 0 {
			dur = 0
		}
		t.Logf("  %2.0f workers: %5.0fs", ev[1], dur)
	}

	if r.popMaxed {
		t.Logf("pop-max at:    t=%.0fs  trees=%d  wood=%.0f", r.popMaxTime, r.treesAtPopMax, r.woodAtPopMax)
		if elapsed := r.endSimTime - r.popMaxTime; elapsed > 0 {
			rate := (r.endWood - r.woodAtPopMax) / elapsed
			t.Logf("steady rate:   ~%.2f wood/s (over %.0fs)", rate, elapsed)
		}
		if r.stallDetected {
			t.Logf("economy stall: t=%.0fs (%.0fs after pop-max)", r.stallTime, r.stallTime-r.popMaxTime)
		} else {
			t.Log("economy stall: none detected in sim window")
		}
	} else {
		t.Log("pop-max at:    never reached in sim window")
	}
}

// snapshot captures all current metrics into a flat struct for file logging.
func (r *balanceScanRunner) snapshot(label string) metricsSnapshot {
	s := metricsSnapshot{
		label:         label,
		startingWood:  r.startingWood,
		startingWater: r.startingWater,
		firstCampSeen: r.firstCampSeen,
		firstCampTime: r.firstCampTime,
		firstDockSeen: r.firstDockSeen,
		firstDockTime: r.firstDockTime,
		workerEvents:  r.workerEvents,
		endSimTime:    r.endSimTime,
		popMaxed:      r.popMaxed,
		popMaxTime:    r.popMaxTime,
		treesAtPopMax: r.treesAtPopMax,
		woodAtPopMax:  r.woodAtPopMax,
		waterAtPopMax: r.waterAtPopMax,
		endWood:       r.endWood,
		endWater:      r.endWater,
		stallDetected: r.stallDetected,
		stallTime:     r.stallTime,
	}
	if db, ok := r.bot.(*DefaultBot); ok {
		s.campCap = db.CampCap
		s.campsPlaced = db.CampsPlaced
		s.spaceBlocked = db.SpaceBlocked
		s.holdPeriods = db.HoldPeriods
		s.blocks = append([]blockPeriod{}, db.Blocks...)
	}
	if wfb, ok := r.bot.(*WaterFrontierBot); ok {
		s.campCap = wfb.CampCap
		s.dockCap = wfb.DockCap
		s.campsPlaced = wfb.CampsPlaced
		s.docksPlaced = wfb.DocksPlaced
		s.spaceBlocked = wfb.SpaceBlocked
		s.holdPeriods = wfb.HoldPeriods
		s.blocks = append([]blockPeriod{}, wfb.Blocks...)
		s.dockUpgradeTimes = append([]float64{}, wfb.DockUpgrades...)
	}
	return s
}

// writeBalanceScanLog writes a formatted comparison table of all snapshots to
// logs/balance-scan-<scenario>.txt (overwrites on each run).
func writeBalanceScanLog(scenario string, snaps []metricsSnapshot) error {
	// go test cwd is the package directory (internal/game/); step up to project root.
	logsDir := "../../logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return err
	}
	slug := strings.NewReplacer(" ", "-", "/", "-", "—", "-").Replace(scenario)
	path := fmt.Sprintf("%s/balance-scan-%s.txt", logsDir, slug)

	var b strings.Builder

	fmt.Fprintf(&b, "balance scan — %s — %s\n", scenario, time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintln(&b, strings.Repeat("═", 72))
	fmt.Fprintln(&b)

	if len(snaps) > 0 {
		s0 := snaps[0]
		fmt.Fprintf(&b, "starting resources: wood=%.0f  water=%.0f\n\n", s0.startingWood, s0.startingWater)
	}

	// ── summary table ────────────────────────────────────────────────────────
	const hdr = "%-5s  %-10s  %-8s  %-9s  %-8s  %-11s  %-12s  %-16s  %s"
	fmt.Fprintf(&b, hdr+"\n", "cap", "first-camp", "pop-max", "trees@max", "wood@max", "steady-rate", "stall", "supply-blocks", "holds")
	fmt.Fprintln(&b, strings.Repeat("─", 98))
	for _, s := range snaps {
		firstCamp := "–"
		if s.firstCampSeen {
			firstCamp = fmt.Sprintf("t=%.0fs", s.firstCampTime)
		}
		popMax, treesStr, woodStr, rateStr := "–", "–", "–", "–"
		if s.popMaxed {
			popMax = fmt.Sprintf("t=%.0fs", s.popMaxTime)
			treesStr = fmt.Sprintf("%d", s.treesAtPopMax)
			woodStr = fmt.Sprintf("%.0f", s.woodAtPopMax)
			if elapsed := s.endSimTime - s.popMaxTime; elapsed > 0 {
				rate := (s.endWood - s.woodAtPopMax) / elapsed
				rateStr = fmt.Sprintf("%.2f/s", rate)
			}
		}
		stallStr := "–"
		if s.stallDetected {
			stallStr = fmt.Sprintf("t=%.0fs (+%.0fs)", s.stallTime, s.stallTime-s.popMaxTime)
		}
		blocksStr := "none"
		if len(s.blocks) > 0 {
			total := 0.0
			for _, blk := range s.blocks {
				total += blk.duration
			}
			label := ""
			if s.spaceBlocked {
				label = " space-blocked"
			}
			blocksStr = fmt.Sprintf("%d (%.0fs total)%s", len(s.blocks), total, label)
		}
		fmt.Fprintf(&b, "%-5d  %-10s  %-8s  %-9s  %-8s  %-11s  %-12s  %-16s  %d\n",
			s.campCap, firstCamp, popMax, treesStr, woodStr, rateStr, stallStr, blocksStr, s.holdPeriods)
	}
	fmt.Fprintln(&b)

	// ── worker timeline ──────────────────────────────────────────────────────
	if len(snaps) > 0 {
		maxCols := 0
		for _, s := range snaps {
			if n := len(s.workerEvents); n > maxCols {
				maxCols = n
			}
		}
		fmt.Fprintln(&b, "worker timeline (seconds at each count):")
		// Header: worker counts from the first snapshot.
		fmt.Fprintf(&b, "%-7s", "count")
		for i := 0; i < maxCols; i++ {
			wc := 0
			for _, s := range snaps {
				if i < len(s.workerEvents) {
					wc = int(s.workerEvents[i][1])
					break
				}
			}
			fmt.Fprintf(&b, " %4d", wc)
		}
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, strings.Repeat("─", 7+maxCols*5))
		for _, s := range snaps {
			fmt.Fprintf(&b, "cap=%-3d", s.campCap)
			for i, ev := range s.workerEvents {
				end := s.endSimTime
				if i+1 < len(s.workerEvents) {
					end = s.workerEvents[i+1][0]
				}
				dur := end - ev[0]
				if dur < 0 {
					dur = 0
				}
				fmt.Fprintf(&b, " %4.0f", dur)
			}
			fmt.Fprintln(&b)
		}
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}

// ── Shared runner ─────────────────────────────────────────────────────────────

// runForestBalanceScan runs campCaps variants through the balance scan and
// writes a comparison table to logs/balance-scan-<scenario>.txt.
// preSetup, if non-nil, is stored on each runner and called in Setup() before
// world detection — use it to bootstrap prerequisites and switch planets.
func runForestBalanceScan(t *testing.T, scenario string, campCaps []int, preSetup func(w *World)) {
	t.Helper()
	var snaps []metricsSnapshot
	for _, campCap := range campCaps {
		campCap := campCap
		t.Run(fmt.Sprintf("camps=%d", campCap), func(t *testing.T) {
			bot := &DefaultBot{CampCap: campCap}
			runner := newBalanceScanRunner(bot)
			runner.preSetup = preSetup
			runSimTrace(t, fmt.Sprintf("%s — %d-camp cap", scenario, campCap), 25, runner)
			runner.printMetrics(t)
			snaps = append(snaps, runner.snapshot(fmt.Sprintf("camps=%d", campCap)))
		})
	}
	if err := writeBalanceScanLog(scenario, snaps); err != nil {
		t.Logf("balance scan: failed to write log: %v", err)
	} else {
		t.Logf("balance scan: metrics written to logs/balance-scan-%s.txt", scenario)
	}
}

// ── WaterFrontierBot ──────────────────────────────────────────────────────────

// DockStrategy controls the order in which the bot places and upgrades docks.
type DockStrategy int

const (
	// DockStrategyBatch places all docks up to DockCap (respecting the 1-worker-per-dock
	// ratio), then upgrades each to Level 2 once all are placed.
	DockStrategyBatch DockStrategy = iota
	// DockStrategySequential upgrades each dock to Level 2 before placing the next,
	// so every dock earns at L2 from the moment the next one goes down.
	DockStrategySequential
)

// WaterFrontierBot is the player AI for the water-frontier planet.
//
// Priority each tick:
//  1. Buy a house immediately when affordable and at worker cap.
//  2. Hold (save wood) when supply-blocked on a house AND growth bar > 50%.
//  3. Dock + upgrade logic (strategy-dependent; respects 1-worker-per-dock ratio).
//  4. Buy a logging camp until CampCap.
//  5. Set LaborFocus: 1 water worker per placed dock, always ≥1 on wood.
//  6. Nurture whenever the attention condition fires.
type WaterFrontierBot struct {
	CampCap   int
	DockCap   int
	DockStrat DockStrategy

	CampsPlaced  int
	DocksPlaced  int
	SpaceBlocked bool
	Blocks       []blockPeriod
	HoldPeriods  int
	DockUpgrades []float64 // simTime of each dock reaching Level 2

	field      *ResourceField
	inBlock    bool
	blockStart float64
	inHold     bool
}

func (b *WaterFrontierBot) Act(w *World) []string {
	if b.field == nil {
		b.field = fieldForKind(w, KindWood)
	}

	var events []string

	atCap := len(w.Workers) >= w.Economy.WorkerCapacity
	popFull := townFieldFull(w)
	canAffordHouse := !popFull && w.Economy.Wood >= townCapacityCost(w)

	growthFrac := 0.0
	if w.Economy.TownGrowthCap > 0 {
		growthFrac = w.Economy.TownGrowth / w.Economy.TownGrowthCap
	}

	if atCap && canAffordHouse {
		b.endBlock(w.SimTime)
		buildTownCapacity(w)
		events = append(events, fmt.Sprintf("+house→cap%d (wood=%.0f)", w.Economy.WorkerCapacity, w.Economy.Wood))
		return events
	}

	supplyBlocked := atCap && !popFull && !canAffordHouse
	if supplyBlocked && !b.inBlock {
		b.inBlock = true
		b.blockStart = w.SimTime
	} else if !supplyBlocked {
		b.endBlock(w.SimTime)
	}

	holdForHouse := supplyBlocked && growthFrac > 0.5
	if holdForHouse && !b.inHold {
		b.HoldPeriods++
		b.inHold = true
	} else if !holdForHouse {
		b.inHold = false
	}

	if !holdForHouse {
		switch b.DockStrat {
		case DockStrategySequential:
			// Find the oldest L1 dock to upgrade before placing the next.
			var l1Dock *Building
			for _, bld := range w.Buildings {
				if bld.Kind == KindDock && bld.Level < 2 {
					l1Dock = bld
					break
				}
			}
			if l1Dock != nil {
				if upgradeDock(w, l1Dock) {
					b.DockUpgrades = append(b.DockUpgrades, w.SimTime)
					events = append(events, fmt.Sprintf("+dock-L2 #%d (wood=%.0f water=%.0f)", len(b.DockUpgrades), w.Economy.Wood, w.Economy.Water))
				}
			} else if len(w.Workers) > b.DocksPlaced && b.DocksPlaced < b.DockCap {
				// All existing docks are L2; place next when worker ratio allows.
				if a, ok := b.bestDockAngle(w); ok {
					if placeBuilding(w, a) {
						b.DocksPlaced++
						events = append(events, fmt.Sprintf("+dock%d@%.2frad (wood=%.0f water=%.0f)", b.DocksPlaced, a, w.Economy.Wood, w.Economy.Water))
					}
				}
			}

		case DockStrategyBatch:
			// Place all docks first (worker-ratio gated), then upgrade.
			if b.DocksPlaced < b.DockCap && len(w.Workers) > b.DocksPlaced {
				if a, ok := b.bestDockAngle(w); ok {
					if placeBuilding(w, a) {
						b.DocksPlaced++
						events = append(events, fmt.Sprintf("+dock%d@%.2frad (wood=%.0f water=%.0f)", b.DocksPlaced, a, w.Economy.Wood, w.Economy.Water))
					}
				}
			} else if b.DocksPlaced >= b.DockCap {
				for _, bld := range w.Buildings {
					if bld.Kind == KindDock && bld.Level < 2 {
						if upgradeDock(w, bld) {
							b.DockUpgrades = append(b.DockUpgrades, w.SimTime)
							events = append(events, fmt.Sprintf("+dock-L2 #%d (wood=%.0f water=%.0f)", len(b.DockUpgrades), w.Economy.Wood, w.Economy.Water))
						}
						break
					}
				}
			}
		}

		if b.CampsPlaced < b.CampCap && !b.SpaceBlocked {
			if w.Economy.Wood >= CampCost(w) {
				a, ok := b.bestCampAngle(w)
				if !ok {
					b.SpaceBlocked = true
					events = append(events, fmt.Sprintf("space-blocked after %d camps", b.CampsPlaced))
				} else if placeBuilding(w, a) {
					b.CampsPlaced++
					events = append(events, fmt.Sprintf("+camp%d@%.2frad (wood=%.0f)", b.CampsPlaced, a, w.Economy.Wood))
				}
			}
		}
	}

	// Maintain worker assignment: 1 water worker per dock, always ≥1 on wood
	// so TownGrowth keeps filling and the player never soft-locks on houses.
	if total := len(w.Workers); total > 0 {
		waterWorkers := min(b.DocksPlaced, total-1)
		w.LaborFocus = laborFocusMap(total-waterWorkers, waterWorkers)
	}

	if nurtureAttentionActive(w) {
		nurtureField(w)
	}

	return events
}

func (b *WaterFrontierBot) endBlock(simTime float64) {
	if b.inBlock {
		b.Blocks = append(b.Blocks, blockPeriod{b.blockStart, simTime - b.blockStart})
		b.inBlock = false
	}
}

// bestDockAngle returns the best valid angle for a dock: first dock goes near the
// shore boundary; subsequent docks maximise coverage (max min-distance to existing docks).
func (b *WaterFrontierBot) bestDockAngle(w *World) (float64, bool) {
	const steps = 360
	hasDocks := false
	for _, bld := range w.Buildings {
		if bld.Kind == KindDock {
			hasDocks = true
			break
		}
	}

	bestAngle := 0.0
	bestScore := -math.MaxFloat64
	found := false

	for i := 0; i < steps; i++ {
		a := float64(i) * 2 * math.Pi / float64(steps)
		pv := buildPreview(w, a)
		if pv.Kind != KindDock || !pv.Valid {
			continue
		}

		var score float64
		if !hasDocks {
			boundary := normAngle(waterFrontierShoreAngle + waterFrontierShoreArc)
			score = -angularDistance(a, boundary)
		} else {
			score = math.Pi
			for _, bld := range w.Buildings {
				if bld.Kind != KindDock {
					continue
				}
				if d := angularDistance(a, bld.Angle); d < score {
					score = d
				}
			}
		}

		if score > bestScore {
			bestScore = score
			bestAngle = a
			found = true
		}
	}
	return bestAngle, found
}

func (b *WaterFrontierBot) bestCampAngle(w *World) (float64, bool) {
	const steps = 360
	hasCamps := false
	for _, bld := range w.Buildings {
		if bld.Kind == KindLoggingCamp {
			hasCamps = true
			break
		}
	}

	bestAngle := 0.0
	bestScore := -math.MaxFloat64
	found := false

	for i := 0; i < steps; i++ {
		a := float64(i) * 2 * math.Pi / float64(steps)
		pv := buildPreview(w, a)
		if pv.Kind != KindLoggingCamp || !pv.Valid {
			continue
		}

		var score float64
		if !hasCamps {
			score = -angularDistance(a, b.field.CenterAngle)
		} else {
			score = math.Pi
			for _, bld := range w.Buildings {
				if bld.Kind != KindLoggingCamp {
					continue
				}
				if d := angularDistance(a, bld.Angle); d < score {
					score = d
				}
			}
		}

		if score > bestScore {
			bestScore = score
			bestAngle = a
			found = true
		}
	}
	return bestAngle, found
}

// ── Water scan runner ─────────────────────────────────────────────────────────

// runWaterBalanceScan runs all campCap×dockCap×strategy combinations through
// the balance scan and writes a comparison table to logs/balance-scan-<scenario>.txt.
// Runs cap out at 600 s so camp=0 scenarios (which never pop-max) still terminate.
func runWaterBalanceScan(t *testing.T, scenario string, campCaps, dockCaps []int, preSetup func(w *World)) {
	t.Helper()
	type stratEntry struct {
		strat DockStrategy
		tag   string
	}
	strategies := []stratEntry{
		{DockStrategyBatch, "bat"},
		{DockStrategySequential, "seq"},
	}
	var snaps []metricsSnapshot
	for _, campCap := range campCaps {
		for _, dockCap := range dockCaps {
			for _, se := range strategies {
				campCap, dockCap, se := campCap, dockCap, se
				label := fmt.Sprintf("c=%d,d=%d,%s", campCap, dockCap, se.tag)
				t.Run(label, func(t *testing.T) {
					bot := &WaterFrontierBot{CampCap: campCap, DockCap: dockCap, DockStrat: se.strat}
					runner := newBalanceScanRunner(bot)
					runner.preSetup = preSetup
					runner.maxScanTime = 600.0
					runSimTrace(t, fmt.Sprintf("%s — %s", scenario, label), 25, runner)
					runner.printMetrics(t)
					snaps = append(snaps, runner.snapshot(label))
				})
			}
		}
	}
	if err := writeWaterBalanceScanLog(scenario, snaps); err != nil {
		t.Logf("balance scan: failed to write log: %v", err)
	} else {
		t.Logf("balance scan: metrics written to logs/balance-scan-%s.txt", scenario)
	}
}

// writeWaterBalanceScanLog writes a water-planet comparison table with dock and
// water-rate columns to logs/balance-scan-<scenario>.txt.
func writeWaterBalanceScanLog(scenario string, snaps []metricsSnapshot) error {
	logsDir := "../../logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return err
	}
	slug := strings.NewReplacer(" ", "-", "/", "-", "—", "-").Replace(scenario)
	path := fmt.Sprintf("%s/balance-scan-%s.txt", logsDir, slug)

	var b strings.Builder

	fmt.Fprintf(&b, "balance scan — %s — %s\n", scenario, time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintln(&b, strings.Repeat("═", 72))
	fmt.Fprintln(&b)

	if len(snaps) > 0 {
		s0 := snaps[0]
		fmt.Fprintf(&b, "starting resources: wood=%.0f  water=%.0f\n\n", s0.startingWood, s0.startingWater)
	}

	const hdr = "%-13s  %-10s  %-10s  %-8s  %-9s  %-8s  %-9s  %-8s  %-8s  %-16s  %s"
	fmt.Fprintf(&b, hdr+"\n", "build", "first-dock", "first-camp", "pop-max", "trees@max", "wood@max", "water@end", "wood/s", "water/s", "supply-blocks", "holds")
	fmt.Fprintln(&b, strings.Repeat("─", 122))

	for _, s := range snaps {
		firstDock := "–"
		if s.firstDockSeen {
			firstDock = fmt.Sprintf("t=%.0fs", s.firstDockTime)
		}
		firstCamp := "–"
		if s.firstCampSeen {
			firstCamp = fmt.Sprintf("t=%.0fs", s.firstCampTime)
		}
		popMax, treesStr, woodMaxStr, woodRateStr, waterRateStr := "–", "–", "–", "–", "–"
		if s.popMaxed {
			popMax = fmt.Sprintf("t=%.0fs", s.popMaxTime)
			treesStr = fmt.Sprintf("%d", s.treesAtPopMax)
			woodMaxStr = fmt.Sprintf("%.0f", s.woodAtPopMax)
			if elapsed := s.endSimTime - s.popMaxTime; elapsed > 0 {
				woodRateStr = fmt.Sprintf("%.2f/s", (s.endWood-s.woodAtPopMax)/elapsed)
				waterRateStr = fmt.Sprintf("%.2f/s", (s.endWater-s.waterAtPopMax)/elapsed)
			}
		}
		waterEndStr := fmt.Sprintf("%.0f", s.endWater)

		blocksStr := "none"
		if len(s.blocks) > 0 {
			total := 0.0
			for _, blk := range s.blocks {
				total += blk.duration
			}
			lbl := ""
			if s.spaceBlocked {
				lbl = " space-blocked"
			}
			blocksStr = fmt.Sprintf("%d (%.0fs total)%s", len(s.blocks), total, lbl)
		}

		fmt.Fprintf(&b, "%-13s  %-10s  %-10s  %-8s  %-9s  %-8s  %-9s  %-8s  %-8s  %-16s  %d\n",
			s.label, firstDock, firstCamp, popMax, treesStr, woodMaxStr, waterEndStr, woodRateStr, waterRateStr, blocksStr, s.holdPeriods)
	}
	fmt.Fprintln(&b)

	if len(snaps) > 0 {
		maxCols := 0
		for _, s := range snaps {
			if n := len(s.workerEvents); n > maxCols {
				maxCols = n
			}
		}
		fmt.Fprintln(&b, "worker timeline (seconds at each count):")
		fmt.Fprintf(&b, "%-13s", "count")
		for i := 0; i < maxCols; i++ {
			wc := 0
			for _, s := range snaps {
				if i < len(s.workerEvents) {
					wc = int(s.workerEvents[i][1])
					break
				}
			}
			fmt.Fprintf(&b, " %4d", wc)
		}
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, strings.Repeat("─", 13+maxCols*5))
		for _, s := range snaps {
			fmt.Fprintf(&b, "%-13s", s.label)
			for i, ev := range s.workerEvents {
				end := s.endSimTime
				if i+1 < len(s.workerEvents) {
					end = s.workerEvents[i+1][0]
				}
				dur := end - ev[0]
				if dur < 0 {
					dur = 0
				}
				fmt.Fprintf(&b, " %4.0f", dur)
			}
			fmt.Fprintln(&b)
		}
	}

	// ── dock upgrade timeline ────────────────────────────────────────────────
	maxUpgrades := 0
	for _, s := range snaps {
		if n := len(s.dockUpgradeTimes); n > maxUpgrades {
			maxUpgrades = n
		}
	}
	if maxUpgrades > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "dock upgrade times (t= seconds from start):")
		fmt.Fprintf(&b, "%-13s", "")
		for i := 0; i < maxUpgrades; i++ {
			fmt.Fprintf(&b, "  %-7s", fmt.Sprintf("L2#%d", i+1))
		}
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, strings.Repeat("─", 13+maxUpgrades*9))
		for _, s := range snaps {
			fmt.Fprintf(&b, "%-13s", s.label)
			for i := 0; i < maxUpgrades; i++ {
				if i < len(s.dockUpgradeTimes) {
					fmt.Fprintf(&b, "  %-7s", fmt.Sprintf("t=%.0fs", s.dockUpgradeTimes[i]))
				} else {
					fmt.Fprintf(&b, "  %-7s", "–")
				}
			}
			fmt.Fprintln(&b)
		}
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}
