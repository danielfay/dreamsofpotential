package main

import (
	"log"

	"github.com/danielfay/dreamsofpotential/internal/game"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	g, err := game.New()
	if err != nil {
		log.Fatal(err)
	}
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("Dreams of Potential")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowClosingHandled(true)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
