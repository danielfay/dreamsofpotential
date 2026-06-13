package game

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"

	"github.com/ebitenui/ebitenui"
	eimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	debugSectionCore = iota
	debugSectionPlacement
	debugSectionGrowth
)

// HUD holds the live EbitenUI widgets for the resource display and action buttons.
//
// Normal mode uses two separate anchored containers:
//   - normalTopBar: small resource/worker info, horizontal row across the top.
//   - normalSidebar: action buttons (build, worker), vertical column on the left.
//
// Debug mode (F3) replaces both with a single verbose debugPanel.
type HUD struct {
	face    text.Face
	sysface text.Face // fixed 8 px face for system overlay; drawn on scene so it scales naturally
	debugSection int

	// debug panel
	debugPanel       *widget.Container
	debugTabs        *widget.Container
	debugCoreBtn     *widget.Button
	debugPlaceBtn    *widget.Button
	debugGrowthBtn   *widget.Button
	woodText         *widget.Text
	workerText       *widget.Text
	nodeText         *widget.Text
	fieldText        *widget.Text
	previewText      *widget.Text
	resourceSquare   *widget.Button
	buildCapacityDbg *widget.Button
	freeCapacityDbg  *widget.Button
	addWorkerDbg     *widget.Button
	buildCampDbg     *widget.Button
	freeCampDbg      *widget.Button
	upgradeFieldDbg  *widget.Button
	growFullDbg      *widget.Button
	resetBtn         *widget.Button

	// normal mode — top bar (resource info, horizontal, full-width)
	normalTopBar *widget.Container
	resourceHUD  *widget.Container // green icon + amount; hidden until discovered
	resourceText *widget.Text
	workerHUD    *widget.Container // yellow icon + ratio; hidden until first worker
	workerRatio  *widget.Text

	// normal mode — left sidebar (action buttons, vertical)
	normalSidebar        *widget.Container
	buildCampBtn         *widget.Button // brown square — always visible
	buildTownCapacityBtn *widget.Button // yellow square — hidden until Town Hall exists

	// settings menu overlay (centered; shown when showMenu is true)
	menuPanel     *widget.Container
	menuSaveBtn   *widget.Button
	menuExportBtn *widget.Button
	menuImportBtn *widget.Button
	menuResetBtn  *widget.Button
}

// pointInHUD reports whether native screen coordinates (sx, sy) fall inside any
// visible HUD area. Called after ui.Update() so Rects are current. Does NOT
// check menuPanel: handleInput already returns early when g.showMenu is true,
// and checking menuPanel here would block clicks using its stale Rect after the
// menu is closed (EbitenUI's AnchorLayout preserves the last-visible Rect).
func (h *HUD) pointInHUD(sx, sy int, debug bool) bool {
	inRect := func(c *widget.Container) bool {
		r := c.GetWidget().Rect
		return r.Min.X <= sx && sx < r.Max.X && r.Min.Y <= sy && sy < r.Max.Y
	}
	if debug {
		return inRect(h.debugPanel)
	}
	return inRect(h.normalSidebar) || inRect(h.normalTopBar)
}

// Refresh updates all HUD labels and visibility states to match the world.
func (h *HUD) Refresh(w *World, placing, debug bool, debugSection int, pv *placementPreview, showMenu bool) {
	h.debugSection = debugSection
	if showMenu {
		h.menuPanel.GetWidget().SetVisibility(widget.Visibility_Show)
		h.debugPanel.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.normalTopBar.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.normalSidebar.GetWidget().SetVisibility(widget.Visibility_Hide)
		return
	} else {
		h.menuPanel.GetWidget().SetVisibility(widget.Visibility_Hide)
	}

	if debug {
		h.debugPanel.GetWidget().SetVisibility(widget.Visibility_Show)
		h.normalTopBar.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.normalSidebar.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.refreshDebug(w, placing, pv)
	} else if w.System.View == ViewSystem {
		// In system view the EbitenUI planet HUD is hidden; the system overlay
		// is drawn manually in drawOverlay.
		h.debugPanel.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.normalTopBar.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.normalSidebar.GetWidget().SetVisibility(widget.Visibility_Hide)
	} else {
		h.debugPanel.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.normalTopBar.GetWidget().SetVisibility(widget.Visibility_Show)
		h.normalSidebar.GetWidget().SetVisibility(widget.Visibility_Show)
		h.refreshNormal(w)
	}
}

func (h *HUD) refreshDebug(w *World, placing bool, pv *placementPreview) {
	if h.debugCoreBtn != nil {
		h.debugCoreBtn.SetText("Core")
		h.debugPlaceBtn.SetText("Placement")
		h.debugGrowthBtn.SetText("Growth")
	}
	freeNodes, reservedNodes, claimedNodes := 0, 0, 0
	for _, n := range w.Nodes {
		if n.OwnerID != -1 {
			claimedNodes++
		} else if n.ReservedByWorkerID != -1 {
			reservedNodes++
		} else {
			freeNodes++
		}
	}

	thWood, campWood := deliveryTotals(w)
	h.woodText.Label = fmt.Sprintf("wood: %.0f (%.2f/s)  TH %.1f  camps %.1f  forest:%d water:%d",
		w.Economy.Wood, EstimateRate(w), thWood, campWood,
		w.Economy.Potential[PotentialForest], w.Economy.Potential[PotentialWater])

	active, returning, settling, reaction, waiting := 0, 0, 0, 0, 0
	for _, wk := range w.Workers {
		if workerInLoop(wk) || wk.State == StateDeparturePulse || wk.State == StateToRim {
			active++
			continue
		}
		switch wk.State {
		case StateReturningHome, StateToIdleSpot:
			returning++
		case StateSettling:
			settling++
		case StateReactionDelay:
			reaction++
		case StateIdleWaiting:
			waiting++
		}
	}
	h.workerText.Label = fmt.Sprintf("workers: %d active  %d return  %d settle  %d react  %d wait  %d total",
		active, returning, settling, reaction, waiting, len(w.Workers))
	h.nodeText.Label = fmt.Sprintf("nodes: %d free  %d reserved  %d claimed  pending %s",
		freeNodes, reservedNodes, claimedNodes, pendingRebalanceText(w))

	{
		fp := w.Planet.FieldProgress[KindWood]
		var fpEXP, fpCap float64
		if fp != nil {
			fpEXP, fpCap = fp.EXP, fp.Cap
		}
		// Count nodes per known wood field so multi-region planets are visible.
		var fieldLines []string
		for i, f := range w.Planet.Fields {
			if f.Kind != KindWood || !f.Known {
				continue
			}
			count := 0
			for _, n := range w.Nodes {
				if n.Kind == KindWood && angleWithinField(f, n.Angle) {
					count++
				}
			}
			canSpawn := fieldCanSpawnNode(w, f)
			satStr := "sat"
			if canSpawn {
				satStr = "open"
			}
			fieldLines = append(fieldLines, fmt.Sprintf("  field[%d] nodes:%d %s", i, count, satStr))
		}
		fullStr := ""
		if townFieldFull(w) {
			fullStr = "  FULL"
		}
		h.fieldText.Label = fmt.Sprintf("wood EXP %.1f/%.1f  ret %.0f%%  last g/b/r %.1f/%.1f/%.1f\n%s\nnurture: %d/press  cue pending: %v\ntown growth %.1f/%.1f  cap %d/%d  used %d  avail %d  next cap %.0f%s",
			fpEXP, fpCap, woodFieldReturnRatio*100,
			w.lastDelivery.Gross, w.lastDelivery.Banked, w.lastDelivery.Returned,
			strings.Join(fieldLines, "\n"),
			nurtureTreesPerPress, nurtureGrowthCuePending(w),
			w.Economy.TownGrowth, w.Economy.TownGrowthCap,
			w.Economy.WorkerCapacity, maxTownSlots(w), w.Economy.WorkerCapacity-availableCapacity(w), availableCapacity(w),
			townCapacityCost(w), fullStr)
	}

	cc := townCapacityCost(w)
	h.buildCapacityDbg.SetText(fmt.Sprintf("Build capacity (%.0f)", cc))
	h.buildCapacityDbg.GetWidget().Disabled = w.Economy.Wood < cc || townHall(w) == nil || townFieldFull(w)
	h.freeCapacityDbg.GetWidget().Disabled = townHall(w) == nil || townFieldFull(w)
	h.addWorkerDbg.GetWidget().Disabled = townHall(w) == nil

	if placing {
		h.buildCampDbg.SetText("Cancel placement")
		h.freeCampDbg.SetText("Cancel / free place")
	} else if len(w.Buildings) == 0 {
		h.buildCampDbg.SetText("Place Town Hall (free)")
		h.freeCampDbg.SetText("Free Town Hall")
	} else {
		h.buildCampDbg.SetText(fmt.Sprintf("Build camp (%.0f)", CampCost(w)))
		h.freeCampDbg.SetText("Free camp")
	}
	// Never disable these — the handler toggles placement/cancel on every click.
	h.buildCampDbg.GetWidget().Disabled = false
	h.freeCampDbg.GetWidget().Disabled = false
	h.upgradeFieldDbg.GetWidget().Disabled = fieldForKind(w, KindWood) == nil
	h.growFullDbg.GetWidget().Disabled = fieldForKind(w, KindWood) == nil

	if pv != nil {
		validity := "valid"
		if !pv.Valid {
			validity = "INVALID"
		}
		dists := make([]string, 0, len(pv.Free))
		for _, pr := range pv.Free {
			dists = append(dists, fmt.Sprintf("%.0f", pr.Dist))
		}
		distStr := "-"
		if len(dists) > 0 {
			distStr = "[" + strings.Join(dists, ",") + "]"
		}
		blocked := len(pv.Blocked) + len(pv.BlockedBuildings)
		afford := "no"
		if pv.Affordable {
			afford = "yes"
		}
		zeroValid := zeroValidPlacementPositions(w)
		h.previewText.Label = fmt.Sprintf("preview: %s  afford %s\nnearby: %d free  %d res  %d claimed\nroutes: %d/%d free  %d/%d unavailable\nblocked: %d  zero valid: %t  d: %s",
			validity, afford, pv.FreeTotal, pv.ReservedTotal, pv.ClaimedTotal,
			len(pv.Free), pv.FreeTotal, len(pv.Reserved)+len(pv.Claimed), pv.ReservedTotal+pv.ClaimedTotal, blocked, zeroValid, distStr)
	} else {
		h.previewText.Label = "preview: —"
	}
	h.applyDebugSectionVisibility()
}

func (h *HUD) applyDebugSectionVisibility() {
	showCore := widget.Visibility_Hide
	showPlacement := widget.Visibility_Hide
	showGrowth := widget.Visibility_Hide
	switch h.debugSection {
	case debugSectionPlacement:
		showPlacement = widget.Visibility_Show
	case debugSectionGrowth:
		showGrowth = widget.Visibility_Show
	default:
		showCore = widget.Visibility_Show
	}

	h.woodText.GetWidget().SetVisibility(showCore)
	h.workerText.GetWidget().SetVisibility(showCore)
	h.nodeText.GetWidget().SetVisibility(showCore)
	h.buildCapacityDbg.GetWidget().SetVisibility(showCore)
	h.freeCapacityDbg.GetWidget().SetVisibility(showCore)
	h.addWorkerDbg.GetWidget().SetVisibility(showCore)
	h.buildCampDbg.GetWidget().SetVisibility(showCore)
	h.resetBtn.GetWidget().SetVisibility(showCore)

	h.previewText.GetWidget().SetVisibility(showPlacement)
	h.freeCampDbg.GetWidget().SetVisibility(showPlacement)

	h.fieldText.GetWidget().SetVisibility(showGrowth)
	h.upgradeFieldDbg.GetWidget().SetVisibility(showGrowth)
	h.growFullDbg.GetWidget().SetVisibility(showGrowth)
}

func (h *HUD) refreshNormal(w *World) {
	hasTownHall := len(w.Buildings) > 0
	discovered := w.ResourceDiscovered

	// --- sidebar: action buttons ---

	// Build button:
	//   - before Town Hall: always enabled (Town Hall is free)
	//   - after Town Hall, pre-discovery: locked (Camp tool dimmed)
	//   - after discovery: enabled when affordable
	campLocked := hasTownHall && !discovered
	h.buildCampBtn.GetWidget().Disabled = campLocked || (hasTownHall && w.Economy.Wood < CampCost(w))

	// Capacity button: visible only while Town Hall exists and town is not yet full.
	if hasTownHall && !townFieldFull(w) {
		h.buildTownCapacityBtn.GetWidget().SetVisibility(widget.Visibility_Show)
		h.buildTownCapacityBtn.GetWidget().Disabled = w.Economy.Wood < townCapacityCost(w)
	} else {
		h.buildTownCapacityBtn.GetWidget().SetVisibility(widget.Visibility_Hide)
	}

	// --- top bar: resource info ---

	// Resource HUD: hidden until first delivery.
	if discovered {
		h.resourceHUD.GetWidget().SetVisibility(widget.Visibility_Show)
		h.resourceText.Label = fmt.Sprintf("%.0f", w.Economy.Wood)
		// Disable the Nurture square when the field is saturated or a growth cue
		// is playing — the button has nothing to do in either state.
		f := fieldForKind(w, KindWood)
		saturated := f != nil && !fieldCanSpawnNode(w, f)
		h.resourceSquare.GetWidget().Disabled = saturated || nurtureGrowthCuePending(w)
	} else {
		h.resourceHUD.GetWidget().SetVisibility(widget.Visibility_Hide)
	}

	// Worker HUD: hidden until first worker exists.
	if len(w.Workers) > 0 {
		h.workerHUD.GetWidget().SetVisibility(widget.Visibility_Show)
		h.workerRatio.Label = fmt.Sprintf("%d", len(w.Workers))
	} else {
		h.workerHUD.GetWidget().SetVisibility(widget.Visibility_Hide)
	}
}

func deliveryTotals(w *World) (townHallWood, campWood float64) {
	for _, b := range w.Buildings {
		if b.Kind == KindTownHall {
			townHallWood += b.DeliveredWood
		} else {
			campWood += b.DeliveredWood
		}
	}
	return townHallWood, campWood
}

func pendingRebalanceText(w *World) string {
	parts := make([]string, 0, 2)
	for _, wk := range w.Workers {
		if wk.PendingNodeID != -1 {
			parts = append(parts, fmt.Sprintf("w%d->n%d", wk.ID, wk.PendingNodeID))
		}
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}

// buildHUD constructs the EbitenUI tree and wires up button handlers.
// scale is the integer view scale factor (1 at ≤320×240, 2 at 640×480, etc.);
// all pixel sizes are multiplied by it so the HUD matches the world scale.
func buildHUD(g *Game, scale int) (*HUD, *ebitenui.UI, error) {
	s := scale
	// sz returns n*s scaled to 75% of the world scale, rounded to nearest pixel.
	sz := func(n int) int {
		v := (n*s*3 + 2) / 4
		if v < 1 {
			v = 1
		}
		return v
	}

	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, nil, err
	}

	hud := &HUD{}
	hud.face = &text.GoTextFace{Source: src, Size: float64(16*s) * 0.75}
	hud.sysface = &text.GoTextFace{Source: src, Size: 8.0 * float64(s)}
	face := &hud.face

	// --- debug panel styling ---
	dbgBtnImg := func() *widget.ButtonImage {
		return &widget.ButtonImage{
			Idle:     eimage.NewNineSliceColor(color.NRGBA{R: 80, G: 120, B: 200, A: 255}),
			Hover:    eimage.NewNineSliceColor(color.NRGBA{R: 100, G: 140, B: 220, A: 255}),
			Pressed:  eimage.NewNineSliceColor(color.NRGBA{R: 60, G: 100, B: 180, A: 255}),
			Disabled: eimage.NewNineSliceColor(color.NRGBA{R: 55, G: 55, B: 70, A: 255}),
		}
	}
	dbgTxtCol := &widget.ButtonTextColor{
		Idle:     color.White,
		Hover:    color.White,
		Disabled: color.RGBA{R: 120, G: 120, B: 130, A: 255},
	}
	dbgPad := &widget.Insets{Top: sz(6), Bottom: sz(6), Left: sz(12), Right: sz(12)}

	mkText := func(initial string) *widget.Text {
		return widget.NewText(
			widget.TextOpts.Text(initial, face, color.NRGBA{R: 180, G: 220, B: 180, A: 255}),
		)
	}
	hud.woodText = mkText("wood: 0 (0.00/s)")
	hud.workerText = mkText("workers: 0 active  0 idle  0 total")
	hud.nodeText = mkText("nodes: 0 free  0 claimed")
	hud.fieldText = mkText("field: 0.0 / 0.0")
	hud.previewText = mkText("preview: —")

	tabBtn := func(label string, section int) *widget.Button {
		return widget.NewButton(
			widget.ButtonOpts.Image(dbgBtnImg()),
			widget.ButtonOpts.Text(label, face, dbgTxtCol),
			widget.ButtonOpts.TextPadding(&widget.Insets{Top: sz(4), Bottom: sz(4), Left: sz(8), Right: sz(8)}),
			widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
				g.debugSection = section
			}),
		)
	}
	hud.debugCoreBtn = tabBtn("Core", debugSectionCore)
	hud.debugPlaceBtn = tabBtn("Placement", debugSectionPlacement)
	hud.debugGrowthBtn = tabBtn("Growth", debugSectionGrowth)
	hud.debugTabs = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(sz(4)),
		)),
	)
	hud.debugTabs.AddChild(hud.debugCoreBtn)
	hud.debugTabs.AddChild(hud.debugPlaceBtn)
	hud.debugTabs.AddChild(hud.debugGrowthBtn)

	hud.buildCapacityDbg = widget.NewButton(
		widget.ButtonOpts.Image(dbgBtnImg()),
		widget.ButtonOpts.Text(fmt.Sprintf("Build capacity (%.0f)", townCapacityBaseCost), face, dbgTxtCol),
		widget.ButtonOpts.TextPadding(dbgPad),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			buildTownCapacity(g.world)
		}),
	)

	hud.freeCapacityDbg = widget.NewButton(
		widget.ButtonOpts.Image(dbgBtnImg()),
		widget.ButtonOpts.Text("Free capacity", face, dbgTxtCol),
		widget.ButtonOpts.TextPadding(dbgPad),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			addFreeCapacity(g.world)
		}),
	)

	hud.addWorkerDbg = widget.NewButton(
		widget.ButtonOpts.Image(dbgBtnImg()),
		widget.ButtonOpts.Text("Add free worker", face, dbgTxtCol),
		widget.ButtonOpts.TextPadding(dbgPad),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			addFreeWorkerAtTownHall(g.world)
		}),
	)

	hud.buildCampDbg = widget.NewButton(
		widget.ButtonOpts.Image(dbgBtnImg()),
		widget.ButtonOpts.Text(fmt.Sprintf("Build camp (%.0f)", campBaseCost), face, dbgTxtCol),
		widget.ButtonOpts.TextPadding(dbgPad),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			if g.placing {
				g.placing = false
				g.freePlacing = false
				return
			}
			g.freePlacing = false
			g.placing = true
		}),
	)

	hud.freeCampDbg = widget.NewButton(
		widget.ButtonOpts.Image(dbgBtnImg()),
		widget.ButtonOpts.Text("Free camp", face, dbgTxtCol),
		widget.ButtonOpts.TextPadding(dbgPad),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			if g.placing {
				g.placing = false
				g.freePlacing = false
				return
			}
			g.freePlacing = true
			g.placing = true
		}),
	)

	hud.upgradeFieldDbg = widget.NewButton(
		widget.ButtonOpts.Image(dbgBtnImg()),
		widget.ButtonOpts.Text("Upgrade all fields", face, dbgTxtCol),
		widget.ButtonOpts.TextPadding(dbgPad),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			upgradeAllFieldsForDebug(g.world)
		}),
	)

	hud.growFullDbg = widget.NewButton(
		widget.ButtonOpts.Image(dbgBtnImg()),
		widget.ButtonOpts.Text("Grow all until blocked", face, dbgTxtCol),
		widget.ButtonOpts.TextPadding(dbgPad),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			growAllFieldsUntilBlockedForDebug(g.world)
		}),
	)

	hud.resetBtn = widget.NewButton(
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    eimage.NewNineSliceColor(color.NRGBA{R: 160, G: 40, B: 40, A: 255}),
			Hover:   eimage.NewNineSliceColor(color.NRGBA{R: 190, G: 60, B: 60, A: 255}),
			Pressed: eimage.NewNineSliceColor(color.NRGBA{R: 130, G: 30, B: 30, A: 255}),
		}),
		widget.ButtonOpts.Text("New Game", face, &widget.ButtonTextColor{
			Idle:  color.White,
			Hover: color.White,
		}),
		widget.ButtonOpts.TextPadding(dbgPad),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			ClearSave()
			g.world = NewWorld()
			g.placing = false
			g.freePlacing = false
		}),
	)

	hud.debugPanel = widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(
			eimage.NewNineSliceColor(color.NRGBA{R: 12, G: 12, B: 24, A: 210}),
		),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(sz(6)),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: sz(8), Left: sz(8), Bottom: sz(8), Right: sz(8)}),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
			}),
		),
	)
	hud.debugPanel.AddChild(hud.debugTabs)
	hud.debugPanel.AddChild(hud.woodText)
	hud.debugPanel.AddChild(hud.workerText)
	hud.debugPanel.AddChild(hud.nodeText)
	hud.debugPanel.AddChild(hud.fieldText)
	hud.debugPanel.AddChild(hud.previewText)
	hud.debugPanel.AddChild(hud.buildCapacityDbg)
	hud.debugPanel.AddChild(hud.freeCapacityDbg)
	hud.debugPanel.AddChild(hud.addWorkerDbg)
	hud.debugPanel.AddChild(hud.buildCampDbg)
	hud.debugPanel.AddChild(hud.freeCampDbg)
	hud.debugPanel.AddChild(hud.upgradeFieldDbg)
	hud.debugPanel.AddChild(hud.growFullDbg)
	hud.debugPanel.AddChild(hud.resetBtn)

	// --- normal mode: top bar ---
	// Stretched horizontally so its Rect covers the full screen width, making
	// pointInHUD reliable even before any children are visible.

	// smallSquare creates a non-interactive solid-color square for the top bar.
	// Both MinSize AND RowLayoutData.MaxWidth/MaxHeight are required: MinSize
	// prevents the button from being smaller, MaxWidth/MaxHeight caps the
	// preferred size (which otherwise defaults to 50×50 for empty buttons).
	smallSquare := func(col color.Color, size int) *widget.Button {
		return widget.NewButton(
			widget.ButtonOpts.Image(&widget.ButtonImage{
				Idle:     eimage.NewNineSliceColor(col),
				Hover:    eimage.NewNineSliceColor(col),
				Pressed:  eimage.NewNineSliceColor(col),
				Disabled: eimage.NewNineSliceColor(col),
			}),
			widget.ButtonOpts.WidgetOpts(
				widget.WidgetOpts.MinSize(size, size),
				widget.WidgetOpts.LayoutData(widget.RowLayoutData{
					MaxWidth:  size,
					MaxHeight: size,
				}),
			),
		)
	}

	iconSz := sz(20)
	digitWidth := sz(10)
	minResourceDigits := 4
	if g.hudDigits > minResourceDigits {
		minResourceDigits = g.hudDigits
	}

	// Use the current wood value as the initial label so the widget's preferred
	// size is correct when the HUD is (re)built.
	initialResourceLabel := fmt.Sprintf("%.0f", g.world.Economy.Wood)

	hud.resourceText = widget.NewText(
		widget.TextOpts.Text(initialResourceLabel, face, color.NRGBA{R: 180, G: 255, B: 180, A: 255}),
		widget.TextOpts.WidgetOpts(widget.WidgetOpts.MinSize(digitWidth*minResourceDigits, 0)),
	)
	hud.resourceHUD = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(sz(4)),
		)),
	)
	hud.resourceSquare = widget.NewButton(
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:     eimage.NewNineSliceColor(colWoodResource),
			Hover:    eimage.NewNineSliceColor(color.NRGBA{R: 60, G: 185, B: 80, A: 255}),
			Pressed:  eimage.NewNineSliceColor(color.NRGBA{R: 30, G: 130, B: 50, A: 255}),
			Disabled: eimage.NewNineSliceColor(colWoodResource),
		}),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(iconSz, iconSz),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{
				MaxWidth:  iconSz,
				MaxHeight: iconSz,
			}),
		),
		widget.ButtonOpts.PressedHandler(func(_ *widget.ButtonPressedEventArgs) {
			g.activateHold(holdNurture)
		}),
	)
	hud.resourceHUD.AddChild(hud.resourceSquare)
	hud.resourceHUD.AddChild(hud.resourceText)
	hud.resourceHUD.GetWidget().SetVisibility(widget.Visibility_Hide)

	hud.workerRatio = widget.NewText(
		widget.TextOpts.Text("0/0", face, color.NRGBA{R: 255, G: 240, B: 180, A: 255}),
		widget.TextOpts.WidgetOpts(widget.WidgetOpts.MinSize(digitWidth*5, 0)),
	)
	hud.workerHUD = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(sz(4)),
		)),
	)
	hud.workerHUD.AddChild(smallSquare(colWorkerLaden, iconSz))
	hud.workerHUD.AddChild(hud.workerRatio)
	hud.workerHUD.GetWidget().SetVisibility(widget.Visibility_Hide)

	hud.normalTopBar = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(sz(12)),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: sz(6), Left: sz(6), Bottom: sz(6), Right: sz(6)}),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
				StretchHorizontal:  true,
			}),
		),
	)
	hud.normalTopBar.AddChild(hud.resourceHUD)
	hud.normalTopBar.AddChild(hud.workerHUD)

	// --- normal mode: left sidebar (action buttons) ---
	// Top padding clears the top bar; proportional to scale.

	btnSz := sz(28)

	// actionSquare creates an interactive button for the sidebar.
	actionSquare := func(idle, hover, pressed, disabled color.Color) *widget.ButtonImage {
		return &widget.ButtonImage{
			Idle:     eimage.NewNineSliceColor(idle),
			Hover:    eimage.NewNineSliceColor(hover),
			Pressed:  eimage.NewNineSliceColor(pressed),
			Disabled: eimage.NewNineSliceColor(disabled),
		}
	}

	hud.buildCampBtn = widget.NewButton(
		widget.ButtonOpts.Image(actionSquare(
			colBuilding,
			color.NRGBA{R: 130, G: 82, B: 48, A: 255},
			color.NRGBA{R: 72, G: 44, B: 26, A: 255},
			color.NRGBA{R: 48, G: 34, B: 24, A: 255},
		)),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(btnSz, btnSz),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{MaxWidth: btnSz, MaxHeight: btnSz}),
		),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			if g.placing {
				g.placing = false
				g.freePlacing = false
				return
			}
			// Town Hall is free; only check affordability once a Town Hall exists.
			if len(g.world.Buildings) > 0 && g.world.Economy.Wood < CampCost(g.world) {
				g.pulseTime = pulseDuration
				g.pulseTarget = 1
				return
			}
			g.freePlacing = false
			g.placing = true
		}),
	)

	hud.buildTownCapacityBtn = widget.NewButton(
		widget.ButtonOpts.Image(actionSquare(
			colTownCapacity,
			color.NRGBA{R: 235, G: 132, B: 64, A: 255},
			color.NRGBA{R: 165, G: 78, B: 36, A: 255},
			color.NRGBA{R: 82, G: 48, B: 34, A: 255},
		)),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(btnSz, btnSz),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{MaxWidth: btnSz, MaxHeight: btnSz}),
		),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			if !buildTownCapacity(g.world) && g.world.Economy.Wood < townCapacityCost(g.world) {
				g.pulseTime = pulseDuration
				g.pulseTarget = 2
			}
		}),
	)
	hud.buildTownCapacityBtn.GetWidget().SetVisibility(widget.Visibility_Hide)

	sidebarSpacer := widget.NewContainer(
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(1, sz(10)),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{MaxHeight: sz(10)}),
		),
	)

	hud.normalSidebar = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(sz(6)),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: sz(52), Left: sz(8), Bottom: sz(8), Right: sz(8)}),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
			}),
		),
	)
	hud.normalSidebar.AddChild(hud.buildCampBtn)
	hud.normalSidebar.AddChild(sidebarSpacer)
	hud.normalSidebar.AddChild(hud.buildTownCapacityBtn)

	// --- settings menu overlay ---
	menuBtnImg := &widget.ButtonImage{
		Idle:     eimage.NewNineSliceColor(color.NRGBA{R: 60, G: 80, B: 120, A: 240}),
		Hover:    eimage.NewNineSliceColor(color.NRGBA{R: 80, G: 110, B: 160, A: 240}),
		Pressed:  eimage.NewNineSliceColor(color.NRGBA{R: 45, G: 65, B: 100, A: 240}),
		Disabled: eimage.NewNineSliceColor(color.NRGBA{R: 40, G: 40, B: 55, A: 240}),
	}
	menuTxtCol := &widget.ButtonTextColor{
		Idle:  color.White,
		Hover: color.White,
	}
	menuPad := &widget.Insets{Top: sz(8), Bottom: sz(8), Left: sz(16), Right: sz(16)}

	menuBtnLayoutData := widget.RowLayoutData{Stretch: true}

	hud.menuSaveBtn = widget.NewButton(
		widget.ButtonOpts.Image(menuBtnImg),
		widget.ButtonOpts.Text("Save", face, menuTxtCol),
		widget.ButtonOpts.TextPadding(menuPad),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(menuBtnLayoutData),
		),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			_ = Save(g.world)
		}),
	)

	hud.menuExportBtn = widget.NewButton(
		widget.ButtonOpts.Image(menuBtnImg),
		widget.ButtonOpts.Text("Export Save", face, menuTxtCol),
		widget.ButtonOpts.TextPadding(menuPad),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(menuBtnLayoutData),
		),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			exportSaveDialog(g)
		}),
	)

	hud.menuImportBtn = widget.NewButton(
		widget.ButtonOpts.Image(menuBtnImg),
		widget.ButtonOpts.Text("Import Save", face, menuTxtCol),
		widget.ButtonOpts.TextPadding(menuPad),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(menuBtnLayoutData),
		),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			importSaveDialog(g)
		}),
	)

	hud.menuResetBtn = widget.NewButton(
		widget.ButtonOpts.Image(menuBtnImg),
		widget.ButtonOpts.Text("Reset Game", face, menuTxtCol),
		widget.ButtonOpts.TextPadding(menuPad),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(menuBtnLayoutData),
		),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			ClearSave()
			g.world = NewWorld()
			g.placing = false
			g.freePlacing = false
			g.showMenu = false
		}),
	)

	hud.menuPanel = widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(
			eimage.NewNineSliceColor(color.NRGBA{R: 15, G: 15, B: 25, A: 220}),
		),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(sz(10)),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: sz(20), Bottom: sz(20), Left: sz(24), Right: sz(24)}),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
			}),
		),
	)
	hud.menuPanel.AddChild(hud.menuSaveBtn)
	hud.menuPanel.AddChild(hud.menuExportBtn)
	hud.menuPanel.AddChild(hud.menuImportBtn)
	hud.menuPanel.AddChild(hud.menuResetBtn)
	hud.menuPanel.GetWidget().SetVisibility(widget.Visibility_Hide)

	// Root: AnchorLayout holding debug panel, both normal-mode containers, and menu overlay.
	root := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	root.AddChild(hud.debugPanel)
	root.AddChild(hud.normalTopBar)
	root.AddChild(hud.normalSidebar)
	root.AddChild(hud.menuPanel)

	// Start in normal (minimalist) mode.
	hud.debugPanel.GetWidget().SetVisibility(widget.Visibility_Hide)

	return hud, &ebitenui.UI{Container: root}, nil
}
