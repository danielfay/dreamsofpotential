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

// placementPreview holds all data needed to render the camp placement preview
// and enforce first-camp validity.
type placementPreview struct {
	Angle   float64
	Pos     Vec
	Valid   bool
	Free    []previewRoute  // free nodes within previewArc, with distance
	Claimed []*ResourceNode // claimed nodes within previewArc, muted context
}

// routeDist returns the short-way rim arc distance between two angles on a
// planet of the given radius. Mirrors the computation in sim.go routeLen.
func routeDist(radius, a, b float64) float64 {
	return math.Abs(normAngle(b-a)) * radius
}

// localNodes partitions all world nodes into free and claimed slices based on
// whether they fall within previewArc radians of angle.
func localNodes(w *World, angle float64) (free []previewRoute, claimed []*ResourceNode) {
	for _, n := range w.Nodes {
		if math.Abs(normAngle(n.Angle-angle)) > previewArc {
			continue
		}
		if n.OwnerID == -1 {
			free = append(free, previewRoute{
				Node: n,
				Dist: routeDist(w.Planet.Radius, angle, n.Angle),
			})
		} else {
			claimed = append(claimed, n)
		}
	}
	return
}

// buildPreview assembles a placementPreview for a ghost at the given rim angle.
// Valid is true when: later camps (CampsBought > 0) always; first camp only
// when at least one free local node is within previewArc.
func buildPreview(w *World, angle float64) placementPreview {
	free, claimed := localNodes(w, angle)
	valid := w.Economy.CampsBought > 0 || len(free) > 0
	return placementPreview{
		Angle:   angle,
		Pos:     w.Planet.RimPoint(angle),
		Valid:   valid,
		Free:    free,
		Claimed: claimed,
	}
}
