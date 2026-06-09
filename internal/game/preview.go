package game

import (
	"math"
	"sort"
)

const (
	// previewArc is the angular half-radius (radians) of the placement preview
	// range. The full span is 2*previewArc ≈ one-quarter of the planet
	// circumference (π/4 * 2 = π/2). Used for both node selection and
	// first-camp validity.
	previewArc = math.Pi / 4

	// rimSnapBand is the max world-px distance from the rim ring within which
	// the placement preview is shown. Beyond this, no ghost is rendered.
	rimSnapBand = 30.0

	previewFreeRouteCap        = 5
	previewUnavailableRouteCap = 3
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
	Kind             BuildingKind
	Angle            float64
	Pos              Vec
	Valid            bool
	Affordable       bool
	NeedsLocalTree   bool
	MissingLocalTree bool
	Free             []previewRoute  // capped free nodes within previewArc
	FreeTotal        int             // all free nodes within previewArc
	Claimed          []*ResourceNode // capped claimed nodes within previewArc
	ClaimedTotal     int             // all claimed nodes within previewArc
	Reserved         []*ResourceNode // capped reserved nodes within previewArc
	ReservedTotal    int             // all reserved nodes within previewArc
	Blocked          []*ResourceNode // nodes whose physical footprint blocks placement
	BlockedBuildings []*Building     // buildings whose physical footprint blocks placement
	Reject           float64         // transient invalid-confirm feedback intensity [0,1]
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

func capPreviewRoutes(angle float64, free []previewRoute, claimed, reserved []*ResourceNode) ([]previewRoute, []*ResourceNode, []*ResourceNode) {
	sort.Slice(free, func(i, j int) bool {
		return free[i].Dist < free[j].Dist
	})
	if len(free) > previewFreeRouteCap {
		free = free[:previewFreeRouteCap]
	}

	unavailable := make([]*ResourceNode, 0, len(claimed)+len(reserved))
	unavailable = append(unavailable, claimed...)
	unavailable = append(unavailable, reserved...)
	sort.Slice(unavailable, func(i, j int) bool {
		if unavailable[i].Angle == unavailable[j].Angle {
			return unavailable[i].ID < unavailable[j].ID
		}
		return angularDistance(unavailable[i].Angle, angle) < angularDistance(unavailable[j].Angle, angle)
	})
	if len(unavailable) > previewUnavailableRouteCap {
		unavailable = unavailable[:previewUnavailableRouteCap]
	}

	cappedClaimed := make([]*ResourceNode, 0, len(unavailable))
	cappedReserved := make([]*ResourceNode, 0, len(unavailable))
	for _, n := range unavailable {
		if n.ReservedByWorkerID != -1 && n.OwnerID == -1 {
			cappedReserved = append(cappedReserved, n)
		} else {
			cappedClaimed = append(cappedClaimed, n)
		}
	}
	return free, cappedClaimed, cappedReserved
}

// buildPreview assembles a placementPreview for a ghost at the given rim angle.
// The first placement (no buildings yet) is a Town Hall and requires at least
// one free local node. Subsequent placements are logging camps and are valid
// regardless of node proximity, but only while affordable.
func buildPreview(w *World, angle float64) placementPreview {
	return buildPreviewWithFreePlacement(w, angle, false)
}

func buildPreviewWithFreePlacement(w *World, angle float64, freePlacement bool) placementPreview {
	free, claimed, reserved := localNodes(w, angle)
	freeTotal := len(free)
	claimedTotal := len(claimed)
	reservedTotal := len(reserved)
	free, claimed, reserved = capPreviewRoutes(angle, free, claimed, reserved)
	hasTownHall := len(w.Buildings) > 0
	affordable := true
	if hasTownHall && !freePlacement {
		affordable = w.Economy.Wood >= CampCost(w)
	}
	kind := KindTownHall
	if hasTownHall {
		kind = KindLoggingCamp
	}
	blocked := placementBlockedNodes(w, kind, angle)
	blockedBuildings := placementBlockedBuildings(w, kind, angle)
	needsLocalTree := !hasTownHall
	missingLocalTree := needsLocalTree && freeTotal == 0
	valid := !missingLocalTree && affordable && len(blocked) == 0 && len(blockedBuildings) == 0
	return placementPreview{
		Kind:             kind,
		Angle:            angle,
		Pos:              w.Planet.RimPoint(angle),
		Valid:            valid,
		Affordable:       affordable,
		NeedsLocalTree:   needsLocalTree,
		MissingLocalTree: missingLocalTree,
		Free:             free,
		FreeTotal:        freeTotal,
		Claimed:          claimed,
		ClaimedTotal:     claimedTotal,
		Reserved:         reserved,
		ReservedTotal:    reservedTotal,
		Blocked:          blocked,
		BlockedBuildings: blockedBuildings,
	}
}

func placementBlockedNodes(w *World, kind BuildingKind, angle float64) []*ResourceNode {
	var blocked []*ResourceNode
	buildingHalfArc := buildingHardHalfArc(kind, w.Planet.Radius)
	for _, n := range w.Nodes {
		if anglesOverlap(angle, buildingHalfArc, n.Angle, nodeBuildingBlockHalfArc(n, w.Planet.Radius)) {
			blocked = append(blocked, n)
		}
	}
	return blocked
}

func placementBlockedBuildings(w *World, kind BuildingKind, angle float64) []*Building {
	var blocked []*Building
	buildingHalfArc := buildingHardHalfArc(kind, w.Planet.Radius)
	for _, b := range w.Buildings {
		if anglesOverlap(angle, buildingHalfArc, b.Angle, buildingHardHalfArc(b.Kind, w.Planet.Radius)) {
			blocked = append(blocked, b)
		}
	}
	return blocked
}

func zeroValidPlacementPositions(w *World) bool {
	const steps = 180
	for i := 0; i < steps; i++ {
		angle := -math.Pi + float64(i)*2*math.Pi/steps
		if buildPreviewWithFreePlacement(w, angle, true).Valid {
			return false
		}
	}
	return true
}
