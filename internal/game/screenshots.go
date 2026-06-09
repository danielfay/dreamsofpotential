package game

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

// WriteScreenshotSet renders a deterministic set of world screenshots into dir.
func WriteScreenshotSet(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	g := &screenshotGame{
		dir:   dir,
		shots: screenshotScenarios(),
	}
	ebiten.SetWindowSize(virtW*2, virtH*2)
	ebiten.SetWindowTitle("Dreams of Potential Screenshots")
	if err := ebiten.RunGame(g); err != nil {
		return err
	}
	return g.err
}

type screenshotScenario struct {
	name    string
	world   *World
	preview *placementPreview
	fullHUD bool
}

func screenshotScenarios() []screenshotScenario {
	return []screenshotScenario{
		freshPlanetScenario(),
		townHallPreviewScenario(),
		townHallIdleScenario(),
		workingLoopScenario(),
		campPreviewScenario(),
		affordabilityButtonsScenario(),
		wideResourceHUDScenario(),
	}
}

func freshPlanetScenario() screenshotScenario {
	return screenshotScenario{
		name:  "01-fresh-planet",
		world: screenshotWorld(11),
	}
}

func townHallPreviewScenario() screenshotScenario {
	w := screenshotWorld(11)
	angle := normAngle(w.Nodes[0].Angle + buildingHardHalfArc(KindTownHall, w.Planet.Radius) + nodeBuildingBlockHalfArc(w.Nodes[0], w.Planet.Radius) + 0.01)
	pv := buildPreview(w, angle)
	return screenshotScenario{
		name:    "02-town-hall-preview",
		world:   w,
		preview: &pv,
	}
}

func townHallIdleScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlaceNearNode(w, w.Nodes[0])
	w.Economy.Wood = 1000
	for range 7 {
		mustBuyWorker(w)
	}
	return screenshotScenario{
		name:  "03-town-hall-idle-home",
		world: w,
	}
}

func workingLoopScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlaceNearNode(w, w.Nodes[0])
	w.Economy.Wood = 1000
	for range 3 {
		mustBuyWorker(w)
	}
	for range 60 * 12 {
		Step(w, dt)
	}
	return screenshotScenario{
		name:  "04-working-loop",
		world: w,
	}
}

func campPreviewScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlaceNearNode(w, w.Nodes[0])
	w.ResourceDiscovered = true
	w.Economy.Wood = CampCost(w)

	angle := normAngle(w.Nodes[0].Angle + math.Pi/3)
	pv := buildPreview(w, angle)
	return screenshotScenario{
		name:    "05-camp-preview",
		world:   w,
		preview: &pv,
	}
}

func affordabilityButtonsScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlaceNearNode(w, w.Nodes[0])
	w.ResourceDiscovered = true
	w.Economy.Wood = 12
	w.Economy.WorkersBought = 3

	return screenshotScenario{
		name:    "06-affordability-buttons",
		world:   w,
		fullHUD: true,
	}
}

func wideResourceHUDScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlaceNearNode(w, w.Nodes[0])
	w.ResourceDiscovered = true
	w.Economy.Wood = 1234
	w.Economy.WorkersBought = 16
	for i := 0; i < 16; i++ {
		state := StateIdleWaiting
		nodeID := -1
		if i < 13 {
			state = StateToForest
			nodeID = 0
		}
		w.Workers = append(w.Workers, &Worker{
			ID:            i,
			State:         state,
			NodeID:        nodeID,
			TargetNodeID:  -1,
			PendingNodeID: -1,
		})
	}

	return screenshotScenario{
		name:    "07-wide-resource-hud",
		world:   w,
		fullHUD: true,
	}
}

func screenshotWorld(seed int64) *World {
	rand.Seed(seed)
	return NewWorld()
}

func mustPlace(w *World, angle float64) {
	if !placeBuilding(w, angle) {
		panic(fmt.Sprintf("screenshot setup failed to place building at %.3f", angle))
	}
}

func mustPlaceNearNode(w *World, node *ResourceNode) {
	kind := KindTownHall
	if len(w.Buildings) > 0 {
		kind = KindLoggingCamp
	}
	clearance := buildingHardHalfArc(kind, w.Planet.Radius) + nodeBuildingBlockHalfArc(node, w.Planet.Radius) + 0.01
	step := 2 / w.Planet.Radius
	for i := 0; i < 120; i++ {
		offset := clearance + float64(i)*step
		for _, sign := range []float64{1, -1} {
			angle := normAngle(node.Angle + sign*offset)
			if buildPreview(w, angle).Valid {
				mustPlace(w, angle)
				return
			}
		}
	}
	panic(fmt.Sprintf("screenshot setup failed to find valid placement near node %d", node.ID))
}

func mustBuyWorker(w *World) {
	if !buyWorker(w) {
		panic("screenshot setup failed to buy worker")
	}
}

type screenshotGame struct {
	dir   string
	shots []screenshotScenario
	done  bool
	err   error
}

func (g *screenshotGame) Update() error {
	if g.done {
		if g.err != nil {
			return g.err
		}
		return ebiten.Termination
	}
	return nil
}

func (g *screenshotGame) Draw(screen *ebiten.Image) {
	scene := ebiten.NewImage(virtW, virtH)
	for _, shot := range g.shots {
		screen.Clear()
		if shot.fullHUD {
			if err := drawHUDScreenshot(screen, shot.world); err != nil {
				g.err = fmt.Errorf("%s: %w", shot.name, err)
				break
			}
		} else {
			scene.Clear()
			DrawWorld(scene, shot.world, shot.preview, false)

			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(2, 2)
			op.Filter = ebiten.FilterNearest
			screen.DrawImage(scene, op)
		}

		path := filepath.Join(g.dir, shot.name+".png")
		if err := writeScreenPNG(path, screen); err != nil {
			g.err = fmt.Errorf("%s: %w", shot.name, err)
			break
		}
	}
	g.done = true
}

func (g *screenshotGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	return virtW * 2, virtH * 2
}

func drawHUDScreenshot(screen *ebiten.Image, w *World) error {
	const scale = 2
	game := &Game{
		world:     w,
		scene:     ebiten.NewImage(virtW, virtH),
		hudScale:  scale,
		hudDigits: woodDigits(w.Economy.Wood),
	}
	hud, ui, err := buildHUD(game, scale)
	if err != nil {
		return err
	}
	game.hud = hud
	game.ui = ui
	game.hud.Refresh(game.world, false, false, nil, false)
	game.Draw(screen)
	return nil
}

func writeScreenPNG(path string, screen *ebiten.Image) error {
	bounds := screen.Bounds()
	img := image.NewRGBA(bounds)
	screen.ReadPixels(img.Pix)

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	return png.Encode(out, img)
}
