package game

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Building / worker render sizes.
const (
	townHallBldHalfW = float32(8)   // half-width along rim tangent (16px wide — fort shape)
	townHallBldHalfH = float32(4.5) // half-height along inward normal (9px tall)
	townHallBldInset = float32(5)   // px inward from rim for town hall art center
	workerBldSize    = float32(3)   // idle slot spacing near Town Hall
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
		case StateDiving, StateDiveLoading, StateSwimmingToDock, StateDockUnloading:
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

// insetPoint returns a world position stepped inward from the rim at angle by
// offset pixels toward the planet center.
func insetPoint(p Planet, angle, offset float64) Vec {
	rim := p.RimPoint(angle)
	return Vec{
		X: rim.X - math.Cos(angle)*offset,
		Y: rim.Y - math.Sin(angle)*offset,
	}
}

// idleTowerSlots returns world positions for idle workers stacked outward above
// the Town Hall, extending into space. No cap — one slot per worker.
func idleTowerSlots(p Planet, th *Building, count int) []Vec {
	if th == nil || count <= 0 {
		return nil
	}
	slots := make([]Vec, count)
	for i := range slots {
		outset := float64(i)*float64(workerBldSize) + float64(workerBldSize)*0.5
		slots[i] = insetPoint(p, th.Angle, -outset)
	}
	return slots
}
