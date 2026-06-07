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
// Widget pointers are retained so Refresh can update them every frame.
type HUD struct {
	// face is stored in the struct so &hud.face is a stable heap pointer
	// (required because EbitenUI buttons store *text.Face).
	face text.Face

	woodText     *widget.Text
	rateText     *widget.Text
	buyWorkerBtn *widget.Button
	buildCampBtn *widget.Button
	panel        *widget.Container // top-left panel; bounds used by pointInHUD
}

// pointInHUD reports whether native screen coordinates (sx, sy) fall inside the
// HUD panel. Called after ui.Update() so the Rect is current.
func (h *HUD) pointInHUD(sx, sy int) bool {
	r := h.panel.GetWidget().Rect
	return r.Min.X <= sx && sx < r.Max.X && r.Min.Y <= sy && sy < r.Max.Y
}

// Refresh updates all HUD labels and button disabled-states to match the world.
// Call this every frame from Game.Update(), after ui.Update() and sim.Step().
func (h *HUD) Refresh(w *World, placing bool) {
	h.woodText.Label = fmt.Sprintf("Wood: %.0f", w.Economy.Wood)
	h.rateText.Label = fmt.Sprintf("+%.2f/s", EstimateRate(w))

	wc := WorkerCost(w)
	h.buyWorkerBtn.SetText(fmt.Sprintf("Buy worker (%.0f)", wc))
	h.buyWorkerBtn.GetWidget().Disabled = w.Economy.Wood < wc || len(w.Buildings) == 0

	cc := CampCost(w)
	h.buildCampBtn.SetText(fmt.Sprintf("Build camp (%.0f)", cc))
	h.buildCampBtn.GetWidget().Disabled = w.Economy.Wood < cc || placing
}

// buildHUD constructs the EbitenUI tree and wires up button handlers.
// It returns the HUD (for per-frame Refresh) and the root ebitenui.UI.
func buildHUD(g *Game) (*HUD, *ebitenui.UI, error) {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, nil, err
	}

	hud := &HUD{}
	hud.face = &text.GoTextFace{Source: src, Size: 16}
	face := &hud.face // stable pointer into the heap-allocated HUD struct

	// Shared button styling.
	btnImg := func() *widget.ButtonImage {
		return &widget.ButtonImage{
			Idle:     eimage.NewNineSliceColor(color.NRGBA{R: 80, G: 120, B: 200, A: 255}),
			Hover:    eimage.NewNineSliceColor(color.NRGBA{R: 100, G: 140, B: 220, A: 255}),
			Pressed:  eimage.NewNineSliceColor(color.NRGBA{R: 60, G: 100, B: 180, A: 255}),
			Disabled: eimage.NewNineSliceColor(color.NRGBA{R: 55, G: 55, B: 70, A: 255}),
		}
	}
	btnTxtCol := &widget.ButtonTextColor{
		Idle:     color.White,
		Hover:    color.White,
		Disabled: color.RGBA{R: 120, G: 120, B: 130, A: 255},
	}
	padding := &widget.Insets{Top: 6, Bottom: 6, Left: 12, Right: 12}

	// --- resource readout ---
	hud.woodText = widget.NewText(
		widget.TextOpts.Text("Wood: 50", face, color.White),
	)
	hud.rateText = widget.NewText(
		widget.TextOpts.Text("+0.00/s", face, color.RGBA{R: 160, G: 240, B: 160, A: 255}),
	)

	// --- buy worker button ---
	hud.buyWorkerBtn = widget.NewButton(
		widget.ButtonOpts.Image(btnImg()),
		widget.ButtonOpts.Text(
			fmt.Sprintf("Buy worker (%.0f)", workerBaseCost),
			face, btnTxtCol,
		),
		widget.ButtonOpts.TextPadding(padding),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			if len(g.world.Buildings) == 0 {
				return
			}
			cost := WorkerCost(g.world)
			if g.world.Economy.Wood < cost {
				return
			}
			g.world.Economy.Wood -= cost
			g.world.Economy.WorkersBought++
			camp := g.world.Buildings[0]
			camp.Workers = append(camp.Workers, &Worker{
				Pos:   camp.Pos,
				State: StateToForest,
				Home:  camp,
			})
		}),
	)

	// --- build camp button ---
	hud.buildCampBtn = widget.NewButton(
		widget.ButtonOpts.Image(btnImg()),
		widget.ButtonOpts.Text(
			fmt.Sprintf("Build camp (%.0f)", campBaseCost),
			face, btnTxtCol,
		),
		widget.ButtonOpts.TextPadding(padding),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			if g.world.Economy.Wood < CampCost(g.world) {
				return
			}
			g.placing = true
		}),
	)

	// --- layout: vertical column in the top-left corner ---
	hud.panel = widget.NewContainer(
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
	hud.panel.AddChild(hud.woodText)
	hud.panel.AddChild(hud.rateText)
	hud.panel.AddChild(hud.buyWorkerBtn)
	hud.panel.AddChild(hud.buildCampBtn)

	root := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	root.AddChild(hud.panel)

	return hud, &ebitenui.UI{Container: root}, nil
}
