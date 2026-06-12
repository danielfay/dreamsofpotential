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

// ViewMode distinguishes the system-level view from the planet-level view.
type ViewMode int

const (
	ViewPlanet ViewMode = iota // player is operating on the starting planet
	ViewSystem                  // player is looking at the planetary system
)

// PlanetKind categorises a system-view planet record.
type PlanetKind int

const (
	PlanetStarting PlanetKind = iota // the live-simulated starting planet
	PlanetEcho                        // an awakened forest producer (abstract only for now)
	PlanetUnknown                     // distant locked frontier silhouette
)

// SystemPlanet is a persistent record for one planet in the system view.
// AbstractRate is the wood/sec this planet contributes to the global bank;
// set at first completion for the starting planet, fixed at boot for echoes.
// Seed is reserved for future world generation. Pos/Radius are in 320×240 virtual space.
type SystemPlanet struct {
	Kind         PlanetKind
	Pos          Vec
	Radius       float64
	AbstractRate float64
	RingColorIdx int   // 0 or 1 — slight visual variation between the two echoes
	Seed         int64 // future world-generation hook
	Awakened     bool  // echo has been awoken (has a durable live PlanetState)
	Completed    bool  // echo reached its completion gate
	LayoutID     int   // which authored echo layout (0 or 1)
}

// zoomable reports whether this planet has a live sim the player can zoom into.
func (p SystemPlanet) zoomable() bool {
	return p.Kind == PlanetStarting || (p.Kind == PlanetEcho && p.Awakened)
}

// PlanetState is one planet's durable live sim state when it is NOT the active
// (viewed) planet. The active planet's state lives in the top-level World fields.
type PlanetState struct {
	Planet             Planet
	Buildings          []*Building
	Nodes              []*ResourceNode
	Workers            []*Worker
	NextNodeID         int
	NextWorkerID       int
	ResourceDiscovered bool
	SimTime            float64
	WorkerCapacity     int
	CapacityBought     int
	CampsBought        int
	TownGrowth         float64
	TownGrowthCap      float64
	Founded            bool
}

// System holds all persistent state for the planetary system layer.
type System struct {
	Unlocked bool           // true once the first reveal has completed
	View     ViewMode       // current view mode; persisted across save/load
	Selected int            // index into Planets; -1 = none selected
	Planets  []SystemPlanet // always 4 entries: starting, echo A, echo B, unknown
}

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
	Wood           float64
	WorkerCapacity int // total worker slots unlocked (founding slot + paid capacity)
	CapacityBought int // paid capacity purchases; drives the cost curve
	CampsBought    int
	TownGrowth     float64 // accumulates gross delivery amount; clamped at TownGrowthCap
	TownGrowthCap  float64 // spawns a worker when TownGrowth reaches this; grows each arrival
}

// SaveVersion is bumped on every backwards-incompatible World JSON change.
// Load discards saves whose Version field doesn't match.
const SaveVersion = 10

// World holds all game state for a single planet plus the system layer.
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
	System             System // system-view unlock state; persisted

	// Multi-planet support: PlanetStates holds parked live state for non-active
	// planets, index-aligned to System.Planets. The entry for Active is always nil
	// because that planet's live state lives in the top-level fields above.
	// Uninstantiated planets (unawakened echoes, unknown) are also nil.
	PlanetStates []*PlanetState
	Active       int // System.Planets index currently loaded into top-level live fields

	growthCue         growthCueState
	pendingGrowthCues []growthCueState
	lastDelivery      deliverySplit
	abstractRateWin   abstractRateWindow
}

type abstractRateWindow struct {
	buckets []float64 // running min per bucket; len == abstractRateBuckets
	idx     int       // current bucket index
	filled  int       // how many buckets have been written at least once
	elapsed float64   // seconds accumulated in the current bucket
	planet  int       // w.Active value this window belongs to; -1 means uninitialised
}

type deliverySplit struct{ Gross, Banked, Returned float64 }

// spawnWorkerAtTownHall appends a new settling worker at the Town Hall and
// returns it. Returns nil if no Town Hall exists.
func spawnWorkerAtTownHall(w *World) *Worker {
	th := townHall(w)
	if th == nil {
		return nil
	}
	id := w.NextWorkerID
	w.NextWorkerID++
	wk := &Worker{
		ID:            id,
		Pos:           th.Pos,
		Angle:         th.Angle,
		State:         StateSettling,
		NodeID:        -1,
		TargetNodeID:  -1,
		PendingNodeID: -1,
		Timer:         settleDelay,
	}
	w.Workers = append(w.Workers, wk)
	return wk
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

// spawnNodeNear places a new node near intended angle within the field's arc,
// searching outward if that exact angle is blocked. Falls back to upgrading the
// nearest field node if no free surface is found.
func spawnNodeNear(w *World, f *ResourceField, intended float64) growthResult {
	result := growthResult{
		Outcome:     growthOutcomeNone,
		Kind:        f.Kind,
		CenterAngle: f.CenterAngle,
		HalfArc:     f.HalfArc,
		NodeID:      -1,
	}
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

// spawnNode places a new node within the field's arc, distributing it among
// existing nodes using a golden-ratio spacing to avoid clustering. If the field
// has no valid free surface left, the nearest same-field node grows instead.
func spawnNode(w *World, f *ResourceField) growthResult {
	count := 0
	for _, n := range w.Nodes {
		if n.Kind == f.Kind {
			count++
		}
	}
	const phi = 2.399 // ≈ golden angle in radians
	frac := math.Mod(float64(count)*phi, math.Pi*2) / (math.Pi * 2)
	intended := normAngle(f.CenterAngle - f.HalfArc + 2*f.HalfArc*frac)
	return spawnNodeNear(w, f, intended)
}

// foundStartingNodes spawns the initial wood trees when the settlement is
// founded. Two trees flank the Town Hall; the rest spread around the planet
// using a golden-angle offset from the opposite side.
func foundStartingNodes(w *World, f *ResourceField, townHallAngle float64) {
	const flankCount = 2
	for range flankCount {
		spawnNodeNear(w, f, townHallAngle)
	}
	const phi = 2.399 // golden angle in radians
	for i := range startingNodes - flankCount {
		intended := normAngle(townHallAngle + math.Pi + float64(i)*phi)
		spawnNodeNear(w, f, intended)
	}
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

func fieldCanSpawnNode(w *World, f *ResourceField) bool {
	if f == nil {
		return false
	}
	candidate := &ResourceNode{Kind: f.Kind, Size: 1}
	step := 2 / w.Planet.Radius
	maxSteps := int(math.Ceil((2*f.HalfArc)/step)) + 1
	for i := 0; i <= maxSteps; i++ {
		angle := normAngle(f.CenterAngle - f.HalfArc + float64(i)*step)
		if nodeSpawnAngleValid(w, f, candidate, angle) {
			return true
		}
	}
	return false
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
	radius := 72.0
	forestAngle := -math.Pi / 2 // top of the rim

	field := &ResourceField{
		Kind:        KindWood,
		CenterAngle: forestAngle,
		HalfArc:     forestHalfArc,
		Cap:         woodFieldBaseEXP,
	}

	planets := []SystemPlanet{
		// Index 0: starting planet (rate set at unlock; Pos is system-view canvas position)
		{Kind: PlanetStarting, Pos: Vec{X: 118, Y: 92}, Radius: 18},
		// Index 1 & 2: echo forest producers — rates snapshotted at unlock, zero until then
		{Kind: PlanetEcho, Pos: Vec{X: 160, Y: 72}, Radius: 12, RingColorIdx: 0, Seed: 42},
		{Kind: PlanetEcho, Pos: Vec{X: 148, Y: 128}, Radius: 12, RingColorIdx: 1, Seed: 43},
		// Index 3: unknown frontier silhouette — non-interactive, no rate
		{Kind: PlanetUnknown, Pos: Vec{X: 242, Y: 162}, Radius: 10},
	}
	return &World{
		Version: SaveVersion,
		Planet: Planet{
			Center:      center,
			Radius:      radius,
			Composition: map[ResourceKind]float64{KindWood: 1.0},
			Fields:      []*ResourceField{field},
		},
		Economy:      Economy{TownGrowthCap: townGrowthBaseCap},
		Active:       0,
		PlanetStates: make([]*PlanetState, len(planets)),
		System: System{
			Unlocked: false,
			View:     ViewPlanet,
			Selected: -1,
			Planets:  planets,
		},
	}
}

// ── Echo planet layouts ───────────────────────────────────────────────────────

// newEchoPlanetState returns a freshly initialised durable live state for an
// awakened echo planet. The planet has a dormant wood field and pre-spawned
// trees but no settlement; the player still places the Town Hall on entry.
// layoutID selects between two stable authored layout variants.
func newEchoPlanetState(layoutID int) *PlanetState {
	center := Vec{X: 160, Y: 120} // same planet-view canvas center as starting
	var radius float64
	var forestAngle float64
	switch layoutID {
	case 1:
		radius = 65
		forestAngle = -math.Pi * 2 / 3 // upper-left arc
	default: // 0
		radius = 60
		forestAngle = math.Pi / 6 // slight right of horizontal
	}

	field := &ResourceField{
		Kind:        KindWood,
		CenterAngle: forestAngle,
		HalfArc:     forestHalfArc,
		Cap:         woodFieldBaseEXP,
	}

	// Build a temporary world so we can reuse the node-spawning helpers.
	tmp := &World{
		Planet: Planet{
			Center:      center,
			Radius:      radius,
			Composition: map[ResourceKind]float64{KindWood: 1.0},
			Fields:      []*ResourceField{field},
		},
	}
	for range startingNodes {
		spawnNode(tmp, field)
	}

	return &PlanetState{
		Planet:        tmp.Planet,
		Nodes:         tmp.Nodes,
		NextNodeID:    tmp.NextNodeID,
		TownGrowthCap: townGrowthBaseCap,
	}
}

// ── Multi-planet park / load / switch ────────────────────────────────────────

// parkActive saves the active planet's live fields into PlanetStates[Active].
// After this call PlanetStates[Active] holds a snapshot; the top-level fields
// are still live. Call loadPlanet afterwards to complete a switch.
func parkActive(w *World) {
	w.PlanetStates[w.Active] = &PlanetState{
		Planet:             w.Planet,
		Buildings:          w.Buildings,
		Nodes:              w.Nodes,
		Workers:            w.Workers,
		NextNodeID:         w.NextNodeID,
		NextWorkerID:       w.NextWorkerID,
		ResourceDiscovered: w.ResourceDiscovered,
		SimTime:            w.SimTime,
		WorkerCapacity:     w.Economy.WorkerCapacity,
		CapacityBought:     w.Economy.CapacityBought,
		CampsBought:        w.Economy.CampsBought,
		TownGrowth:         w.Economy.TownGrowth,
		TownGrowthCap:      w.Economy.TownGrowthCap,
		Founded:            townHall(w) != nil,
	}
}

// loadPlanet replaces the top-level live fields with PlanetStates[idx] and
// clears the parked slot (active slot is always nil). Economy.Wood is global
// and is never touched. Transient cue state is cleared; it rebuilds at runtime.
func loadPlanet(w *World, idx int) {
	ps := w.PlanetStates[idx]
	w.Planet             = ps.Planet
	w.Buildings          = ps.Buildings
	w.Nodes              = ps.Nodes
	w.Workers            = ps.Workers
	w.NextNodeID         = ps.NextNodeID
	w.NextWorkerID       = ps.NextWorkerID
	w.ResourceDiscovered = ps.ResourceDiscovered
	w.SimTime            = ps.SimTime
	w.Economy.WorkerCapacity = ps.WorkerCapacity
	w.Economy.CapacityBought = ps.CapacityBought
	w.Economy.CampsBought    = ps.CampsBought
	w.Economy.TownGrowth     = ps.TownGrowth
	w.Economy.TownGrowthCap  = ps.TownGrowthCap
	w.PlanetStates[idx]  = nil // active slot is always nil
	w.Active             = idx
	w.growthCue          = growthCueState{}
	w.pendingGrowthCues  = nil
	w.lastDelivery       = deliverySplit{}
}

// switchToPlanet parks the current active planet and loads the one at idx.
// No-op if idx is already active.
func switchToPlanet(w *World, idx int) {
	if idx == w.Active {
		return
	}
	parkActive(w)
	loadPlanet(w, idx)
}
