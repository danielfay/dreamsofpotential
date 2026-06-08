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

// HUD holds the live EbitenUI widgets for the resource display and action buttons.
//
// Normal mode uses two separate anchored containers:
//   - normalTopBar: small resource/worker info, horizontal row across the top.
//   - normalSidebar: action buttons (build, worker), vertical column on the left.
//
// Debug mode (F3) replaces both with a single verbose debugPanel.
type HUD struct {
	face text.Face

	// debug panel
	debugPanel   *widget.Container
	woodText     *widget.Text
	workerText   *widget.Text
	nodeText     *widget.Text
	fieldText    *widget.Text
	previewText  *widget.Text
	buyWorkerDbg *widget.Button
	buildCampDbg *widget.Button
	resetBtn     *widget.Button

	// normal mode — top bar (resource info, horizontal, full-width)
	normalTopBar *widget.Container
	resourceHUD  *widget.Container // green icon + amount; hidden until discovered
	resourceText *widget.Text
	workerHUD    *widget.Container // yellow icon + ratio; hidden until first worker
	workerRatio  *widget.Text

	// normal mode — left sidebar (action buttons, vertical)
	normalSidebar *widget.Container
	buildCampBtn  *widget.Button // brown square — always visible
	buyWorkerBtn  *widget.Button // yellow square — hidden until first camp
}

// pointInHUD reports whether native screen coordinates (sx, sy) fall inside any
// visible HUD area. Called after ui.Update() so Rects are current.
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
func (h *HUD) Refresh(w *World, placing, debug bool, pv *placementPreview) {
	if debug {
		h.debugPanel.GetWidget().SetVisibility(widget.Visibility_Show)
		h.normalTopBar.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.normalSidebar.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.refreshDebug(w, placing, pv)
	} else {
		h.debugPanel.GetWidget().SetVisibility(widget.Visibility_Hide)
		h.normalTopBar.GetWidget().SetVisibility(widget.Visibility_Show)
		h.normalSidebar.GetWidget().SetVisibility(widget.Visibility_Show)
		h.refreshNormal(w)
	}
}

func (h *HUD) refreshDebug(w *World, placing bool, pv *placementPreview) {
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
		h.previewText.Label = fmt.Sprintf("preview: %s  nearby %d (%d free / %d claimed)  d=%s",
			validity, len(pv.Free)+len(pv.Claimed), len(pv.Free), len(pv.Claimed), distStr)
	} else {
		h.previewText.Label = "preview: —"
	}
}

func (h *HUD) refreshNormal(w *World) {
	hasCamp := len(w.Buildings) > 0
	discovered := w.ResourceDiscovered

	// --- sidebar: action buttons ---

	// Build button: disabled if locked (pre-discovery repeat) or unaffordable.
	campLocked := w.Economy.CampsBought > 0 && !discovered
	h.buildCampBtn.GetWidget().Disabled = campLocked || w.Economy.Wood < CampCost(w)

	// Worker button: hidden until first camp exists; disabled if locked or unaffordable.
	if hasCamp {
		h.buyWorkerBtn.GetWidget().SetVisibility(widget.Visibility_Show)
		workerLocked := w.Economy.WorkersBought > 0 && !discovered
		h.buyWorkerBtn.GetWidget().Disabled = workerLocked || w.Economy.Wood < WorkerCost(w)
	} else {
		h.buyWorkerBtn.GetWidget().SetVisibility(widget.Visibility_Hide)
	}

	// --- top bar: resource info ---

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
// scale is the integer view scale factor (1 at ≤320×240, 2 at 640×480, etc.);
// all pixel sizes are multiplied by it so the HUD matches the world scale.
func buildHUD(g *Game, scale int) (*HUD, *ebitenui.UI, error) {
	s := scale
	// sz returns n*s scaled to 75% of the world scale, rounded to nearest pixel.
	sz := func(n int) int {
		v := (n * s * 3 + 2) / 4
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
	hud.debugPanel.AddChild(hud.woodText)
	hud.debugPanel.AddChild(hud.workerText)
	hud.debugPanel.AddChild(hud.nodeText)
	hud.debugPanel.AddChild(hud.fieldText)
	hud.debugPanel.AddChild(hud.previewText)
	hud.debugPanel.AddChild(hud.buyWorkerDbg)
	hud.debugPanel.AddChild(hud.buildCampDbg)
	hud.debugPanel.AddChild(hud.resetBtn)

	// --- normal mode: top bar ---
	// Stretched horizontally so its Rect covers the full screen width, making
	// pointInHUD reliable even before any children are visible.

	// smallSquare creates a non-interactive solid-color square for the top bar.
	// Both MinSize AND RowLayoutData.MaxWidth/MaxHeight are required: MinSize
	// prevents the button from being smaller, MaxWidth/MaxHeight caps the
	// preferred size (which otherwise defaults to 50×50 for empty buttons).
	smallSquare := func(col color.NRGBA, size int) *widget.Button {
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

	hud.resourceText = widget.NewText(
		widget.TextOpts.Text("0", face, color.NRGBA{R: 180, G: 255, B: 180, A: 255}),
	)
	hud.resourceHUD = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(sz(4)),
		)),
	)
	hud.resourceHUD.AddChild(smallSquare(color.NRGBA{R: 40, G: 160, B: 60, A: 255}, iconSz))
	hud.resourceHUD.AddChild(hud.resourceText)
	hud.resourceHUD.GetWidget().SetVisibility(widget.Visibility_Hide)

	hud.workerRatio = widget.NewText(
		widget.TextOpts.Text("0/0", face, color.NRGBA{R: 255, G: 240, B: 180, A: 255}),
	)
	hud.workerHUD = widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(sz(4)),
		)),
	)
	hud.workerHUD.AddChild(smallSquare(color.NRGBA{R: 220, G: 200, B: 60, A: 255}, iconSz))
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
	actionSquare := func(idle, hover, pressed, disabled color.NRGBA) *widget.ButtonImage {
		return &widget.ButtonImage{
			Idle:     eimage.NewNineSliceColor(idle),
			Hover:    eimage.NewNineSliceColor(hover),
			Pressed:  eimage.NewNineSliceColor(pressed),
			Disabled: eimage.NewNineSliceColor(disabled),
		}
	}

	hud.buildCampBtn = widget.NewButton(
		widget.ButtonOpts.Image(actionSquare(
			color.NRGBA{R: 140, G: 90, B: 50, A: 255},
			color.NRGBA{R: 170, G: 115, B: 70, A: 255},
			color.NRGBA{R: 110, G: 70, B: 35, A: 255},
			color.NRGBA{R: 70, G: 48, B: 30, A: 255},
		)),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(btnSz, btnSz),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{MaxWidth: btnSz, MaxHeight: btnSz}),
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

	hud.buyWorkerBtn = widget.NewButton(
		widget.ButtonOpts.Image(actionSquare(
			color.NRGBA{R: 220, G: 200, B: 60, A: 255},
			color.NRGBA{R: 255, G: 240, B: 80, A: 255},
			color.NRGBA{R: 180, G: 160, B: 40, A: 255},
			color.NRGBA{R: 88, G: 80, B: 34, A: 255},
		)),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(btnSz, btnSz),
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{MaxWidth: btnSz, MaxHeight: btnSz}),
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
	hud.normalSidebar.AddChild(hud.buyWorkerBtn)

	// Root: AnchorLayout holding debug panel and both normal-mode containers.
	root := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	root.AddChild(hud.debugPanel)
	root.AddChild(hud.normalTopBar)
	root.AddChild(hud.normalSidebar)

	// Start in normal (minimalist) mode.
	hud.debugPanel.GetWidget().SetVisibility(widget.Visibility_Hide)

	return hud, &ebitenui.UI{Container: root}, nil
}
