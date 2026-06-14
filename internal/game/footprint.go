package game

import "math"

func buildingHardHalfArc(kind BuildingKind, radius float64) float64 {
	if kind == KindTownHall {
		return 10 / radius
	}
	return 6 / radius
}

// dockCoversAngle reports whether any placed dock building's footprint covers angle.
// Used by arcCost to skip the lake-speed penalty on docked water-rim segments.
func dockCoversAngle(w *World, angle float64) bool {
	half := buildingHardHalfArc(KindDock, w.Planet.Radius)
	for _, b := range w.Buildings {
		if b.Kind == KindDock && math.Abs(normAngle(b.Angle-angle)) <= half {
			return true
		}
	}
	return false
}

func nodeBuildingBlockHalfArc(node *ResourceNode, radius float64) float64 {
	return (4*node.Size + 2) / radius
}

func nodeSoftHalfArc(node *ResourceNode, radius float64) float64 {
	return (2.5 * node.Size) / radius
}

func anglesOverlap(a, halfA, b, halfB float64) bool {
	return math.Abs(normAngle(a-b)) <= halfA+halfB
}

func angleWithinField(field *ResourceField, angle float64) bool {
	return math.Abs(normAngle(angle-field.CenterAngle)) <= field.HalfArc
}

func angularDistance(a, b float64) float64 {
	return math.Abs(normAngle(a - b))
}
