package game

import (
	"bytes"
	"fmt"
	"image/color"

	"github.com/ebitenui/ebitenui"
	eimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

// HUD holds the live EbitenUI widgets for the resource display and action buttons.
// It maintains two sibling panels: a minimalist normal panel and a verbose debug
// panel toggled by F3. Only one is visible at a time.
type HUD struct {
	face text.Face

	// debug panel
	debugPanel   *widget.Container
	woodText     *widget.Text
	workerText   *widget.Text
	nodeText     *widget.Text
	fieldText    *widget.Text
	buyWorkerDbg *widget.Button
	buildCampDbg *widget.Button
	resetBtn     *widget.Button

	// normal symbolic panel
	normalPanel  *widget.Container
	buildCampBtn *widget.Button // brown square — always visible
	buyWorkerBtn *widget.Button // yellow square — hidden until first camp
	resourceHUD  *widget.Container
	resourceText *widget.Text
	workerHUD    *widget.Container
	workerRatio  *widget.Text
}

// pointInHUD reports whether native screen coordinates (sx, sy) fall inside the
// visible HUD panel. Called after ui.Update() so the Rect is current.
func (h *HUD) pointInHUD(sx, sy int, debug bool) bool {
	var panel *widget.Container
	if debug {
		panel = h.debugPanel
	} else {
		panel = h.normalPanel
	}
	r := panel.GetWidget().Rect
	return r.Min.X <= sx && sx < r.Max.X && r.Min.Y <= sy && sy < r.Max.Y
}

// Refresh updates all HUD labels and visibility states to match the world.
func (h *HUD) Refresh(w *World, placing, debug bool) {
	if debug {
		h.debugPanel.GetWidget().SetVisibility(widget.Visibility_Show)
		h.normalPanel.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.refreshDebug(w, placing)
	} else {
		h.debugPanel.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.normalPanel.GetWidget().SetVisibility(widget.Visibility_Show)
		h.refreshNormal(w)
	}
}

func (h *HUD) refreshDebug(w *World, placing bool) {
	freeNodes, claimedNodes := 0, 0
	for _, n := range w.Nodes {
		if n.OwnerID == -1 {
			freeNodes++
		} else {
			claimedNodes++
		}
	}

	h.woodText.Label = fmt.Sprintf("wood: %.0f (%.2f/s)", w.Economy.Wood, EstimateRate(w))

	active, idle := 0, 0
	for _, wk := range w.Workers {
		if wk.NodeID == -1 {
			idle++
		} else {
			active++
		}
	}
	h.workerText.Label = fmt.Sprintf("workers: %d active  %d idle  %d total", active, idle, len(w.Workers))
	h.nodeText.Label = fmt.Sprintf("nodes: %d free  %d claimed", freeNodes, claimedNodes)

	if len(w.Planet.Fields) > 0 {
		f := w.Planet.Fields[0]
		h.fieldText.Label = fmt.Sprintf("field: %.1f / %.1f", f.Counter, f.Cap)
	}

	wc := WorkerCost(w)
	h.buyWorkerDbg.SetText(fmt.Sprintf("Buy worker (%.0f)", wc))
	h.buyWorkerDbg.GetWidget().Disabled = w.Economy.Wood < wc || len(w.Buildings) == 0

	cc := CampCost(w)
	h.buildCampDbg.SetText(fmt.Sprintf("Build camp (%.0f)", cc))
	h.buildCampDbg.GetWidget().Disabled = placing
}

func (h *HUD) refreshNormal(w *World) {
	hasCamp := len(w.Buildings) > 0
	discovered := w.ResourceDiscovered

	// Build button: always visible; enabled when free (first camp) or discovered.
	h.buildCampBtn.GetWidget().Disabled = w.Economy.CampsBought > 0 && !discovered

	// Worker button: hidden until first camp exists.
	if hasCamp {
		h.buyWorkerBtn.GetWidget().SetVisibility(widget.Visibility_Show)
		h.buyWorkerBtn.GetWidget().Disabled = w.Economy.WorkersBought > 0 && !discovered
	} else {
		h.buyWorkerBtn.GetWidget().SetVisibility(widget.Visibility_Hide)
	}

	// Resource HUD: hidden until first delivery.
	if discovered {
		h.resourceHUD.GetWidget().SetVisibility(widget.Visibility_Show)
		h.resourceText.Label = fmt.Sprintf("%.0f", w.Economy.Wood)
	} else {
		h.resourceHUD.GetWidget().SetVisibility(widget.Visibility_Hide)
	}

	// Worker HUD: hidden until first worker exists.
	if len(w.Workers) > 0 {
		h.workerHUD.GetWidget().SetVisibility(widget.Visibility_Show)
		active := 0
		for _, wk := range w.Workers {
			if wk.NodeID != -1 {
				active++
			}
		}
		h.workerRatio.Label = fmt.Sprintf("%d/%d", active, len(w.Workers))
	} else {
		h.workerHUD.GetWidget().SetVisibility(widget.Visibility_Hide)
	}
}

// buildHUD constructs the EbitenUI tree and wires up button handlers.
func buildHUD(g *Game) (*HUD, *ebitenui.UI, error) {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, nil, err
	}

	hud := &HUD{}
	hud.face = &text.GoTextFace{Source: src, Size: 16}
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
	dbgPad := &widget.Insets{Top: 6, Bottom: 6, Left: 12, Right: 12}

	mkText := func(initial string) *widget.Text {
		return widget.NewText(
			widget.TextOpts.Text(initial, face, color.NRGBA{R: 180, G: 220, B: 180, A: 255}),
		)
	}
	hud.woodText = mkText("wood: 0 (0.00/s)")
	hud.workerText = mkText("workers: 0 active  0 idle  0 total")
	hud.nodeText = mkText("nodes: 0 free  0 claimed")
	hud.fieldText = mkText("field: 0.0 / 0.0")

	hud.buyWorkerDbg = widget.NewButton(
		widget.ButtonOpts.Image(dbgBtnImg()),
		widget.ButtonOpts.Text(fmt.Sprintf("Buy worker (%.0f)", workerBaseCost), face, dbgTxtCol),
		widget.ButtonOpts.TextPadding(dbgPad),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			buyWorker(g.world)
		}),
	)

	hud.buildCampDbg = widget.NewButton(
		widget.ButtonOpts.Image(dbgBtnImg()),
		widget.ButtonOpts.Text(fmt.Sprintf("Build camp (%.0f)", campBaseCost), face, dbgTxtCol),
		widget.ButtonOpts.TextPadding(dbgPad),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			if g.placing {
				g.placing = false
				return
			}
			g.placing = true
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
		}),
	)

	hud.debugPanel = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(6),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: 8, Left: 8, Bottom: 8, Right: 8}),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
			}),
		),
	)
	hud.debugPanel.AddChild(hud.woodText)
	hud.debugPanel.AddChild(hud.workerText)
	hud.debugPanel.AddChild(hud.nodeText)
	hud.debugPanel.AddChild(hud.fieldText)
	hud.debugPanel.AddChild(hud.buyWorkerDbg)
	hud.debugPanel.AddChild(hud.buildCampDbg)
	hud.debugPanel.AddChild(hud.resetBtn)

	// --- normal symbolic panel ---

	// symSquare creates a non-interactive 28×28 solid-colored square widget for icons.
	symSquare := func(col color.NRGBA) *widget.Button {
		return widget.NewButton(
			widget.ButtonOpts.Image(&widget.ButtonImage{
				Idle:     eimage.NewNineSliceColor(col),
				Hover:    eimage.NewNineSliceColor(col),
				Pressed:  eimage.NewNineSliceColor(col),
				Disabled: eimage.NewNineSliceColor(col),
			}),
			widget.ButtonOpts.WidgetOpts(
				widget.WidgetOpts.MinSize(28, 28),
			),
		)
	}

	// Brown build-camp button. Click toggles placement mode or triggers pulse.
	hud.buildCampBtn = widget.NewButton(
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:     eimage.NewNineSliceColor(color.NRGBA{R: 140, G: 90, B: 50, A: 255}),
			Hover:    eimage.NewNineSliceColor(color.NRGBA{R: 170, G: 115, B: 70, A: 255}),
			Pressed:  eimage.NewNineSliceColor(color.NRGBA{R: 110, G: 70, B: 35, A: 255}),
			Disabled: eimage.NewNineSliceColor(color.NRGBA{R: 80, G: 55, B: 35, A: 255}),
		}),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(28, 28),
		),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			if g.placing {
				g.placing = false
				return
			}
			if g.world.Economy.Wood < CampCost(g.world) {
				g.pulseTime = pulseDuration
				g.pulseTarget = 1
				return
			}
			g.placing = true
		}),
	)

	// Yellow buy-worker button. Click buys or triggers pulse.
	hud.buyWorkerBtn = widget.NewButton(
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:     eimage.NewNineSliceColor(color.NRGBA{R: 220, G: 200, B: 60, A: 255}),
			Hover:    eimage.NewNineSliceColor(color.NRGBA{R: 255, G: 240, B: 80, A: 255}),
			Pressed:  eimage.NewNineSliceColor(color.NRGBA{R: 180, G: 160, B: 40, A: 255}),
			Disabled: eimage.NewNineSliceColor(color.NRGBA{R: 100, G: 90, B: 35, A: 255}),
		}),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(28, 28),
		),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			if buyWorker(g.world) {
				return
			}
			if g.world.ResourceDiscovered && g.world.Economy.Wood < WorkerCost(g.world) {
				g.pulseTime = pulseDuration
				g.pulseTarget = 2
			}
		}),
	)
	hud.buyWorkerBtn.GetWidget().SetVisibility(widget.Visibility_Hide)

	// Resource HUD: non-interactive green icon + amount number.
	hud.resourceText = widget.NewText(
		widget.TextOpts.Text("0", face, color.NRGBA{R: 180, G: 255, B: 180, A: 255}),
	)
	hud.resourceHUD = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(6),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(0, 28),
		),
	)
	hud.resourceHUD.AddChild(symSquare(color.NRGBA{R: 40, G: 160, B: 60, A: 255}))
	hud.resourceHUD.AddChild(hud.resourceText)
	hud.resourceHUD.GetWidget().SetVisibility(widget.Visibility_Hide)

	// Worker HUD: non-interactive yellow icon + active/total ratio.
	hud.workerRatio = widget.NewText(
		widget.TextOpts.Text("0/0", face, color.NRGBA{R: 255, G: 240, B: 180, A: 255}),
	)
	hud.workerHUD = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(6),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(0, 28),
		),
	)
	hud.workerHUD.AddChild(symSquare(color.NRGBA{R: 220, G: 200, B: 60, A: 255}))
	hud.workerHUD.AddChild(hud.workerRatio)
	hud.workerHUD.GetWidget().SetVisibility(widget.Visibility_Hide)

	hud.normalPanel = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(6),
			widget.RowLayoutOpts.Padding(&widget.Insets{Top: 8, Left: 8, Bottom: 8, Right: 8}),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				HorizontalPosition: widget.AnchorLayoutPositionStart,
				VerticalPosition:   widget.AnchorLayoutPositionStart,
			}),
		),
	)
	hud.normalPanel.AddChild(hud.buildCampBtn)
	hud.normalPanel.AddChild(hud.buyWorkerBtn)
	hud.normalPanel.AddChild(hud.resourceHUD)
	hud.normalPanel.AddChild(hud.workerHUD)

	root := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	root.AddChild(hud.debugPanel)
	root.AddChild(hud.normalPanel)

	// Start hidden — normal (minimalist) mode is on.
	hud.debugPanel.GetWidget().SetVisibility(widget.Visibility_Hide)

	return hud, &ebitenui.UI{Container: root}, nil
}
