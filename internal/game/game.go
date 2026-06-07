package game

import (
	"github.com/ebitenui/ebitenui"
	"github.com/hajimehoshi/ebiten/v2"
)

const dt = 1.0 / 60.0

// Game is the root ebiten game object.
type Game struct {
	world   *World
	scene   *ebiten.Image  // low-res 320×240 canvas; scaled 2× to the window
	ui      *ebitenui.UI
	hud     *HUD
	placing bool // true while waiting for player to click a camp location
}

// New constructs and returns a ready-to-run Game.
func New() (*Game, error) {
	g := &Game{
		world: NewWorld(),
		scene: ebiten.NewImage(virtW, virtH),
	}
	hud, ui, err := buildHUD(g)
	if err != nil {
		return nil, err
	}
	g.hud = hud
	g.ui = ui
	return g, nil
}

func (g *Game) Update() error {
	g.ui.Update()
	Step(g.world, dt)
	g.hud.Refresh(g.world, g.placing)
	g.handleInput()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	DrawWorld(g.scene, g.world, g.ghostPos())

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scaleX, scaleY)
	op.Filter = ebiten.FilterNearest
	screen.DrawImage(g.scene, op)

	g.ui.Draw(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}
