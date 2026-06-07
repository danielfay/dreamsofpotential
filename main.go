package main

import (
	"bytes"
	"fmt"
	"image/color"
	"log"

	"github.com/ebitenui/ebitenui"
	eimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

type Game struct {
	ui    *ebitenui.UI
	count int
	label *widget.Text
}

func (g *Game) Update() error {
	g.ui.Update()
	g.label.Label = fmt.Sprintf("Clicked: %d", g.count)
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.ui.Draw(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

func main() {
	g := &Game{}

	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		log.Fatal(err)
	}
	var face text.Face = &text.GoTextFace{Source: src, Size: 16}

	root := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)

	panel := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(10),
		)),
		widget.ContainerOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
			}),
		),
	)

	g.label = widget.NewText(
		widget.TextOpts.Text("Clicked: 0", &face, color.White),
		widget.TextOpts.Position(widget.TextPositionCenter, widget.TextPositionCenter),
	)

	btn := widget.NewButton(
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:    eimage.NewNineSliceColor(color.NRGBA{R: 80, G: 120, B: 200, A: 255}),
			Hover:   eimage.NewNineSliceColor(color.NRGBA{R: 100, G: 140, B: 220, A: 255}),
			Pressed: eimage.NewNineSliceColor(color.NRGBA{R: 60, G: 100, B: 180, A: 255}),
		}),
		widget.ButtonOpts.Text("Click me!", &face, &widget.ButtonTextColor{
			Idle:  color.White,
			Hover: color.White,
		}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Top: 8, Bottom: 8, Left: 16, Right: 16}),
		widget.ButtonOpts.ClickedHandler(func(_ *widget.ButtonClickedEventArgs) {
			g.count++
		}),
	)

	panel.AddChild(g.label)
	panel.AddChild(btn)
	root.AddChild(panel)

	g.ui = &ebitenui.UI{Container: root}

	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Dreams of Potential")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
