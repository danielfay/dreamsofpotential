package game

import (
	"math"
	"testing"
)

// newLakeFixtureWorld returns a world with a KindWater arc centred at π/2 with
// halfArc π/4 (spanning [π/4, 3π/4]).  All nodes and buildings are cleared so
// tests start with a predictable surface.
func newLakeFixtureWorld() *World {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.Buildings = nil
	lake := &ResourceField{Kind: KindWater, CenterAngle: math.Pi / 2, HalfArc: math.Pi / 4}
	w.Planet.Fields = append(w.Planet.Fields, lake)
	return w
}

func TestInLake(t *testing.T) {
	w := newLakeFixtureWorld()
	// Inside: lake spans [π/4, 3π/4].
	if !inLake(w, math.Pi/2) {
		t.Error("centre of lake arc should be inLake")
	}
	if !inLake(w, math.Pi/4+0.01) {
		t.Error("just inside lake edge should be inLake")
	}
	// Outside.
	if inLake(w, 0) {
		t.Error("angle 0 should not be inLake")
	}
	if inLake(w, -math.Pi/2) {
		t.Error("angle -π/2 should not be inLake")
	}
}

func TestEffectiveArcNoLake(t *testing.T) {
	w := NewWorld() // only KindWood field — no lake
	radius := w.Planet.Radius
	a, b := 0.0, math.Pi/2
	want := math.Abs(normAngle(b-a)) * radius
	got := effectiveArc(w, a, b)
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("effectiveArc no lake: got %v, want %v", got, want)
	}
}

// TestArcCostLake verifies the lake-penalty formula inside arcCost directly.
// Lake at [π/4, 3π/4]; CW arc from 0 to π: land π/4, lake π/2, land π/4.
func TestArcCostLake(t *testing.T) {
	w := newLakeFixtureWorld()
	radius := w.Planet.Radius
	lakeLen := (math.Pi / 2) * radius
	landLen := (math.Pi / 2) * radius
	want := landLen + lakeLen/lakeSpeedFactor
	got := arcCost(w, 0, math.Pi)
	if math.Abs(got-want)/want > 0.01 {
		t.Errorf("arcCost crosses lake: got %v, want ~%v", got, want)
	}
	if got <= math.Pi*radius {
		t.Errorf("arcCost crossing lake should exceed geometric distance: got %v, geo %v",
			got, math.Pi*radius)
	}
}

// TestEffectiveArcAvoidsLake verifies that effectiveArc routes around a lake
// when the other arc is clear and of equal angular length.
// Lake at [π/4, 3π/4]; both arcs from 0 to π are π rad, but the CCW arc
// ([0, -π]) is lake-free, so effectiveArc should equal the geometric distance.
func TestEffectiveArcAvoidsLake(t *testing.T) {
	w := newLakeFixtureWorld()
	radius := w.Planet.Radius
	want := math.Pi * radius // CCW arc: no lake penalty
	got := effectiveArc(w, 0, math.Pi)
	if math.Abs(got-want)/want > 0.01 {
		t.Errorf("effectiveArc (clear arc available): got %v, want ~%v", got, want)
	}
}

func TestRouteLenLake(t *testing.T) {
	w := newLakeFixtureWorld()
	// Lake spans [π/4, 3π/4]. Camp just past the lake at 3π/4+0.2; node at 0.
	// Short CW arc (≈2.56 rad) crosses the lake; the free CCW arc (≈3.73 rad)
	// is cheaper. routeLen follows the detour and exceeds the geometric distance.
	campAngle := 3*math.Pi/4 + 0.2
	camp := &Building{Kind: KindLoggingCamp, Angle: campAngle, Pos: w.Planet.RimPoint(campAngle)}
	w.Buildings = []*Building{camp}
	node := newNode(w, KindWood, 0)
	node.OwnerID = -1
	w.Nodes = []*ResourceNode{node}

	geo := campAngle * w.Planet.Radius // shortest angular distance
	if rl := routeLen(w, node); rl <= geo {
		t.Errorf("routeLen with lake detour should exceed geometric distance: got %v, geo %v", rl, geo)
	}
}

func TestNearestCampPrefersLand(t *testing.T) {
	w := newLakeFixtureWorld()
	// Lake spans [π/4, 3π/4].  Node at angle 0.
	// Camp A: angle -1.0 (land, geometric dist 1.0·R).
	// Camp B: angle 0.9  (inside lake zone, geometric dist 0.9·R but path crosses lake).
	campA := &Building{Kind: KindLoggingCamp, Angle: -1.0, Pos: w.Planet.RimPoint(-1.0)}
	campB := &Building{Kind: KindLoggingCamp, Angle: 0.9, Pos: w.Planet.RimPoint(0.9)}
	w.Buildings = []*Building{campA, campB}

	node := newNode(w, KindWood, 0)
	node.OwnerID = -1
	w.Nodes = []*ResourceNode{node}

	best := nearestCamp(w, node)
	if best != campA {
		t.Error("nearestCamp should prefer land camp over geometrically closer lake-crossing camp")
	}
}

func TestBuildingRejectedInLake(t *testing.T) {
	w := newLakeFixtureWorld()
	// Attempt to place the first building (Town Hall) inside the lake arc.
	if placeBuildingWithFreePlacement(w, math.Pi/2, true) {
		t.Error("building inside lake arc should be rejected")
	}
	// Outside lake should succeed on an empty planet.
	if !placeBuildingWithFreePlacement(w, 0.0, true) {
		t.Error("building outside lake arc should succeed on an empty planet")
	}
}

func TestNodeSpawnRejectedInLake(t *testing.T) {
	w := newLakeFixtureWorld()
	// A wide wood field covering the full circle so the angle check passes first.
	fullField := &ResourceField{Kind: KindWood, CenterAngle: 0, HalfArc: math.Pi, Known: true}
	node := newNode(w, KindWood, math.Pi/2)
	node.Size = 1

	if nodeSpawnAngleValid(w, fullField, node, math.Pi/2) {
		t.Error("node spawn inside lake should be rejected")
	}
	if !nodeSpawnAngleValid(w, fullField, node, 0.0) {
		t.Error("node spawn outside lake should be valid")
	}
}
