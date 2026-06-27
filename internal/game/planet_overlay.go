package game

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// drawReturnToSystemButton draws and records the top-right return-to-system button.
func (g *Game) drawReturnToSystemButton(screen *ebiten.Image) {
	scale, _, _ := viewGeom(g.screenW, g.screenH)
	sp := float32(scale)
	btnSize := float32(16 * scale)
	btnX := float32(g.screenW) - btnSize - float32(4*scale)
	btnY := float32(3 * scale)

	vector.FillRect(screen, btnX, btnY, btnSize, btnSize, colReturnFill, false)
	vector.StrokeRect(screen, btnX, btnY, btnSize, btnSize, 1, colReturnRim, false)
	bCx := btnX + btnSize/2
	bCy := btnY + btnSize/2
	bR := float32(4 * sp)
	vector.FillCircle(screen, bCx, bCy, bR, colReturnGlyph, false)
	drawSystemOrbitRing(screen, bCx, bCy, bR+float32(3*sp), 1, colReturnOrbit)
	for _, a := range []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2} {
		ox := bCx + (bR+float32(6*sp))*float32(math.Cos(a))
		oy := bCy + (bR+float32(6*sp))*float32(math.Sin(a))
		vector.FillRect(screen, ox-sp/2, oy-sp/2, sp, sp, colReturnDot, false)
	}

	g.sysReturnRect = sysRect{x: btnX, y: btnY, w: btnSize, h: btnSize}
}

// drawDockTray draws a bottom tray when a dock building is selected.
// Shows dock identity on the left and an upgrade action on the right.
// Level 1 docks show L1→L2 affordability via fill progress. Level 2 docks show
// a quiet Level-3 tease (unnamed future resource, not purchasable).
func (g *Game) drawDockTray(screen *ebiten.Image) {
	if g.selectedBuildingID < 0 || g.selectedBuildingID >= len(g.world.Buildings) {
		g.dockUpgradeRect = sysRect{}
		g.dockTrayRect = sysRect{}
		return
	}
	b := g.world.Buildings[g.selectedBuildingID]
	if b.Kind != KindDock {
		g.dockUpgradeRect = sysRect{}
		g.dockTrayRect = sysRect{}
		return
	}

	scale, _, _ := viewGeom(g.screenW, g.screenH)
	selectedHUD := func(base float64) float32 {
		return scaledHUDFloat(scale, selectedBuildingHUDScale, base)
	}

	th := selectedHUD(bottomHUDHeightBase)
	tx := float32(0)
	tw := float32(g.screenW)
	ty := float32(g.screenH) - th

	vector.FillRect(screen, tx, ty, tw, th, colSysTrayFill, false)
	vector.StrokeLine(screen, tx, ty, tx+tw, ty, 1, colSysTrayBorder, false)
	g.dockTrayRect = sysRect{x: tx, y: ty, w: tw, h: th}

	sp := selectedHUD(1)
	swSize := selectedHUD(bottomHUDSwatchBase)
	swY := ty + (th-swSize)/2

	level := b.Level
	if level == 0 {
		level = 1
	}
	pipHalfW := selectedHUD(3)
	pipGap := selectedHUD(2)
	pipCY := ty + th/2
	pipsW := float32(level)*(pipHalfW*2) + float32(level-1)*pipGap

	actionWpx := selectedHUD(selectedHUDActionWidthBase)
	actionInset := selectedHUD(selectedHUDActionInsetBase)
	actionH := th - actionInset*2
	ayTop := ty + actionInset
	contentGap := selectedHUD(selectedHUDContentGapBase)
	identityGap := selectedHUD(selectedHUDIdentityGapBase)
	totalW := swSize + identityGap + pipsW + contentGap + actionWpx
	cx := float32(g.screenW)/2 - totalW/2

	swX := cx
	vector.FillRect(screen, swX, swY, swSize, swSize, colDock, false)
	pipX := swX + swSize + identityGap + pipHalfW
	for i := 0; i < level; i++ {
		drawUpTriangle(screen, pipX+float32(i)*(pipHalfW*2+pipGap), pipCY, pipHalfW, colSysTrayBorder)
	}

	axLeft := swX + swSize + identityGap + pipsW + contentGap

	if b.Level >= 2 {
		g.dockUpgradeRect = sysRect{}
		greyCol := color.RGBA{R: 60, G: 60, B: 80, A: 200}
		qSize := selectedHUD(8)
		qX := axLeft + 4*sp
		qY := ayTop + (actionH-qSize)/2
		vector.FillRect(screen, qX, qY, qSize, qSize, greyCol, false)
		pipHalfW := selectedHUD(3)
		pipGap := selectedHUD(2)
		pipCY := qY + qSize/2
		pipX := qX + qSize + 3*sp + pipHalfW
		for i := 0; i < 3; i++ {
			drawUpTriangle(screen, pipX+float32(i)*(pipHalfW*2+pipGap), pipCY, pipHalfW, greyCol)
		}
	} else {
		btnX := axLeft
		btnW := actionWpx
		btnH := actionH

		affordable := canUpgradeDock(g.world, b)
		vector.FillRect(screen, btnX, ayTop, btnW, btnH, colSysTrayFill, false)

		woodFrac := affordabilityFrac(g.world.Economy.Wood, dockL2WoodCost)
		waterFrac := affordabilityFrac(g.world.Economy.Water, dockL2WaterCost)
		frac := woodFrac
		if waterFrac < frac {
			frac = waterFrac
		}
		if frac > 0 && frac < 1 {
			fillH := btnH * frac
			vector.FillRect(screen, btnX, ayTop+btnH-fillH, btnW, fillH,
				color.RGBA{R: 30, G: 55, B: 100, A: 200}, false)
		} else if affordable {
			vector.FillRect(screen, btnX, ayTop, btnW, btnH,
				color.RGBA{R: 30, G: 55, B: 100, A: 200}, false)
		}

		borderCol := colSysTrayBorder
		if affordable {
			borderCol = color.RGBA{R: 80, G: 120, B: 200, A: 220}
		}
		vector.StrokeRect(screen, btnX, ayTop, btnW, btnH, 1, borderCol, false)

		if pulseActive(g.world, b.Pulse) {
			atCol := colNurtureAttention
			atCol.A = 180
			vector.StrokeRect(screen, btnX-1, ayTop-1, btnW+2, btnH+2, 1.5, atCol, false)
		}

		g.dockUpgradeRect = sysRect{x: btnX, y: ayTop, w: btnW, h: btnH}
	}
}

// drawTownHallTray draws a bottom tray when the Town Hall is selected.
// Mirrors the dock tray: TH swatch + one level pip, then a capacity action button.
// When the town field is full the button is greyed out and non-interactive.
func (g *Game) drawTownHallTray(screen *ebiten.Image) {
	if g.selectedBuildingID < 0 || g.selectedBuildingID >= len(g.world.Buildings) {
		g.thCapacityRect = sysRect{}
		g.thTrayRect = sysRect{}
		return
	}
	b := g.world.Buildings[g.selectedBuildingID]
	if b.Kind != KindTownHall {
		g.thCapacityRect = sysRect{}
		g.thTrayRect = sysRect{}
		return
	}

	scale, _, _ := viewGeom(g.screenW, g.screenH)
	selectedHUD := func(base float64) float32 {
		return scaledHUDFloat(scale, selectedBuildingHUDScale, base)
	}

	th := selectedHUD(bottomHUDHeightBase)
	tx := float32(0)
	tw := float32(g.screenW)
	ty := float32(g.screenH) - th

	vector.FillRect(screen, tx, ty, tw, th, colSysTrayFill, false)
	vector.StrokeLine(screen, tx, ty, tx+tw, ty, 1, colSysTrayBorder, false)
	g.thTrayRect = sysRect{x: tx, y: ty, w: tw, h: th}

	swSize := selectedHUD(bottomHUDSwatchBase)
	swY := ty + (th-swSize)/2

	pipHalfW := selectedHUD(3)
	pipCY := ty + th/2
	pipsW := pipHalfW * 2

	actionWpx := selectedHUD(selectedHUDActionWidthBase)
	actionInset := selectedHUD(selectedHUDActionInsetBase)
	actionH := th - actionInset*2
	ayTop := ty + actionInset
	contentGap := selectedHUD(selectedHUDContentGapBase)
	identityGap := selectedHUD(selectedHUDIdentityGapBase)

	totalW := swSize + identityGap + pipsW + contentGap + actionWpx
	cx := float32(g.screenW)/2 - totalW/2

	swX := cx
	vector.FillRect(screen, swX, swY, swSize, swSize, colTownHall, false)
	drawUpTriangle(screen, swX+swSize+identityGap+pipHalfW, pipCY, pipHalfW, colSysTrayBorder)

	axLeft := swX + swSize + identityGap + pipsW + contentGap

	isFull := townFieldFull(g.world)
	if isFull {
		g.thCapacityRect = sysRect{}
		vector.FillRect(screen, axLeft, ayTop, actionWpx, actionH,
			color.RGBA{R: 30, G: 30, B: 45, A: 200}, false)
		vector.StrokeRect(screen, axLeft, ayTop, actionWpx, actionH, 1, colSysTrayBorder, false)
	} else {
		cost := townCapacityCost(g.world)
		payKind := townCapacityPaymentKind(g.world)
		affordable := townCapacityAffordable(g.world)
		btnX := axLeft
		btnW := actionWpx
		btnH := actionH

		vector.FillRect(screen, btnX, ayTop, btnW, btnH, colSysTrayFill, false)

		fillCol := colWoodResource
		fillA := uint8(200)
		frac := affordabilityFrac(g.world.Economy.Wood, cost)
		if payKind == KindWater {
			fillCol = colSparkle
			frac = affordabilityFrac(g.world.Economy.Water, cost)
		}
		if frac > 0 && frac < 1 {
			fillH := btnH * frac
			vector.FillRect(screen, btnX, ayTop+btnH-fillH, btnW, fillH,
				color.RGBA{R: fillCol.R, G: fillCol.G, B: fillCol.B, A: fillA}, false)
		} else if affordable {
			vector.FillRect(screen, btnX, ayTop, btnW, btnH,
				color.RGBA{R: fillCol.R, G: fillCol.G, B: fillCol.B, A: fillA}, false)
		}

		borderCol := colTownCapacity
		if affordable {
			borderCol = color.RGBA{R: colTownCapacity.R, G: colTownCapacity.G, B: colTownCapacity.B, A: 220}
		}
		vector.StrokeRect(screen, btnX, ayTop, btnW, btnH, 1, borderCol, false)

		if g.thCapacityAttentionLeft > 0 {
			atCol := colNurtureAttention
			atCol.A = 180
			vector.StrokeRect(screen, btnX-1, ayTop-1, btnW+2, btnH+2, 1.5, atCol, false)
		}

		g.thCapacityRect = sysRect{x: btnX, y: ayTop, w: btnW, h: btnH}
	}
}

// drawWorkerHUDOverlay replaces the plain worker icon and ratio text with a
// mini slider icon and per-kind worker counts once LaborFocus is set.
func (g *Game) drawWorkerHUDOverlay(screen *ebiten.Image) {
	w := g.world
	if !laborFocusControlAvailable(w) {
		return
	}
	if g.hud == nil {
		return
	}

	nWood, nWater, nIdle := activeWorkerHUDCounts(w)

	sqR := g.hud.workerSquare.GetWidget().Rect
	sqX := float32(sqR.Min.X)
	sqY := float32(sqR.Min.Y)
	sqW := float32(sqR.Dx())
	sqH := float32(sqR.Dy())

	vector.FillRect(screen, sqX, sqY, sqW, sqH, colSysTrayFill, false)
	vector.StrokeRect(screen, sqX, sqY, sqW, sqH, 1, colWorkerLaden, false)

	inset := sqH * 0.22
	innerX := sqX + inset
	innerY := sqY + inset
	innerW := sqW - inset*2
	innerH := sqH - inset*2
	halfGap := float32(1)
	halfW := (innerW - halfGap) / 2
	vector.FillRect(screen, innerX, innerY, halfW, innerH, colWoodResource, false)
	vector.FillRect(screen, innerX+halfW+halfGap, innerY, halfW, innerH, colSparkle, false)
	vector.StrokeLine(screen, innerX+halfW+halfGap/2, innerY, innerX+halfW+halfGap/2, innerY+innerH, 1, colWorkerLaden, false)

	if len(w.LaborFocus) == 0 {
		return
	}

	txtR := g.hud.workerRatio.GetWidget().Rect
	txtX := float32(txtR.Min.X)
	txtW := float32(txtR.Dx())

	bgPad := float32(2)
	vector.FillRect(screen, txtX-bgPad, sqY, txtW+bgPad*2, sqH, colSysTrayFill, false)

	barH := sqH * 0.48
	barY := sqY + (sqH-barH)/2
	barX := txtX
	barW := txtW
	vector.StrokeRect(screen, barX, barY, barW, barH, 1, colSysTrayBorder, false)

	total := nWood + nWater + nIdle
	if total == 0 {
		return
	}
	type workerSeg struct {
		count int
		col   color.RGBA
	}
	segments := []workerSeg{
		{nWood, colWoodResource},
		{nWater, color.RGBA{R: colSparkle.R, G: colSparkle.G, B: colSparkle.B, A: 240}},
		{nIdle, colWorkerLaden},
	}
	nonZero := 0
	for _, seg := range segments {
		if seg.count > 0 {
			nonZero++
		}
	}
	fillX := barX + 1
	fillW := barW - 2
	remainingW := fillW
	drawn := 0
	for _, seg := range segments {
		if seg.count <= 0 {
			continue
		}
		drawn++
		w := remainingW
		if drawn < nonZero {
			w = fillW * float32(seg.count) / float32(total)
			if w < 2 {
				w = 2
			}
			if w > remainingW {
				w = remainingW
			}
		}
		vector.FillRect(screen, fillX, barY+1, w, barH-2, seg.col, false)
		fillX += w
		remainingW -= w
		if remainingW <= 0 {
			break
		}
	}
}
