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
	clear(g.sysResourceRect)
	g.drawSystemChannelOverlay(screen)
	sel := g.world.System.Selected
	if sel >= 0 && sel < len(g.world.System.Planets) {
		drawSystemTopRateHUD(screen, g.world, sel, face, systemTopHUD, g.screenW)
	}
	if sel < 0 || sel >= len(g.world.System.Planets) {
		g.sysEnterRect = sysRect{}
		return
	}
	p := g.world.System.Planets[sel]

	th := systemHUD(bottomHUDHeightBase)
	tx := float32(0)
	tw := float32(g.screenW)
	ty := float32(g.screenH) - th
	vector.FillRect(screen, tx, ty, tw, th, colSysTrayFill, false)
	vector.StrokeLine(screen, tx, ty, tx+tw, ty, 1, colSysTrayBorder, false)

	g.sysEnterRect = sysRect{}
	btnSize := systemHUD(bottomHUDButtonBase)
	btnY := ty + (th-btnSize)/2
	showEnter := p.zoomable()

	contentGap := systemHUD(bottomHUDContentGapBase)
	bodyWidth := systemTrayBodyWidth(g.world, sel, face, systemHUD)
	totalW := bodyWidth
	if showEnter {
		if totalW > 0 {
			totalW += contentGap
		}
		totalW += btnSize
	}

	cx := float32(g.screenW)/2 - totalW/2
	if p.Completed {
		cx = drawCompletedPlanetTrayBody(screen, g, sel, cx, ty+(th-btnSize)/2, btnSize, contentGap)
	} else if !p.Awakened {
		cx = drawDormantPlanetTrayBody(screen, g.world, sel, cx, ty, th, face, systemHUD)
	}
	if showEnter {
		btnX := cx
		if bodyWidth > 0 {
			btnX += contentGap
		}
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

func drawSystemTopRateHUD(screen *ebiten.Image, w *World, selected int, face text.Face, systemTopHUD func(float64) float32, screenW int) {
	cols := systemTopRateColumns(w, selected, face, systemTopHUD)
	if len(cols) == 0 {
		return
	}
	h := systemTopHUD(systemTopHUDHeightBase)
	vector.FillRect(screen, 0, 0, float32(screenW), h, colSysTrayFill, false)
	vector.StrokeLine(screen, 0, h, float32(screenW), h, 1, colSysTrayBorder, false)

	topY := systemTopHUD(systemTopHUDPaddingVBase)
	columnGap := systemTopHUD(systemTopHUDColumnGapBase)
	totalW := float32(0)
	for i, col := range cols {
		if i > 0 {
			totalW += columnGap
		}
		totalW += col.width
	}
	cx := float32(screenW)/2 - totalW/2
	for i, col := range cols {
		if i > 0 {
			cx += columnGap
		}
		vector.FillRect(screen, cx, topY, col.squareSize, col.squareSize, col.squareCol, false)
		_, th := text.Measure(col.label, face, 0)
		drawSysText(screen, col.label, cx+col.squareSize+col.textGap, topY+col.squareSize-float32(th), col.textCol, face)
		cx += col.width
	}
}

type systemRateColumn struct {
	label      string
	squareCol  color.RGBA
	textCol    color.RGBA
	squareSize float32
	textGap    float32
	width      float32
}

func systemTopRateColumns(w *World, selected int, face text.Face, systemTopHUD func(float64) float32) []systemRateColumn {
	squareSize := systemTopHUD(systemTopHUDSquareBase)
	textGap := systemTopHUD(bottomHUDTextGapBase)
	cols := make([]systemRateColumn, 0, len(resourceFamilies))
	for i := range resourceFamilies {
		fam := &resourceFamilies[i]
		if !systemPlanetResourceRelevant(w, selected, fam.Resource) {
			continue
		}
		label := systemPlanetDisplayLabel(w, selected, fam.Resource)
		tw, _ := text.Measure(label, face, 0)
		cols = append(cols, systemRateColumn{
			label:      label,
			squareCol:  fam.PotentialColor,
			textCol:    systemRateTextColor(w, selected, fam),
			squareSize: squareSize,
			textGap:    textGap,
			width:      squareSize + textGap + float32(tw),
		})
	}
	return cols
}

func systemTrayBodyWidth(w *World, selected int, face text.Face, systemHUD func(float64) float32) float32 {
	p := w.System.Planets[selected]
	if p.Completed {
		return completedPlanetTrayBodyWidth(w, selected, face, systemHUD)
	}
	if !p.Awakened {
		return dormantPlanetTrayBodyWidth(w, selected, face, systemHUD)
	}
	return 0
}

func completedPlanetTrayBodyWidth(w *World, selected int, face text.Face, systemHUD func(float64) float32) float32 {
	swatchSize := systemHUD(bottomHUDButtonBase)
	itemGap := systemHUD(bottomHUDContentGapBase)
	total := float32(0)
	count := 0
	for i := range resourceFamilies {
		fam := &resourceFamilies[i]
		if !systemPlanetResourceRelevant(w, selected, fam.Resource) {
			continue
		}
		if count > 0 {
			total += itemGap
		}
		total += swatchSize
		count++
	}
	return total
}

func drawCompletedPlanetTrayBody(screen *ebiten.Image, g *Game, selected int, x, y, swatchSize, itemGap float32) float32 {
	count := 0
	for i := range resourceFamilies {
		fam := &resourceFamilies[i]
		if !systemPlanetResourceRelevant(g.world, selected, fam.Resource) {
			continue
		}
		if count > 0 {
			x += itemGap
		}
		fillCol := color.RGBA{R: colSysTrayFill.R, G: colSysTrayFill.G, B: colSysTrayFill.B, A: 240}
		vector.FillRect(screen, x, y, swatchSize, swatchSize, fillCol, false)
		borderCol := fam.PotentialColor
		borderCol.A = 210
		if ch := findChannel(g.world, selected, fam.Resource); ch != nil {
			fillGlow := fam.PotentialColor
			fillGlow.A = 44
			vector.FillRect(screen, x+1, y+1, swatchSize-2, swatchSize-2, fillGlow, false)
			borderCol = brighten(fam.PotentialColor, 96)
		}
		if g.pendingChannelActive && g.pendingChannelResource == fam.Resource {
			borderCol = color.RGBA{R: colSysSelect.R, G: colSysSelect.G, B: colSysSelect.B, A: 255}
		}
		vector.StrokeRect(screen, x, y, swatchSize, swatchSize, 1, borderCol, false)
		drawTransferButtonIcon(screen, x, y, swatchSize, fam.PotentialColor)
		g.sysResourceRect[fam.Resource] = sysRect{x: x, y: y, w: swatchSize, h: swatchSize}
		x += swatchSize
		count++
	}
	return x
}

func dormantPlanetTrayBodyWidth(w *World, selected int, face text.Face, systemHUD func(float64) float32) float32 {
	items := dormantResourceItems(w, selected)
	if len(items) == 0 {
		return 0
	}
	iconSize := systemHUD(bottomHUDCircleBase) * 2
	barW := systemHUD(34)
	textGap := systemHUD(bottomHUDTextGapBase)
	itemGap := systemHUD(bottomHUDContentGapBase)
	total := float32(0)
	for i, item := range items {
		if i > 0 {
			total += itemGap
		}
		label := systemDormantRequirementLabel(w, item.resource, item.req)
		tw, _ := text.Measure(label, face, 0)
		total += iconSize + textGap + barW + textGap + float32(tw)
	}
	return total
}

type dormantResourceItem struct {
	resource ResourceKind
	req      float64
	fill     float64
}

func dormantResourceItems(w *World, selected int) []dormantResourceItem {
	p := w.System.Planets[selected]
	items := make([]dormantResourceItem, 0, len(resourceFamilies))
	for i := range resourceFamilies {
		fam := &resourceFamilies[i]
		req := *fam.AwakenReq(&p)
		fill := *fam.AwakenFill(&p)
		if req <= 0 && !systemPlanetHasChannelState(w, selected, fam.Resource) {
			continue
		}
		items = append(items, dormantResourceItem{resource: fam.Resource, req: req, fill: fill})
	}
	return items
}

func drawDormantPlanetTrayBody(screen *ebiten.Image, w *World, selected int, x, y, trayH float32, face text.Face, systemHUD func(float64) float32) float32 {
	items := dormantResourceItems(w, selected)
	if len(items) == 0 {
		return x
	}
	iconR := systemHUD(bottomHUDCircleBase)
	iconSize := iconR * 2
	barW := systemHUD(34)
	barH := systemHUD(4)
	textGap := systemHUD(bottomHUDTextGapBase)
	itemGap := systemHUD(bottomHUDContentGapBase)
	_, textH := text.Measure("0", face, 0)
	for i, item := range items {
		if i > 0 {
			x += itemGap
		}
		fam := familyForResource(item.resource)
		if fam == nil {
			continue
		}
		cx := x + iconR
		cy := y + trayH/2
		drawPotentialCircle(screen, cx, cy, iconR, brighten(fam.PotentialColor, 24))
		frac := 0.0
		if item.req > 0 {
			frac = item.fill / item.req
		}
		if frac > 1 {
			frac = 1
		}
		drawFractionalCircle(screen, cx, cy, iconR, frac, fam.PotentialColor)

		barX := x + iconSize + textGap
		barY := y + (trayH-barH)/2
		vector.StrokeRect(screen, barX, barY, barW, barH, 1, colSysTrayBorder, false)
		if frac > 0 {
			vector.FillRect(screen, barX+1, barY+1, (barW-2)*float32(frac), barH-2, fam.PotentialColor, false)
		}

		label := systemDormantRequirementLabel(w, item.resource, item.req)
		drawSysText(screen, label, barX+barW+textGap, y+trayH/2-float32(textH)/2, fam.PotentialLabelColor, face)
		tw, _ := text.Measure(label, face, 0)
		x += iconSize + textGap + barW + textGap + float32(tw)
	}
	return x
}

func systemPlanetDisplayRate(w *World, selected int, resource ResourceKind) float64 {
	if selected < 0 || selected >= len(w.System.Planets) {
		return 0
	}
	p := w.System.Planets[selected]
	if p.zoomable() {
		if selected == w.Active {
			return liveWorldEstimateRate(w, resource)
		}
		if selected >= 0 && selected < len(w.PlanetStates) && w.PlanetStates[selected] != nil {
			return parkedPlanetEstimateRate(w.PlanetStates[selected], resource)
		}
	}
	return 0
}

func systemPlanetEffectiveAbstractRate(p SystemPlanet, resource ResourceKind) float64 {
	fam := familyForResource(resource)
	if fam == nil {
		return 0
	}
	rate := *fam.AbstractRate(&p)
	if resource == KindWater && p.Completed && p.Kind == PlanetEcho && p.LayoutID == 0 && rate <= 0 {
		return lakewoodLatentWaterRate
	}
	return rate
}

func systemPlanetDisplayLabel(w *World, selected int, resource ResourceKind) string {
	if selected < 0 || selected >= len(w.System.Planets) {
		return ""
	}
	rate := systemPlanetDisplayRate(w, selected, resource)
	if rate > 0 {
		return systemRateText(rate)
	}
	if systemPlanetResourceOperational(w, selected, resource) {
		return systemRateText(0)
	}
	if systemPlanetResourceRelevant(w, selected, resource) {
		return "???"
	}
	return ""
}

func systemPlanetResourceRelevant(w *World, selected int, resource ResourceKind) bool {
	if selected < 0 || selected >= len(w.System.Planets) {
		return false
	}
	p := w.System.Planets[selected]
	if systemPlanetHasChannelState(w, selected, resource) {
		return true
	}
	if systemPlanetHasEcology(w, selected, resource) {
		return true
	}
	if systemPlanetDisplayRate(w, selected, resource) > 0 {
		return true
	}
	if !p.Completed {
		fam := familyForResource(resource)
		if fam != nil && *fam.ProjectedRate(&p) > 0 {
			return true
		}
	}
	fam := familyForResource(resource)
	return fam != nil && *fam.AwakenReq(&p) > 0
}

func systemPlanetHasChannelState(w *World, selected int, resource ResourceKind) bool {
	for _, ch := range w.System.Channels {
		if ch.Resource == resource && (ch.Source == selected || ch.Target == selected) {
			return true
		}
	}
	return false
}

func systemPlanetHasEcology(w *World, selected int, resource ResourceKind) bool {
	if selected == w.Active {
		return planetHasResourceEcology(w.Planet, resource)
	}
	if selected >= 0 && selected < len(w.PlanetStates) && w.PlanetStates[selected] != nil {
		return planetHasResourceEcology(w.PlanetStates[selected].Planet, resource)
	}
	p := w.System.Planets[selected]
	switch resource {
	case KindWood:
		return p.Kind == PlanetStarting || p.Kind == PlanetEcho || p.AwakenReqWood > 0
	case KindWater:
		return p.Kind == PlanetUnknown || p.ProjectedWaterRate > 0 || p.AbstractWaterRate > 0 || p.AwakenReqWater > 0 ||
			(p.Kind == PlanetEcho && p.RingColorIdx == 0) // layout 0 (Lakewood) has water ecology
	default:
		return false
	}
}

func planetHasResourceEcology(planet Planet, resource ResourceKind) bool {
	if planet.Composition != nil && planet.Composition[resource] > 0 {
		return true
	}
	for _, f := range planet.Fields {
		if f.Kind == resource {
			return true
		}
	}
	return false
}

func systemPlanetResourceOperational(w *World, selected int, resource ResourceKind) bool {
	if selected < 0 || selected >= len(w.System.Planets) {
		return false
	}
	if selected == w.Active {
		return planetResourceOperational(w.Planet, w.Buildings, w.ResourceDiscovered, resource)
	}
	if selected >= 0 && selected < len(w.PlanetStates) && w.PlanetStates[selected] != nil {
		ps := w.PlanetStates[selected]
		return planetResourceOperational(ps.Planet, ps.Buildings, ps.ResourceDiscovered, resource)
	}
	return false
}

func planetResourceOperational(planet Planet, buildings []*Building, resourceDiscovered bool, resource ResourceKind) bool {
	if !planetHasResourceEcology(planet, resource) {
		return false
	}
	switch resource {
	case KindWood:
		if !resourceDiscovered {
			return false
		}
		for _, b := range buildings {
			if b.Kind == KindTownHall || b.Kind == KindLoggingCamp {
				return true
			}
		}
		return false
	case KindWater:
		for _, b := range buildings {
			if b.Kind == KindDock {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func systemDormantRequirementLabel(w *World, resource ResourceKind, req float64) string {
	if resource == KindWater && !w.Economy.WaterDiscovered && req > 0 {
		return "???"
	}
	return fmt.Sprintf("%.0f", req)
}

func systemRateTextColor(w *World, selected int, fam *resourceFamily) color.RGBA {
	if systemPlanetDisplayRate(w, selected, fam.Resource) > 0 {
		return fam.RateLabelColor
	}
	if systemPlanetResourceOperational(w, selected, fam.Resource) {
		return fam.RateLabelColor
	}
	return fam.ProjectedRateLabelColor
}

func liveWorldEstimateRate(w *World, resource ResourceKind) float64 {
	switch resource {
	case KindWood:
		return EstimateRate(w)
	case KindWater:
		return EstimateWaterRate(w)
	default:
		return 0
	}
}

func parkedPlanetEstimateRate(ps *PlanetState, resource ResourceKind) float64 {
	if ps == nil {
		return 0
	}
	temp := &World{
		Planet:             ps.Planet,
		Buildings:          ps.Buildings,
		Nodes:              ps.Nodes,
		Workers:            ps.Workers,
		ResourceDiscovered: ps.ResourceDiscovered,
		LaborFocus:         ps.LaborFocus,
		SavedLaborRatio:    ps.SavedLaborRatio,
	}
	temp.Economy.Wood = ps.LocalWood
	temp.Economy.Water = ps.LocalWater
	temp.Economy.TownGrowth = ps.TownGrowth
	temp.Economy.TownGrowthCap = ps.TownGrowthCap
	temp.Economy.TownGrowthOverflow = ps.TownGrowthOverflow
	temp.Economy.LastWorkerSpawnTime = ps.LastWorkerSpawnTime
	switch resource {
	case KindWood:
		return EstimateRate(temp)
	case KindWater:
		return EstimateWaterRate(temp)
	default:
		return 0
	}
}

func drawTransferButtonIcon(screen *ebiten.Image, x, y, size float32, tint color.RGBA) {
	img := transferButtonSprite()
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	pixel := float32(math.Floor(float64(size) / 10))
	if pixel < 1 {
		pixel = 1
	}
	iconW := float32(w) * pixel
	iconH := float32(h) * pixel
	ix := x + (size-iconW)/2
	iy := y + (size-iconH)/2

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(pixel), float64(pixel))
	op.GeoM.Translate(float64(ix), float64(iy))
	col := brighten(tint, 120)
	op.ColorScale.Scale(
		float32(col.R)/255,
		float32(col.G)/255,
		float32(col.B)/255,
		1,
	)
	screen.DrawImage(img, op)
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
