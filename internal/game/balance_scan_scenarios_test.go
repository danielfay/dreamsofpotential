package game

// Balance scan scenarios: one Test function per planet.
// Infrastructure (BotAI, DefaultBot, balanceScanRunner, writeBalanceScanLog,
// runForestBalanceScan) lives in balance_scan_test.go.

import "testing"

// bootstrapStartingPlanet fully sets up the starting planet so that echo
// planets can be awakened. Shared by all echo preSetup functions.
func bootstrapStartingPlanet(w *World) {
	f0 := fieldForKind(w, KindWood)
	if f0 == nil || !placeBuilding(w, f0.CenterAngle) {
		panic("bootstrapStartingPlanet: cannot place starting TH")
	}
	w.Economy.WorkerCapacity = maxTownSlots(w)
	addWorker(w)
	fillWoodFieldNodes(w, false)
	w.ResourceDiscovered = true
	triggerUnlock(w)
}

// echoPreSetup returns a preSetup func that bootstraps the starting planet,
// grants the required awakening potential, and switches to the target echo.
func echoPreSetup(echoIdx int) func(w *World) {
	return func(w *World) {
		bootstrapStartingPlanet(w)
		for kind, cost := range planetAwakenCost(w, echoIdx) {
			w.Economy.Potential[kind] += float64(cost)
		}
		awakenPlanet(w, echoIdx)
		switchToPlanet(w, echoIdx)
		enterPlanetView(w)
	}
}

// ── Scenarios ─────────────────────────────────────────────────────────────────

// TestSimTraceBalanceScan scans the starting (wood) planet under camp-cap
// variants 1, 3, and 6. Writes logs/balance-scan-starting-planet.txt.
//
//	go test -v -run TestSimTraceBalanceScan ./internal/game/
func TestSimTraceBalanceScan(t *testing.T) {
	if testing.Short() {
		t.Skip("balance scan: skipped in short mode")
	}
	runForestBalanceScan(t, "starting-planet", []int{1, 3, 6}, nil)
}

// TestSimTraceBalanceScanLakewood scans Lakewood (echo planet 1: forest + lake)
// under camp-cap variants. Writes logs/balance-scan-lakewood.txt.
//
//	go test -v -run TestSimTraceBalanceScanLakewood ./internal/game/
func TestSimTraceBalanceScanLakewood(t *testing.T) {
	if testing.Short() {
		t.Skip("balance scan: skipped in short mode")
	}
	runForestBalanceScan(t, "lakewood", []int{1, 3, 6}, echoPreSetup(1))
}

// TestSimTraceBalanceScanTightGrove scans Tight Grove (echo planet 2: full forest)
// under camp-cap variants. Writes logs/balance-scan-tight-grove.txt.
//
//	go test -v -run TestSimTraceBalanceScanTightGrove ./internal/game/
func TestSimTraceBalanceScanTightGrove(t *testing.T) {
	if testing.Short() {
		t.Skip("balance scan: skipped in short mode")
	}
	runForestBalanceScan(t, "tight-grove", []int{1, 3, 6}, echoPreSetup(2))
}

// TestSimTraceBalanceScanWaterFrontierBatch scans the Water Frontier
// (PlanetUnknown, idx 3) under dock-cap variants with no camps: 1/3/6 docks,
// using the "place all docks, then upgrade" strategy.
// Writes logs/balance-scan-water-frontier-batch.txt.
//
//	go test -v -run TestSimTraceBalanceScanWaterFrontierBatch ./internal/game/
func TestSimTraceBalanceScanWaterFrontierBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("balance scan: skipped in short mode")
	}
	runWaterBalanceScan(t, "water-frontier-batch", []int{0}, []int{1, 3, 6}, DockStrategyBatch, echoPreSetup(3))
}

// TestSimTraceBalanceScanWaterFrontierSequential scans the Water Frontier
// (PlanetUnknown, idx 3) under dock-cap variants with no camps: 1/3/6 docks,
// using the "upgrade before next dock" strategy.
// Writes logs/balance-scan-water-frontier-sequential.txt.
//
//	go test -v -run TestSimTraceBalanceScanWaterFrontierSequential ./internal/game/
func TestSimTraceBalanceScanWaterFrontierSequential(t *testing.T) {
	if testing.Short() {
		t.Skip("balance scan: skipped in short mode")
	}
	runWaterBalanceScan(t, "water-frontier-sequential", []int{0}, []int{1, 3, 6}, DockStrategySequential, echoPreSetup(3))
}
