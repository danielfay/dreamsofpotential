package game

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// drawSystemOverlay draws the system HUD and selected-planet tray.
func (g *Game) drawSystemOverlay(screen *ebiten.Image) {
	scale, _, _ := viewGeom(g.screenW, g.screenH)
	face := g.hud.sysface
	systemHUD := func(base float64) float32 {
		return scaledHUDFloat(scale, systemBottomHUDScale, base)
	}
	systemTopHUD := func(base float64) float32 {
		return scaledHUDFloat(scale, systemTopHUDScale, base)
	}

	for i := range resourceFamilies {
		fam := &resourceFamilies[i]
		g.sysInjectRect[fam.Potential] = sysRect{}
		g.sysAllocRect[fam.Potential] = sysRect{}
	}

	if g.world.ResourceDiscovered || g.world.System.Unlocked {
		type systemResourceColumn struct {
			showSquare    bool
			squareText    string
			squareCol     color.RGBA
			squareTextCol color.RGBA
			showCircle    bool
			circleCount   float64
			circleCol     color.RGBA
			circleTextCol color.RGBA
			potKind       PotentialKind
			width         float32
		}

		squareSize := systemTopHUD(systemTopHUDSquareBase)
		circleR := systemTopHUD(systemTopHUDCircleBase)
		textGap := systemTopHUD(bottomHUDTextGapBase)
		columnGap := systemTopHUD(systemTopHUDColumnGapBase)

		mkColumn := func(showSquare bool, squareText string, squareCol, squareTextCol color.RGBA, potKind PotentialKind, circleCol, circleTextCol color.RGBA) systemResourceColumn {
			count, earned := g.world.Economy.Potential[potKind]
			col := systemResourceColumn{
				showSquare:    showSquare,
				squareText:    squareText,
				squareCol:     squareCol,
				squareTextCol: squareTextCol,
				showCircle:    earned,
				circleCount:   count,
				circleCol:     circleCol,
				circleTextCol: circleTextCol,
				potKind:       potKind,
			}
			if showSquare {
				tw, _ := text.Measure(squareText, face, 0)
				col.width = squareSize + textGap + float32(tw)
			}
			if earned {
				whole := int(math.Floor(count))
				circleStr := fmt.Sprintf("%d", whole)
				tw, _ := text.Measure(circleStr, face, 0)
				w := circleR*2 + textGap + float32(tw)
				if w > col.width {
					col.width = w
				}
			}
			if g.world.System.View == ViewSystem && g.world.System.Unlocked {
				allocPipSz := systemTopHUD(systemTopHUDAllocPipBase)
				allocPipGap := systemTopHUD(1)
				allocW := 4*allocPipSz + 3*allocPipGap
				if allocW > col.width {
					col.width = allocW
				}
			}
			return col
		}

		var woodText, waterText string
		if g.world.System.View == ViewSystem {
			woodText = systemRateText(g.world.SystemEconomy.WoodRate)
			waterText = systemRateText(g.world.SystemEconomy.WaterRate)
		} else {
			woodText = fmt.Sprintf("%.0f (%s)", g.world.Economy.Wood, systemRateText(EstimateRate(g.world)))
			waterText = fmt.Sprintf("%.0f (%s)", g.world.Economy.Water, systemRateText(EstimateWaterRate(g.world)))
		}
		columns := []systemResourceColumn{
			mkColumn(
				g.world.ResourceDiscovered || g.world.System.Unlocked,
				woodText,
				colWoodResource, colWoodLabel,
				PotentialForest, colForestPotential, colForestPotentialLabel,
			),
			mkColumn(
				g.world.Economy.WaterDiscovered,
				waterText,
				colSparkle, color.RGBA{R: 100, G: 200, B: 255, A: 220},
				PotentialWater, colWaterPotential, colWaterPotentialLabel,
			),
		}

		var visibleCols []systemResourceColumn
		totalW := float32(0)
		for _, col := range columns {
			if !col.showSquare && !col.showCircle {
				continue
			}
			if len(visibleCols) > 0 {
				totalW += columnGap
			}
			totalW += col.width
			visibleCols = append(visibleCols, col)
		}

		if len(visibleCols) > 0 {
			h := systemTopHUD(systemTopHUDHeightBase)
			x := float32(0)
			y := float32(0)
			w := float32(g.screenW)
			vector.FillRect(screen, x, y, w, h, colSysTrayFill, false)
			vector.StrokeLine(screen, x, h, x+w, h, 1, colSysTrayBorder, false)

			paddingY := systemTopHUD(systemTopHUDPaddingVBase)
			rowGap := systemTopHUD(systemTopHUDRowGapBase)
			topY := paddingY
			bottomY := topY + squareSize + rowGap
			cx := float32(g.screenW)/2 - totalW/2
			for i, col := range visibleCols {
				if i > 0 {
					cx += columnGap
				}
				if col.showSquare {
					rowW := squareSize
					if col.squareText != "" {
						tw, _ := text.Measure(col.squareText, face, 0)
						rowW += textGap + float32(tw)
					}
					rowX := cx + (col.width-rowW)/2
					vector.FillRect(screen, rowX, topY, squareSize, squareSize, col.squareCol, false)
					if col.squareText != "" {
						_, th := text.Measure(col.squareText, face, 0)
						drawSysText(screen, col.squareText, rowX+squareSize+textGap, topY+squareSize-float32(th), col.squareTextCol, face)
					}
				}
				if col.showCircle {
					count := col.circleCount
					whole := int(math.Floor(count))
					frac := count - float64(whole)
					circleStr := fmt.Sprintf("%d", whole)
					tw, _ := text.Measure(circleStr, face, 0)
					rowW := circleR*2 + textGap + float32(tw)
					rowX := cx + (col.width-rowW)/2
					circleY := bottomY + circleR
					circleCX := rowX + circleR
					if frac > 0.01 {
						bgCol := col.circleCol
						bgCol.A = 50
						drawPotentialCircle(screen, circleCX, circleY, circleR, bgCol)
						drawFractionalCircle(screen, circleCX, circleY, circleR, frac, col.circleCol)
					} else {
						drawPotentialCircle(screen, circleCX, circleY, circleR, col.circleCol)
					}
					_, th := text.Measure(circleStr, face, 0)
					drawSysText(screen, circleStr, rowX+circleR*2+textGap, circleY-float32(th)/2, col.circleTextCol, face)
					if g.world.System.View == ViewSystem && g.world.System.Unlocked {
						r := sysRect{x: rowX, y: bottomY, w: rowW, h: circleR * 2}
						g.sysInjectRect[col.potKind] = r
						mx, my := ebiten.CursorPosition()
						if r.contains(float32(mx), float32(my)) {
							vector.StrokeCircle(screen, circleCX, circleY, circleR+1.5, 1.5, colInjectHover, false)
						}
					}
					if g.world.System.View == ViewSystem && g.world.System.Unlocked {
						var allocVal float64
						if fam := familyForPotential(col.potKind); fam != nil {
							allocVal = *fam.AllocPotential(&g.world.SystemEconomy)
						}
						allocLevel := int(math.Round(allocVal * 4))
						const nAllocPips = 4
						allocPipSz := systemTopHUD(systemTopHUDAllocPipBase)
						allocPipGap := systemTopHUD(1)
						allocW := float32(nAllocPips)*allocPipSz + float32(nAllocPips-1)*allocPipGap
						allocX := cx + (col.width-allocW)/2
						allocY := bottomY + circleR*2 + systemTopHUD(systemTopHUDRowGapBase)
						for i := 0; i < nAllocPips; i++ {
							px := allocX + float32(i)*(allocPipSz+allocPipGap)
							var pipCol color.RGBA
							if i < allocLevel {
								pipCol = col.circleCol
							} else {
								pipCol = color.RGBA{R: 55, G: 55, B: 75, A: 220}
							}
							vector.FillRect(screen, px, allocY, allocPipSz, allocPipSz, pipCol, false)
						}
						r := sysRect{x: allocX, y: allocY, w: allocW, h: allocPipSz}
						g.sysAllocRect[col.potKind] = r
					}
				}
				cx += col.width
			}
		}
	}

	sel := g.world.System.Selected
	if sel < 0 || sel >= len(g.world.System.Planets) {
		g.sysEnterRect = sysRect{}
		g.sysAwakenRect = sysRect{}
		return
	}
	p := g.world.System.Planets[sel]

	th := systemHUD(bottomHUDHeightBase)
	tx := float32(0)
	tw := float32(g.screenW)
	ty := float32(g.screenH) - th
	vector.FillRect(screen, tx, ty, tw, th, colSysTrayFill, false)
	vector.StrokeLine(screen, tx, ty, tx+tw, ty, 1, colSysTrayBorder, false)

	swSize := systemHUD(bottomHUDSwatchBase)
	swY := ty + (th-swSize)/2

	_, rateH := text.Measure("0", face, 0)
	type rateItem struct {
		label   string
		textCol color.RGBA
		width   float32
	}
	var rateItems []rateItem
	if p.Completed {
		for i := range resourceFamilies {
			fam := &resourceFamilies[i]
			rate := *fam.AbstractRate(&p)
			if rate <= 0 {
				continue
			}
			rateStr := fmt.Sprintf("%.1f/s", rate)
			rw, _ := text.Measure(rateStr, face, 0)
			rateItems = append(rateItems, rateItem{label: rateStr, textCol: fam.RateLabelColor, width: float32(rw)})
		}
	} else {
		for i := range resourceFamilies {
			fam := &resourceFamilies[i]
			rate := *fam.ProjectedRate(&p)
			if rate <= 0 {
				continue
			}
			rateStr := fmt.Sprintf("%.1f/s", rate)
			rw, _ := text.Measure(rateStr, face, 0)
			rateItems = append(rateItems, rateItem{label: rateStr, textCol: fam.ProjectedRateLabelColor, width: float32(rw)})
		}
	}
	if len(rateItems) == 0 {
		placeholder := "-.--/s"
		pw, _ := text.Measure(placeholder, face, 0)
		rateItems = append(rateItems, rateItem{
			label:   placeholder,
			textCol: colWoodLabel,
			width:   float32(pw),
		})
	}

	g.sysEnterRect = sysRect{}
	g.sysAwakenRect = sysRect{}
	btnSize := systemHUD(bottomHUDButtonBase)
	btnY := ty + (th-btnSize)/2
	showAwaken := (p.Kind == PlanetEcho || p.Kind == PlanetUnknown) && !p.Awakened
	showEnter := !showAwaken && p.zoomable()

	type potCostItem struct {
		kind     PotentialKind
		col      color.RGBA
		labelCol color.RGBA
	}
	potOrder := []potCostItem{
		{PotentialWater, colWaterPotential, colWaterPotentialLabel},
		{PotentialForest, colForestPotential, colForestPotentialLabel},
	}
	awakenCostMap := planetAwakenCost(g.world, sel)
	costStr := "1"
	costW, costH := text.Measure(costStr, face, 0)
	circR := systemHUD(bottomHUDCircleBase)
	costItemGap := systemHUD(bottomHUDCostGapBase)
	costWidth := float32(0)
	if showAwaken {
		for _, pk := range potOrder {
			if awakenCostMap[pk.kind] == 0 {
				continue
			}
			if costWidth > 0 {
				costWidth += costItemGap
			}
			costWidth += float32(costW) + systemHUD(3) + circR*2
		}
	}

	rateGap := systemHUD(bottomHUDRateGapBase)
	contentGap := systemHUD(bottomHUDContentGapBase)
	totalW := swSize
	if len(rateItems) > 0 {
		totalW += contentGap
		for i, item := range rateItems {
			if i > 0 {
				totalW += rateGap
			}
			totalW += item.width
		}
	}
	if costWidth > 0 {
		totalW += contentGap + costWidth
	}
	if showAwaken || showEnter {
		totalW += contentGap + btnSize
	}

	cx := float32(g.screenW)/2 - totalW/2
	swX := cx
	drawSystemTrayPlanetSwatch(screen, p, swX, swY, swSize)
	cx += swSize + contentGap

	rateY := swY + swSize - float32(rateH)
	for i, item := range rateItems {
		if i > 0 {
			cx += rateGap
		}
		drawSysText(screen, item.label, cx, rateY, item.textCol, face)
		cx += item.width
	}
	cx += contentGap

	if showAwaken && costWidth > 0 {
		costY := btnY + (btnSize-float32(costH))/2
		circY := btnY + btnSize/2
		drewCost := false
		for _, pk := range potOrder {
			if awakenCostMap[pk.kind] == 0 {
				continue
			}
			if drewCost {
				cx += costItemGap
			}
			drawSysText(screen, costStr, cx, costY, pk.labelCol, face)
			cx += float32(costW) + systemHUD(3) + circR
			drawPotentialCircle(screen, cx, circY, circR, pk.col)
			cx += circR
			drewCost = true
		}
		cx += contentGap
	}

	btnX := cx

	if showAwaken {
		canAff := canAwaken(g.world, sel)
		fillCol, rimCol, glyphCol := colAwakenFill, colAwakenRim, colAwakenGlyph
		if !canAff {
			fillCol, rimCol, glyphCol = colAwakenFillDim, colAwakenRimDim, colAwakenGlyphDim
		}
		vector.FillRect(screen, btnX, btnY, btnSize, btnSize, fillCol, false)
		vector.StrokeRect(screen, btnX, btnY, btnSize, btnSize, 1, rimCol, false)

		cx2 := btnX + btnSize/2
		cy2 := btnY + btnSize/2
		ray := systemHUD(4)
		half := systemHUD(1.5)
		vector.StrokeLine(screen, cx2-ray, cy2-ray, cx2+ray, cy2+ray, half, glyphCol, false)
		vector.StrokeLine(screen, cx2+ray, cy2-ray, cx2-ray, cy2+ray, half, glyphCol, false)
		vector.FillCircle(screen, cx2, cy2, half+float32(scale), glyphCol, false)
		g.sysAwakenRect = sysRect{x: btnX, y: btnY, w: btnSize, h: btnSize}
	} else if showEnter {
		vector.FillRect(screen, btnX, btnY, btnSize, btnSize, colEnterFill, false)
		vector.StrokeRect(screen, btnX, btnY, btnSize, btnSize, 1, colEnterRim, false)
		bCx := btnX + btnSize/2
		bCy := btnY + btnSize/2
		bR := systemHUD(4)
		vector.FillCircle(screen, bCx, bCy, bR, colEnterGlyph, false)
		drawSystemOrbitRing(screen, bCx, bCy, bR+systemHUD(2), 1, colEnterOrbit)
		g.sysEnterRect = sysRect{x: btnX, y: btnY, w: btnSize, h: btnSize}
	}

	for _, d := range g.sysInjectDots {
		t := d.age / d.life
		x := d.ox + (d.tx-d.ox)*t
		y := d.oy + (d.ty-d.oy)*t
		r := float32(3)*(1-t) + 1
		c := d.col
		c.A = uint8(float32(200) * (1 - t))
		vector.FillCircle(screen, x, y, r, c, false)
	}
}

// spawnInjectDots spawns 8 particles from the inject circle toward the selected planet.
func (g *Game) spawnInjectDots(kind PotentialKind) {
	sel := g.world.System.Selected
	if sel < 0 || sel >= len(g.world.System.Planets) {
		return
	}
	fam := familyForPotential(kind)
	if fam == nil {
		return
	}
	r := g.sysInjectRect[kind]
	col := fam.PotentialColor
	if r.w <= 0 {
		return
	}
	px, py := g.worldToScreen(g.world.System.Planets[sel].Pos)
	circleR := r.h / 2
	cx := r.x + circleR
	cy := r.y + circleR
	const n = 8
	const spread = float32(4)
	const life = float32(0.85)
	for i := 0; i < n; i++ {
		angle := float64(i) * math.Pi * 2 / n
		g.sysInjectDots = append(g.sysInjectDots, sysInjectDot{
			ox:   cx + float32(math.Cos(angle))*spread,
			oy:   cy + float32(math.Sin(angle))*spread,
			tx:   px,
			ty:   py,
			age:  0,
			life: life,
			col:  col,
		})
	}
}

// drawPotentialCircle draws a single Potential token circle centered at (cx, cy).
func drawPotentialCircle(dst *ebiten.Image, cx, cy, r float32, col color.RGBA) {
	vector.FillCircle(dst, cx, cy, r, col, false)
}

// drawFractionalCircle fills a pie sector covering frac (0–1) of a circle at
// (cx,cy) with radius r, sweeping clockwise from 12 o'clock.
func drawFractionalCircle(dst *ebiten.Image, cx, cy, r float32, frac float64, col color.RGBA) {
	if frac <= 0 {
		return
	}
	if frac >= 1 {
		vector.FillCircle(dst, cx, cy, r, col, false)
		return
	}
	startAngle := float32(-math.Pi / 2)
	endAngle := startAngle + float32(2*math.Pi*frac)
	var p vector.Path
	p.MoveTo(cx, cy)
	p.Arc(cx, cy, r, startAngle, endAngle, vector.Clockwise)
	p.Close()
	op := &vector.DrawPathOptions{}
	op.ColorScale.ScaleWithColor(col)
	vector.FillPath(dst, &p, nil, op)
}

func drawSystemTrayPlanetSwatch(screen *ebiten.Image, p SystemPlanet, x, y, size float32) {
	if systemPlanetHybridSwatch(p) {
		halfW := size / 2
		vector.FillRect(screen, x, y, halfW, size, colWoodResource, false)
		vector.FillRect(screen, x+halfW, y, size-halfW, size, colWaterPotential, false)
		return
	}
	vector.FillRect(screen, x, y, size, size, sysSwatchColor(p), false)
}

func systemPlanetHybridSwatch(p SystemPlanet) bool {
	return p.AbstractRate > 0 && p.AbstractWaterRate > 0
}

func sysSwatchColor(p SystemPlanet) color.RGBA {
	switch p.Kind {
	case PlanetStarting, PlanetEcho:
		return colWoodResource
	case PlanetUnknown:
		if p.Awakened {
			return colSysFrontierSwatch
		}
		return colSysUnknownSwatch
	default:
		return colSysUnknownSwatch
	}
}

func drawSysText(target *ebiten.Image, s string, x, y float32, col color.RGBA, face text.Face) {
	if face == nil {
		return
	}
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(col)
	text.Draw(target, s, face, op)
}
