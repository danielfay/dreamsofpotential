package game

import (
	"math"
	"testing"
)

func TestRouteDist(t *testing.T) {
	w := NewWorld() // no lake fields — effective arc equals geometric arc
	radius := w.Planet.Radius
	// Same angle → 0.
	if got := routeDist(w, 0, 0); got != 0 {
		t.Errorf("routeDist same angle: got %v, want 0", got)
	}
	// Quarter turn.
	want := (math.Pi / 2) * radius
	if got := routeDist(w, 0, math.Pi/2); math.Abs(got-want) > 1e-9 {
		t.Errorf("routeDist quarter turn: got %v, want %v", got, want)
	}
	// Short-way through the ±π boundary.
	want2 := 0.2 * radius
	if got := routeDist(w, math.Pi-0.1, -math.Pi+0.1); math.Abs(got-want2) > 1e-9 {
		t.Errorf("routeDist wraparound: got %v, want %v", got, want2)
	}
}

func TestLocalNodes(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	center := 0.0
	near := newNode(w, KindWood, center+previewArc*0.5)
	near.OwnerID = -1
	atEdge := newNode(w, KindWood, center+previewArc) // exactly at boundary → included
	atEdge.OwnerID = -1
	outside := newNode(w, KindWood, center+previewArc*1.01)
	outside.OwnerID = -1
	claimed := newNode(w, KindWood, center+previewArc*0.3)
	claimed.OwnerID = 99

	w.Nodes = []*ResourceNode{near, atEdge, outside, claimed}

	free, cl, reserved := localNodes(w, center)

	if len(free) != 2 {
		t.Errorf("free count: got %d, want 2", len(free))
	}
	if len(cl) != 1 {
		t.Errorf("claimed count: got %d, want 1", len(cl))
	}
	if len(reserved) != 0 {
		t.Errorf("reserved count: got %d, want 0", len(reserved))
	}
	for _, r := range free {
		if r.Dist < 0 {
			t.Error("negative Dist in free route")
		}
		want := routeDist(w, center, r.Node.Angle)
		if math.Abs(r.Dist-want) > 1e-9 {
			t.Errorf("Dist mismatch: got %v, want %v", r.Dist, want)
		}
	}
}

func TestLocalNodesWraparound(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	// Node just past +π, queried from just past -π.
	n := newNode(w, KindWood, math.Pi-0.1)
	n.OwnerID = -1
	w.Nodes = []*ResourceNode{n}

	free, _, _ := localNodes(w, -math.Pi+0.1)
	if len(free) != 1 {
		t.Errorf("wraparound: expected 1 free node, got %d", len(free))
	}
}

func TestBuildPreviewTownHallValidOnEmptyPlanet(t *testing.T) {
	w := NewWorld()

	// Town Hall is valid on an empty planet — no nearby trees required.
	pv := buildPreview(w, 0)
	if !pv.Valid {
		t.Error("Town Hall preview on empty planet should be valid")
	}
	if pv.Kind != KindTownHall {
		t.Errorf("first preview Kind: got %v, want KindTownHall", pv.Kind)
	}
}

func TestBuildPreviewTownHallBlockedByNodeFootprint(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	n := newNode(w, KindWood, 0)
	n.Size = 1
	w.Nodes = []*ResourceNode{n}

	pv := buildPreview(w, 0)
	if pv.Valid {
		t.Error("Town Hall preview overlapping a node footprint should be invalid")
	}
	if len(pv.Blocked) != 1 || pv.Blocked[0] != n {
		t.Fatalf("expected overlapping node to be reported as blocker")
	}
}


func TestBuildPreviewLaterCampRequiresAffordability(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	// A Town Hall must exist before camps; once it does, camp placement is
	// distance-valid but still needs enough wood for the next camp.
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}
	angle := math.Pi / 2

	pv := buildPreview(w, angle)
	if pv.Valid {
		t.Error("unaffordable camp preview should be invalid/red")
	}
	if pv.Affordable {
		t.Error("camp preview should report unaffordable")
	}

	w.Economy.Wood = CampCost(w)
	pv = buildPreview(w, angle)
	if !pv.Valid {
		t.Error("affordable camp with Town Hall and no nodes should still be valid")
	}
	if !pv.Affordable {
		t.Error("camp preview should report affordable")
	}
	if pv.Kind != KindLoggingCamp {
		t.Errorf("preview Kind: got %v, want KindLoggingCamp", pv.Kind)
	}
}

func TestBuildPreviewLoggingCampBlockedByNodeFootprint(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: math.Pi, Pos: w.Planet.RimPoint(math.Pi)}}
	w.Economy.Wood = CampCost(w)

	n := newNode(w, KindWood, 0)
	n.Size = 1
	w.Nodes = []*ResourceNode{n}

	pv := buildPreview(w, 0)
	if pv.Valid {
		t.Error("logging camp preview overlapping a node footprint should be invalid")
	}

	clearAngle := buildingHardHalfArc(KindLoggingCamp, w.Planet.Radius) + nodeBuildingBlockHalfArc(n, w.Planet.Radius) + 0.001
	pv = buildPreview(w, clearAngle)
	if !pv.Valid {
		t.Error("logging camp preview just outside combined footprints should be valid")
	}
}

func TestBuildPreviewKind(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil

	// Before Town Hall: Kind is KindTownHall.
	freeAngle := buildingHardHalfArc(KindTownHall, w.Planet.Radius) + nodeBuildingBlockHalfArc(&ResourceNode{Size: 1}, w.Planet.Radius) + 0.01
	free := newNode(w, KindWood, freeAngle)
	free.Size = 1
	free.OwnerID = -1
	w.Nodes = []*ResourceNode{free}
	pv := buildPreview(w, 0)
	if pv.Kind != KindTownHall {
		t.Errorf("first preview Kind: got %v, want KindTownHall", pv.Kind)
	}

	// After Town Hall: Kind is KindLoggingCamp.
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}
	pv = buildPreview(w, 0)
	if pv.Kind != KindLoggingCamp {
		t.Errorf("later preview Kind: got %v, want KindLoggingCamp", pv.Kind)
	}
}

func TestBuildPreviewPos(t *testing.T) {
	w := NewWorld()
	angle := math.Pi / 3
	pv := buildPreview(w, angle)
	want := w.Planet.RimPoint(angle)
	if math.Abs(pv.Pos.X-want.X) > 1e-9 || math.Abs(pv.Pos.Y-want.Y) > 1e-9 {
		t.Errorf("Pos mismatch: got %v, want %v", pv.Pos, want)
	}
	if pv.Angle != angle {
		t.Errorf("Angle: got %v, want %v", pv.Angle, angle)
	}
}

func TestBuildPreviewFreeRoutesSortedAndCapped(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: math.Pi, Pos: w.Planet.RimPoint(math.Pi)}}
	w.Economy.Wood = CampCost(w)

	for _, a := range []float64{0.5, 0.1, -0.4, 0.3, -0.2, 0.7} {
		n := newNode(w, KindWood, a)
		n.Size = 1
		w.Nodes = append(w.Nodes, n)
	}

	pv := buildPreview(w, 0)
	if pv.FreeTotal != 6 {
		t.Fatalf("free total got %d, want 6", pv.FreeTotal)
	}
	if len(pv.Free) != previewFreeRouteCap {
		t.Fatalf("free routes got %d, want cap %d", len(pv.Free), previewFreeRouteCap)
	}
	for i := 1; i < len(pv.Free); i++ {
		if pv.Free[i].Dist < pv.Free[i-1].Dist {
			t.Fatalf("free routes should be sorted by distance: %.3f before %.3f", pv.Free[i-1].Dist, pv.Free[i].Dist)
		}
	}
}

func TestBuildPreviewUnavailableRoutesCappedAcrossClaimedAndReserved(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: math.Pi, Pos: w.Planet.RimPoint(math.Pi)}}
	w.Economy.Wood = CampCost(w)

	for i, a := range []float64{0.1, 0.2, 0.3, 0.4, 0.5} {
		n := newNode(w, KindWood, a)
		n.Size = 1
		if i%2 == 0 {
			n.OwnerID = 10 + i
		} else {
			n.ReservedByWorkerID = 10 + i
		}
		w.Nodes = append(w.Nodes, n)
	}

	pv := buildPreview(w, 0)
	if pv.ClaimedTotal != 3 || pv.ReservedTotal != 2 {
		t.Fatalf("totals got %d claimed / %d reserved, want 3 / 2", pv.ClaimedTotal, pv.ReservedTotal)
	}
	if got := len(pv.Claimed) + len(pv.Reserved); got != previewUnavailableRouteCap {
		t.Fatalf("unavailable routes got %d, want cap %d", got, previewUnavailableRouteCap)
	}
}

func TestBuildPreviewLoggingCampBlockedByBuildingFootprint(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}
	w.Economy.Wood = CampCost(w)

	pv := buildPreview(w, 0)
	if pv.Valid {
		t.Fatal("logging camp preview overlapping Town Hall should be invalid")
	}
	if len(pv.BlockedBuildings) != 1 || pv.BlockedBuildings[0] != w.Buildings[0] {
		t.Fatal("expected Town Hall to be reported as building blocker")
	}

	clearAngle := buildingHardHalfArc(KindLoggingCamp, w.Planet.Radius) +
		buildingHardHalfArc(KindTownHall, w.Planet.Radius) + 0.001
	pv = buildPreview(w, clearAngle)
	if !pv.Valid {
		t.Fatal("logging camp preview just outside building footprints should be valid")
	}
}

func TestZeroValidPlacementPositions(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: math.Pi, Pos: w.Planet.RimPoint(math.Pi)}}
	w.Economy.Wood = CampCost(w)

	if zeroValidPlacementPositions(w) {
		t.Fatal("empty surface should have valid placement positions")
	}

	for i := 0; i < 180; i++ {
		n := newNode(w, KindWood, -math.Pi+float64(i)*2*math.Pi/180)
		n.Size = 2
		w.Nodes = append(w.Nodes, n)
	}
	if !zeroValidPlacementPositions(w) {
		t.Fatal("densely occupied rim should report no valid placement positions")
	}
}
