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
// and enforce validity. Kind indicates whether the ghost is a Town Hall, logging
// camp, or dock, which controls ghost art and validity rules.
// Extension is true when the dock variant is a water-extension (not a shore dock).
type placementPreview struct {
	Kind             BuildingKind
	Hidden           bool
	Extension        bool // dock only: extension dock connected to an existing dock
	Angle            float64
	Pos              Vec
	Valid            bool
	Affordable       bool
	Free             []previewRoute  // capped free nodes within previewArc
	FreeTotal        int             // all free nodes within previewArc
	Claimed          []*ResourceNode // capped claimed nodes within previewArc
	ClaimedTotal     int             // all claimed nodes within previewArc
	Reserved         []*ResourceNode // capped reserved nodes within previewArc
	ReservedTotal    int             // all reserved nodes within previewArc
	Blocked          []*ResourceNode // nodes whose physical footprint blocks placement
	BlockedBuildings []*Building     // buildings whose physical footprint blocks placement
	Reject           float64         // transient invalid-confirm feedback intensity [0,1]
	Locked           bool            // true while waiting for second click to confirm a destructive placement
}

// routeDist returns the effective arc distance between two angles.
// Mirrors routeLen in sim.go — lake arcs cost 1/lakeSpeedFactor more.
func routeDist(w *World, a, b float64) float64 {
	return effectiveArc(w, a, b)
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
				Dist: routeDist(w, angle, n.Angle),
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
	if placementSuppressedByUnknownField(w, angle) {
		return placementPreview{
			Hidden: true,
			Angle:  angle,
			Pos:    w.Planet.RimPoint(angle),
		}
	}

	// Town Hall placement.
	if !hasTownHall {
		blocked := placementBlockedNodes(w, KindTownHall, angle)
		blockedBuildings := placementBlockedBuildings(w, KindTownHall, angle)
		return placementPreview{
			Kind:             KindTownHall,
			Angle:            angle,
			Pos:              w.Planet.RimPoint(angle),
			Valid:            len(blocked) == 0 && len(blockedBuildings) == 0 && !inLake(w, angle),
			Affordable:       true,
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

	// After Town Hall: contextual placement — dock on water edge, camp on land.
	if knownLakeAtAngle(w, angle) {
		return dockPreview(w, angle, freePlacement, free, freeTotal, claimed, claimedTotal, reserved, reservedTotal)
	}

	// Land / forest → logging camp. Tree nodes in the footprint are highlighted
	// red and cleared on placement; only building collisions block validity.
	affordable := freePlacement || w.Economy.Wood >= CampCost(w)
	blocked := placementBlockedNodes(w, KindLoggingCamp, angle)
	blockedBuildings := placementBlockedBuildings(w, KindLoggingCamp, angle)
	valid := affordable && len(blockedBuildings) == 0
	return placementPreview{
		Kind:             KindLoggingCamp,
		Angle:            angle,
		Pos:              w.Planet.RimPoint(angle),
		Valid:            valid,
		Affordable:       affordable,
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

func placementSuppressedByUnknownField(w *World, angle float64) bool {
	for _, f := range w.Planet.Fields {
		if f.Known || f.Kind == KindWaterInfluence {
			continue
		}
		if angleWithinField(f, angle) {
			return true
		}
	}
	return false
}

func knownLakeAtAngle(w *World, angle float64) bool {
	for _, f := range w.Planet.Fields {
		if f.Kind == KindWater && f.Known && angleWithinField(f, angle) {
			return true
		}
	}
	return false
}

// dockPreview builds a placement preview for an angle that is in a water field.
// Shore docks (footprint touches non-water) cost Wood only; open-water docks
// cost Wood + Water. Any position in a water field is valid.
func dockPreview(w *World, angle float64, freePlacement bool,
	free []previewRoute, freeTotal int,
	claimed []*ResourceNode, claimedTotal int,
	reserved []*ResourceNode, reservedTotal int,
) placementPreview {
	half := buildingHardHalfArc(KindDock, w.Planet.Radius)
	// Docks are terrain — node footprints never block them.
	blockedBuildings := placementBlockedBuildings(w, KindDock, angle)

	extension := false
	var affordable bool

	extension = !isShore(w, angle, half)
	affordable = freePlacement || (w.Economy.Wood >= dockExtWoodCost && w.Economy.Water >= dockExtWaterCost)

	valid := affordable && len(blockedBuildings) == 0
	return placementPreview{
		Kind:             KindDock,
		Extension:        extension,
		Angle:            angle,
		Pos:              w.Planet.RimPoint(angle),
		Valid:            valid,
		Affordable:       affordable,
		Free:             free,
		FreeTotal:        freeTotal,
		Claimed:          claimed,
		ClaimedTotal:     claimedTotal,
		Reserved:         reserved,
		ReservedTotal:    reservedTotal,
		BlockedBuildings: blockedBuildings,
	}
}

// isShore reports whether angle's dock footprint [angle-half, angle+half] straddles
// the water/land boundary: the angle itself is in water but at least one sampled
// point in the footprint is outside any water field.
func isShore(w *World, angle, halfArc float64) bool {
	const steps = 8
	for i := 0; i <= steps; i++ {
		a := normAngle(angle - halfArc + float64(i)*2*halfArc/float64(steps))
		if !inLake(w, a) {
			return true
		}
	}
	return false
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
