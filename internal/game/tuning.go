package game

import "math"

const (
	// economy / cost constants
	workerBaseCost   = 40.0
	workerCostGrowth = 1.15
	campBaseCost     = 120.0
	campCostGrowth   = 1.50

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
	nurtureCost      = 80.0 // wood spent per Nurture click
	nurtureEXP       = 5.0  // field EXP gained per Nurture click
	forestHalfArc    = math.Pi
	startingNodes    = 5
)
