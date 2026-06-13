package game

import "math"

const (
	// economy / cost constants
	campBaseCost           = 200.0 // first camp clearly unreachable during the first-lesson window
	campCostGrowth         = 1.35  // gentler growth keeps the 4th camp affordable
	townCapacityBaseCost   = 80.0  // house lights up a few trips after the initial growth fill
	townCapacityCostGrowth = 1.15  // per paid capacity purchase

	// Two-phase town growth.
	// Phase 1 (scripted lesson): the initial cap is small so the growth bar
	// fills before the house button lights up, teaching the bar→worker link.
	// Phase 2 (normal play): after the first worker spawns the cap jumps to
	// townGrowthBaseCap and grows geometrically from there, giving a looser
	// but meaningful ramp for the rest of the planet.
	townGrowthInitialCap = 70.0  // first fill only — fires just before house is affordable
	townGrowthBaseCap    = 250.0 // normal-play base, used from the second fill onward
	townGrowthCapGrowth  = 1.18  // cap multiplier per worker arrival (phase 2)

	// Town field geometry — the settlement wedge anchored to the Town Hall angle.
	townFieldHalfArc     = 0.60 // half angular width of the town wedge (radians)
	townFieldDepthFrac   = 1.00 // inward depth as a fraction of planet radius (1.0 = center)
	townFieldRimInset    = 16.0 // px inside the rim where the first slot row sits
	townFieldSlotSpacing = 10.0 // px between dwelling slots (row + column pitch)

	// simulation constants
	lakeSpeedFactor     = 0.20 // workers cross lake arcs at 20% normal speed
	workerSpawnCooldown = 3.0  // minimum seconds between worker spawns (prevents overflow burst)
	workerSpeed         = 40.0 // world px / second
	loadTime            = 0.5  // seconds to load at the node
	unloadTime          = 0.3  // seconds to unload at the camp
	baseLoadAmount      = 5.0  // resource units carried per trip (×node.Size)
	settleDelay         = 0.25 // seconds before a new worker can claim work
	reactionDelay       = 0.25 // seconds before an idle worker departs for new work
	woodFieldBaseEXP    = 10.0 // field EXP needed for the first growth cycle
	woodFieldEXPGrowth  = 2.0  // field EXP cap multiplier (geometric phase)
	woodFieldEXPMaxStep = 10.0 // cap on how much the EXP threshold can grow per cycle;
	// once the geometric step would exceed this, growth becomes
	// additive — keeping late-game trees naturally reachable
	woodFieldReturnRatio = 0.27 // share of gross delivered load that becomes field EXP
	nurtureTreesPerPress = 5    // trees spawned directly per Nurture button press
	forestHalfArc        = math.Pi
	startingNodes        = 5

	// ── Tight Grove (echoA, layoutID 0) ─────────────────────────────────────
	// Smaller radius and narrower forest arc so the rim fills up faster,
	// teaching that placement decisions matter under surface pressure.
	tightGroveRadius      = 50.0
	tightGroveForestAngle = -math.Pi / 2  // top of rim
	tightGroveForestArc   = math.Pi * 0.6 // ~108° half-arc, noticeably narrower than π
	tightGroveStartNodes  = 8             // pre-crowded from the start

	// ── Lakewood (echoB, layoutID 1) ─────────────────────────────────────────
	// Forest split by a lake arc so workers naturally avoid the island region
	// until a local camp is built there. Completion awards Water Potential.
	lakewoodRadius = 65.0

	// Main forest: upper arc where the town hall gets placed.
	lakewoodMainForestAngle = -math.Pi / 2 // top of rim
	lakewoodMainForestArc   = 0.90         // wide enough for comfortable placement

	// Large lake: sits on the clockwise short-arc route from main forest to island,
	// making that route expensive so workers avoid the island until a camp is built.
	lakewoodLargeLakeAngle = math.Pi / 6  // right side (~30°)
	lakewoodLargeLakeArc   = 0.85         // wide — a real barrier

	// Island forest: across the large lake; visible but expensive to reach from main.
	lakewoodIslandForestAngle = math.Pi * 0.55 // lower-right (~99°)
	lakewoodIslandForestArc   = 0.55

	// Small shaping lake: left side adds visual variety and partially penalises
	// the counter-clockwise detour around the ring.
	lakewoodSmallLakeAngle = -math.Pi * 0.85 // left (~-153°)
	lakewoodSmallLakeArc   = 0.45

	lakewoodMainStartNodes   = 5 // main forest seeding
	lakewoodIslandStartNodes = 2 // a few visible island trees — tempting but not packed

	// ── system view / abstract production
	echoRateFracA  = 0.55  // echo A rate as fraction of starting planet's snapshotted rate
	echoRateFracB  = 0.45  // echo B rate — slightly lower for variance
	revealDuration = 3.5   // seconds for the one-time unlock reveal animation
	awakenCost          = 500.0 // global wood to awaken a dormant echo planet
	completionAmplifier = 1.25  // echo AbstractRate multiplier on completion

	// Rolling window that ratchets AbstractRate upward at runtime (monotonic, raise-only).
	// The window prevents enter/exit fishing: the sustained floor must exceed the stored rate
	// for a full window before it sticks. Revisit if planets gain damage/decay mechanics.
	abstractRateWindowSec = 60.0 // rolling window length in seconds
	abstractRateBuckets   = 12   // sub-buckets (each spans abstractRateWindowSec/abstractRateBuckets s)
)
