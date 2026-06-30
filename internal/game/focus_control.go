package game

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// openFocusControl opens the labor focus control, initialising the draft split
// from the current LaborFocus (or all wood if none has been set yet).
func (g *Game) openFocusControl() {
	total := len(g.world.Workers)
	if total == 0 {
		return
	}
	if !laborFocusControlAvailable(g.world) {
		return
	}
	g.world.WorkerRatioSeen = true
	g.workerRatioAttentionLeft = 0
	g.workerRatioAttentionReady = false
	if w, ok := g.world.LaborFocus[KindWater]; ok {
		g.focusDraftWater = w
	} else {
		g.focusDraftWater = 0
	}
	g.showFocusControl = true
	g.focusDragging = false
}

// drawFocusControl renders the two-resource labor focus overlay and records
// hit-test rects for input handling.
func (g *Game) drawFocusControl(screen *ebiten.Image) {
	if !g.showFocusControl {
		g.focusCtrlRect = sysRect{}
		g.focusSlotsRect = sysRect{}
		g.focusConfirmRect = sysRect{}
		g.focusCancelRect = sysRect{}
		return
	}

	total := len(g.world.Workers)
	if total == 0 {
		g.showFocusControl = false
		return
	}

	scale, _, _ := viewGeom(g.screenW, g.screenH)
	sp := float32(scale)
	face := g.hud.sysface
	if face == nil {
		return
	}

	nWater := g.focusDraftWater
	if nWater < 0 {
		nWater = 0
	}
	if nWater > total {
		nWater = total
	}
	nWood := total - nWater

	const (
		trackW   = float64(86)
		trackH   = float64(5)
		hitH     = float64(18)
		knobW    = float64(5)
		labelW   = float64(14)
		labelGap = float64(4)
		padding  = float64(8)
		btnSize  = float64(16)
		btnGap   = float64(6)
	)

	panelW := trackW + 2*labelW + 2*labelGap + 2*padding
	panelH := padding + hitH + padding*0.75 + btnSize + padding

	pw := float32(panelW * float64(scale))
	ph := float32(panelH * float64(scale))
	px := float32(g.screenW)/2 - pw/2
	py := float32(g.screenH)/2 - ph/2

	vector.FillRect(screen, px, py, pw, ph, colSysTrayFill, false)
	vector.StrokeRect(screen, px, py, pw, ph, 1, colSysTrayBorder, false)
	g.focusCtrlRect = sysRect{x: px, y: py, w: pw, h: ph}

	labelWpx := float32(labelW * float64(scale))
	labelGapPx := float32(labelGap * float64(scale))
	trackX := px + float32(padding*float64(scale)) + labelWpx + labelGapPx
	trackWpx := float32(trackW * float64(scale))
	trackHpx := float32(trackH * float64(scale))
	hitHpx := float32(hitH * float64(scale))
	trackY := py + float32(padding*float64(scale)) + (hitHpx-trackHpx)/2
	splitT := float32(nWood) / float32(total)
	splitX := trackX + trackWpx*splitT

	labelCol := color.RGBA{R: 205, G: 220, B: 215, A: 235}
	labelY := trackY + trackHpx/2 - 4*sp
	woodStr := fmt.Sprintf("%d", nWood)
	woodTextW, _ := text.Measure(woodStr, face, 0)
	drawSysText(screen, woodStr, trackX-labelGapPx-float32(woodTextW), labelY, labelCol, face)
	waterStr := fmt.Sprintf("%d", nWater)
	drawSysText(screen, waterStr, trackX+trackWpx+labelGapPx, labelY, labelCol, face)

	vector.FillRect(screen, trackX, trackY, trackWpx, trackHpx, color.RGBA{R: 18, G: 28, B: 36, A: 230}, false)
	if nWood > 0 {
		vector.FillRect(screen, trackX, trackY, splitX-trackX, trackHpx, colWoodResource, false)
	}
	if nWater > 0 {
		vector.FillRect(screen, splitX, trackY, trackX+trackWpx-splitX, trackHpx, colSparkle, false)
	}
	vector.StrokeRect(screen, trackX, trackY, trackWpx, trackHpx, 1, color.RGBA{R: 95, G: 115, B: 125, A: 210}, false)

	knobWpx := float32(knobW * float64(scale))
	knobHpx := float32(14 * scale)
	knobX := splitX - knobWpx/2
	knobY := trackY + trackHpx/2 - knobHpx/2
	vector.FillRect(screen, knobX, knobY, knobWpx, knobHpx, color.RGBA{R: 225, G: 235, B: 225, A: 240}, false)
	vector.StrokeRect(screen, knobX, knobY, knobWpx, knobHpx, 1, color.RGBA{R: 30, G: 45, B: 40, A: 230}, false)

	g.focusSlotsRect = sysRect{x: trackX, y: trackY - (hitHpx-trackHpx)/2, w: trackWpx, h: hitHpx}

	btnY := py + ph - float32((padding+btnSize)*float64(scale))
	btnW := float32(btnSize * float64(scale))
	gap := float32(btnGap * float64(scale))
	btnStartX := px + pw/2 - btnW - gap/2
	g.focusConfirmRect = sysRect{x: btnStartX, y: btnY, w: btnW, h: btnW}
	g.focusCancelRect = sysRect{x: btnStartX + btnW + gap, y: btnY, w: btnW, h: btnW}

	confirmCol := color.RGBA{R: 24, G: 76, B: 42, A: 215}
	cancelCol := color.RGBA{R: 78, G: 30, B: 38, A: 215}
	vector.FillRect(screen, g.focusConfirmRect.x, g.focusConfirmRect.y, g.focusConfirmRect.w, g.focusConfirmRect.h, confirmCol, false)
	vector.StrokeRect(screen, g.focusConfirmRect.x, g.focusConfirmRect.y, g.focusConfirmRect.w, g.focusConfirmRect.h, 1, colSysTrayBorder, false)
	vector.FillRect(screen, g.focusCancelRect.x, g.focusCancelRect.y, g.focusCancelRect.w, g.focusCancelRect.h, cancelCol, false)
	vector.StrokeRect(screen, g.focusCancelRect.x, g.focusCancelRect.y, g.focusCancelRect.w, g.focusCancelRect.h, 1, colSysTrayBorder, false)

	iconPad := 4 * sp
	checkCol := color.RGBA{R: 178, G: 245, B: 190, A: 245}
	vector.StrokeLine(screen,
		g.focusConfirmRect.x+iconPad, g.focusConfirmRect.y+g.focusConfirmRect.h*0.55,
		g.focusConfirmRect.x+g.focusConfirmRect.w*0.43, g.focusConfirmRect.y+g.focusConfirmRect.h-iconPad,
		2, checkCol, false)
	vector.StrokeLine(screen,
		g.focusConfirmRect.x+g.focusConfirmRect.w*0.43, g.focusConfirmRect.y+g.focusConfirmRect.h-iconPad,
		g.focusConfirmRect.x+g.focusConfirmRect.w-iconPad, g.focusConfirmRect.y+iconPad,
		2, checkCol, false)

	xCol := color.RGBA{R: 245, G: 175, B: 180, A: 245}
	vector.StrokeLine(screen,
		g.focusCancelRect.x+iconPad, g.focusCancelRect.y+iconPad,
		g.focusCancelRect.x+g.focusCancelRect.w-iconPad, g.focusCancelRect.y+g.focusCancelRect.h-iconPad,
		2, xCol, false)
	vector.StrokeLine(screen,
		g.focusCancelRect.x+g.focusCancelRect.w-iconPad, g.focusCancelRect.y+iconPad,
		g.focusCancelRect.x+iconPad, g.focusCancelRect.y+g.focusCancelRect.h-iconPad,
		2, xCol, false)
}

// handleFocusControlInput processes mouse events while the focus control is open.
func (g *Game) handleFocusControlInput() {
	total := len(g.world.Workers)
	mx, my := ebiten.CursorPosition()
	fmx, fmy := float32(mx), float32(my)

	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if g.focusSlotsRect.contains(fmx, fmy) || g.focusDragging {
			if g.focusSlotsRect.contains(fmx, fmy) {
				g.focusDragging = true
			}
			if g.focusDragging && g.focusSlotsRect.w > 0 {
				t := (fmx - g.focusSlotsRect.x) / g.focusSlotsRect.w
				if t < 0 {
					t = 0
				}
				if t > 1 {
					t = 1
				}
				rawWood := int(math.Round(float64(t) * float64(total)))
				nWater := total - rawWood
				if nWater < 0 {
					nWater = 0
				}
				if nWater > total {
					nWater = total
				}
				g.focusDraftWater = nWater
			}
		}
	} else {
		g.focusDragging = false
	}

	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}

	if g.focusConfirmRect.contains(fmx, fmy) {
		nWater := g.focusDraftWater
		nWood := total - nWater
		g.world.LaborFocus = laborFocusMap(nWood, nWater)
		g.world.SavedLaborRatio = laborFocusMap(nWood, nWater)
		g.showFocusControl = false
		return
	}
	if g.focusCancelRect.contains(fmx, fmy) || !g.focusCtrlRect.contains(fmx, fmy) {
		g.showFocusControl = false
	}
}
