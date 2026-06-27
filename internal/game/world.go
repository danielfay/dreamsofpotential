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
	KindWood           ResourceKind = iota
	KindWater                       // lake terrain; unknown fields carry this kind
	KindWaterInfluence              // invisible, overlap-allowed; widens a lake's reach into adjacent forest

	focusKindNone ResourceKind = -1 // worker has no focus assignment
)

func kindName(k ResourceKind) string {
	switch k {
	case KindWood:
		return "wood"
	case KindWater:
		return "water"
	case KindWaterInfluence:
		return "water-influence"
	}
	return "unknown"
}

// PotentialKind identifies a type of system-tier Potential token.
// Potential is banked in the global Economy and spent on planet-scale actions.
// Visual convention: material resources use square swatches; Potential uses circles.
type PotentialKind int

const (
	PotentialForest PotentialKind = iota // green circle — spent to awaken forest planets
	PotentialWater                       // blue circle — hinted by Lakewood; not yet spendable
)

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
	StateToDock                            // moving along rim to claimed dock
	StateDiving                            // moving interior straight-line to collect sparkles
	StateDiveLoading                       // pausing at a sparkle to gather it (mirrors StateLoading)
	StateSwimmingToDock                    // swimming back to dock after collecting all reachable sparkles
	StateDockUnloading                     // unloading water cargo at the dock
)

// PulseState stores a guarded micro-pulse. Rapid activations become steady-lit
// instead of restarting a flickering pulse every frame.
type PulseState struct {
	Remaining     float64
	LastActivated float64
	SteadyUntil   float64
}

// ResourceNode is a single harvestable point on the planet.
// OwnerID is the worker ID that has claimed it, or -1 if free.
// ReservedByWorkerID is the worker ID that has spoken for it, or -1 if free.
// Size scales both the visual and the load carried per trip (range ~0.6–1.4).
// Interior is true for water sparkles placed inside a water field rather than
// on the rim; their Pos is a free interior position and Angle is the direction
// from the planet center to that position (for field membership checks).
// ServicingDockID links a sparkle to the dock that will harvest it (Phase 5); -1 if unassigned.
type ResourceNode struct {
	ID                 int
	Kind               ResourceKind
	Pos                Vec
	Angle              float64
	OwnerID            int
	ReservedByWorkerID int
	Size               float64
	Pulse              PulseState
	Interior           bool
	ServicingDockID    int
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
	DockID        int          // building ID of the claimed dock; -1 if not a water worker
	FocusedKind   ResourceKind // resource this worker is assigned to harvest; focusKindNone (-1) = unfocused
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
	KindDock                            // 2 — water-edge pier; shore or extension
)

// ViewMode distinguishes the system-level view from the planet-level view.
type ViewMode int

const (
	ViewPlanet ViewMode = iota // player is operating on the starting planet
	ViewSystem                 // player is looking at the planetary system
)

// PlanetKind categorises a system-view planet record.
type PlanetKind int

const (
	PlanetStarting PlanetKind = iota // the live-simulated starting planet
	PlanetEcho                       // an awakened forest producer (abstract only for now)
	PlanetUnknown                    // distant locked frontier silhouette
)

// SystemPlanet is a persistent record for one planet in the system view.
// AbstractRate is the wood/sec this planet contributes to the global bank;
// set at first completion for the starting planet, fixed at boot for echoes.
// Seed is reserved for future world generation. Pos/Radius are in 320×240 virtual space.
type SystemPlanet struct {
	Kind               PlanetKind
	Pos                Vec
	Radius             float64
	AbstractRate       float64
	AbstractWaterRate  float64 // water/sec contributed to the global bank; set on completion
	ProjectedRate      float64 // authored future rate shown for dormant/incomplete planets
	ProjectedWaterRate float64
	RingColorIdx       int     // 0 or 1 — slight visual variation between the two echoes
	Seed               int64   // future world-generation hook
	Awakened           bool    // echo or frontier has been awoken (has a durable live PlanetState)
	Completed          bool    // reached its completion gate
	CompletedAt        float64 // planet SimTime when completion triggered; drives atmosphere intro
	LayoutID           int     // which authored layout (0 or 1)
}

// zoomable reports whether this planet has a live sim the player can zoom into.
func (p SystemPlanet) zoomable() bool {
	return p.Kind == PlanetStarting || (p.Awakened && (p.Kind == PlanetEcho || p.Kind == PlanetUnknown))
}

// PlanetState is one planet's durable live sim state when it is NOT the active
// (viewed) planet. The active planet's state lives in the top-level World fields.
type PlanetState struct {
	Planet              Planet
	Buildings           []*Building
	Nodes               []*ResourceNode
	Workers             []*Worker
	NextNodeID          int
	NextWorkerID        int
	NextBuildingID      int
	ResourceDiscovered  bool
	SimTime             float64
	WorkerCapacity      int
	CapacityBought      int
	CampsBought         int
	TownGrowth          float64
	TownGrowthCap       float64
	TownGrowthOverflow  float64
	LastWorkerSpawnTime float64
	Founded             bool
	LaborFocus          map[ResourceKind]int // target worker counts per resource kind; nil = no focus
	SavedLaborRatio     map[ResourceKind]int // ratio proportions saved by the player; guides overflow assignment
	LocalWood           float64              // per-planet wood stockpile saved on park, restored on load
	LocalWater          float64              // per-planet water stockpile saved on park, restored on load
}

// System holds all persistent state for the planetary system layer.
type System struct {
	Unlocked bool           // true once the first reveal has completed
	View     ViewMode       // current view mode; persisted across save/load
	Selected int            // index into Planets; -1 = none selected
	Planets  []SystemPlanet // always 4 entries: starting, echo A, echo B, unknown
}

// Building is a player-placed structure on the planet rim.
// Pos is the rim point at Angle; Kind distinguishes TH, camps, and docks.
// Extension marks a dock placed over water connected to an existing dock.
type Building struct {
	ID            int
	Kind          BuildingKind
	Pos           Vec
	Angle         float64
	Level         int
	Extension     bool
	DeliveredWood float64
	DeliveryCount int
	Pulse         PulseState
}

// KindProgress holds planet-level EXP accumulation for one resource kind.
// All known regions of that kind share a single progress counter.
type KindProgress struct {
	EXP float64
	Cap float64
}

// ResourceField is a region arc on the planet rim where a resource kind grows.
// EXP/Cap moved to Planet.FieldProgress so multiple regions can share progress.
type ResourceField struct {
	Kind        ResourceKind
	CenterAngle float64
	HalfArc     float64
	Known       bool // false for teaser/unknown regions (never accrue EXP, never gate completion)
}

type growthOutcome int

const (
	growthOutcomeNone growthOutcome = iota
	growthOutcomeSpawnedNode
	growthOutcomeUpgradedNode
)

type growthResult struct {
	Outcome         growthOutcome
	Kind            ResourceKind
	CenterAngle     float64
	HalfArc         float64
	NodeID          int
	WaterInfluenced bool
}

type growthCueState struct {
	Outcome         growthOutcome
	Kind            ResourceKind
	CenterAngle     float64
	HalfArc         float64
	NodeID          int
	WaterInfluenced bool
	GaugeRelease    float64
	GaugeAfterglow  float64
	FieldDelay      float64
	FieldPulse      float64
	NodeDelay       float64
	NodeCue         float64
}

// Planet is the disc the player operates on.
type Planet struct {
	Center        Vec
	Radius        float64
	Composition   map[ResourceKind]float64
	Fields        []*ResourceField
	FieldProgress map[ResourceKind]*KindProgress // planet-level EXP/Cap per kind; shared across all regions of that kind
	StartingNodes int                            // override for foundStartingNodes count; 0 means use the global default
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
	Wood                float64
	Water               float64 // harvested blue water material; revealed on first dock delivery
	WaterDiscovered     bool    // true after the first water delivery (controls HUD visibility)
	WorkerCapacity      int     // total worker slots unlocked (founding slot + paid capacity)
	CapacityBought      int     // paid capacity purchases; drives the cost curve
	CampsBought         int
	TownGrowth          float64                   // accumulates gross delivery amount; clamped at TownGrowthCap
	TownGrowthCap       float64                   // spawns a worker when TownGrowth reaches this; grows each arrival
	TownGrowthOverflow  float64                   // excess growth banked while capacity-blocked; drains on next open slot
	LastWorkerSpawnTime float64                   // SimTime of most recent spawn; used to enforce workerSpawnCooldown
	Potential           map[PotentialKind]float64 // banked system-tier Potential tokens (fractional); spendable count is math.Floor(value)
}

// SystemEconomy tracks system-wide resource rates and allocation.
type SystemEconomy struct {
	WoodRate  float64 // wood/sec from all completed planets
	WaterRate float64 // water/sec from all completed planets

	// Allocation: fraction [0,1] directed to Potential; remainder goes to Research
	WoodAllocPotential  float64
	WaterAllocPotential float64

	// Fractional research progress per family (accumulated until a threshold unlocks a benefit)
	WoodResearch  float64
	WaterResearch float64
}

// SaveVersion is bumped on every backwards-incompatible World JSON change.
// Load discards saves whose Version field doesn't match.
const SaveVersion = 20

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
	NextBuildingID     int
	ResourceDiscovered bool // true after the first wood delivery
	SimTime            float64
	LaborFocus         map[ResourceKind]int // target worker counts per resource kind; nil = no focus
	SavedLaborRatio    map[ResourceKind]int // ratio proportions saved by the player; guides overflow assignment
	WorkerRatioSeen    bool                 // true once the labor focus HUD button has been opened
	System             System               // system-view unlock state; persisted

	SystemEconomy SystemEconomy // system-wide rates and allocation; see SystemEconomy struct

	// Multi-planet support: PlanetStates holds parked live state for non-active
	// planets, index-aligned to System.Planets. The entry for Active is always nil
	// because that planet's live state lives in the top-level fields above.
	// Uninstantiated planets (unawakened echoes, unknown) are also nil.
	PlanetStates []*PlanetState
	Active       int // System.Planets index currently loaded into top-level live fields

	growthCue         growthCueState
	pendingGrowthCues []growthCueState
	lastDelivery      deliverySplit
	abstractRateWins  []abstractRateWindow // one per entry in activeAbstractRateSpecs; nil = uninitialised
	rng               *rand.Rand           // seeded per-world; use this instead of the global source
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
		DockID:        -1,
		FocusedKind:   focusKindNone,
		Timer:         settleDelay,
	}
	w.Workers = append(w.Workers, wk)
	assignFocusToNewWorker(w, wk)
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

// initTransientWorldState restores fields that are not persisted in JSON.
// Call after any JSON unmarshal so the world is safe to use immediately.
func initTransientWorldState(w *World) {
	if w.rng == nil {
		w.rng = rand.New(rand.NewSource(0))
	}
}

// newNode allocates a ResourceNode with the next available ID at the given rim angle.
// Size is randomised in [0.6, 1.4] and affects both the visual and yield per trip.
func newNode(w *World, kind ResourceKind, angle float64) *ResourceNode {
	if w.rng == nil {
		w.rng = rand.New(rand.NewSource(0))
	}
	id := w.NextNodeID
	w.NextNodeID++
	return &ResourceNode{
		ID:                 id,
		Kind:               kind,
		Angle:              angle,
		Pos:                w.Planet.RimPoint(angle),
		OwnerID:            -1,
		ReservedByWorkerID: -1,
		Size:               0.6 + w.rng.Float64()*0.8,
	}
}

// newSparkle allocates an interior water sparkle at the given world position.
// Angle is derived from the position so angleWithinField filtering works.
// ServicingDockID is -1 until a dock claims the sparkle in Phase 5.
func newSparkle(w *World, pos Vec) *ResourceNode {
	if w.rng == nil {
		w.rng = rand.New(rand.NewSource(0))
	}
	id := w.NextNodeID
	w.NextNodeID++
	return &ResourceNode{
		ID:                 id,
		Kind:               KindWater,
		Pos:                pos,
		Angle:              w.Planet.AngleOf(pos),
		OwnerID:            -1,
		ReservedByWorkerID: -1,
		Size:               0.6 + w.rng.Float64()*0.8,
		Interior:           true,
		ServicingDockID:    -1,
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
		if f.Kind == KindWood && waterInfluenced(w, angle) {
			candidate.Size = math.Min(candidate.Size+waterForestSpawnSizeBonus, 2.0)
			result.WaterInfluenced = true
		}
		w.Nodes = append(w.Nodes, candidate)
		activatePulse(w, &candidate.Pulse)
		result.Outcome = growthOutcomeSpawnedNode
		result.NodeID = candidate.ID
		return result
	}
	if upgraded := upgradeNearestFieldNode(w, f, intended); upgraded != nil {
		result.Outcome = growthOutcomeUpgradedNode
		result.NodeID = upgraded.ID
		result.WaterInfluenced = upgraded.Kind == KindWood && waterInfluenced(w, upgraded.Angle)
	}
	return result
}

// spawnNode places a new node within the field's arc, distributing it among
// existing nodes in that region using a golden-ratio spacing to avoid clustering.
// If the field has no valid free surface left, the nearest same-field node grows instead.
func spawnNode(w *World, f *ResourceField) growthResult {
	count := 0
	for _, n := range w.Nodes {
		if n.Kind == f.Kind && angleWithinField(f, n.Angle) {
			count++
		}
	}
	const phi = 2.399 // ≈ golden angle in radians
	frac := math.Mod(float64(count)*phi, math.Pi*2) / (math.Pi * 2)
	intended := normAngle(f.CenterAngle - f.HalfArc + 2*f.HalfArc*frac)
	return spawnNodeNear(w, f, intended)
}

// foundStartingNodes spawns the initial nodes when the settlement is founded.
// Two nodes flank the Town Hall (in whichever field contains it); the rest are
// distributed one at a time, each independently picking a random known field.
func foundStartingNodes(w *World, townHallAngle float64) {
	var fields []*ResourceField
	for _, f := range w.Planet.Fields {
		if f.Known {
			fields = append(fields, f)
		}
	}
	if len(fields) == 0 {
		return
	}

	count := startingNodes
	if w.Planet.StartingNodes > 0 {
		count = w.Planet.StartingNodes
	}

	// Flanking nodes go in the field that contains the Town Hall angle.
	thField := fields[0]
	for _, f := range fields {
		if angleWithinField(f, townHallAngle) {
			thField = f
			break
		}
	}
	const flankCount = 2
	for range flankCount {
		spawnNodeNear(w, thField, townHallAngle)
	}

	// Remaining nodes: each picks a random known field independently.
	for range count - flankCount {
		f := fields[w.rng.Intn(len(fields))]
		intended := normAngle(f.CenterAngle + (w.rng.Float64()*2-1)*f.HalfArc*0.8)
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
	if inLake(w, angle) {
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
		if n.Interior || n.Kind != candidate.Kind || !angleWithinField(f, n.Angle) {
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

// inLake reports whether angle is inside any KindWater field arc.
func inLake(w *World, angle float64) bool {
	for _, f := range w.Planet.Fields {
		if f.Kind == KindWater && angleWithinField(f, angle) {
			return true
		}
	}
	return false
}

// effectiveArc returns the cheaper of the two arcs from a to b in world px,
// where each lake sub-arc costs 1/lakeSpeedFactor more (workers go slower
// through lakes). Uses a fixed-step midpoint integration (64 samples) per
// candidate arc so workers route around lakes when it's faster to do so.
func effectiveArc(w *World, a, b float64) float64 {
	short := normAngle(b - a) // in (-π, π]
	if short == 0 {
		return 0
	}
	// Fast path: no lake fields → geometric arc is always optimal.
	if !hasAnyLake(w) {
		return math.Abs(short) * w.Planet.Radius
	}
	shortCost := arcCost(w, a, short)
	// If the short arc is all land it can't be beaten: it's both shorter AND penalty-free.
	if shortCost == math.Abs(short)*w.Planet.Radius {
		return shortCost
	}
	long := short - math.Copysign(2*math.Pi, short)
	return min(shortCost, arcCost(w, a, long))
}

// arcCost integrates the effective travel cost along a signed arc of `arc`
// radians starting from angle `a`. Positive arc = clockwise, negative = CCW.
// Dock-covered lake segments pay no lake penalty — they count as regular rim.
func arcCost(w *World, a, arc float64) float64 {
	totalLen := math.Abs(arc) * w.Planet.Radius
	const steps = 64
	var lakeAngle float64
	for i := 0; i < steps; i++ {
		frac := (float64(i) + 0.5) / float64(steps)
		sample := normAngle(a + arc*frac)
		if inLake(w, sample) && !dockCoversAngle(w, sample) {
			lakeAngle += math.Abs(arc) / float64(steps)
		}
	}
	lakeLen := lakeAngle * w.Planet.Radius
	return (totalLen - lakeLen) + lakeLen/lakeSpeedFactor
}

// hasAnyLake reports whether the active planet has any KindWater field.
func hasAnyLake(w *World) bool {
	for _, f := range w.Planet.Fields {
		if f.Kind == KindWater {
			return true
		}
	}
	return false
}

// waterInfluenced reports whether the given rim angle falls within any
// KindWaterInfluence field. Influence fields are authored co-centered with
// lakes but wider, are Known:false (so they project before discovery),
// overlap-allowed, and non-stacking (boolean OR).
func waterInfluenced(w *World, angle float64) bool {
	for _, f := range w.Planet.Fields {
		if f.Kind == KindWaterInfluence && angleWithinField(f, angle) {
			return true
		}
	}
	return false
}

func upgradeNearestFieldNode(w *World, f *ResourceField, intended float64) *ResourceNode {
	var best *ResourceNode
	bestDist := math.MaxFloat64
	for _, n := range w.Nodes {
		if n.Interior || n.Kind != f.Kind || !angleWithinField(f, n.Angle) {
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
	inc := 0.15
	if best.Kind == KindWood && waterInfluenced(w, best.Angle) {
		inc += waterForestUpgradeSizeBonus
	}
	best.Size += inc
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
	return newWorldWithSeed(0)
}

func newWorldWithSeed(seed int64) *World {
	center := Vec{X: 160, Y: 120}
	radius := 72.0
	forestAngle := -math.Pi / 2 // top of the rim

	field := &ResourceField{
		Kind:        KindWood,
		CenterAngle: forestAngle,
		HalfArc:     forestHalfArc,
		Known:       true,
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
			FieldProgress: map[ResourceKind]*KindProgress{
				KindWood: {Cap: woodFieldBaseEXP},
			},
		},
		Economy:       Economy{TownGrowthCap: townGrowthBaseCap, Potential: make(map[PotentialKind]float64)},
		SystemEconomy: SystemEconomy{WoodAllocPotential: 1.0, WaterAllocPotential: 1.0},
		Active:        0,
		PlanetStates:  make([]*PlanetState, len(planets)),
		System: System{
			Unlocked: false,
			View:     ViewPlanet,
			Selected: -1,
			Planets:  planets,
		},
		rng: rand.New(rand.NewSource(seed)),
	}
}

// sparkleSpawnPosValid reports whether pos is a valid spawn location for a new
// interior sparkle in field f: inside the radial band, within the field arc,
// and not soft-overlapping any existing interior node.
func sparkleSpawnPosValid(w *World, f *ResourceField, pos Vec) bool {
	angle := w.Planet.AngleOf(pos)
	if !angleWithinField(f, angle) {
		return false
	}
	r := pos.Dist(w.Planet.Center)
	innerR := w.Planet.Radius * sparkleInnerFrac
	outerR := w.Planet.Radius * sparkleOuterFrac
	if r < innerR || r > outerR {
		return false
	}
	const candidateSize = 1.0
	candidateSoftR := sparkleSoftRadiusFactor * candidateSize
	for _, n := range w.Nodes {
		if !n.Interior {
			continue
		}
		if pos.Dist(n.Pos) < candidateSoftR+sparkleSoftRadiusFactor*n.Size {
			return false
		}
	}
	return true
}

// waterFieldCanSpawnSparkle reports whether there is at least one valid
// interior position in f for a new sparkle.
func waterFieldCanSpawnSparkle(w *World, f *ResourceField) bool {
	if f == nil {
		return false
	}
	if !waterSparkleSpawningUnlocked(w) {
		return false
	}
	return waterFieldCanSpawnSparkleRaw(w, f)
}

func forEachSparkleGridPos(w *World, f *ResourceField, fn func(pos Vec) bool) {
	if w == nil || f == nil || fn == nil {
		return
	}
	innerR := w.Planet.Radius * sparkleInnerFrac
	outerR := w.Planet.Radius * sparkleOuterFrac
	const angularSteps = 16
	const radialSteps = 4
	for ai := range angularSteps {
		angle := normAngle(f.CenterAngle - f.HalfArc + 2*f.HalfArc*float64(ai)/float64(angularSteps-1))
		for ri := range radialSteps {
			r := innerR + (outerR-innerR)*float64(ri)/float64(radialSteps-1)
			pos := Vec{
				X: w.Planet.Center.X + r*math.Cos(angle),
				Y: w.Planet.Center.Y + r*math.Sin(angle),
			}
			if !fn(pos) {
				return
			}
		}
	}
}

func waterFieldCanSpawnSparkleRaw(w *World, f *ResourceField) bool {
	if f == nil {
		return false
	}
	canSpawn := false
	forEachSparkleGridPos(w, f, func(pos Vec) bool {
		if sparkleSpawnPosValid(w, f, pos) {
			canSpawn = true
			return false
		}
		return true
	})
	return canSpawn
}

func waterSparkleSpawningUnlocked(w *World) bool {
	return dockExists(w)
}

// spawnSparkle places a new interior water sparkle in f. Tries golden-angle-
// jittered positions first; if those are all blocked, scans the same 16×4 grid
// that waterFieldCanSpawnSparkle uses so organic placement can always reach any
// slot the gate considers available. Falls back to upgradeNearestSparkle only
// when the field is truly saturated.
func spawnSparkle(w *World, f *ResourceField) growthResult {
	result := growthResult{
		Outcome:     growthOutcomeNone,
		Kind:        f.Kind,
		CenterAngle: f.CenterAngle,
		HalfArc:     f.HalfArc,
		NodeID:      -1,
	}
	count := 0
	for _, n := range w.Nodes {
		if n.Interior && n.Kind == KindWater && angleWithinField(f, n.Angle) {
			count++
		}
	}
	const phi = 2.399 // golden angle ≈ radians
	innerR := w.Planet.Radius * sparkleInnerFrac
	outerR := w.Planet.Radius * sparkleOuterFrac
	// Try golden-angle-jittered candidate positions; more attempts improves
	// fill quality before falling back to upgrade.
	const attempts = 24
	for i := range attempts {
		angularFrac := math.Mod(float64(count+i)*phi, math.Pi*2) / (math.Pi * 2)
		angle := normAngle(f.CenterAngle - f.HalfArc + 2*f.HalfArc*angularFrac)
		r := innerR + w.rng.Float64()*(outerR-innerR)
		pos := Vec{
			X: w.Planet.Center.X + r*math.Cos(angle),
			Y: w.Planet.Center.Y + r*math.Sin(angle),
		}
		if sparkleSpawnPosValid(w, f, pos) {
			n := newSparkle(w, pos)
			w.Nodes = append(w.Nodes, n)
			activatePulse(w, &n.Pulse)
			result.Outcome = growthOutcomeSpawnedNode
			result.NodeID = n.ID
			return result
		}
	}
	// Golden-angle attempts all blocked — try the same 16×4 grid positions that
	// waterFieldCanSpawnSparkle uses, so organic placement can always reach any
	// valid slot that the gate check considers available.
	forEachSparkleGridPos(w, f, func(pos Vec) bool {
		if sparkleSpawnPosValid(w, f, pos) {
			n := newSparkle(w, pos)
			w.Nodes = append(w.Nodes, n)
			activatePulse(w, &n.Pulse)
			result.Outcome = growthOutcomeSpawnedNode
			result.NodeID = n.ID
			return false
		}
		return true
	})
	if result.Outcome == growthOutcomeSpawnedNode {
		return result
	}
	// Field truly saturated; upgrade nearest sparkle instead.
	if upgraded := upgradeNearestSparkle(w, f); upgraded != nil {
		result.Outcome = growthOutcomeUpgradedNode
		result.NodeID = upgraded.ID
	}
	return result
}

func spawnSparkleAt(w *World, f *ResourceField, pos Vec) growthResult {
	result := growthResult{
		Outcome:     growthOutcomeNone,
		Kind:        f.Kind,
		CenterAngle: f.CenterAngle,
		HalfArc:     f.HalfArc,
		NodeID:      -1,
	}
	if !sparkleSpawnPosValid(w, f, pos) {
		return result
	}
	n := newSparkle(w, pos)
	w.Nodes = append(w.Nodes, n)
	activatePulse(w, &n.Pulse)
	result.Outcome = growthOutcomeSpawnedNode
	result.NodeID = n.ID
	return result
}

func spawnSparkleInDockReach(w *World, f *ResourceField, dock *Building) growthResult {
	midR := w.Planet.Radius * 0.75
	innerR := w.Planet.Radius * sparkleInnerFrac
	outerR := w.Planet.Radius * sparkleOuterFrac
	if midR < innerR {
		midR = innerR
	}
	if midR > outerR {
		midR = outerR
	}
	radii := []float64{midR, w.Planet.Radius * 0.70, w.Planet.Radius * 0.80}
	offsets := []float64{0, dockWedgeHalfArc * 0.35, -dockWedgeHalfArc * 0.35, dockWedgeHalfArc * 0.7, -dockWedgeHalfArc * 0.7}
	for _, off := range offsets {
		angle := normAngle(dock.Angle + off)
		if !angleWithinField(f, angle) {
			continue
		}
		for _, r := range radii {
			if r < innerR || r > outerR {
				continue
			}
			pos := Vec{
				X: w.Planet.Center.X + r*math.Cos(angle),
				Y: w.Planet.Center.Y + r*math.Sin(angle),
			}
			if result := spawnSparkleAt(w, f, pos); result.Outcome != growthOutcomeNone {
				return result
			}
		}
	}
	return growthResult{
		Outcome:     growthOutcomeNone,
		Kind:        f.Kind,
		CenterAngle: f.CenterAngle,
		HalfArc:     f.HalfArc,
		NodeID:      -1,
	}
}

func seedInitialDockSparkles(w *World, dock *Building) {
	guaranteed := false
	for _, f := range w.Planet.Fields {
		if f.Kind != KindWater || !f.Known {
			continue
		}
		if !guaranteed && angleWithinField(f, dock.Angle) {
			if result := spawnSparkleInDockReach(w, f, dock); result.Outcome != growthOutcomeNone {
				guaranteed = true
			}
		}
		for i := 0; i < initialDockSparkles; i++ {
			if !waterFieldCanSpawnSparkleRaw(w, f) {
				break
			}
			spawnSparkle(w, f)
		}
	}
	assignServicingDocks(w)
	if guaranteed {
		return
	}
	for _, f := range w.Planet.Fields {
		if f.Kind == KindWater && f.Known && angleWithinField(f, dock.Angle) {
			spawnSparkleInDockReach(w, f, dock)
			assignServicingDocks(w)
			return
		}
	}
}

// upgradeNearestSparkle grows the interior sparkle in f closest to the field
// center by 0.15, clamped to 2.0, mirroring upgradeNearestFieldNode.
func upgradeNearestSparkle(w *World, f *ResourceField) *ResourceNode {
	// Reference point: the midpoint of the field's interior at field center angle.
	midR := w.Planet.Radius * (sparkleInnerFrac + sparkleOuterFrac) / 2
	ref := Vec{
		X: w.Planet.Center.X + midR*math.Cos(f.CenterAngle),
		Y: w.Planet.Center.Y + midR*math.Sin(f.CenterAngle),
	}
	var best *ResourceNode
	bestDist := math.MaxFloat64
	for _, n := range w.Nodes {
		if !n.Interior || n.Kind != KindWater || !angleWithinField(f, n.Angle) {
			continue
		}
		if d := n.Pos.Dist(ref); d < bestDist {
			bestDist = d
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

// ── Multi-planet park / load / switch ────────────────────────────────────────

func writePlanetStateFromWorld(ps *PlanetState, w *World) {
	ps.Planet = w.Planet
	ps.Buildings = w.Buildings
	ps.Nodes = w.Nodes
	ps.Workers = w.Workers
	ps.NextNodeID = w.NextNodeID
	ps.NextWorkerID = w.NextWorkerID
	ps.NextBuildingID = w.NextBuildingID
	ps.ResourceDiscovered = w.ResourceDiscovered
	ps.SimTime = w.SimTime
	ps.WorkerCapacity = w.Economy.WorkerCapacity
	ps.CapacityBought = w.Economy.CapacityBought
	ps.CampsBought = w.Economy.CampsBought
	ps.TownGrowth = w.Economy.TownGrowth
	ps.TownGrowthCap = w.Economy.TownGrowthCap
	ps.TownGrowthOverflow = w.Economy.TownGrowthOverflow
	ps.LastWorkerSpawnTime = w.Economy.LastWorkerSpawnTime
	ps.Founded = townHall(w) != nil
	ps.LaborFocus = w.LaborFocus
	ps.SavedLaborRatio = w.SavedLaborRatio
	ps.LocalWood = w.Economy.Wood
	ps.LocalWater = w.Economy.Water
}

func loadWorldFromPlanetState(w *World, ps *PlanetState) {
	w.Planet = ps.Planet
	w.Buildings = ps.Buildings
	w.Nodes = ps.Nodes
	w.Workers = ps.Workers
	w.NextNodeID = ps.NextNodeID
	w.NextWorkerID = ps.NextWorkerID
	w.NextBuildingID = ps.NextBuildingID
	w.ResourceDiscovered = ps.ResourceDiscovered
	w.SimTime = ps.SimTime
	w.LaborFocus = ps.LaborFocus
	w.SavedLaborRatio = ps.SavedLaborRatio
	w.Economy.Wood = ps.LocalWood
	w.Economy.Water = ps.LocalWater
	w.Economy.WorkerCapacity = ps.WorkerCapacity
	w.Economy.CapacityBought = ps.CapacityBought
	w.Economy.CampsBought = ps.CampsBought
	w.Economy.TownGrowth = ps.TownGrowth
	w.Economy.TownGrowthCap = ps.TownGrowthCap
	w.Economy.TownGrowthOverflow = ps.TownGrowthOverflow
	w.Economy.LastWorkerSpawnTime = ps.LastWorkerSpawnTime
}

// parkActive saves the active planet's live fields into PlanetStates[Active].
// After this call PlanetStates[Active] holds a snapshot; the top-level fields
// are still live. Call loadPlanet afterwards to complete a switch.
func parkActive(w *World) {
	ps := &PlanetState{}
	writePlanetStateFromWorld(ps, w)
	w.PlanetStates[w.Active] = ps
}

// loadPlanet replaces the top-level live fields with PlanetStates[idx] and
// clears the parked slot (active slot is always nil). Transient cue state is
// cleared; it rebuilds at runtime.
func loadPlanet(w *World, idx int) {
	ps := w.PlanetStates[idx]
	loadWorldFromPlanetState(w, ps)
	w.PlanetStates[idx] = nil // active slot is always nil
	w.Active = idx
	w.growthCue = growthCueState{}
	w.pendingGrowthCues = nil
	w.lastDelivery = deliverySplit{}
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
