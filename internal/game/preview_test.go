package game

import (
	"math"
	"testing"
)

func TestRouteDist(t *testing.T) {
	radius := 90.0
	// Same angle → 0.
	if got := routeDist(radius, 0, 0); got != 0 {
		t.Errorf("routeDist same angle: got %v, want 0", got)
	}
	// Quarter turn.
	want := (math.Pi / 2) * radius
	if got := routeDist(radius, 0, math.Pi/2); math.Abs(got-want) > 1e-9 {
		t.Errorf("routeDist quarter turn: got %v, want %v", got, want)
	}
	// Short-way through the ±π boundary.
	want2 := 0.2 * radius
	if got := routeDist(radius, math.Pi-0.1, -math.Pi+0.1); math.Abs(got-want2) > 1e-9 {
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
		want := routeDist(w.Planet.Radius, center, r.Node.Angle)
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

func TestBuildPreviewFirstCampNeedsFreNode(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	w.NextNodeID = 0

	// No nodes anywhere.
	pv := buildPreview(w, 0)
	if pv.Valid {
		t.Error("first camp with no nodes should be invalid")
	}

	// Add a claimed node only.
	cl := newNode(w, KindWood, 0)
	cl.OwnerID = 5
	w.Nodes = []*ResourceNode{cl}
	pv = buildPreview(w, 0)
	if pv.Valid {
		t.Error("first camp with only claimed nodes should be invalid")
	}

	// Add a free node.
	free := newNode(w, KindWood, 0.1)
	free.OwnerID = -1
	w.Nodes = append(w.Nodes, free)
	pv = buildPreview(w, 0)
	if !pv.Valid {
		t.Error("first camp with a free local node should be valid")
	}
	if pv.Kind != KindTownHall {
		t.Errorf("first valid preview Kind: got %v, want KindTownHall", pv.Kind)
	}
}

func TestBuildPreviewLaterCampAlwaysValid(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil
	// A Town Hall must exist before camps; once it does, camp placement is always valid.
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: 0, Pos: w.Planet.RimPoint(0)}}

	pv := buildPreview(w, 0)
	if !pv.Valid {
		t.Error("camp with Town Hall and no nodes should still be valid")
	}
	if pv.Kind != KindLoggingCamp {
		t.Errorf("preview Kind: got %v, want KindLoggingCamp", pv.Kind)
	}
}

func TestBuildPreviewKind(t *testing.T) {
	w := NewWorld()
	w.Nodes = nil

	// Before Town Hall: Kind is KindTownHall.
	free := newNode(w, KindWood, 0)
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
