package game

import (
	"encoding/json"
	"math"
	"testing"
)

// newWaterFrontierFixture returns a world with the water frontier planet active
// and a Town Hall placed at the shore center. The fixture has no nodes and
// enough wood/water for any dock placement.
func newWaterFrontierFixture() *World {
	w := newWorldWithSeed(11)
	// Install water frontier fields directly (bypass QA preset machinery).
	w.Planet.Fields = []*ResourceField{
		{Kind: KindWood, CenterAngle: waterFrontierShoreAngle, HalfArc: waterFrontierShoreArc, Known: true},
		{Kind: KindWater, CenterAngle: waterFrontierLakeAngle, HalfArc: waterFrontierLakeArc, Known: true},
	}
	w.Planet.FieldProgress = map[ResourceKind]*KindProgress{
		KindWood:  {Cap: woodFieldBaseEXP},
		KindWater: {Cap: waterFieldBaseEXP},
	}
	w.Nodes = nil
	w.Economy.Wood = 10000
	w.Economy.Water = 10000

	// Place Town Hall at shore center so hasTownHall == true.
	placeBuildingWithFreePlacement(w, waterFrontierShoreAngle, true)
	return w
}

// shoreEdgeAngle returns an angle that is just inside the water field at the
// shore boundary (so it is inLake but the footprint straddles the shore).
func shoreEdgeAngle() float64 {
	half := buildingHardHalfArc(KindDock, waterFrontierRadius)
	// The shore ends and lake begins at waterFrontierShoreAngle + waterFrontierShoreArc.
	boundary := normAngle(waterFrontierShoreAngle + waterFrontierShoreArc)
	// Step just past the boundary into water (so the center is in water but the
	// footprint still overlaps the shore).
	return normAngle(boundary + half*0.5)
}

// TestDockPlacementShore verifies shore dock placement (water edge touching land).
func TestDockPlacementShore(t *testing.T) {
	w := newWaterFrontierFixture()
	angle := shoreEdgeAngle()

	if !inLake(w, angle) {
		t.Fatalf("shoreEdgeAngle %.4f should be in water field", angle)
	}

	pv := buildPreviewWithFreePlacement(w, angle, true)
	if pv.Kind != KindDock {
		t.Fatalf("preview at shore edge should be KindDock, got %v", pv.Kind)
	}
	if pv.Extension {
		t.Error("shore dock preview should not be Extension")
	}
	if !pv.Valid {
		t.Error("shore dock preview should be Valid")
	}

	if !placeBuildingWithFreePlacement(w, angle, true) {
		t.Fatal("shore dock placement should succeed")
	}
	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
		}
	}
	if dock == nil {
		t.Fatal("no dock building found after placement")
	}
	if dock.Extension {
		t.Error("placed shore dock should not have Extension=true")
	}
}

// TestDockPlacementExtension verifies that any dock placed in open water
// (fully within the water field, not straddling the shore) is classified as an extension.
func TestDockPlacementExtension(t *testing.T) {
	w := newWaterFrontierFixture()

	// Deep water: center of the lake arc, far from the shore boundary.
	extAngle := normAngle(waterFrontierLakeAngle)

	if !inLake(w, extAngle) {
		t.Skipf("extAngle %.4f is not in water — geometry changed", extAngle)
	}

	pv := buildPreviewWithFreePlacement(w, extAngle, true)
	if pv.Kind != KindDock {
		t.Fatalf("extension preview should be KindDock, got %v", pv.Kind)
	}
	if !pv.Extension {
		t.Error("open-water dock preview should be Extension")
	}
	if !pv.Valid {
		t.Error("open-water dock preview should be Valid")
	}

	if !placeBuildingWithFreePlacement(w, extAngle, true) {
		t.Fatal("open-water dock placement should succeed")
	}
	var ext *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock && math.Abs(normAngle(b.Angle-extAngle)) < 0.01 {
			ext = b
		}
	}
	if ext == nil {
		t.Fatal("no extension dock building found after placement")
	}
	if !ext.Extension {
		t.Error("placed open-water dock should have Extension=true")
	}
}

// TestDockPlacementOpenWaterValid verifies that open-water angles are now valid
// dock placements without requiring shore or dock adjacency.
func TestDockPlacementOpenWaterValid(t *testing.T) {
	w := newWaterFrontierFixture()
	// Well inside the water field, far from the shore boundary.
	deepWater := normAngle(waterFrontierLakeAngle) // center of the lake arc

	if !inLake(w, deepWater) {
		t.Fatalf("deepWater %.4f should be inLake", deepWater)
	}

	pv := buildPreviewWithFreePlacement(w, deepWater, true)
	if pv.Kind != KindDock {
		t.Fatalf("deep-water preview should be KindDock, got %v", pv.Kind)
	}
	if !pv.Valid {
		t.Error("deep-water placement should be valid")
	}
	if !placeBuildingWithFreePlacement(w, deepWater, true) {
		t.Error("deep-water placement should succeed")
	}
}

// TestLoggingCampStillOnLand verifies that land/forest angles still produce
// a logging camp preview (not a dock) after a Town Hall exists.
func TestLoggingCampStillOnLand(t *testing.T) {
	w := newWaterFrontierFixture()
	landAngle := waterFrontierShoreAngle // center of the shore (forest) field

	if inLake(w, landAngle) {
		t.Fatalf("shore center %.4f should not be inLake", landAngle)
	}

	pv := buildPreviewWithFreePlacement(w, landAngle, true)
	if pv.Kind != KindLoggingCamp {
		t.Errorf("land preview should be KindLoggingCamp, got %v", pv.Kind)
	}
}

// TestDockShoreCost verifies that shore dock placement deducts dockShoreCost wood.
func TestDockShoreCost(t *testing.T) {
	w := newWaterFrontierFixture()
	w.Economy.Wood = 1000
	angle := shoreEdgeAngle()

	before := w.Economy.Wood
	if !placeBuildingWithFreePlacement(w, angle, false) {
		t.Fatal("paid shore dock placement failed")
	}
	if got, want := before-w.Economy.Wood, dockShoreCost; math.Abs(got-want) > 1e-9 {
		t.Errorf("shore dock wood cost: deducted %.1f, want %.1f", got, want)
	}
}

// TestDockExtensionCost verifies that open-water dock placement deducts both wood and water.
func TestDockExtensionCost(t *testing.T) {
	w := newWaterFrontierFixture()
	w.Economy.Wood = 10000
	w.Economy.Water = 10000

	deepWater := normAngle(waterFrontierLakeAngle)
	if !inLake(w, deepWater) {
		t.Skipf("deepWater %.4f not in water", deepWater)
	}

	woodBefore := w.Economy.Wood
	waterBefore := w.Economy.Water
	if !placeBuildingWithFreePlacement(w, deepWater, false) {
		t.Fatal("paid open-water dock placement failed")
	}
	if got, want := woodBefore-w.Economy.Wood, dockExtWoodCost; math.Abs(got-want) > 1e-9 {
		t.Errorf("open-water dock wood cost: deducted %.1f, want %.1f", got, want)
	}
	if got, want := waterBefore-w.Economy.Water, dockExtWaterCost; math.Abs(got-want) > 1e-9 {
		t.Errorf("open-water dock water cost: deducted %.1f, want %.1f", got, want)
	}
}

// TestDockTraversalReducesLakePenalty verifies that a dock covering a lake arc
// reduces the arcCost for that segment (workers no longer pay the full lake penalty).
func TestDockTraversalReducesLakePenalty(t *testing.T) {
	w := newWaterFrontierFixture()
	shoreAngle := shoreEdgeAngle()

	// Cost from shore boundary angle to well past it, without a dock.
	dest := normAngle(shoreAngle + 0.3)
	costBefore := arcCost(w, shoreAngle, 0.3)

	// Place a dock at the shore edge.
	placeBuildingWithFreePlacement(w, shoreAngle, true)

	costAfter := arcCost(w, shoreAngle, 0.3)
	if costAfter >= costBefore {
		t.Errorf("dock should reduce arcCost: before=%.3f after=%.3f", costBefore, costAfter)
	}
	_ = dest
}

// TestDockUpgrade_CostAndLevel verifies that upgradeDock deducts the correct
// resources and increments the dock's Level from 1 to 2.
func TestDockUpgrade_CostAndLevel(t *testing.T) {
	w := newWaterFrontierFixture()
	w.Economy.Wood = 10000
	w.Economy.Water = 10000
	shoreAngle := shoreEdgeAngle()
	if !placeBuildingWithFreePlacement(w, shoreAngle, true) {
		t.Fatal("could not place shore dock")
	}
	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
		}
	}
	if dock == nil {
		t.Fatal("no dock after placement")
	}
	if dock.Level != 1 {
		t.Fatalf("new dock should start at Level 1, got %d", dock.Level)
	}

	woodBefore := w.Economy.Wood
	waterBefore := w.Economy.Water
	if !upgradeDock(w, dock) {
		t.Fatal("upgradeDock returned false (expected success)")
	}
	if dock.Level != 2 {
		t.Errorf("after upgrade: Level = %d, want 2", dock.Level)
	}
	if got, want := woodBefore-w.Economy.Wood, dockL2WoodCost; math.Abs(got-want) > 1e-9 {
		t.Errorf("wood deducted %.1f, want %.1f", got, want)
	}
	if got, want := waterBefore-w.Economy.Water, dockL2WaterCost; math.Abs(got-want) > 1e-9 {
		t.Errorf("water deducted %.1f, want %.1f", got, want)
	}
}

// TestDockUpgrade_Level2Blocked verifies that a Level-2 dock cannot be upgraded further.
func TestDockUpgrade_Level2Blocked(t *testing.T) {
	w := newWaterFrontierFixture()
	w.Economy.Wood = 10000
	w.Economy.Water = 10000
	shoreAngle := shoreEdgeAngle()
	if !placeBuildingWithFreePlacement(w, shoreAngle, true) {
		t.Fatal("could not place shore dock")
	}
	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
		}
	}
	dock.Level = 2
	if upgradeDock(w, dock) {
		t.Error("upgradeDock should return false for a Level-2 dock")
	}
	if dock.Level != 2 {
		t.Errorf("Level should remain 2, got %d", dock.Level)
	}
}

// TestDockUpgrade_InsufficientResources verifies that upgradeDock fails when
// wood or water is below the upgrade cost.
func TestDockUpgrade_InsufficientResources(t *testing.T) {
	w := newWaterFrontierFixture()
	shoreAngle := shoreEdgeAngle()
	if !placeBuildingWithFreePlacement(w, shoreAngle, true) {
		t.Fatal("could not place shore dock")
	}
	var dock *Building
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			dock = b
		}
	}

	w.Economy.Wood = dockL2WoodCost - 1
	w.Economy.Water = dockL2WaterCost
	if upgradeDock(w, dock) {
		t.Error("upgradeDock should fail when wood insufficient")
	}
	w.Economy.Wood = dockL2WoodCost
	w.Economy.Water = dockL2WaterCost - 1
	if upgradeDock(w, dock) {
		t.Error("upgradeDock should fail when water insufficient")
	}
	if dock.Level != 1 {
		t.Errorf("Level should remain 1 after failed upgrades, got %d", dock.Level)
	}
}

// TestDockUpgradeAttention verifies dockUpgradeAttentionDock returns nil when
// fields can still spawn, and a dock when all fields are saturated but no L2 dock.
func TestDockUpgradeAttention(t *testing.T) {
	w := newWaterFrontierFixture()
	w.Economy.Wood = 10000
	w.Economy.Water = 10000
	shoreAngle := shoreEdgeAngle()
	if !placeBuildingWithFreePlacement(w, shoreAngle, true) {
		t.Fatal("could not place shore dock")
	}

	// Town not full: attention should be nil.
	if dock := dockUpgradeAttentionDock(w); dock != nil {
		t.Error("dockUpgradeAttentionDock should return nil when town not full")
	}

	// Fill town capacity.
	if max := maxTownSlots(w); max > w.Economy.WorkerCapacity {
		w.Economy.WorkerCapacity = max
	}

	// Fields not saturated yet: still nil.
	if dock := dockUpgradeAttentionDock(w); dock != nil {
		t.Error("dockUpgradeAttentionDock should return nil when fields not saturated")
	}

	// Saturate all fields.
	fillWoodFieldNodes(w, false)
	fillWaterFieldSparkles(w)

	// Now should return the upgradeable dock.
	dock := dockUpgradeAttentionDock(w)
	if dock == nil {
		t.Fatal("dockUpgradeAttentionDock should return a dock when all conditions met")
	}
	if dock.Level >= 2 {
		t.Errorf("returned dock has Level %d (should be < 2)", dock.Level)
	}

	// Upgrade the dock: attention should return nil (no L1 dock remains).
	upgradeDock(w, dock)
	if dock := dockUpgradeAttentionDock(w); dock != nil {
		t.Errorf("dockUpgradeAttentionDock should return nil after all docks reach L2, got dock Level=%d", dock.Level)
	}
}

// TestDockSaveRoundTrip verifies that dock Kind, Extension, and Level fields
// survive a JSON marshal/unmarshal cycle.
func TestDockSaveRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	w := newWaterFrontierFixture()
	placeBuildingWithFreePlacement(w, shoreEdgeAngle(), true)

	// Stamp a non-zero Level on the dock to confirm it round-trips.
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			b.Level = 2
			b.Extension = false
		}
	}

	data, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got World
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	var dockCount int
	for _, b := range got.Buildings {
		if b.Kind == KindDock {
			dockCount++
			if b.Level != 2 {
				t.Errorf("dock Level: got %d, want 2", b.Level)
			}
			if b.Extension {
				t.Error("dock Extension: got true, want false")
			}
		}
	}
	if dockCount == 0 {
		t.Error("no dock found in round-tripped world")
	}

	if got.NextBuildingID != w.NextBuildingID {
		t.Errorf("NextBuildingID: got %d, want %d", got.NextBuildingID, w.NextBuildingID)
	}
}
