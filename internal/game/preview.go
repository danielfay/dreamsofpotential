package game

import "math"

const (
	// previewArc is the angular half-radius (radians) of the placement preview
	// range. The full span is 2*previewArc ≈ one-quarter of the planet
	// circumference (π/4 * 2 = π/2). Used for both node selection and
	// first-camp validity.
	previewArc = math.Pi / 4

	// rimSnapBand is the max world-px distance from the rim ring within which
	// the placement preview is shown. Beyond this, no ghost is rendered.
	rimSnapBand = 30.0
)

// previewRoute pairs a free node with its rim-path distance to the ghost angle.
type previewRoute struct {
	Node *ResourceNode
	Dist float64 // arc length in world px
}

// placementPreview holds all data needed to render the building placement preview
// and enforce validity. Kind indicates whether the ghost is a Town Hall or a
// logging camp, which controls ghost art and validity rules.
type placementPreview struct {
	Kind       BuildingKind
	Angle      float64
	Pos        Vec
	Valid      bool
	Affordable bool
	Free       []previewRoute  // free nodes within previewArc, with distance
	Claimed    []*ResourceNode // claimed nodes within previewArc, muted context
	Reserved   []*ResourceNode // reserved nodes within previewArc, debug context
}

// routeDist returns the short-way rim arc distance between two angles on a
// planet of the given radius. Mirrors the computation in sim.go routeLen.
func routeDist(radius, a, b float64) float64 {
	return math.Abs(normAngle(b-a)) * radius
}

// localNodes partitions all world nodes into free and claimed slices based on
// whether they fall within previewArc radians of angle.
func localNodes(w *World, angle float64) (free []previewRoute, claimed, reserved []*ResourceNode) {
	for _, n := range w.Nodes {
		if math.Abs(normAngle(n.Angle-angle)) > previewArc {
			continue
		}
		if n.OwnerID == -1 && n.ReservedByWorkerID == -1 {
			free = append(free, previewRoute{
				Node: n,
				Dist: routeDist(w.Planet.Radius, angle, n.Angle),
			})
		} else if n.ReservedByWorkerID != -1 && n.OwnerID == -1 {
			reserved = append(reserved, n)
		} else {
			claimed = append(claimed, n)
		}
	}
	return
}

// buildPreview assembles a placementPreview for a ghost at the given rim angle.
// The first placement (no buildings yet) is a Town Hall and requires at least
// one free local node. Subsequent placements are logging camps and are valid
// regardless of node proximity, but only while affordable.
func buildPreview(w *World, angle float64) placementPreview {
	free, claimed, reserved := localNodes(w, angle)
	hasTownHall := len(w.Buildings) > 0
	affordable := true
	if hasTownHall {
		affordable = w.Economy.Wood >= CampCost(w)
	}
	valid := (hasTownHall || len(free) > 0) && affordable
	kind := KindTownHall
	if hasTownHall {
		kind = KindLoggingCamp
	}
	return placementPreview{
		Kind:       kind,
		Angle:      angle,
		Pos:        w.Planet.RimPoint(angle),
		Valid:      valid,
		Affordable: affordable,
		Free:       free,
		Claimed:    claimed,
		Reserved:   reserved,
	}
}
