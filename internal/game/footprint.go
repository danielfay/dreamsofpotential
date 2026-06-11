package game

import "math"

func buildingHardHalfArc(kind BuildingKind, radius float64) float64 {
	if kind == KindTownHall {
		return 10 / radius
	}
	return 6 / radius
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
