package game

import (
	"math"
	"testing"
)

func TestIdleHomeSlots(t *testing.T) {
	p := Planet{Center: Vec{X: 160, Y: 120}, Radius: 90}
	th := &Building{Kind: KindTownHall, Angle: 0, Pos: p.RimPoint(0)}

	// Zero count → nil.
	if got := idleHomeSlots(p, th, 0); got != nil {
		t.Error("expected nil for count 0")
	}

	// Exactly idleMaxSlots slots returned.
	slots := idleHomeSlots(p, th, idleMaxSlots)
	if len(slots) != idleMaxSlots {
		t.Fatalf("expected %d slots, got %d", idleMaxSlots, len(slots))
	}

	// Count above idleMaxSlots is capped.
	capped := idleHomeSlots(p, th, idleMaxSlots+10)
	if len(capped) != idleMaxSlots {
		t.Fatalf("expected %d slots (capped), got %d", idleMaxSlots, len(capped))
	}

	// All slots must be inside the rim (distance to center < radius).
	for i, s := range slots {
		dist := math.Sqrt((s.X-p.Center.X)*(s.X-p.Center.X) + (s.Y-p.Center.Y)*(s.Y-p.Center.Y))
		if dist >= p.Radius {
			t.Errorf("slot[%d] distance %.2f is not inside the rim (radius %.2f)", i, dist, p.Radius)
		}
	}

	// Deterministic: same inputs produce same positions.
	s1 := idleHomeSlots(p, th, 3)
	s2 := idleHomeSlots(p, th, 3)
	for i := range s1 {
		if s1[i] != s2[i] {
			t.Errorf("slot[%d] not deterministic: %v vs %v", i, s1[i], s2[i])
		}
	}
}

func TestIdleHomeSlotsNilTownHall(t *testing.T) {
	p := Planet{Center: Vec{X: 160, Y: 120}, Radius: 90}
	if got := idleHomeSlots(p, nil, 3); got != nil {
		t.Error("expected nil slots when Town Hall is nil")
	}
}

func TestInsetPoint(t *testing.T) {
	p := Planet{Center: Vec{X: 160, Y: 120}, Radius: 90}

	// Inset 0 should equal the rim point.
	angle := 0.0
	rim := p.RimPoint(angle)
	ip := insetPoint(p, angle, 0)
	if math.Abs(ip.X-rim.X) > 1e-9 || math.Abs(ip.Y-rim.Y) > 1e-9 {
		t.Errorf("insetPoint offset 0: got %v, want %v", ip, rim)
	}

	// Inset by radius should reach the center.
	center := insetPoint(p, angle, p.Radius)
	if math.Abs(center.X-p.Center.X) > 1e-9 || math.Abs(center.Y-p.Center.Y) > 1e-9 {
		t.Errorf("insetPoint offset=radius: got %v, want center %v", center, p.Center)
	}

	// Inset point must be closer to center than the rim.
	ip2 := insetPoint(p, angle, 5)
	distRim := math.Sqrt((rim.X-p.Center.X)*(rim.X-p.Center.X) + (rim.Y-p.Center.Y)*(rim.Y-p.Center.Y))
	distInset := math.Sqrt((ip2.X-p.Center.X)*(ip2.X-p.Center.X) + (ip2.Y-p.Center.Y)*(ip2.Y-p.Center.Y))
	if distInset >= distRim {
		t.Errorf("insetPoint(5): inset distance %.4f should be less than rim distance %.4f", distInset, distRim)
	}
}
