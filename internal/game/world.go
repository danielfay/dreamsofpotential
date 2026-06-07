package game

import "math"

// Vec is a 2D position in low-res world space (320×240 virtual pixels).
type Vec struct{ X, Y float64 }

// Dist returns the Euclidean distance between two vectors.
func (a Vec) Dist(b Vec) float64 {
	dx, dy := a.X-b.X, a.Y-b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// WorkerState represents which leg of the round-trip a worker is on.
type WorkerState int

const (
	StateToForest   WorkerState = iota // walking from camp to forest
	StateLoading                       // loading wood at the forest
	StateToBuilding                    // walking from forest back to camp
	StateUnloading                     // depositing wood at the camp
)

// Worker is a single labourer attached to a Building.
// Angle is the authoritative position on the rim; Pos is derived from it each tick.
type Worker struct {
	Pos     Vec
	Angle   float64 // radians around planet center, position on the rim
	State   WorkerState
	Carried float64  // wood units currently held
	Timer   float64  // seconds remaining in Loading/Unloading dwell
	Home    *Building
}

// Building is a player-placed logging camp on the planet rim.
// Angle is the authoritative position; Pos must always equal planet.RimPoint(Angle).
type Building struct {
	Pos     Vec
	Angle   float64 // radians around planet center, position on the rim
	Workers []*Worker
}

// Forest is the resource deposit the workers harvest from.
// Reserves are treated as infinite for the first slice.
type Forest struct {
	Pos Vec
}

// Planet is the disc the player operates on.
type Planet struct {
	Center Vec
	Radius float64
}

// RimPoint returns the world point on the planet's rim at the given angle.
func (p Planet) RimPoint(theta float64) Vec {
	return Vec{
		X: p.Center.X + p.Radius*math.Cos(theta),
		Y: p.Center.Y + p.Radius*math.Sin(theta),
	}
}

// AngleOf returns the angle (about the planet center) of an arbitrary world point.
func (p Planet) AngleOf(v Vec) float64 {
	return math.Atan2(v.Y-p.Center.Y, v.X-p.Center.X)
}

// normAngle normalizes an angle to the range (-π, π].
func normAngle(a float64) float64 {
	return math.Atan2(math.Sin(a), math.Cos(a))
}

// Economy tracks global resource counts and purchase history (for escalating costs).
type Economy struct {
	Wood          float64
	WorkersBought int
	CampsBought   int
}

// World holds all game state for a single planet.
// Future: World will hold []*Planet when multi-planet is introduced.
type World struct {
	Planet    Planet
	Forest    Forest
	Buildings []*Building
	Economy   Economy
}

// --- cost helpers ---

const (
	workerBaseCost   = 10.0
	workerCostGrowth = 1.15
	campBaseCost     = 30.0
	campCostGrowth   = 1.50
)

// WorkerCost returns the wood cost to buy the next worker.
func WorkerCost(w *World) float64 {
	return workerBaseCost * math.Pow(workerCostGrowth, float64(w.Economy.WorkersBought))
}

// CampCost returns the wood cost to place the next logging camp.
func CampCost(w *World) float64 {
	return campBaseCost * math.Pow(campCostGrowth, float64(w.Economy.CampsBought))
}

// --- rate helper ---

const (
	workerSpeed = 40.0 // world px / second
	loadTime    = 0.5  // seconds to load at the forest
	unloadTime  = 0.3  // seconds to unload at the camp
	loadAmount  = 1.0  // wood units carried per trip
)

// EstimateRate returns the analytic wood/sec based on current workers and
// their camps' arc distances from the forest along the rim.
func EstimateRate(w *World) float64 {
	forestAngle := w.Planet.AngleOf(w.Forest.Pos)
	var rate float64
	for _, b := range w.Buildings {
		delta := normAngle(b.Angle - forestAngle)
		dist := math.Abs(delta) * w.Planet.Radius
		tripTime := loadTime + unloadTime + 2*dist/workerSpeed
		rate += float64(len(b.Workers)) * (loadAmount / tripTime)
	}
	return rate
}

// --- world constructor ---

// NewWorld returns a freshly initialised world ready to start the game.
// The planet is centered in the 320×240 virtual canvas. The forest sits
// on the top of the rim. The player starts with 50 wood and no camps.
func NewWorld() *World {
	center := Vec{X: 160, Y: 120}
	radius := 90.0
	return &World{
		Planet: Planet{
			Center: center,
			Radius: radius,
		},
		Forest: Forest{
			// Forest sits on the rim directly above the center (angle = -π/2).
			Pos: Vec{X: center.X, Y: center.Y - radius},
		},
		Economy: Economy{
			Wood: 50,
		},
	}
}
