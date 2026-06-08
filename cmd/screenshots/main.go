package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/danielfay/dreamsofpotential/internal/game"
)

func main() {
	outDir := flag.String("out", "screenshots", "directory to write PNG screenshots")
	flag.Parse()

	if err := game.WriteScreenshotSet(*outDir); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote screenshots to %s\n", *outDir)
}
