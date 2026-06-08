package game

import (
	"math"
	"math/rand"
)

// Vec is a 2D position in low-res world space (320×240 virtual pixels).
type Vec struct{ X, Y float64 }

// Dist returns the Euclidean distance between two vectors.
func (a Vec) Dist(b Vec) float64 {
	dx, dy := a.X-b.X, a.Y-b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// ResourceKind identifies a type of harvestable resource.
type ResourceKind int

const (
	KindWood ResourceKind = iota
)

func kindName(k ResourceKind) string {
	if k == KindWood {
		return "wood"
	}
	return "unknown"
}

// WorkerState is the leg of the delivery round-trip a worker is currently on.
type WorkerState int

const (
	StateToForest  WorkerState = iota // walking to owned resource node
	StateLoading                       // loading at the node
	StateToBuilding                    // walking to nearest camp
	StateUnloading                     // depositing at the camp
)

// ResourceNode is a single harvestable point on the planet rim.
// OwnerID is the worker ID that has claimed it, or -1 if free.
// Size scales both the visual and the load carried per trip (range ~0.6–1.4).
type ResourceNode struct {
	ID      int
	Kind    ResourceKind
	Pos     Vec
	Angle   float64
	OwnerID int
	Size    float64
}

// Worker is a labourer that claims a node and delivers to the nearest camp.
// NodeID == -1 means the worker is idle (no node claimed).
type Worker struct {
	ID      int
	Pos     Vec
	Angle   float64
	State   WorkerState
	NodeID  int
	Carried float64
	Timer   float64
}

// Building is a player-placed logging camp on the planet rim.
type Building struct {
	Pos   Vec
	Angle float64
}

// ResourceField tracks per-kind progress toward spawning a new resource node.
type ResourceField struct {
	Kind        ResourceKind
	CenterAngle float64
	HalfArc     float64
	Counter     float64
	Cap         float64
}

// Planet is the disc the player operates on.
type Planet struct {
	Center      Vec
	Radius      float64
	Composition map[ResourceKind]float64
	Fields      []*ResourceField
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

// Economy tracks global resource counts and purchase history.
type Economy struct {
	Wood          float64
	WorkersBought int
	CampsBought   int
}

// SaveVersion is bumped on every backwards-incompatible World JSON change.
// Load discards saves whose Version field doesn't match.
const SaveVersion = 3

// World holds all game state for a single planet.
type World struct {
	Version            int
	Planet             Planet
	Buildings          []*Building
	Nodes              []*ResourceNode
	Workers            []*Worker
	Economy            Economy
	NextNodeID         int
	NextWorkerID       int
	ResourceDiscovered bool // true after the first wood delivery
}

// --- cost helpers ---

const (
	workerBaseCost   = 10.0
	workerCostGrowth = 1.15
	campBaseCost     = 30.0
	campCostGrowth   = 1.50

	// firstCampLocalArc is the angular radius (radians) within which at least
	// one resource node must exist for the free first camp to be placeable.
	// Tuning constant: increase to make the first placement easier to land.
	firstCampLocalArc = 0.6

	// pulseDuration is how long (seconds) the unaffordable-cost flash lasts.
	pulseDuration = 0.4
)

// WorkerCost returns the wood cost to buy the next worker.
// The first worker (WorkersBought==0) is free.
func WorkerCost(w *World) float64 {
	if w.Economy.WorkersBought == 0 {
		return 0
	}
	return workerBaseCost * math.Pow(workerCostGrowth, float64(w.Economy.WorkersBought))
}

// CampCost returns the wood cost to place the next logging camp.
// The first camp (CampsBought==0) is free.
func CampCost(w *World) float64 {
	if w.Economy.CampsBought == 0 {
		return 0
	}
	return campBaseCost * math.Pow(campCostGrowth, float64(w.Economy.CampsBought))
}

// hasLocalNode reports whether any resource node lies within arc radians of
// the given rim angle. Used to validate free first-camp placement.
func hasLocalNode(w *World, angle, arc float64) bool {
	for _, n := range w.Nodes {
		if math.Abs(normAngle(n.Angle-angle)) <= arc {
			return true
		}
	}
	return false
}

// buyWorker attempts to purchase a worker. The first worker (WorkersBought==0)
// is free. Returns true if the purchase succeeded.
func buyWorker(w *World) bool {
	if len(w.Buildings) == 0 {
		return false
	}
	cost := WorkerCost(w)
	if w.Economy.Wood < cost {
		return false
	}
	w.Economy.Wood -= cost
	w.Economy.WorkersBought++
	camp := w.Buildings[0]
	id := w.NextWorkerID
	w.NextWorkerID++
	w.Workers = append(w.Workers, &Worker{
		ID:     id,
		Pos:    camp.Pos,
		Angle:  camp.Angle,
		State:  StateToForest,
		NodeID: -1,
	})
	return true
}

// --- simulation constants ---

const (
	workerSpeed      = 40.0 // world px / second
	loadTime         = 0.5  // seconds to load at the node
	unloadTime       = 0.3  // seconds to unload at the camp
	loadAmount       = 1.0  // resource units carried per trip
	nodeSpawnBaseCap = 20.0      // deliveries needed for the first new node
	nodeCapGrowth    = 1.5      // cap multiplier each time a node spawns
	forestHalfArc    = math.Pi  // full surface coverage for wood (100% composition)
	startingNodes    = 5
)

// newNode allocates a ResourceNode with the next available ID at the given rim angle.
// Size is randomised in [0.6, 1.4] and affects both the visual and yield per trip.
func newNode(w *World, kind ResourceKind, angle float64) *ResourceNode {
	id := w.NextNodeID
	w.NextNodeID++
	return &ResourceNode{
		ID:      id,
		Kind:    kind,
		Angle:   angle,
		Pos:     w.Planet.RimPoint(angle),
		OwnerID: -1,
		Size:    0.6 + rand.Float64()*0.8,
	}
}

// spawnNode places a new node within the field's arc, distributing it among
// existing nodes using a golden-ratio spacing to avoid clustering.
func spawnNode(w *World, f *ResourceField) {
	count := 0
	for _, n := range w.Nodes {
		if n.Kind == f.Kind {
			count++
		}
	}
	const phi = 2.399 // ≈ golden angle in radians
	frac := math.Mod(float64(count)*phi, math.Pi*2) / (math.Pi * 2)
	angle := normAngle(f.CenterAngle - f.HalfArc + 2*f.HalfArc*frac)
	w.Nodes = append(w.Nodes, newNode(w, f.Kind, angle))
}

// NewWorld returns a freshly initialised world ready to start the game.
func NewWorld() *World {
	center := Vec{X: 160, Y: 120}
	radius := 90.0
	forestAngle := -math.Pi / 2 // top of the rim

	field := &ResourceField{
		Kind:        KindWood,
		CenterAngle: forestAngle,
		HalfArc:     forestHalfArc,
		Cap:         nodeSpawnBaseCap,
	}

	w := &World{
		Version: SaveVersion,
		Planet: Planet{
			Center:      center,
			Radius:      radius,
			Composition: map[ResourceKind]float64{KindWood: 1.0},
			Fields:      []*ResourceField{field},
		},
		Economy: Economy{Wood: 0},
	}

	// Seed starting nodes at random positions within the field arc.
	for i := 0; i < startingNodes; i++ {
		angle := normAngle(field.CenterAngle - field.HalfArc + 2*field.HalfArc*rand.Float64())
		w.Nodes = append(w.Nodes, newNode(w, KindWood, angle))
	}

	return w
}
