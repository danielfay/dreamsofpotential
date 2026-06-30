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

	if g.world.ResourceDiscovered || g.world.System.Unlocked {
		type systemResourceColumn struct {
			show    bool
			text    string
			col     color.RGBA
			textCol color.RGBA
			width   float32
		}

		squareSize := systemTopHUD(systemTopHUDSquareBase)
		textGap := systemTopHUD(bottomHUDTextGapBase)
		columnGap := systemTopHUD(systemTopHUDColumnGapBase)

		mkColumn := func(show bool, squareText string, squareCol, squareTextCol color.RGBA) systemResourceColumn {
			col := systemResourceColumn{
				show:    show,
				text:    squareText,
				col:     squareCol,
				textCol: squareTextCol,
			}
			if show {
				tw, _ := text.Measure(squareText, face, 0)
				col.width = squareSize + textGap + float32(tw)
			}
			return col
		}

		var woodText, waterText string
		if g.world.System.View == ViewSystem {
			var woodSysRate, waterSysRate float64
			for _, p := range g.world.System.Planets {
				if p.Completed {
					woodSysRate += p.AbstractRate
					waterSysRate += p.AbstractWaterRate
				}
			}
			woodText = systemRateText(woodSysRate)
			waterText = systemRateText(waterSysRate)
		} else {
			woodText = fmt.Sprintf("%.0f (%s)", g.world.Economy.Wood, systemRateText(EstimateRate(g.world)))
			waterText = fmt.Sprintf("%.0f (%s)", g.world.Economy.Water, systemRateText(EstimateWaterRate(g.world)))
		}
		columns := []systemResourceColumn{
			mkColumn(
				g.world.ResourceDiscovered || g.world.System.Unlocked,
				woodText,
				colWoodResource, colWoodLabel,
			),
			mkColumn(
				g.world.Economy.WaterDiscovered,
				waterText,
				colSparkle, color.RGBA{R: 100, G: 200, B: 255, A: 220},
			),
		}

		var visibleCols []systemResourceColumn
		totalW := float32(0)
		for _, col := range columns {
			if !col.show {
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
			topY := paddingY
			cx := float32(g.screenW)/2 - totalW/2
			for i, col := range visibleCols {
				if i > 0 {
					cx += columnGap
				}
				if col.show {
					rowW := squareSize
					if col.text != "" {
						tw, _ := text.Measure(col.text, face, 0)
						rowW += textGap + float32(tw)
					}
					rowX := cx + (col.width-rowW)/2
					vector.FillRect(screen, rowX, topY, squareSize, squareSize, col.col, false)
					if col.text != "" {
						_, th := text.Measure(col.text, face, 0)
						drawSysText(screen, col.text, rowX+squareSize+textGap, topY+squareSize-float32(th), col.textCol, face)
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
