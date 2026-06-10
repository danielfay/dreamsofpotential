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

// WorkerState is the leg of the delivery loop a worker is currently on.
type WorkerState int

const (
	StateIdleWaiting    WorkerState = iota // waiting in a Town Hall idle spot
	StateSettling                          // newly bought, briefly unavailable
	StateReactionDelay                     // idle worker has noticed work, not yet leaving
	StateDeparturePulse                    // committed to a target before movement starts
	StateToRim                             // moving from idle home to Town Hall rim angle
	StateToForest                          // walking to owned resource node
	StateLoading                           // loading at the node
	StateToBuilding                        // walking to nearest delivery building
	StateUnloading                         // depositing at the delivery building
	StateReturningHome                     // walking along rim to Town Hall
	StateToIdleSpot                        // stepping inward into idle home
)

// PulseState stores a guarded micro-pulse. Rapid activations become steady-lit
// instead of restarting a flickering pulse every frame.
type PulseState struct {
	Remaining     float64
	LastActivated float64
	SteadyUntil   float64
}

// ResourceNode is a single harvestable point on the planet rim.
// OwnerID is the worker ID that has claimed it, or -1 if free.
// ReservedByWorkerID is the worker ID that has spoken for it, or -1 if free.
// Size scales both the visual and the load carried per trip (range ~0.6–1.4).
type ResourceNode struct {
	ID                 int
	Kind               ResourceKind
	Pos                Vec
	Angle              float64
	OwnerID            int
	ReservedByWorkerID int
	Size               float64
	Pulse              PulseState
}

// Worker is a labourer that claims a node and delivers to the nearest camp.
// NodeID is the currently owned/worked node, or -1 if none.
// TargetNodeID is a not-yet-owned node selected during reaction/departure.
// PendingNodeID is a delayed rebalance target reserved until an unload checkpoint.
type Worker struct {
	ID            int
	Pos           Vec
	Angle         float64
	State         WorkerState
	NodeID        int
	TargetNodeID  int
	PendingNodeID int
	DeliveryKind  BuildingKind
	Carried       float64
	Timer         float64
	Pulse         PulseState
}

// BuildingKind identifies the type of a placed building.
type BuildingKind int

const (
	KindTownHall    BuildingKind = iota // 0 — settlement anchor and delivery point
	KindLoggingCamp                     // 1 — resource harvesting camp
)

// Building is a player-placed structure on the planet rim.
// Pos is the rim point at Angle; Kind distinguishes the Town Hall from camps.
type Building struct {
	Kind          BuildingKind
	Pos           Vec
	Angle         float64
	DeliveredWood float64
	DeliveryCount int
	Pulse         PulseState
}

// ResourceField tracks per-kind progress toward spawning a new resource node.
type ResourceField struct {
	Kind           ResourceKind
	CenterAngle    float64
	HalfArc        float64
	EXP            float64
	Cap            float64
	NurtureCharges int // boosted-delivery charges remaining (0 = inactive)
}

type growthOutcome int

const (
	growthOutcomeNone growthOutcome = iota
	growthOutcomeSpawnedNode
	growthOutcomeUpgradedNode
)

type growthResult struct {
	Outcome     growthOutcome
	Kind        ResourceKind
	CenterAngle float64
	HalfArc     float64
	NodeID      int
}

type growthCueState struct {
	Outcome        growthOutcome
	Kind           ResourceKind
	CenterAngle    float64
	HalfArc        float64
	NodeID         int
	GaugeRelease   float64
	GaugeAfterglow float64
	FieldDelay     float64
	FieldPulse     float64
	NodeDelay      float64
	NodeCue        float64
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
const SaveVersion = 7

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
	SimTime            float64

	growthCue      growthCueState
	lastDelivery   deliverySplit
	nurtureBoostCue float64 // seconds remaining on the boosted-delivery flash; transient, not saved
}

type deliverySplit struct{ Gross, Banked, Returned float64 }

// WorkerCost returns the wood cost to buy the next worker.
// The first worker (WorkersBought==0) is free.
func WorkerCost(w *World) float64 {
	if w.Economy.WorkersBought == 0 {
		return 0
	}
	return workerBaseCost * math.Pow(workerCostGrowth, float64(w.Economy.WorkersBought))
}

// CampCost returns the wood cost to place the next logging camp.
// The Town Hall is free and separate from camp cost progression; the first
// logging camp (CampsBought==0) costs campBaseCost.
func CampCost(w *World) float64 {
	return campBaseCost * math.Pow(campCostGrowth, float64(w.Economy.CampsBought))
}

// townHall returns the Town Hall building, or nil if none has been placed.
func townHall(w *World) *Building {
	for _, b := range w.Buildings {
		if b.Kind == KindTownHall {
			return b
		}
	}
	return nil
}

// buyWorker attempts to purchase a worker. The first worker (WorkersBought==0)
// is free. Returns true if the purchase succeeded. New workers spawn at the
// Town Hall idle area.
func buyWorker(w *World) bool {
	th := townHall(w)
	if th == nil {
		return false
	}
	cost := WorkerCost(w)
	if w.Economy.Wood < cost {
		return false
	}
	w.Economy.Wood -= cost
	w.Economy.WorkersBought++
	id := w.NextWorkerID
	w.NextWorkerID++
	w.Workers = append(w.Workers, &Worker{
		ID:            id,
		Pos:           th.Pos,
		Angle:         th.Angle,
		State:         StateSettling,
		NodeID:        -1,
		TargetNodeID:  -1,
		PendingNodeID: -1,
		Timer:         settleDelay,
	})
	return true
}

// newNode allocates a ResourceNode with the next available ID at the given rim angle.
// Size is randomised in [0.6, 1.4] and affects both the visual and yield per trip.
func newNode(w *World, kind ResourceKind, angle float64) *ResourceNode {
	id := w.NextNodeID
	w.NextNodeID++
	return &ResourceNode{
		ID:                 id,
		Kind:               kind,
		Angle:              angle,
		Pos:                w.Planet.RimPoint(angle),
		OwnerID:            -1,
		ReservedByWorkerID: -1,
		Size:               0.6 + rand.Float64()*0.8,
	}
}

// spawnNode places a new node within the field's arc, distributing it among
// existing nodes using a golden-ratio spacing to avoid clustering. If the field
// has no valid free surface left, the nearest same-field node grows instead.
func spawnNode(w *World, f *ResourceField) growthResult {
	result := growthResult{
		Outcome:     growthOutcomeNone,
		Kind:        f.Kind,
		CenterAngle: f.CenterAngle,
		HalfArc:     f.HalfArc,
		NodeID:      -1,
	}
	count := 0
	for _, n := range w.Nodes {
		if n.Kind == f.Kind {
			count++
		}
	}
	const phi = 2.399 // ≈ golden angle in radians
	frac := math.Mod(float64(count)*phi, math.Pi*2) / (math.Pi * 2)
	intended := normAngle(f.CenterAngle - f.HalfArc + 2*f.HalfArc*frac)
	candidate := newNode(w, f.Kind, intended)
	if angle, ok := findValidNodeSpawnAngle(w, f, candidate, intended); ok {
		candidate.Angle = angle
		candidate.Pos = w.Planet.RimPoint(angle)
		w.Nodes = append(w.Nodes, candidate)
		activatePulse(w, &candidate.Pulse)
		result.Outcome = growthOutcomeSpawnedNode
		result.NodeID = candidate.ID
		return result
	}
	if upgraded := upgradeNearestFieldNode(w, f, intended); upgraded != nil {
		result.Outcome = growthOutcomeUpgradedNode
		result.NodeID = upgraded.ID
	}
	return result
}

func findValidNodeSpawnAngle(w *World, f *ResourceField, candidate *ResourceNode, intended float64) (float64, bool) {
	if nodeSpawnAngleValid(w, f, candidate, intended) {
		return intended, true
	}

	step := 2 / w.Planet.Radius
	maxSteps := int(math.Ceil((2*f.HalfArc)/step)) + 1
	for i := 1; i <= maxSteps; i++ {
		offset := float64(i) * step
		for _, sign := range []float64{1, -1} {
			angle := normAngle(intended + sign*offset)
			if nodeSpawnAngleValid(w, f, candidate, angle) {
				return angle, true
			}
		}
	}
	return 0, false
}

func nodeSpawnAngleValid(w *World, f *ResourceField, candidate *ResourceNode, angle float64) bool {
	if !angleWithinField(f, angle) {
		return false
	}
	candidateHalf := nodeBuildingBlockHalfArc(candidate, w.Planet.Radius)
	for _, b := range w.Buildings {
		if anglesOverlap(angle, candidateHalf, b.Angle, buildingHardHalfArc(b.Kind, w.Planet.Radius)) {
			return false
		}
	}

	candidateSoftHalf := nodeSoftHalfArc(candidate, w.Planet.Radius)
	for _, n := range w.Nodes {
		if n.Kind != candidate.Kind || !angleWithinField(f, n.Angle) {
			continue
		}
		if anglesOverlap(angle, candidateSoftHalf, n.Angle, nodeSoftHalfArc(n, w.Planet.Radius)) {
			return false
		}
	}
	return true
}

func upgradeNearestFieldNode(w *World, f *ResourceField, intended float64) *ResourceNode {
	var best *ResourceNode
	bestDist := math.MaxFloat64
	for _, n := range w.Nodes {
		if n.Kind != f.Kind || !angleWithinField(f, n.Angle) {
			continue
		}
		if dist := angularDistance(n.Angle, intended); dist < bestDist {
			bestDist = dist
			best = n
		}
	}
	if best == nil {
		return nil
	}
	best.Size += 0.15
	if best.Size > 2.0 {
		best.Size = 2.0
	}
	activatePulse(w, &best.Pulse)
	return best
}

// fieldForKind returns the first ResourceField with the given kind, or nil.
func fieldForKind(w *World, kind ResourceKind) *ResourceField {
	for _, f := range w.Planet.Fields {
		if f.Kind == kind {
			return f
		}
	}
	return nil
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
		Cap:         fieldBaseEXP,
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
