package game

import "math"

// depositToField increments the planet-level field EXP for kind and spawns a
// new node each time EXP meets or exceeds the cap. The spawn target region is
// chosen by pickGrowthRegion (random among eligible known regions of that kind).
func depositToField(w *World, kind ResourceKind, amount float64) {
	fp := w.Planet.FieldProgress[kind]
	if fp == nil {
		return
	}
	fp.EXP += amount
	for fp.EXP >= fp.Cap {
		f := pickGrowthRegion(w, kind)
		if f == nil {
			break
		}
		fp.EXP -= fp.Cap
		// Capped geometric: grow the threshold exponentially while the step is
		// small, then switch to additive so late-game trees stay naturally reachable.
		if step := fp.Cap * (woodFieldEXPGrowth - 1); step < woodFieldEXPMaxStep {
			fp.Cap *= woodFieldEXPGrowth
		} else {
			fp.Cap += woodFieldEXPMaxStep
		}
		result := spawnIntoField(w, f)
		activateGrowthCue(w, result)
	}
}

func fieldCanSpawn(w *World, f *ResourceField) bool {
	if f == nil || !f.Known {
		return false
	}
	switch f.Kind {
	case KindWater:
		return waterFieldCanSpawnSparkle(w, f)
	case KindWaterInfluence:
		return false
	default:
		return fieldCanSpawnNode(w, f)
	}
}

func spawnIntoField(w *World, f *ResourceField) growthResult {
	if f == nil {
		return growthResult{}
	}
	if f.Kind == KindWater {
		return spawnSparkle(w, f)
	}
	return spawnNode(w, f)
}

// pickGrowthRegion selects a known region of the given kind to receive a new
// spawn. Prefers regions that can still accept a node/sparkle; falls back to
// any known region when all are saturated (spawnNode/spawnSparkle handles the
// upgrade path).
func pickGrowthRegion(w *World, kind ResourceKind) *ResourceField {
	var eligible []*ResourceField
	var fallback *ResourceField
	for _, f := range w.Planet.Fields {
		if f.Kind != kind || !f.Known {
			continue
		}
		if kind == KindWater && !waterSparkleSpawningUnlocked(w) {
			continue
		}
		if fallback == nil {
			fallback = f
		}
		if fieldCanSpawn(w, f) {
			eligible = append(eligible, f)
		}
	}
	if len(eligible) > 0 {
		return eligible[w.rng.Intn(len(eligible))]
	}
	return fallback
}

func activateGrowthCue(w *World, result growthResult) {
	if result.Outcome == growthOutcomeNone {
		return
	}
	cue := growthCueState{
		Outcome:         result.Outcome,
		Kind:            result.Kind,
		CenterAngle:     result.CenterAngle,
		HalfArc:         result.HalfArc,
		NodeID:          result.NodeID,
		WaterInfluenced: result.WaterInfluenced,
		GaugeRelease:    growthGaugeReleaseTime,
		GaugeAfterglow:  growthGaugeAfterglowTime,
		FieldDelay:      growthFieldPulseDelay,
		FieldPulse:      growthFieldPulseTime,
		NodeDelay:       growthNodeCueDelay,
		NodeCue:         growthNodeCueTime,
	}
	if growthCueActive(w.growthCue) {
		w.pendingGrowthCues = append(w.pendingGrowthCues, cue)
		return
	}
	w.growthCue = cue
}

func tickGrowthCue(w *World, dt float64) {
	if !growthCueActive(w.growthCue) {
		startNextGrowthCue(w)
	}
	tickTimer(&w.growthCue.GaugeRelease, dt)
	tickTimer(&w.growthCue.GaugeAfterglow, dt)
	if w.growthCue.FieldDelay > 0 {
		tickTimer(&w.growthCue.FieldDelay, dt)
	} else {
		tickTimer(&w.growthCue.FieldPulse, dt)
	}
	if w.growthCue.NodeDelay > 0 {
		tickTimer(&w.growthCue.NodeDelay, dt)
	} else {
		tickTimer(&w.growthCue.NodeCue, dt)
	}
	if growthCueTimersDone(w.growthCue) {
		w.growthCue = growthCueState{NodeID: -1}
		startNextGrowthCue(w)
	}
}

func growthCueActive(c growthCueState) bool {
	return c.Outcome != growthOutcomeNone || !growthCueTimersDone(c)
}

func growthCueTimersDone(c growthCueState) bool {
	return c.GaugeRelease == 0 &&
		c.GaugeAfterglow == 0 &&
		c.FieldDelay == 0 &&
		c.FieldPulse == 0 &&
		c.NodeDelay == 0 &&
		c.NodeCue == 0
}

func startNextGrowthCue(w *World) {
	if len(w.pendingGrowthCues) == 0 {
		return
	}
	w.growthCue = w.pendingGrowthCues[0]
	copy(w.pendingGrowthCues, w.pendingGrowthCues[1:])
	w.pendingGrowthCues = w.pendingGrowthCues[:len(w.pendingGrowthCues)-1]
}

func upgradeAllFieldsForDebug(w *World) bool {
	fp := w.Planet.FieldProgress[KindWood]
	if fp == nil {
		return false
	}
	amount := fp.Cap - fp.EXP
	if amount <= 0 {
		amount = fp.Cap
	}
	depositToField(w, KindWood, amount)
	return true
}

func growAllFieldsUntilBlockedForDebug(w *World) bool {
	const maxDebugGrowthSteps = 512
	for i := 0; i < maxDebugGrowthSteps; i++ {
		before := len(w.Nodes)
		if !upgradeAllFieldsForDebug(w) {
			return false
		}
		if len(w.Nodes) == before {
			return true
		}
	}
	return true
}

// nurtureGrowthCuePending reports whether a growth cue is currently playing or
// queued. Nurture is blocked while cues are pending so each press has
// unambiguous visual feedback before the next can fire.
func nurtureGrowthCuePending(w *World) bool {
	return growthCueActive(w.growthCue) || len(w.pendingGrowthCues) > 0
}

// pickPlanetNurtureField selects a known, non-influence field across all kinds
// for a Nurture press. Fields that can still accept a node/sparkle are eligible
// (selected uniformly); if all are saturated, any known non-influence field is
// returned so spawnNode/spawnSparkle can apply the upgrade path.
func pickPlanetNurtureField(w *World) *ResourceField {
	var eligible []*ResourceField
	var fallback *ResourceField
	for _, f := range w.Planet.Fields {
		if !f.Known || f.Kind == KindWaterInfluence {
			continue
		}
		if f.Kind == KindWater && !waterSparkleSpawningUnlocked(w) {
			continue
		}
		if fallback == nil {
			fallback = f
		}
		if fieldCanSpawn(w, f) {
			eligible = append(eligible, f)
		}
	}
	if len(eligible) > 0 {
		return eligible[w.rng.Intn(len(eligible))]
	}
	return fallback
}

// nurtureField directly spawns up to nurtureTreesPerPress new nodes/sparkles
// across all known eligible fields on the active planet. Returns false if
// resources are not yet discovered, no known field exists, or a growth cue is
// already playing.
func nurtureField(w *World) bool {
	if !w.ResourceDiscovered || nurtureGrowthCuePending(w) {
		return false
	}
	if pickPlanetNurtureField(w) == nil {
		return false
	}
	for range nurtureTreesPerPress {
		f := pickPlanetNurtureField(w)
		if f == nil {
			break
		}
		activateGrowthCue(w, spawnIntoField(w, f))
	}
	return true
}

// nurtureAttentionActive reports whether the Nurture button should show its
// attention pulse. Fires when:
//   - planet has reached its minimum completion population + any known field can still spawn, or
//   - at least one dock exists but no reachable sparkle work exists.
func nurtureAttentionActive(w *World) bool {
	if !w.ResourceDiscovered || nurtureGrowthCuePending(w) {
		return false
	}
	// Dock-no-work rule: nudge player to grow sparkles only when docks have no
	// reachable water work. A dock can have zero estimated rate simply because no
	// worker is currently assigned.
	if dockExists(w) && !dockHasServiceableSparkles(w) {
		return true
	}
	// Standard rule: minimum completion population reached + any field can still spawn.
	if !planetPopComplete(w) {
		return false
	}
	for _, f := range w.Planet.Fields {
		if !f.Known || f.Kind == KindWaterInfluence {
			continue
		}
		if fieldCanSpawn(w, f) {
			return true
		}
	}
	return false
}

// anyFieldCanSpawn reports whether any known non-influence field on the active
// planet can still accept a new node or sparkle.
func anyFieldCanSpawn(w *World) bool {
	for _, f := range w.Planet.Fields {
		if !f.Known || f.Kind == KindWaterInfluence {
			continue
		}
		if fieldCanSpawn(w, f) {
			return true
		}
	}
	return false
}

// dockExists reports whether the world has at least one dock building.
func dockExists(w *World) bool {
	for _, b := range w.Buildings {
		if b.Kind == KindDock {
			return true
		}
	}
	return false
}

// upgradeDock spends dockL2WoodCost wood and dockL2WaterCost water to upgrade
// a dock from Level 1 to Level 2. Returns false if the dock is nil, not a dock,
// already at Level ≥ 2, or resources are insufficient.
func upgradeDock(w *World, dock *Building) bool {
	if dock == nil || dock.Kind != KindDock || dock.Level >= 2 {
		return false
	}
	if w.Economy.Wood < dockL2WoodCost || w.Economy.Water < dockL2WaterCost {
		return false
	}
	w.Economy.Wood -= dockL2WoodCost
	w.Economy.Water -= dockL2WaterCost
	dock.Level = 2
	activatePulse(w, &dock.Pulse)
	return true
}

// canUpgradeDock reports whether dock can be upgraded to Level 2 right now.
func canUpgradeDock(w *World, dock *Building) bool {
	if dock == nil || dock.Kind != KindDock || dock.Level >= 2 {
		return false
	}
	return w.Economy.Wood >= dockL2WoodCost && w.Economy.Water >= dockL2WaterCost
}

// dockUpgradeAttentionDock returns the first Level-1 dock to pulse when the
// upgrade attention cue should fire: minimum population reached, all known fields are
// saturated, and no dock has reached Level 2 yet.
func dockUpgradeAttentionDock(w *World) *Building {
	if !planetPopComplete(w) {
		return nil
	}
	for _, f := range w.Planet.Fields {
		if !f.Known || f.Kind == KindWaterInfluence {
			continue
		}
		if fieldCanSpawn(w, f) {
			return nil
		}
	}
	for _, b := range w.Buildings {
		if b.Kind == KindDock && b.Level < 2 {
			return b
		}
	}
	return nil
}

// EstimateRate returns the analytic resource/sec for all active workers.
func EstimateRate(w *World) float64 {
	var rate float64
	for _, wk := range w.Workers {
		if !workerInLoop(wk) {
			continue
		}
		node := findNode(w, wk.NodeID)
		if node == nil {
			continue
		}
		dist := routeLen(w, node)
		if dist == math.MaxFloat64 {
			continue
		}
		tripTime := loadTime + unloadTime + 2*dist/workerSpeed
		rate += (baseLoadAmount * node.Size * (1 - woodFieldReturnRatio)) / tripTime
	}
	return rate
}

// EstimateWaterRate returns the analytic water/sec for active water workers.
func EstimateWaterRate(w *World) float64 {
	var rate float64
	th := townHall(w)
	if th == nil {
		return 0
	}
	for _, wk := range w.Workers {
		if !workerInWaterLoop(wk) {
			continue
		}
		dock := findBuilding(w, wk.DockID)
		if dock == nil {
			continue
		}
		sparkles := dockServiceableSparkles(w, dock)
		if len(sparkles) == 0 {
			continue
		}
		var totalCarry float64
		for _, s := range sparkles {
			totalCarry += baseLoadAmount * s.Size
		}
		rimDist := effectiveArc(w, th.Angle, dock.Angle) * 2
		diveDist := math.Sqrt(float64(len(sparkles))) * 15.0
		tripTime := (rimDist+diveDist)/workerSpeed + unloadTime
		if tripTime > 0 {
			rate += totalCarry * (1 - woodFieldReturnRatio) / tripTime
		}
	}
	return rate
}

func activeWorkerCount(w *World) int {
	active := 0
	for _, wk := range w.Workers {
		if workerInLoop(wk) {
			active++
		}
	}
	return active
}

// claimableNodeCount returns the number of exclusive work claims available on
// the planet. Each slot supports one owned worker role and contributes to the
// soft population cap.
func claimableNodeCount(w *World) int {
	count := 0
	for _, n := range w.Nodes {
		if n.ClaimableWorkSlot {
			count++
		}
	}
	for _, b := range w.Buildings {
		if b.ClaimableWorkSlot {
			count++
		}
	}
	return count
}

func planetPopComplete(w *World) bool {
	return w.Planet.MinCompletionPop > 0 && len(w.Workers) >= w.Planet.MinCompletionPop
}

// availableCapacity returns the number of unused worker slots implied by
// claimable work targets. Clamped to 0 so debug free-spawns past capacity do
// not yield a negative value that could trigger spurious growth spawns.
func availableCapacity(w *World) int {
	avail := claimableNodeCount(w) - len(w.Workers)
	if avail < 0 {
		return 0
	}
	return avail
}

func planetMinCompletionPop(p Planet) int {
	return len(townFieldSlots(p, &Building{Angle: 0}))
}

// townFieldSlots returns the world positions of every potential dwelling slot
// inside the town field, anchored to th.Angle and stepping inward by rows.
// len() of the result is the planet's max worker capacity.
// Returns nil if th is nil.
func townFieldSlots(p Planet, th *Building) []Vec {
	if th == nil {
		return nil
	}
	cos := math.Cos(th.Angle)
	sin := math.Sin(th.Angle)
	inx, iny := -cos, -sin // inward unit vector (toward planet center)
	tx, ty := -sin, cos    // tangent unit vector (counterclockwise along rim)

	rim := p.RimPoint(th.Angle)
	maxDepth := townFieldDepthFrac * p.Radius

	var slots []Vec
	for d := townFieldRimInset; d <= maxDepth; d += townFieldSlotSpacing {
		arcLen := 2 * (p.Radius - d) * townFieldHalfArc
		cols := int(arcLen / townFieldSlotSpacing)
		if cols == 0 {
			continue
		}
		for order := 0; order < cols; order++ {
			col := townFieldColumnIndex(order, cols)
			tangOffset := (float64(col) - float64(cols-1)/2) * townFieldSlotSpacing
			slots = append(slots, Vec{
				X: rim.X + inx*d + tx*tangOffset,
				Y: rim.Y + iny*d + ty*tangOffset,
			})
		}
	}
	return slots
}

func townFieldColumnIndex(order, cols int) int {
	centerLeft := (cols - 1) / 2
	if order == 0 {
		return centerLeft
	}
	step := (order + 1) / 2
	if order%2 == 1 {
		return centerLeft + step
	}
	return centerLeft - step
}

// tryConsumeGrowth spawns at most one worker when Town Growth has reached its
// cap and a slot is free, then resets growth and raises the cap.
//
// When all claimable work is filled, excess growth is banked in
// TownGrowthOverflow. It drains when field growth or dock placement creates
// another work target.
//
// A workerSpawnCooldown prevents overflow from triggering rapid-fire spawns.
func tryConsumeGrowth(w *World) bool {
	if w.Economy.TownGrowth < w.Economy.TownGrowthCap {
		return false
	}
	if availableCapacity(w) <= 0 {
		if excess := w.Economy.TownGrowth - w.Economy.TownGrowthCap; excess > 0 {
			w.Economy.TownGrowthOverflow += excess
		}
		w.Economy.TownGrowth = w.Economy.TownGrowthCap
		return false
	}
	// Enforce minimum gap between spawns so overflow doesn't burst all at once.
	if w.Economy.LastWorkerSpawnTime > 0 && w.SimTime-w.Economy.LastWorkerSpawnTime < workerSpawnCooldown {
		w.Economy.TownGrowth = w.Economy.TownGrowthCap
		return false
	}
	// Bank any excess above the cap before resetting.
	if excess := w.Economy.TownGrowth - w.Economy.TownGrowthCap; excess > 0 {
		w.Economy.TownGrowthOverflow += excess
	}
	th := townHall(w)
	if spawnWorkerAtTownHall(w) == nil {
		return false
	}
	w.Economy.LastWorkerSpawnTime = w.SimTime
	if th != nil {
		activatePulse(w, &th.Pulse)
	}
	// Transition from the scripted first-lesson cap to the normal-play ramp.
	// After that, grow geometrically like every other fill.
	if w.Economy.TownGrowthCap < townGrowthBaseCap {
		w.Economy.TownGrowthCap = townGrowthBaseCap
	} else {
		w.Economy.TownGrowthCap *= townGrowthCapGrowth
	}
	// Immediately drain overflow into the fresh gauge so the bar visually
	// refills and the next spawn is ready once the cooldown expires.
	w.Economy.TownGrowth = 0
	if w.Economy.TownGrowthOverflow > 0 {
		drain := math.Min(w.Economy.TownGrowthOverflow, w.Economy.TownGrowthCap)
		w.Economy.TownGrowth = drain
		w.Economy.TownGrowthOverflow -= drain
	}
	return true
}

// tickOverflowGrowth drains banked overflow into the growth gauge each tick
// and tries to spawn once the cooldown has expired. This allows overflow-driven
// spawns to proceed even when no delivery is incoming.
func tickOverflowGrowth(w *World) {
	if availableCapacity(w) <= 0 {
		return
	}
	if w.Economy.TownGrowthOverflow > 0 {
		if needed := w.Economy.TownGrowthCap - w.Economy.TownGrowth; needed > 0 {
			drain := math.Min(w.Economy.TownGrowthOverflow, needed)
			w.Economy.TownGrowth += drain
			w.Economy.TownGrowthOverflow -= drain
		}
	}
	tryConsumeGrowth(w)
}

func addFreeWorkerAtTownHall(w *World) bool {
	return spawnWorkerAtTownHall(w) != nil
}

// revealKindFields promotes every unknown field of the given kind to known on
// all planets in the system (active and parked). It also initialises
// FieldProgress for the kind so EXP can start accumulating immediately.
// Nurture re-arms itself automatically on the next frame because anyFieldCanSpawn
// and nurtureAttentionActive re-evaluate from field state.
func revealKindFields(w *World, kind ResourceKind) {
	reveal := func(p *Planet) {
		if p.FieldProgress == nil {
			p.FieldProgress = make(map[ResourceKind]*KindProgress)
		}
		for _, f := range p.Fields {
			if f.Kind == kind && !f.Known {
				f.Known = true
			}
		}
		if p.FieldProgress[kind] == nil {
			p.FieldProgress[kind] = &KindProgress{Cap: woodFieldBaseEXP}
		}
	}
	reveal(&w.Planet)
	for _, ps := range w.PlanetStates {
		if ps != nil {
			reveal(&ps.Planet)
		}
	}
}
