package game

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Building / worker render sizes.
const (
	campBldHalf      = float32(3.5) // half of 7×7 camp square
	campBldSize      = float32(7)
	townHallBldHalfW = float32(8)   // half-width along rim tangent (16px wide — fort shape)
	townHallBldHalfH = float32(4.5) // half-height along inward normal (9px tall)
	townHallBldInset = float32(5)   // px inward from rim for town hall art center
	workerBldHalf    = float32(1)   // half of 3×3 worker square
	workerBldSize    = float32(3)
	idleMaxSlots     = 5 // max visible idle-worker spots near the town hall
)

func workerUsesIdleHome(wk *Worker) bool {
	switch wk.State {
	case StateIdleWaiting, StateSettling, StateReactionDelay:
		return true
	default:
		return false
	}
}

func workerColor(w *World, wk *Worker) color.RGBA {
	col := colWorkerEmpty
	if wk.State == StateReturningHome || wk.State == StateToIdleSpot {
		col = colWorkerReturn
	}
	if wk.Carried > 0 || wk.State == StateToBuilding || wk.State == StateUnloading {
		col = colWorkerLaden
	}
	if wk.DeliveryKind == KindDock {
		switch wk.State {
		case StateDiving, StateSwimmingToDock, StateDockUnloading:
			col = colWorkerLadenWater
		default:
			if wk.Carried > 0 {
				col = colWorkerLadenWater
			}
		}
	}
	if pulseActive(w, wk.Pulse) {
		col = brighten(col, 35)
	}
	return col
}

// drawTownHallArt draws the Town Hall as a wide fort-shaped rectangle oriented
// along the rim (wide in the tangential direction, short inward).
func drawTownHallArt(scene *ebiten.Image, p Planet, angle float64, col color.RGBA) {
	ip := insetPoint(p, angle, float64(townHallBldInset))
	// Inward normal (toward planet center) and tangent (along rim, ccw).
	ix := float32(-math.Cos(angle))
	iy := float32(-math.Sin(angle))
	tx := float32(-math.Sin(angle))
	ty := float32(math.Cos(angle))
	drawOrientedRect(scene, float32(ip.X), float32(ip.Y), tx, ty, ix, iy,
		townHallBldHalfW, townHallBldHalfH, col)
}

// drawDockArt draws a |_| dock on the rim: a flat deck along the tangent with a
// short post at each end extending outward.
func drawDockArt(scene *ebiten.Image, p Planet, angle float64, col color.RGBA) {
	pos := p.RimPoint(angle)
	ox := float32(math.Cos(angle))
	oy := float32(math.Sin(angle))
	tx := float32(-math.Sin(angle))
	ty := float32(math.Cos(angle))
	cx, cy := float32(pos.X), float32(pos.Y)
	// Deck along the rim tangent, offset inward so its outward edge is flush with the rim.
	dcx := cx - ox*dockDeckHalfH
	dcy := cy - oy*dockDeckHalfH
	drawOrientedRect(scene, dcx, dcy, tx, ty, ox, oy, dockDeckHalfLen, dockDeckHalfH, col)
	// Posts at each end, offset outward so they extend above the rim.
	for _, s := range [...]float32{-1, 1} {
		px := cx + tx*(s*dockDeckHalfLen) + ox*dockPostHalfH
		py := cy + ty*(s*dockDeckHalfLen) + oy*dockPostHalfH
		drawOrientedRect(scene, px, py, tx, ty, ox, oy, dockPostHalfW, dockPostHalfH, col)
	}
}

// drawTownGrowthGauge draws a small progress bar below the Town Hall art,
// aligned with the rim tangent, showing Town Growth / TownGrowthCap.
func drawTownGrowthGauge(scene *ebiten.Image, p Planet, th *Building, growth, cap float64) {
	if cap <= 0 {
		return
	}
	frac := float32(growth / cap)
	if frac > 1 {
		frac = 1
	}
	const gaugeInset = float32(townHallBldInset) + float32(townHallBldHalfH) + 3
	anchor := insetPoint(p, th.Angle, float64(gaugeInset))
	ax, ay := float32(anchor.X), float32(anchor.Y)
	ix := float32(-math.Cos(th.Angle))
	iy := float32(-math.Sin(th.Angle))
	tx := float32(-math.Sin(th.Angle))
	ty := float32(math.Cos(th.Angle))
	const halfW = float32(townHallBldHalfW) - 2
	const halfH = float32(1)
	// Frame
	drawOrientedRect(scene, ax, ay, tx, ty, ix, iy, halfW, halfH, colTownGrowthGaugeFrame)
	// Fill (from one end along the tangent)
	if frac > 0 {
		fillHW := halfW * frac
		fcx := ax - tx*(halfW-fillHW)
		fcy := ay - ty*(halfW-fillHW)
		drawOrientedRect(scene, fcx, fcy, tx, ty, ix, iy, fillHW, halfH, colTownGrowthGaugeFill)
	}
}

// insetPoint returns a world position stepped inward from the rim at angle by
// offset pixels toward the planet center.
func insetPoint(p Planet, angle, offset float64) Vec {
	rim := p.RimPoint(angle)
	return Vec{
		X: rim.X - math.Cos(angle)*offset,
		Y: rim.Y - math.Sin(angle)*offset,
	}
}

// idleHomeSlots returns up to idleMaxSlots distinct world positions for idle
// workers, arranged in a small 2-column grid inset inside the rim near th.
// Returns nil if th is nil or count ≤ 0. Count is capped at idleMaxSlots.
func idleHomeSlots(p Planet, th *Building, count int) []Vec {
	if th == nil || count <= 0 {
		return nil
	}
	if count > idleMaxSlots {
		count = idleMaxSlots
	}
	// Inward and tangent unit vectors at the Town Hall angle.
	cos := math.Cos(th.Angle)
	sin := math.Sin(th.Angle)
	inx, iny := -cos, -sin // inward (toward planet center)
	tx, ty := -sin, cos    // tangent (counterclockwise along rim)
	// Anchor: 9 px inside the rim.
	rim := p.RimPoint(th.Angle)
	ax := rim.X + inx*9
	ay := rim.Y + iny*9
	// Grid: 2 columns × up to 3 rows, with a centred 5th slot.
	type off struct{ t, i float64 }
	slotOffsets := [idleMaxSlots]off{
		{-2.5, 0}, {+2.5, 0}, // row 0
		{-2.5, 4}, {+2.5, 4}, // row 1
		{0, 8}, // row 2 (centred)
	}
	slots := make([]Vec, count)
	for i := 0; i < count; i++ {
		o := slotOffsets[i]
		slots[i] = Vec{
			X: ax + tx*o.t + inx*o.i,
			Y: ay + ty*o.t + iny*o.i,
		}
	}
	return slots
}

// drawIdleOverflow draws a compact dot on/inside the Town Hall for idle workers
// beyond the idleMaxSlots visible spots. Dot size and brightness scale subtly
// with overflowCount (bounded).
func drawIdleOverflow(scene *ebiten.Image, p Planet, th *Building, overflowCount int) {
	t := float32(overflowCount-1) / 19.0 // 0 at 1, 1 at 20+
	if t > 1 {
		t = 1
	}
	radius := float32(1.5) + 2.0*t
	bright := uint8(120 + uint8(100*t))
	col := color.RGBA{R: bright, G: bright, B: bright + 20, A: 200}
	ip := insetPoint(p, th.Angle, float64(townHallBldInset))
	vector.FillCircle(scene, float32(ip.X), float32(ip.Y), radius, col, false)
}

// drawTownField renders the settlement wedge inside the planet at the Town Hall
// angle, with visible dwelling slots for built capacity. No-op until a Town
// Hall exists.
func drawTownField(scene *ebiten.Image, w *World, radius float32) {
	th := townHall(w)
	if th == nil {
		return
	}
	cx, cy := float32(w.Planet.Center.X), float32(w.Planet.Center.Y)
	start := th.Angle - townFieldHalfArc
	end := th.Angle + townFieldHalfArc

	// Warm clay wedge fill — full pizza slice from center to rim.
	drawFilledSector(scene, cx, cy, radius, start, end, colTownFieldBase)
	// Outer edge definition.
	drawFieldSectorBand(scene, cx, cy, radius-0.5, 1.5, start, end, colTownFieldEdge)

	// Dwelling slots — only built capacity is visible, so fresh towns start with
	// one house and fill in one purchase at a time.
	slots := townFieldSlots(w.Planet, th)
	if len(slots) == 0 {
		return
	}
	builtSlots := w.Economy.WorkerCapacity
	if builtSlots < 0 {
		builtSlots = 0
	}
	if builtSlots > len(slots) {
		builtSlots = len(slots)
	}
	occupiedSlots := len(w.Workers)
	if occupiedSlots > builtSlots {
		occupiedSlots = builtSlots
	}
	cos := float32(math.Cos(th.Angle))
	sin := float32(math.Sin(th.Angle))
	ix := -cos // inward
	iy := -sin
	tx := -sin // tangent
	ty := cos
	for i, pos := range slots[:builtSlots] {
		col := colTownFieldSlot
		if i < occupiedSlots {
			col = colTownFieldSlotOccupied
		}
		drawOrientedRect(scene, float32(pos.X), float32(pos.Y), tx, ty, ix, iy, 1.5, 1.5, col)
	}
}
