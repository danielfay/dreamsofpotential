package game

// Balance scan: headless scenario runner for tuning questions.
//
// Architecture:
//   - BotAI interface  — player decision logic (swappable)
//   - DefaultBot       — the standard player AI
//   - balanceScanRunner — SimTraceRunner that wraps any BotAI + collects metrics
//
// Run with:
//   go test -v -run TestSimTraceBalanceScan ./internal/game/

import (
	"fmt"
	"math"
	"testing"
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

	field      *ResourceField
	inBlock    bool
	blockStart float64
}

type blockPeriod struct {
	startTime float64
	duration  float64
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
	bot   BotAI
	field *ResourceField

	prevWorkers int

	// worker spawn timeline: [[simTime, workerCount], ...]
	workerEvents [][2]float64

	// pop-max tracking
	popMaxed     bool
	popMaxTime   float64
	woodAtPopMax float64

	// end-of-run snapshot (updated every tick so printMetrics can read it)
	endWood    float64
	endSimTime float64
}

func newBalanceScanRunner(bot BotAI) *balanceScanRunner {
	return &balanceScanRunner{bot: bot}
}

func (r *balanceScanRunner) Setup(w *World) {
	r.field = fieldForKind(w, KindWood)
	if r.field == nil {
		panic("balanceScanRunner: no wood field")
	}
	// Town Hall 90° from forest centre — realistic ~6s trip times.
	thAngle := normAngle(r.field.CenterAngle + math.Pi/2)
	if !placeBuilding(w, thAngle) {
		panic("balanceScanRunner: could not place Town Hall")
	}
	w.ResourceDiscovered = true
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
		events = append(events, fmt.Sprintf("*** pop maxed — %d workers ***", len(w.Workers)))
	}

	r.endWood = w.Economy.Wood
	r.endSimTime = w.SimTime

	return events
}

func (r *balanceScanRunner) Complete(w *World) bool {
	// Run 120s past pop-max to capture steady-state rate.
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

	if db, ok := r.bot.(*DefaultBot); ok {
		label := ""
		if db.SpaceBlocked {
			label = "  (space-blocked)"
		}
		t.Logf("camps:         %d / %d%s", db.CampsPlaced, db.CampCap, label)

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
		t.Logf("pop-max at:    t=%.0fs", r.popMaxTime)
		if elapsed := r.endSimTime - r.popMaxTime; elapsed > 0 {
			rate := (r.endWood - r.woodAtPopMax) / elapsed
			t.Logf("steady rate:   ~%.2f wood/s (over %.0fs)", rate, elapsed)
		}
	} else {
		t.Log("pop-max at:    never reached in sim window")
	}
}

// ── Test ──────────────────────────────────────────────────────────────────────

// TestSimTraceBalanceScan runs the starting planet under different camp-cap
// targets using DefaultBot and prints comparative balance metrics. Useful for
// answering questions like "does a 3-camp vs 6-camp build order produce better
// worker throughput and steady-state rate?"
//
//	go test -v -run TestSimTraceBalanceScan ./internal/game/
func TestSimTraceBalanceScan(t *testing.T) {
	if testing.Short() {
		t.Skip("balance scan: skipped in short mode")
	}
	for _, campCap := range []int{1, 3, 6} {
		campCap := campCap
		t.Run(fmt.Sprintf("camps=%d", campCap), func(t *testing.T) {
			bot := &DefaultBot{CampCap: campCap}
			runner := newBalanceScanRunner(bot)
			runSimTrace(t, fmt.Sprintf("starting planet — %d-camp cap", campCap), 25, runner)
			runner.printMetrics(t)
		})
	}
}
