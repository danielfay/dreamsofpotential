package main

import (
	"flag"
	"log"

	"github.com/danielfay/dreamsofpotential/internal/debuglog"
	"github.com/danielfay/dreamsofpotential/internal/game"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	debugCats := flag.String("debug", "", "comma-separated debug categories, e.g. -debug=workers,economy")
	flag.Parse()

	if *debugCats != "" {
		if err := debuglog.Init(*debugCats, "logs/debug.log"); err != nil {
			log.Printf("debug log: %v", err)
		} else {
			defer debuglog.Close()
			log.Printf("debug log active: categories=%s", *debugCats)
		}
	}

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
