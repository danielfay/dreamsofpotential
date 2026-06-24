package game

// newEchoPlanetState returns a freshly initialised durable live state for an
// awakened echo planet. The planet has field geometry but no settlement or
// harvestable nodes; founding with a Town Hall spawns its starting nodes.
// layoutID 0 = Lakewood (blue-rim planet), layoutID 1 = Tight Grove (yellow-rim planet).
func newEchoPlanetState(layoutID int) *PlanetState {
	center := Vec{X: 160, Y: 120}
	switch layoutID {
	case 0:
		return newLakewoodState(center)
	default:
		return newTightGroveState(center)
	}
}

// newTightGroveState builds echoB: a compact full-forest planet where TH
// placement immediately spawns many trees, leaving only 1-2 valid camp spots.
func newTightGroveState(center Vec) *PlanetState {
	return &PlanetState{
		Planet: Planet{
			Center:      center,
			Radius:      tightGroveRadius,
			Composition: map[ResourceKind]float64{KindWood: 1.0},
			Fields: []*ResourceField{{
				Kind:        KindWood,
				CenterAngle: 0,
				HalfArc:     forestHalfArc, // full circle
				Known:       true,
			}},
			FieldProgress: map[ResourceKind]*KindProgress{
				KindWood: {Cap: woodFieldBaseEXP},
			},
			StartingNodes: tightGroveStartNodes,
		},
		TownGrowthCap: townGrowthBaseCap,
	}
}

// newLakewoodState builds echoA: a forest split by a lake arc so workers
// naturally avoid the island region until a local camp is built there.
// Completion awards both Forest and Water Potential.
func newLakewoodState(center Vec) *PlanetState {
	mainForest := &ResourceField{
		Kind:        KindWood,
		CenterAngle: lakewoodMainForestAngle,
		HalfArc:     lakewoodMainForestArc,
		Known:       true,
	}
	islandForest := &ResourceField{
		Kind:        KindWood,
		CenterAngle: lakewoodIslandForestAngle,
		HalfArc:     lakewoodIslandForestArc,
		Known:       true,
	}
	largeLake := &ResourceField{
		Kind:        KindWater,
		CenterAngle: lakewoodLargeLakeAngle,
		HalfArc:     lakewoodLargeLakeArc,
		Known:       false,
	}
	smallLake := &ResourceField{
		Kind:        KindWater,
		CenterAngle: lakewoodSmallLakeAngle,
		HalfArc:     lakewoodSmallLakeArc,
		Known:       false,
	}
	// Invisible influence fields — co-centered with each lake but wider by
	// waterInfluenceArcPadding so they reach into neighboring forest. Overlap-
	// allowed; unknown (so they project influence before discovery).
	largeLakeInfluence := &ResourceField{
		Kind:        KindWaterInfluence,
		CenterAngle: lakewoodLargeLakeAngle,
		HalfArc:     lakewoodLargeLakeArc + waterInfluenceArcPadding,
		Known:       false,
	}
	smallLakeInfluence := &ResourceField{
		Kind:        KindWaterInfluence,
		CenterAngle: lakewoodSmallLakeAngle,
		HalfArc:     lakewoodSmallLakeArc + waterInfluenceArcPadding,
		Known:       false,
	}
	return &PlanetState{
		Planet: Planet{
			Center:      center,
			Radius:      lakewoodRadius,
			Composition: map[ResourceKind]float64{KindWood: 1.0},
			Fields:      []*ResourceField{mainForest, islandForest, largeLake, smallLake, largeLakeInfluence, smallLakeInfluence},
			FieldProgress: map[ResourceKind]*KindProgress{
				KindWood: {Cap: woodFieldBaseEXP},
			},
		},
		TownGrowthCap: townGrowthBaseCap,
	}
}

// newWaterFrontierState returns the authored live state for the awakened water
// frontier planet: a tiny forest shore (90°) and a dominant water field (270°).
// The water field is Known so sparkle plumbing can target it immediately;
// StartingNodes is kept small so founding fills only the shore.
func newWaterFrontierState() *PlanetState {
	center := Vec{X: 160, Y: 120}
	return &PlanetState{
		Planet: Planet{
			Center:      center,
			Radius:      waterFrontierRadius,
			Composition: map[ResourceKind]float64{KindWood: 0.4, KindWater: 0.6},
			Fields: []*ResourceField{
				{
					Kind:        KindWood,
					CenterAngle: waterFrontierShoreAngle,
					HalfArc:     waterFrontierShoreArc,
					Known:       true,
				},
				{
					Kind:        KindWater,
					CenterAngle: waterFrontierLakeAngle,
					HalfArc:     waterFrontierLakeArc,
					Known:       true,
				},
			},
			FieldProgress: map[ResourceKind]*KindProgress{
				KindWood:  {Cap: woodFieldBaseEXP},
				KindWater: {Cap: waterFieldBaseEXP},
			},
			StartingNodes: waterFrontierStartNodes,
		},
		TownGrowthCap: townGrowthBaseCap,
	}
}
