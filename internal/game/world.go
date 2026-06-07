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
type Worker struct {
	Pos     Vec
	State   WorkerState
	Carried float64  // wood units currently held
	Timer   float64  // seconds remaining in Loading/Unloading dwell
	Home    *Building
}

// Building is a player-placed logging camp.
type Building struct {
	Pos     Vec
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
	workerBaseCost  = 10.0
	workerCostGrowth = 1.15
	campBaseCost    = 30.0
	campCostGrowth  = 1.50
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
	workerSpeed  = 40.0 // world px / second
	loadTime     = 0.5  // seconds to load at the forest
	unloadTime   = 0.3  // seconds to unload at the camp
	loadAmount   = 1.0  // wood units carried per trip
)

// EstimateRate returns the analytic wood/sec based on current workers and
// their camps' distances from the forest. This is stable (no EMA jitter).
func EstimateRate(w *World) float64 {
	var rate float64
	for _, b := range w.Buildings {
		dist := b.Pos.Dist(w.Forest.Pos)
		tripTime := loadTime + unloadTime + 2*dist/workerSpeed
		rate += float64(len(b.Workers)) * (loadAmount / tripTime)
	}
	return rate
}

// --- world constructor ---

// NewWorld returns a freshly initialised world ready to start the game.
// The planet is centered in the 320×240 virtual canvas. The forest sits
// near the top of the disc. The player starts with 50 wood and no camps —
// so their first action is to build a logging camp.
func NewWorld() *World {
	center := Vec{X: 160, Y: 120}
	return &World{
		Planet: Planet{
			Center: center,
			Radius: 90,
		},
		Forest: Forest{
			// Forest sits at the top of the disc, roughly 60px above center.
			Pos: Vec{X: 160, Y: 60},
		},
		Economy: Economy{
			Wood: 50,
		},
	}
}
