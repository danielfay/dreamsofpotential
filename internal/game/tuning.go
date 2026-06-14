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

	// ── Tight Grove (echoB, layoutID 1) ─────────────────────────────────────
	// Compact full-forest planet. TH placement immediately spawns many more trees
	// than usual, leaving only 1-2 valid camp spots and teaching pressure decisions.
	tightGroveRadius     = 50.0
	tightGroveStartNodes = 12 // burst of trees on TH placement — nearly fills the rim

	// ── Lakewood (echoA, layoutID 0) ─────────────────────────────────────────
	// Forest split by a lake arc so workers naturally avoid the island region
	// until a local camp is built there. Completion awards Water Potential.
	// The four fields tile the full ring without gaps or overlaps:
	//   main forest 140° + large lake 100° + island forest 60° + small lake 60° = 360°
	lakewoodRadius = 65.0

	// Main forest: upper arc where the town hall gets placed. 140° total.
	lakewoodMainForestAngle = -math.Pi / 2     // top of rim (-90°)
	lakewoodMainForestArc   = 7 * math.Pi / 18 // 70° half-arc → spans -160° to -20°

	// Large lake: clockwise from main forest. 100° total — a real barrier.
	// Workers crossing CW to the island pay the full lake penalty.
	lakewoodLargeLakeAngle = math.Pi / 6      // right side (30°)
	lakewoodLargeLakeArc   = 5 * math.Pi / 18 // 50° half-arc → spans -20° to 80°

	// Island forest: across the large lake. 60° total — tempting but expensive.
	lakewoodIslandForestAngle = 11 * math.Pi / 18 // lower-right (110°)
	lakewoodIslandForestArc   = math.Pi / 6       // 30° half-arc → spans 80° to 140°

	// Small lake: completes the ring on the left side. 60° total.
	// Partially penalises the CCW detour, but less than the large lake CW.
	lakewoodSmallLakeAngle = 17 * math.Pi / 18 // left side (170°)
	lakewoodSmallLakeArc   = math.Pi / 6       // 30° half-arc → spans 140° to -160°

	// ── Water-to-forest field influence ──────────────────────────────────────
	// KindWaterInfluence fields are authored co-centered with each lake but wider,
	// so they reach into adjacent forest. Influence is a boolean — no stacking.
	waterInfluenceArcPadding    = 0.4  // radians an influence field extends past its lake into forest (~23°)
	waterForestSpawnSizeBonus   = 0.25 // fixed size added to a forest node spawned inside water influence
	waterForestUpgradeSizeBonus = 0.10 // extra size on top of the normal +0.15 upgrade when influenced

	// ── Water Frontier (PlanetUnknown awakened) ──────────────────────────────────
	// Shore (90° arc) + lake (270° arc) tile the full ring. Shore is at the top,
	// lake wraps the bottom/sides. Fields edge-to-edge: shore ends at ±135°, lake starts there.
	waterFrontierRadius     = lakewoodRadius
	waterFrontierShoreAngle = -math.Pi / 2    // top of rim (–90°) — tiny forest shore
	waterFrontierShoreArc   = math.Pi / 4     // 45° half-arc → 90° arc total
	waterFrontierLakeAngle  = math.Pi / 2     // 90° — tiles edge-to-edge with shore
	waterFrontierLakeArc    = 3 * math.Pi / 4 // 135° half-arc → 270° arc total (dominant water field)
	waterFrontierStartNodes = 2               // TH + flanking pair only — tiny shore leaves minimal camp room
	waterFieldBaseEXP       = woodFieldBaseEXP // placeholder cap; Phase 4 will tune water-field growth rate

	// ── system view / abstract production
	echoRateFracA       = 0.55 // echo A rate as fraction of starting planet's snapshotted rate
	echoRateFracB       = 0.45 // echo B rate — slightly lower for variance
	revealDuration      = 3.5  // seconds for the one-time unlock reveal animation
	completionAmplifier = 1.25 // echo AbstractRate multiplier on completion

	// Rolling window that ratchets AbstractRate upward at runtime (monotonic, raise-only).
	// The window prevents enter/exit fishing: the sustained floor must exceed the stored rate
	// for a full window before it sticks. Revisit if planets gain damage/decay mechanics.
	abstractRateWindowSec = 60.0 // rolling window length in seconds
	abstractRateBuckets   = 12   // sub-buckets (each spans abstractRateWindowSec/abstractRateBuckets s)
)
