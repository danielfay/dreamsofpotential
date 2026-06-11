package game

import "math"

const (
	// economy / cost constants
	campBaseCost           = 120.0
	campCostGrowth         = 1.50
	townCapacityBaseCost   = 40.0 // cost of the first paid capacity slot (CapacityBought==0)
	townCapacityCostGrowth = 1.15 // per paid capacity purchase
	townGrowthBaseCap      = 10.0 // initial Town Growth threshold
	townGrowthCapGrowth    = 1.35 // cap multiplier per worker arrival

	// Town field geometry — the settlement wedge anchored to the Town Hall angle.
	townFieldHalfArc     = 0.34 // half angular width of the town wedge (radians)
	townFieldDepthFrac   = 1.00 // inward depth as a fraction of planet radius (1.0 = center)
	townFieldRimInset    = 16.0 // px inside the rim where the first slot row sits
	townFieldSlotSpacing = 10.0 // px between dwelling slots (row + column pitch)

	// simulation constants
	workerSpeed      = 40.0 // world px / second
	loadTime         = 0.5  // seconds to load at the node
	unloadTime       = 0.3  // seconds to unload at the camp
	baseLoadAmount   = 5.0  // resource units carried per trip (×node.Size)
	settleDelay      = 0.25 // seconds before a new worker can claim work
	reactionDelay    = 0.25 // seconds before an idle worker departs for new work
	fieldBaseEXP     = 10.0 // field EXP needed for the first growth cycle
	fieldEXPGrowth   = 2.0  // field EXP cap multiplier after each growth cycle
	fieldReturnRatio = 0.20 // share of gross delivered load that becomes field EXP
	nurtureTreesPerPress = 3 // trees spawned directly per Nurture button press
	forestHalfArc    = math.Pi
	startingNodes    = 5

	// system view / abstract production
	echoRateFracA  = 0.55 // echo A rate as fraction of starting planet's snapshotted rate
	echoRateFracB  = 0.45 // echo B rate — slightly lower for variance
	revealDuration = 3.5 // seconds for the one-time unlock reveal animation
)
