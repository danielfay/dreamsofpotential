package game

import "image/color"

// resourceFamily ties together every per-family scalar/kind/amount so callers
// iterate families instead of hand-enumerating wood vs water.
type resourceFamily struct {
	Resource                ResourceKind
	Potential               PotentialKind
	CirclePacket            float64
	Stockpile               func(*Economy) *float64
	LocalStockpile          func(*PlanetState) *float64
	AbstractRate            func(*SystemPlanet) *float64
	ProjectedRate           func(*SystemPlanet) *float64
	SystemRate              func(*SystemEconomy) *float64
	Research                func(*SystemEconomy) *float64
	AllocPotential          func(*SystemEconomy) *float64
	Estimate                func(*World) float64
	RateLabelColor          color.RGBA
	ProjectedRateLabelColor color.RGBA
}

var resourceFamilies = []resourceFamily{
	{
		Resource:     KindWood,
		Potential:    PotentialForest,
		CirclePacket: circlePacketWood,
		Stockpile:    func(e *Economy) *float64 { return &e.Wood },
		LocalStockpile: func(ps *PlanetState) *float64 {
			return &ps.LocalWood
		},
		AbstractRate:            func(p *SystemPlanet) *float64 { return &p.AbstractRate },
		ProjectedRate:           func(p *SystemPlanet) *float64 { return &p.ProjectedRate },
		SystemRate:              func(se *SystemEconomy) *float64 { return &se.WoodRate },
		Research:                func(se *SystemEconomy) *float64 { return &se.WoodResearch },
		AllocPotential:          func(se *SystemEconomy) *float64 { return &se.WoodAllocPotential },
		Estimate:                EstimateRate,
		RateLabelColor:          colWoodLabel,
		ProjectedRateLabelColor: color.RGBA{R: colWoodLabel.R, G: colWoodLabel.G, B: colWoodLabel.B, A: colWoodLabel.A / 2},
	},
	{
		Resource:     KindWater,
		Potential:    PotentialWater,
		CirclePacket: circlePacketWater,
		Stockpile:    func(e *Economy) *float64 { return &e.Water },
		LocalStockpile: func(ps *PlanetState) *float64 {
			return &ps.LocalWater
		},
		AbstractRate:            func(p *SystemPlanet) *float64 { return &p.AbstractWaterRate },
		ProjectedRate:           func(p *SystemPlanet) *float64 { return &p.ProjectedWaterRate },
		SystemRate:              func(se *SystemEconomy) *float64 { return &se.WaterRate },
		Research:                func(se *SystemEconomy) *float64 { return &se.WaterResearch },
		AllocPotential:          func(se *SystemEconomy) *float64 { return &se.WaterAllocPotential },
		Estimate:                EstimateWaterRate,
		RateLabelColor:          color.RGBA{R: 100, G: 200, B: 255, A: 220},
		ProjectedRateLabelColor: color.RGBA{R: 100, G: 200, B: 255, A: 110},
	},
}

func familyForResource(kind ResourceKind) *resourceFamily {
	for i := range resourceFamilies {
		if resourceFamilies[i].Resource == kind {
			return &resourceFamilies[i]
		}
	}
	return nil
}

func familyForPotential(kind PotentialKind) *resourceFamily {
	for i := range resourceFamilies {
		if resourceFamilies[i].Potential == kind {
			return &resourceFamilies[i]
		}
	}
	return nil
}

type workerFamily struct {
	Resource    ResourceKind
	inLoop      func(*Worker) bool
	hasFreeWork func(*World) bool
}

var workerFamilies = []workerFamily{
	{
		Resource:    KindWood,
		inLoop:      workerInWoodAssignment,
		hasFreeWork: func(w *World) bool { return bestFreeNodeForKind(w, KindWood) != nil },
	},
	{
		Resource:    KindWater,
		inLoop:      workerInWaterLoop,
		hasFreeWork: func(w *World) bool { return bestFreeDock(w) != nil },
	},
}

func workerFamilyForResource(kind ResourceKind) *workerFamily {
	for i := range workerFamilies {
		if workerFamilies[i].Resource == kind {
			return &workerFamilies[i]
		}
	}
	return nil
}
