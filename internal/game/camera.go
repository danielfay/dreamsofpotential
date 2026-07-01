package game

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	// Planet view
	planetZoomMin    = 0.75
	planetZoomMax    = 2.5
	planetNudgeZone  = 20.0 // screen pixels from edge that triggers nudge
	planetNudgeSpeed = 15.0 // virtual pixels per second
	planetZoomStep   = 0.12 // fractional zoom change per wheel notch

	// System view
	sysZoomMax      = 3.0
	sysEdgeZone     = 20.0 // screen pixels from edge that triggers scrolling
	sysScrollSpeed  = 60.0 // virtual pixels per second
	sysZoomStep     = 0.12
	sysZoomMinFloor = 0.3 // absolute floor regardless of planet layout
)

// viewCamera describes a camera in virtual (scene) coordinate space.
// The visible area in world coords is centred on (x, y) and spans
// (virtW/zoom) × (virtH/zoom) virtual pixels.
type viewCamera struct {
	x, y float64
	zoom float64
}

// nudgeSpeed returns the effective planet nudge speed based on user settings.
// At PlanetNudgeSpeedPct=50 (or 0/unset) it equals planetNudgeSpeed.
func (s *Settings) nudgeSpeed() float64 {
	pct := s.PlanetNudgeSpeedPct
	if pct <= 0 {
		pct = 50
	}
	return planetNudgeSpeed * float64(pct) / 50.0
}

// sysSpeed returns the effective system-view edge scroll speed based on user settings.
// At SysScrollSpeedPct=50 (or 0/unset) it equals sysScrollSpeed.
func (s *Settings) sysSpeed() float64 {
	pct := s.SysScrollSpeedPct
	if pct <= 0 {
		pct = 50
	}
	return sysScrollSpeed * float64(pct) / 50.0
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// activeCamera returns the camera for the currently active view.
// During reveal the caller uses a neutral camera instead.
func (g *Game) activeCamera() viewCamera {
	if g.world.System.View == ViewSystem {
		return g.sysCam
	}
	return g.planetCam
}

// updatePlanetCamera handles scroll-wheel zoom and edge-nudge for the planet view.
func (g *Game) updatePlanetCamera() {
	// Scroll wheel zoom.
	_, wy := ebiten.Wheel()
	if wy != 0 {
		g.planetCam.zoom *= math.Pow(1+planetZoomStep, wy)
		g.planetCam.zoom = clampF(g.planetCam.zoom, planetZoomMin, planetZoomMax)
	}

	// Left-button drag-to-pan.
	mx, my := ebiten.CursorPosition()
	inWindow := mx >= 0 && mx < g.screenW && my >= 0 && my < g.screenH
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && inWindow {
		if !g.planetDragActive {
			g.planetDragActive = true
			g.planetDragLastX = mx
			g.planetDragLastY = my
		} else {
			scale, _, _ := viewGeom(g.screenW, g.screenH)
			totalScale := scale * g.planetCam.zoom
			if totalScale > 0 {
				g.planetCam.x -= float64(mx-g.planetDragLastX) / totalScale
				g.planetCam.y -= float64(my-g.planetDragLastY) / totalScale
			}
			g.planetDragLastX = mx
			g.planetDragLastY = my
		}
	} else {
		g.planetDragActive = false
	}

	// Edge nudge: subtle drift when cursor is near a screen edge.
	// Only active while the cursor is inside the window.
	fmx, fmy := float64(mx), float64(my)
	sw, sh := float64(g.screenW), float64(g.screenH)
	if sw > 0 && sh > 0 && inWindow {
		nudge := g.world.Settings.nudgeSpeed() * dt / g.planetCam.zoom
		if fmx < planetNudgeZone {
			g.planetCam.x -= nudge
		} else if fmx > sw-planetNudgeZone {
			g.planetCam.x += nudge
		}
		if fmy < planetNudgeZone {
			g.planetCam.y -= nudge
		} else if fmy > sh-planetNudgeZone {
			g.planetCam.y += nudge
		}
	}

	// Clamp so the planet centre remains visible on screen.
	g.clampPlanetCamera()
}

// clampPlanetCamera constrains the planet camera so the planet centre stays
// within a margin of the screen centre. The margin is expressed in world coords
// and shrinks with zoom so the planet stays framed at all zoom levels.
func (g *Game) clampPlanetCamera() {
	const margin = 10.0
	pcX := g.world.Planet.Center.X
	pcY := g.world.Planet.Center.Y

	halfW := float64(virtW)/(2*g.planetCam.zoom) - margin
	halfH := float64(virtH)/(2*g.planetCam.zoom) - margin
	if halfW < 0 {
		halfW = 0
	}
	if halfH < 0 {
		halfH = 0
	}

	g.planetCam.x = clampF(g.planetCam.x, pcX-halfW, pcX+halfW)
	g.planetCam.y = clampF(g.planetCam.y, pcY-halfH, pcY+halfH)
}

// resetPlanetCamera snaps the planet camera back to the planet centre at default zoom.
func (g *Game) resetPlanetCamera() {
	g.planetCam = viewCamera{
		x:    g.world.Planet.Center.X,
		y:    g.world.Planet.Center.Y,
		zoom: 1.0,
	}
}

// updateSystemCamera handles scroll-wheel zoom, left-button drag-to-pan, and
// edge scrolling for the system view.
func (g *Game) updateSystemCamera() {
	// Scroll wheel zoom.
	_, wy := ebiten.Wheel()
	if wy != 0 {
		g.sysCam.zoom *= math.Pow(1+sysZoomStep, wy)
		minZ := g.sysMinZoom()
		g.sysCam.zoom = clampF(g.sysCam.zoom, minZ, sysZoomMax)
	}

	mx, my := ebiten.CursorPosition()
	inWindow := mx >= 0 && mx < g.screenW && my >= 0 && my < g.screenH

	// Left-button drag-to-pan: convert pixel delta to world-space delta.
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && inWindow {
		if !g.sysDragActive {
			g.sysDragActive = true
			g.sysDragLastX = mx
			g.sysDragLastY = my
		} else {
			scale, _, _ := viewGeom(g.screenW, g.screenH)
			totalScale := scale * g.sysCam.zoom
			if totalScale > 0 {
				g.sysCam.x -= float64(mx-g.sysDragLastX) / totalScale
				g.sysCam.y -= float64(my-g.sysDragLastY) / totalScale
			}
			g.sysDragLastX = mx
			g.sysDragLastY = my
		}
	} else {
		g.sysDragActive = false
	}

	// Edge scrolling: only active while the cursor is inside the window.
	fmx, fmy := float64(mx), float64(my)
	sw, sh := float64(g.screenW), float64(g.screenH)
	if sw > 0 && sh > 0 && inWindow {
		speed := g.world.Settings.sysSpeed() * dt
		if fmx < sysEdgeZone {
			g.sysCam.x -= speed
		} else if fmx > sw-sysEdgeZone {
			g.sysCam.x += speed
		}
		if fmy < sysEdgeZone {
			g.sysCam.y -= speed
		} else if fmy > sh-sysEdgeZone {
			g.sysCam.y += speed
		}
	}

	g.clampSysCamera()
}

// sysMinZoom computes the minimum zoom that still lets all revealed planets
// fit in the viewport (with the camera centred on their bounding box).
// The returned value decreases as more planets are revealed (more generous
// zoom-out allowed for a wider system).
func (g *Game) sysMinZoom() float64 {
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	n := 0
	for _, p := range g.world.System.Planets {
		if p.Kind == PlanetUnknown && !p.Awakened {
			continue // dormant silhouette excluded from zoom calculation
		}
		minX = math.Min(minX, p.Pos.X-float64(p.Radius))
		minY = math.Min(minY, p.Pos.Y-float64(p.Radius))
		maxX = math.Max(maxX, p.Pos.X+float64(p.Radius))
		maxY = math.Max(maxY, p.Pos.Y+float64(p.Radius))
		n++
	}
	if n == 0 {
		return sysZoomMinFloor
	}
	const pad = 24.0
	spanX := maxX - minX + 2*pad
	spanY := maxY - minY + 2*pad
	// zoomFit = max zoom at which all planets still fit; halve it for breathing room.
	zoomFit := math.Min(float64(virtW)/spanX, float64(virtH)/spanY)
	minZoom := zoomFit * 0.5
	return math.Max(minZoom, sysZoomMinFloor)
}

// clampSysCamera prevents the camera from scrolling past the planet bounding
// box boundaries. When zoomed out enough to see all content, the camera is
// snapped to the content centre.
func (g *Game) clampSysCamera() {
	// Build the scrollable region from all planet positions (including unknown silhouette).
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	for _, p := range g.world.System.Planets {
		const boundPad = 28.0
		minX = math.Min(minX, p.Pos.X-float64(p.Radius)-boundPad)
		minY = math.Min(minY, p.Pos.Y-float64(p.Radius)-boundPad)
		maxX = math.Max(maxX, p.Pos.X+float64(p.Radius)+boundPad)
		maxY = math.Max(maxY, p.Pos.Y+float64(p.Radius)+boundPad)
	}
	if minX > maxX {
		return
	}

	halfW := float64(virtW) / (2 * g.sysCam.zoom)
	halfH := float64(virtH) / (2 * g.sysCam.zoom)

	if 2*halfW >= maxX-minX {
		g.sysCam.x = (minX + maxX) / 2
	} else {
		g.sysCam.x = clampF(g.sysCam.x, minX+halfW, maxX-halfW)
	}
	if 2*halfH >= maxY-minY {
		g.sysCam.y = (minY + maxY) / 2
	} else {
		g.sysCam.y = clampF(g.sysCam.y, minY+halfH, maxY-halfH)
	}
}

// centerSystemCamOn smoothly moves the system camera to centre on pos.
// If centring would violate the boundary clamp the camera settles at the
// nearest valid position.
func (g *Game) centerSystemCamOn(pos Vec) {
	g.sysCam.x = pos.X
	g.sysCam.y = pos.Y
	g.clampSysCamera()
}
