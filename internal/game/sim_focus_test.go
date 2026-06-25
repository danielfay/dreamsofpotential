package game

import (
	"testing"
)

// newFocusWorld builds a world with both wood and water resources available for
// focus tests. Returns a world with a Town Hall, one wood node, one dock with a
// sparkle, and N workers settled at idle.
func newFocusWorld(t *testing.T, numWorkers int) *World {
	t.Helper()
	w := newWaterFrontierFixture()
	w.Economy.WaterDiscovered = true
	w.Economy.Wood = 1000

	// Place a wood node at shore so workers can harvest.
	woodField := fieldForKind(w, KindWood)
	if woodField == nil {
		t.Fatal("setup: no wood field in water frontier world")
	}
	woodResult := spawnNode(w, woodField)
	if woodResult.Outcome == growthOutcomeNone {
		t.Fatal("setup: could not spawn wood node")
	}

	// Place a dock and a sparkle.
	dockAngle := waterFrontierLakeAngle
	if !placeBuildingWithFreePlacement(w, dockAngle, true) {
		t.Fatal("setup: could not place dock")
	}
	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
			break
		}
	}
	if dock == nil {
		t.Fatal("setup: no dock after placement")
	}

	wf := fieldForKind(w, KindWater)
	if wf != nil {
		sparkleResult := spawnSparkle(w, wf)
		if sparkleResult.Outcome != growthOutcomeNone {
			if sparkleNode := findNode(w, sparkleResult.NodeID); sparkleNode != nil {
				sparkleNode.ServicingDockID = dock.ID
			}
		}
	}

	// Spawn workers.
	w.Economy.WorkerCapacity = numWorkers + 10
	for len(w.Workers) < numWorkers {
		if spawnWorkerAtTownHall(w) == nil {
			t.Fatalf("setup: could not spawn worker %d", len(w.Workers)+1)
		}
	}

	// Settle workers to StateIdleWaiting.
	for range 120 {
		Step(w, dt)
	}
	return w
}

// focusedCounts returns the number of workers with each FocusedKind.
func focusedCounts(w *World) (wood, water, none int) {
	for _, wk := range w.Workers {
		switch wk.FocusedKind {
		case KindWood:
			wood++
		case KindWater:
			water++
		default:
			none++
		}
	}
	return
}

func TestOpenFocusControlPreservesZeroWaterTarget(t *testing.T) {
	g := &Game{world: newFocusWorld(t, 3)}
	g.world.LaborFocus = map[ResourceKind]int{KindWood: 3, KindWater: 0}

	g.openFocusControl()

	if g.focusDraftWater != 0 {
		t.Errorf("focusDraftWater = %d, want 0", g.focusDraftWater)
	}
}

func TestFocusZeroWaterTargetAssignsAllWood(t *testing.T) {
	w := newFocusWorld(t, 3)
	w.LaborFocus = map[ResourceKind]int{KindWood: 3, KindWater: 0}
	for _, wk := range w.Workers {
		wk.FocusedKind = focusKindNone
		wk.State = StateIdleWaiting
		assignFocusToIdleWorker(w, wk)
	}

	wood, water, none := focusedCounts(w)
	if wood != 3 || water != 0 || none != 0 {
		t.Fatalf("focused counts = wood:%d water:%d none:%d, want 3/0/0", wood, water, none)
	}
}

// TestFocusGatedJobSeeking verifies that wood-focused workers never take water
// jobs and water-focused workers never take wood jobs.
func TestFocusGatedJobSeeking(t *testing.T) {
	w := newFocusWorld(t, 2)

	w.LaborFocus = map[ResourceKind]int{KindWood: 1, KindWater: 1}

	// Run enough steps for workers to reach their first job.
	for range 300 {
		Step(w, dt)
	}

	for _, wk := range w.Workers {
		switch wk.FocusedKind {
		case KindWood:
			// Wood-focused worker must not be in a water loop.
			if workerInWaterLoop(wk) {
				t.Errorf("wood-focused worker %d entered water loop (state=%v)", wk.ID, wk.State)
			}
		case KindWater:
			// Water-focused worker must not be in the wood loop.
			if workerInLoop(wk) {
				t.Errorf("water-focused worker %d entered wood loop (state=%v)", wk.ID, wk.State)
			}
		}
	}
}

// TestFocusIdleWhenNoWork verifies that a water-focused worker stays idle at
// Town Hall when there are no serviceable sparkles (dock exists but no sparkles).
func TestFocusIdleWhenNoWork(t *testing.T) {
	w := newFocusWorld(t, 2)

	// Clear all sparkles so there is no water work.
	for _, n := range w.Nodes {
		if n.Interior {
			n.OwnerID = w.Workers[0].ID // mark as claimed so bestFreeDock returns nil
		}
	}
	// Remove interior nodes entirely to ensure no water work.
	filtered := w.Nodes[:0]
	for _, n := range w.Nodes {
		if !n.Interior {
			filtered = append(filtered, n)
		}
	}
	w.Nodes = filtered

	w.LaborFocus = map[ResourceKind]int{KindWood: 1, KindWater: 1}

	// Reset all workers to idle.
	for _, wk := range w.Workers {
		wk.State = StateIdleWaiting
		wk.NodeID = -1
		wk.DockID = -1
		wk.FocusedKind = focusKindNone
	}

	for range 120 {
		Step(w, dt)
	}

	// The water-focused worker should be idle (not in a loop).
	for _, wk := range w.Workers {
		if wk.FocusedKind == KindWater {
			if workerInLoop(wk) || workerInWaterLoop(wk) {
				t.Errorf("water-focused worker %d should be idle, got state=%v", wk.ID, wk.State)
			}
		}
	}
}

// TestFocusRatioMaintained verifies that new workers spawned after LaborFocus is
// set are assigned to fill the most under-represented kind.
func TestFocusRatioMaintained(t *testing.T) {
	// Start with a world that has no LaborFocus yet, so the founding worker is unfocused.
	w := newFocusWorld(t, 1)
	foundingWorker := w.Workers[0]
	if foundingWorker.FocusedKind != focusKindNone {
		t.Errorf("founding worker should be unfocused before LaborFocus is set, got %v", foundingWorker.FocusedKind)
	}

	// Set LaborFocus to 2 wood + 1 water.
	w.LaborFocus = map[ResourceKind]int{KindWood: 2, KindWater: 1}

	// Spawn two additional workers: they should fill the deficit in order.
	w.Economy.WorkerCapacity = 10
	wk2 := spawnWorkerAtTownHall(w)
	if wk2 == nil {
		t.Fatal("failed to spawn second worker")
	}
	wk3 := spawnWorkerAtTownHall(w)
	if wk3 == nil {
		t.Fatal("failed to spawn third worker")
	}

	// wk2 sees: counts={} (founding worker is focusKindNone), LaborFocus={wood:2,water:1}
	// Deficit: wood=2, water=1 → wood is larger → wk2 gets KindWood.
	if wk2.FocusedKind != KindWood {
		t.Errorf("wk2.FocusedKind = %v, want KindWood", wk2.FocusedKind)
	}

	// wk3 sees: counts={wood:1}, LaborFocus={wood:2,water:1}
	// Deficit: wood=1, water=1 → tie → tiebreak prefers KindWater (higher value) → wk3 gets KindWater.
	if wk3.FocusedKind != KindWater {
		t.Errorf("wk3.FocusedKind = %v, want KindWater", wk3.FocusedKind)
	}
}

// TestAutoProofSplit verifies that the first water delivery triggers LaborFocus
// when a dock has reachable serviceable sparkles.
func TestAutoProofSplit(t *testing.T) {
	// Build a world with a dock and a sparkle that is properly assigned to the dock.
	// Use a shore-edge dock so assignServicingDocks can assign nearby sparkles.
	w := newWaterFrontierFixture()
	w.Economy.WaterDiscovered = false
	w.LaborFocus = nil

	shoreAngle := shoreEdgeAngle()
	if !placeBuildingWithFreePlacement(w, shoreAngle, true) {
		t.Fatal("setup: could not place dock")
	}
	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
			break
		}
	}
	if dock == nil {
		t.Fatal("setup: no dock")
	}

	// Fill water field with sparkles and run assignServicingDocks to assign them.
	fillWaterFieldSparkles(w)
	assignServicingDocks(w)

	if !dockHasServiceableSparkles(w) {
		t.Skip("setup: dock has no serviceable sparkles in this geometry — skipping")
	}

	// Spawn 3 workers (without LaborFocus set, so they start unfocused).
	w.Economy.WorkerCapacity = 10
	for range 3 {
		spawnWorkerAtTownHall(w)
	}

	// Simulate first water delivery.
	wk := w.Workers[0]
	wk.Carried = 1.0

	completeWaterUnload(w, wk, dock)

	if len(w.LaborFocus) == 0 {
		t.Fatal("LaborFocus should be set after first water delivery with serviceable sparkles")
	}
	if w.LaborFocus[KindWater] != 1 {
		t.Errorf("LaborFocus[KindWater] = %d, want 1", w.LaborFocus[KindWater])
	}
	wantWood := len(w.Workers) - 1
	if w.LaborFocus[KindWood] != wantWood {
		t.Errorf("LaborFocus[KindWood] = %d, want %d", w.LaborFocus[KindWood], wantWood)
	}
}

// TestSetAutoProofSplit verifies that setAutoProofSplit correctly sets
// LaborFocus to 1 water + remaining workers as wood.
func TestSetAutoProofSplit(t *testing.T) {
	w := NewWorld()
	// Simulate 4 workers (no Town Hall needed for this low-level test).
	for range 4 {
		w.Workers = append(w.Workers, &Worker{
			ID:            w.NextWorkerID,
			FocusedKind:   focusKindNone,
			NodeID:        -1,
			TargetNodeID:  -1,
			PendingNodeID: -1,
			DockID:        -1,
		})
		w.NextWorkerID++
	}

	setAutoProofSplit(w)

	if len(w.LaborFocus) == 0 {
		t.Fatal("LaborFocus not set by setAutoProofSplit")
	}
	if w.LaborFocus[KindWater] != 1 {
		t.Errorf("LaborFocus[KindWater] = %d, want 1", w.LaborFocus[KindWater])
	}
	if w.LaborFocus[KindWood] != 3 {
		t.Errorf("LaborFocus[KindWood] = %d, want 3", w.LaborFocus[KindWood])
	}
}

// TestAutoProofSplitNoTriggerIfAlreadySet verifies auto proof split does not
// overwrite an existing LaborFocus.
func TestAutoProofSplitNoTriggerIfAlreadySet(t *testing.T) {
	w := newFocusWorld(t, 3)

	w.LaborFocus = map[ResourceKind]int{KindWood: 2, KindWater: 1}

	waterWorker := w.Workers[0]
	waterWorker.Carried = 1.0

	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
			break
		}
	}

	completeWaterUnload(w, waterWorker, dock)

	// LaborFocus should be unchanged.
	if w.LaborFocus[KindWood] != 2 || w.LaborFocus[KindWater] != 1 {
		t.Errorf("LaborFocus changed after completeWaterUnload with existing focus: %v", w.LaborFocus)
	}
}

// TestWaterWorkerAbandonsDockOnFocusChange verifies that workers in the water
// loop stop and return home when the focus is changed to 0 water workers.
// Uses direct state injection to avoid geometry-dependent sparkle placement.
func TestWaterWorkerAbandonsDockOnFocusChange(t *testing.T) {
	w := newFocusWorld(t, 2)

	// Place both workers explicitly into StateToDock so they are mid water-trip.
	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
			break
		}
	}
	if dock == nil {
		t.Fatal("setup: no dock")
	}
	for _, wk := range w.Workers {
		wk.State = StateToDock
		wk.DockID = dock.ID
		wk.FocusedKind = KindWater
		wk.NodeID = -1
		wk.Carried = 0
	}

	// Set focus to 0 water — workers should abort immediately on the next tick.
	w.LaborFocus = map[ResourceKind]int{KindWood: 2, KindWater: 0}

	// Run enough steps for workers to reach home (StateToDock abort → returnHome).
	for range 600 {
		Step(w, dt)
	}

	for _, wk := range w.Workers {
		if workerInWaterLoop(wk) {
			t.Errorf("worker %d still in water loop after focus→0 water (state=%v)", wk.ID, wk.State)
		}
	}
}

// TestWaterWorkerAbortsMidUnload verifies that completeWaterUnload does not
// re-dispatch to StateDiving when the worker is over the water quota.
func TestWaterWorkerAbortsMidUnload(t *testing.T) {
	w := newFocusWorld(t, 2)

	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
			break
		}
	}
	if dock == nil {
		t.Fatal("setup: no dock")
	}

	// Fill sparkles so nextDiveSparkle would normally find work.
	fillWaterFieldSparkles(w)
	assignServicingDocks(w)

	// Set focus to 0 water BEFORE the unload.
	w.LaborFocus = map[ResourceKind]int{KindWood: 2, KindWater: 0}

	wk := w.Workers[0]
	wk.Carried = 1.0
	completeWaterUnload(w, wk, dock)

	if workerInWaterLoop(wk) {
		t.Errorf("worker re-entered water loop after completeWaterUnload with 0 water quota (state=%v)", wk.State)
	}
}
