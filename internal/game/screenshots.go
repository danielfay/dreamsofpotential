package game

import (
	"fmt"
	"image"
	"image/png"
	"math"
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
	name             string
	world            *World
	preview          *placementPreview
	fullHUD          bool
	debug            bool
	debugSection     int
	placing          bool
	revealActive     bool
	revealElapsed    float64
	selectBuilding   *int // non-nil: select this building index (for tray screenshots)
	showFocusControl bool
	focusDraftWater  int
}

func screenshotScenarios() []screenshotScenario {
	return []screenshotScenario{
		freshPlanetScenario(),
		townHallPreviewScenario(),
		townHallIdleScenario(),
		townFieldFreshScenario(),
		townFieldFullScenario(),
		workingLoopScenario(),
		campPreviewScenario(),
		invalidFullRimPreviewScenario(),
		debugPlacementDiagnosticsScenario(),
		affordabilityButtonsScenario(),
		townHallSelectedScenario(),
		wideResourceHUDScenario(),
		fieldGrowthSpawnCueScenario(),
		fieldGrowthUpgradeCueScenario(),
		fieldGrowthPlacementCueScenario(),
		systemViewScenario(),
		systemViewEchoSelectedScenario(),
		revealMidpointScenario(),
		planetViewReturnButtonScenario(),
		systemViewEchoAwakenablePotentialScenario(),
		systemViewEchoAwakenedScenario(),
		lakewoodFreshScenario(),
		systemViewOneEchoCompletedScenario(),
		systemViewBothEchoesCompletedScenario(),
		systemViewForestPotentialScenario(),
		tightGroveFreshScenario(),
		lakewoodNearCompleteScenario(),
		systemViewLakewoodCompletedScenario(),
		lakewoodDebugInfluenceScenario(),
		systemViewUnknownWaterResonanceScenario(),
		systemViewFrontierPreAwakenable(),
		systemViewFrontierAwakenableScenario(),
		waterFrontierFreshScenario(),
		systemViewFrontierAwakenedScenario(),
		waterPlanetFirstDockScenario(),
		waterPlanetSparklesScenario(),
		waterPlanetDockUpgradeSelectedScenario(),
		waterPlanetNearCompleteNoL2DockScenario(),
		dockConeVisibilityScenario(),
		workerRatioUIOpenScenario(),              // 38
		workerRatioHUDScenario(),                 // 39
		waterPlanetCompletedSystemViewScenario(), // 40
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
	angle := normAngle(w.Planet.Fields[0].CenterAngle + 0.3)
	pv := buildPreview(w, angle)
	return screenshotScenario{
		name:    "02-town-hall-preview",
		world:   w,
		preview: &pv,
	}
}

func townHallIdleScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlace(w, w.Planet.Fields[0].CenterAngle)
	w.Economy.Wood = 1000
	for range 7 {
		mustBuyWorker(w)
	}
	return screenshotScenario{
		name:  "03-town-hall-idle-home",
		world: w,
	}
}

func townFieldFreshScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlace(w, w.Planet.Fields[0].CenterAngle)
	return screenshotScenario{
		name:  "04-town-field-fresh",
		world: w,
	}
}

func townFieldFullScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlace(w, w.Planet.Fields[0].CenterAngle)
	w.Economy.WorkerCapacity = maxTownSlots(w)
	w.Economy.Wood = 200
	return screenshotScenario{
		name:  "05-town-field-full",
		world: w,
	}
}

func workingLoopScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlace(w, w.Planet.Fields[0].CenterAngle)
	w.Economy.Wood = 1000
	for range 3 {
		mustBuyWorker(w)
	}
	for range 60 * 12 {
		Step(w, dt)
	}
	return screenshotScenario{
		name:  "06-working-loop",
		world: w,
	}
}

func campPreviewScenario() screenshotScenario {
	w := screenshotWorld(11)
	w.Nodes = nil
	w.NextNodeID = 0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: math.Pi, Pos: w.Planet.RimPoint(math.Pi)}}
	w.ResourceDiscovered = true
	w.Economy.Wood = CampCost(w)

	angle := 0.0
	for _, a := range []float64{0.15, 0.22, -0.25, 0.35, -0.42, 0.55, -0.65} {
		n := newNode(w, KindWood, a)
		n.Size = 1
		w.Nodes = append(w.Nodes, n)
	}
	claimed := newNode(w, KindWood, 0.72)
	claimed.OwnerID = 42
	w.Nodes = append(w.Nodes, claimed)

	pv := buildPreview(w, angle)
	return screenshotScenario{
		name:    "07-camp-preview",
		world:   w,
		preview: &pv,
	}
}

func invalidFullRimPreviewScenario() screenshotScenario {
	w := screenshotWorld(11)
	w.Nodes = nil
	w.NextNodeID = 0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: math.Pi, Pos: w.Planet.RimPoint(math.Pi)}}
	w.ResourceDiscovered = true
	w.Economy.Wood = CampCost(w)

	for i := 0; i < 48; i++ {
		a := normAngle(float64(i) * math.Pi * 2 / 48)
		n := newNode(w, KindWood, a)
		n.Size = 1.3
		w.Nodes = append(w.Nodes, n)
	}
	pv := buildPreview(w, 0)
	return screenshotScenario{
		name:    "08-invalid-full-rim-preview",
		world:   w,
		preview: &pv,
	}
}

func debugPlacementDiagnosticsScenario() screenshotScenario {
	w := screenshotWorld(11)
	w.Nodes = nil
	w.NextNodeID = 0
	w.Buildings = []*Building{{Kind: KindTownHall, Angle: math.Pi, Pos: w.Planet.RimPoint(math.Pi)}}
	w.ResourceDiscovered = true
	w.Economy.Wood = CampCost(w)
	for _, a := range []float64{0.18, -0.28, 0.38, -0.58} {
		n := newNode(w, KindWood, a)
		n.Size = 1
		w.Nodes = append(w.Nodes, n)
	}
	pv := buildPreview(w, 0)
	return screenshotScenario{
		name:         "09-debug-placement-diagnostics",
		world:        w,
		preview:      &pv,
		fullHUD:      true,
		debug:        true,
		debugSection: debugSectionPlacement,
		placing:      true,
	}
}

func affordabilityButtonsScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlace(w, w.Planet.Fields[0].CenterAngle)
	w.ResourceDiscovered = true
	w.Economy.Wood = 12
	w.Economy.CapacityBought = 3
	w.Economy.WorkerCapacity = 3

	return screenshotScenario{
		name:    "10-affordability-buttons",
		world:   w,
		fullHUD: true,
	}
}

func townHallSelectedScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlace(w, w.Planet.Fields[0].CenterAngle)
	w.ResourceDiscovered = true
	w.Economy.Wood = 80
	w.Economy.CapacityBought = 1
	w.Economy.WorkerCapacity = 2
	thIdx := 0
	return screenshotScenario{
		name:           "11-town-hall-selected",
		world:          w,
		fullHUD:        true,
		selectBuilding: &thIdx,
	}
}

func wideResourceHUDScenario() screenshotScenario {
	w := screenshotWorld(11)
	mustPlace(w, w.Planet.Fields[0].CenterAngle)
	w.ResourceDiscovered = true
	w.Economy.Wood = 1234
	w.Economy.WorkerCapacity = 16
	w.Workers = nil
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
		name:    "11-wide-resource-hud",
		world:   w,
		fullHUD: true,
	}
}

func fieldGrowthSpawnCueScenario() screenshotScenario {
	w := screenshotWorld(11)
	w.ResourceDiscovered = true
	w.Economy.Wood = 42
	field := w.Planet.Fields[0]
	fp := w.Planet.FieldProgress[field.Kind]
	fp.EXP = 0
	fp.Cap = woodFieldBaseEXP * woodFieldEXPGrowth

	before := len(w.Nodes)
	result := spawnNode(w, field)
	if result.Outcome != growthOutcomeSpawnedNode || len(w.Nodes) == before {
		panic("screenshot setup failed to spawn growth-cue node")
	}
	activateGrowthCue(w, result)
	w.growthCue.GaugeRelease = growthGaugeReleaseTime * 0.7
	w.growthCue.GaugeAfterglow = growthGaugeAfterglowTime * 0.85
	w.growthCue.FieldDelay = 0
	w.growthCue.FieldPulse = growthFieldPulseTime * 0.75
	w.growthCue.NodeDelay = 0
	w.growthCue.NodeCue = growthNodeCueTime * 0.9

	return screenshotScenario{
		name:    "12-field-growth-spawn-cue",
		world:   w,
		fullHUD: true,
	}
}

func fieldGrowthUpgradeCueScenario() screenshotScenario {
	w := screenshotWorld(11)
	w.ResourceDiscovered = true
	w.Economy.Wood = 42
	field := w.Planet.Fields[0]
	node := newNode(w, KindWood, field.CenterAngle)
	node.Size = 1.45
	w.Nodes = append(w.Nodes, node)
	activateGrowthCue(w, growthResult{
		Outcome:     growthOutcomeUpgradedNode,
		Kind:        field.Kind,
		CenterAngle: field.CenterAngle,
		HalfArc:     field.HalfArc,
		NodeID:      node.ID,
	})
	w.growthCue.GaugeRelease = growthGaugeReleaseTime * 0.55
	w.growthCue.GaugeAfterglow = growthGaugeAfterglowTime * 0.75
	w.growthCue.FieldDelay = 0
	w.growthCue.FieldPulse = growthFieldPulseTime * 0.65
	w.growthCue.NodeDelay = 0
	w.growthCue.NodeCue = growthNodeCueTime * 0.55

	return screenshotScenario{
		name:    "13-field-growth-upgrade-cue",
		world:   w,
		fullHUD: true,
	}
}

func fieldGrowthPlacementCueScenario() screenshotScenario {
	w := screenshotWorld(11)
	w.ResourceDiscovered = true
	w.Economy.Wood = CampCost(w)
	mustPlace(w, w.Planet.Fields[0].CenterAngle)
	field := w.Planet.Fields[0]
	node := w.Nodes[1]
	activateGrowthCue(w, growthResult{
		Outcome:     growthOutcomeSpawnedNode,
		Kind:        field.Kind,
		CenterAngle: field.CenterAngle,
		HalfArc:     field.HalfArc,
		NodeID:      node.ID,
	})
	w.growthCue.GaugeRelease = growthGaugeReleaseTime * 0.45
	w.growthCue.GaugeAfterglow = growthGaugeAfterglowTime * 0.65
	w.growthCue.FieldDelay = 0
	w.growthCue.FieldPulse = growthFieldPulseTime * 0.9
	w.growthCue.NodeDelay = 0
	w.growthCue.NodeCue = growthNodeCueTime * 0.5

	angle := normAngle(node.Angle + previewArc*0.45)
	pv := buildPreview(w, angle)
	return screenshotScenario{
		name:    "14-field-growth-placement-cue",
		world:   w,
		preview: &pv,
		fullHUD: true,
		placing: true,
	}
}

func buildSystemWorld() *World {
	wood := 50.0
	p := QAPreset{
		Seed: 11, PlaceTownHall: true, Workers: 5, SettleSeconds: 2,
		FillTownCapacity: true, SaturateWoodField: true, Reveal: true,
		Wood: &wood,
	}
	w, err := BuildQAWorld(p)
	if err != nil {
		panic(fmt.Sprintf("buildSystemWorld: %v", err))
	}
	return w
}

func systemViewScenario() screenshotScenario {
	return screenshotScenario{name: "15-system-view", world: buildSystemWorld(), fullHUD: true}
}

func systemViewEchoSelectedScenario() screenshotScenario {
	w := buildSystemWorld()
	w.System.Selected = 1
	return screenshotScenario{name: "16-system-view-echo-selected", world: w, fullHUD: true}
}

func revealMidpointScenario() screenshotScenario {
	w := buildSystemWorld()
	w.System.View = ViewPlanet
	return screenshotScenario{
		name:          "17-reveal-midpoint",
		world:         w,
		fullHUD:       true,
		revealActive:  true,
		revealElapsed: revealPhaseASecs * 0.5,
	}
}

func planetViewReturnButtonScenario() screenshotScenario {
	w := buildSystemWorld()
	w.System.View = ViewPlanet
	return screenshotScenario{name: "18-planet-view-return-button", world: w, fullHUD: true}
}

func mustBuildQAWorld(p QAPreset) *World {
	w, err := BuildQAWorld(p)
	if err != nil {
		panic(fmt.Sprintf("screenshot setup failed: %v", err))
	}
	return w
}

func systemViewEchoAwakenablePotentialScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		SelectPlanet: intPtr(1),
		Wood:         &wood,
	})
	return screenshotScenario{name: "19-system-view-echo-awakenable-potential", world: w, fullHUD: true}
}

func systemViewEchoAwakenedScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		AwakenEchoes: []int{1},
		SelectPlanet: intPtr(1),
		Wood:         &wood,
	})
	return screenshotScenario{name: "20-system-view-echo-awakened", world: w, fullHUD: true}
}

func lakewoodFreshScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		AwakenEchoes: []int{1},
		EnterPlanet:  intPtr(1),
		Wood:         &wood,
	})
	return screenshotScenario{name: "21-lakewood-fresh", world: w, fullHUD: true}
}

func systemViewOneEchoCompletedScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes: []int{1},
		SelectPlanet:   intPtr(1),
		Wood:           &wood,
	})
	return screenshotScenario{name: "22-system-view-one-echo-completed", world: w, fullHUD: true}
}

func systemViewBothEchoesCompletedScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes: []int{1, 2},
		Wood:           &wood,
	})
	return screenshotScenario{name: "23-system-view-both-echoes-completed", world: w, fullHUD: true}
}

func systemViewForestPotentialScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		Wood: &wood,
	})
	return screenshotScenario{name: "24-system-view-forest-potential", world: w, fullHUD: true}
}

func tightGroveFreshScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		AwakenEchoes:      []int{2},
		EnterPlanet:       intPtr(2),
		EchoPlaceTownHall: true,
		Wood:              &wood,
	})
	return screenshotScenario{name: "25-tight-grove-fresh", world: w, fullHUD: true}
}

func lakewoodNearCompleteScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		AwakenEchoes:         []int{1},
		EnterPlanet:          intPtr(1),
		EchoPlaceTownHall:    true,
		EchoFillTownCapacity: true,
		EchoNearSaturate:     true,
		Wood:                 &wood,
	})
	return screenshotScenario{name: "26-lakewood-near-complete", world: w, fullHUD: true}
}

func lakewoodDebugInfluenceScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		AwakenEchoes: []int{1},
		EnterPlanet:  intPtr(1),
		Wood:         &wood,
	})
	return screenshotScenario{name: "28-lakewood-debug-influence", world: w, fullHUD: true, debug: true}
}

func systemViewLakewoodCompletedScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes: []int{1},
		Wood:           &wood,
	})
	return screenshotScenario{name: "27-system-view-lakewood-completed-water-potential", world: w, fullHUD: true}
}

func systemViewUnknownWaterResonanceScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes: []int{1},
		Wood:           &wood,
	})
	// Advance SimTime to maximize frontier shimmer visibility.
	w.SimTime = math.Pi / 3
	return screenshotScenario{name: "28-system-view-unknown-water-resonance", world: w, fullHUD: true}
}

func systemViewFrontierPreAwakenable() screenshotScenario {
	wood := 50.0
	sel := 3
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		SelectPlanet: &sel,
		Wood:         &wood,
	})
	return screenshotScenario{name: "29-system-view-frontier-pre-awakenable", world: w, fullHUD: true}
}

func systemViewFrontierAwakenableScenario() screenshotScenario {
	wood := 50.0
	forestPot := 1
	sel := 3
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes:  []int{1},
		ForestPotential: &forestPot,
		SelectPlanet:    &sel,
		Wood:            &wood,
	})
	return screenshotScenario{name: "30-system-view-frontier-awakenable", world: w, fullHUD: true}
}

func waterFrontierFreshScenario() screenshotScenario {
	wood := 50.0
	enter := 3
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes:    []int{1},
		AwakenFrontier:    true,
		EnterPlanet:       &enter,
		EchoPlaceTownHall: true,
		Wood:              &wood,
	})
	return screenshotScenario{name: "31-water-frontier-fresh", world: w, fullHUD: true}
}

func systemViewFrontierAwakenedScenario() screenshotScenario {
	wood := 50.0
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes: []int{1},
		AwakenFrontier: true,
		SelectPlanet:   intPtr(3),
		Wood:           &wood,
	})
	return screenshotScenario{name: "32-system-view-frontier-awakened", world: w, fullHUD: true}
}

func waterPlanetFirstDockScenario() screenshotScenario {
	wood := 500.0
	enter := 3
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes:    []int{1},
		AwakenFrontier:    true,
		EnterPlanet:       &enter,
		EchoPlaceTownHall: true,
		EchoDocks:         []float64{waterFrontierLakeAngle},
		Wood:              &wood,
	})
	dockIdx := -1
	for i, b := range w.Buildings {
		if b.Kind == KindDock {
			dockIdx = i
			break
		}
	}
	return screenshotScenario{name: "33-water-planet-first-dock", world: w, fullHUD: true, selectBuilding: &dockIdx}
}

func waterPlanetSparklesScenario() screenshotScenario {
	wood := 50.0
	enter := 3
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes:     []int{1},
		AwakenFrontier:     true,
		EnterPlanet:        &enter,
		EchoPlaceTownHall:  true,
		EchoDocks:          []float64{waterFrontierLakeAngle},
		SaturateWaterField: true,
		Wood:               &wood,
	})
	return screenshotScenario{name: "34-water-planet-sparkles", world: w, fullHUD: true}
}

// waterPlanetDockUpgradeSelectedScenario shows the water frontier with a dock
// selected and the upgrade tray visible. The dock is at Level 1 and the player
// has enough resources to upgrade.
func waterPlanetDockUpgradeSelectedScenario() screenshotScenario {
	wood := dockL2WoodCost * 2
	water := dockL2WaterCost * 2
	enter := 3
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes:    []int{1},
		AwakenFrontier:    true,
		EnterPlanet:       &enter,
		EchoPlaceTownHall: true,
		EchoDocks:         []float64{waterFrontierLakeAngle},
		Wood:              &wood,
	})
	w.Economy.Water = water
	dockIdx := -1
	for i, b := range w.Buildings {
		if b.Kind == KindDock {
			dockIdx = i
			break
		}
	}
	return screenshotScenario{name: "35-water-planet-dock-upgrade-selected", world: w, fullHUD: true, selectBuilding: &dockIdx}
}

// waterPlanetNearCompleteNoL2DockScenario shows the water frontier near
// completion: town capacity full, both fields saturated, dock exists but is
// still Level 1.
func waterPlanetNearCompleteNoL2DockScenario() screenshotScenario {
	wood := 50.0
	enter := 3
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes:     []int{1},
		AwakenFrontier:     true,
		EnterPlanet:        &enter,
		EchoPlaceTownHall:  true,
		EchoDocks:          []float64{waterFrontierLakeAngle},
		SaturateWaterField: true,
		Wood:               &wood,
	})
	return screenshotScenario{name: "36-water-planet-near-complete-no-l2-dock", world: w, fullHUD: true}
}

// dockConeVisibilityScenario places a single L1 dock at the bottom-center of the
// water field (most visible position) with no other clutter, specifically to
// verify the cone color and transparency.
func dockConeVisibilityScenario() screenshotScenario {
	enter := 3
	// waterFrontierLakeAngle = Pi/2, the bottom-center of the water arc.
	dockAngle := waterFrontierLakeAngle
	w := mustBuildQAWorld(QAPreset{
		Seed: 11, PlaceTownHall: true, FillTownCapacity: true,
		SaturateWoodField: true, Reveal: true,
		CompleteEchoes:    []int{1},
		AwakenFrontier:    true,
		EnterPlanet:       &enter,
		EchoPlaceTownHall: true,
		EchoDocks:         []float64{dockAngle},
	})
	return screenshotScenario{name: "37-dock-cone-visibility", world: w, fullHUD: false}
}

// workerRatioUIOpenScenario shows the two-resource labor focus control overlaid
// on the water planet with a 1:3 draft split.
// workerRatioUIOpenScenario shows the two-resource labor focus control overlaid
// on the water planet with a 1:3 draft split.
func workerRatioUIOpenScenario() screenshotScenario {
	frontierIdx := 3
	p, err := BuildQAWorld(QAPreset{
		Name:                 "water-planet-worker-ratio",
		AwakenFrontier:       true,
		EnterPlanet:          &frontierIdx,
		EchoPlaceTownHall:    true,
		EchoFillTownCapacity: true,
		SaturateWaterField:   true,
	})
	if err != nil {
		// Return a fallback world so the screenshot harness doesn't crash.
		return screenshotScenario{name: "38-worker-ratio-ui-open", world: NewWorld(), fullHUD: true}
	}
	// Ensure water is discovered and workers are present.
	p.Economy.WaterDiscovered = true
	p.ResourceDiscovered = true
	p.Economy.WorkerCapacity = 10
	for len(p.Workers) < 4 {
		spawnWorkerAtTownHall(p)
	}
	// Set a 3:1 wood/water focus ratio so the HUD overlay is active.
	p.LaborFocus = map[ResourceKind]int{KindWood: 3, KindWater: 1}
	// Settle workers so they pick up their focus kind.
	for range 120 {
		Step(p, dt)
	}
	return screenshotScenario{
		name:             "38-worker-ratio-ui-open",
		world:            p,
		fullHUD:          true,
		showFocusControl: true,
		focusDraftWater:  1,
	}
}

// workerRatioHUDScenario shows the new per-kind worker counts in the HUD (no
// focus control dialog).
func workerRatioHUDScenario() screenshotScenario {
	frontierIdx := 3
	p, err := BuildQAWorld(QAPreset{
		Name:                 "water-planet-worker-ratio",
		AwakenFrontier:       true,
		EnterPlanet:          &frontierIdx,
		EchoPlaceTownHall:    true,
		EchoFillTownCapacity: true,
		SaturateWaterField:   true,
	})
	if err != nil {
		return screenshotScenario{name: "39-worker-ratio-hud", world: NewWorld(), fullHUD: true}
	}
	p.Economy.WaterDiscovered = true
	p.ResourceDiscovered = true
	p.Economy.WorkerCapacity = 10
	for len(p.Workers) < 4 {
		spawnWorkerAtTownHall(p)
	}
	p.LaborFocus = map[ResourceKind]int{KindWood: 3, KindWater: 1}
	for range 120 {
		Step(p, dt)
	}
	return screenshotScenario{
		name:    "39-worker-ratio-hud",
		world:   p,
		fullHUD: true,
	}
}

// waterPlanetCompletedSystemViewScenario shows the system view immediately after
// the water frontier has completed: dual abstract rates (Wood/sec + Water/sec)
// visible in the system-view overlay and both Potential tokens banked.
func waterPlanetCompletedSystemViewScenario() screenshotScenario {
	w, err := BuildQAWorld(QAPreset{
		Name:              "water-planet-completed",
		Seed:              11,
		PlaceTownHall:     true,
		FillTownCapacity:  true,
		SaturateWoodField: true,
		Reveal:            true,
		CompleteEchoes:    []int{1, 2},
		AwakenFrontier:    true,
		CompleteFrontier:  true,
	})
	if err != nil {
		return screenshotScenario{name: "40-water-planet-completed-system-view", world: NewWorld(), fullHUD: true}
	}
	// Stamp illustrative non-zero abstract rates so the system-view rate display
	// is readable in the screenshot (the preset has no live workers, so rates
	// snapshot at zero — this is purely visual).
	w.System.Planets[3].AbstractRate = 1.4
	w.System.Planets[3].AbstractWaterRate = 0.6
	return screenshotScenario{name: "40-water-planet-completed-system-view", world: w, fullHUD: true}
}

func intPtr(v int) *int { return &v }

func screenshotWorld(seed int64) *World {
	return newWorldWithSeed(seed)
}

func mustPlace(w *World, angle float64) {
	if !placeBuilding(w, angle) {
		panic(fmt.Sprintf("screenshot setup failed to place building at %.3f", angle))
	}
}

func mustBuyWorker(w *World) {
	w.Economy.WorkerCapacity++
	if spawnWorkerAtTownHall(w) == nil {
		panic("screenshot setup failed to spawn worker")
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
			if err := drawHUDScreenshot(screen, shot); err != nil {
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

func drawHUDScreenshot(screen *ebiten.Image, shot screenshotScenario) error {
	const scale = 2
	selectedBldID := -1
	if shot.selectBuilding != nil {
		selectedBldID = *shot.selectBuilding
	}
	game := &Game{
		world:              shot.world,
		scene:              ebiten.NewImage(virtW, virtH),
		hudScale:           scale,
		hudDigits:          woodDigits(shot.world.Economy.Wood),
		preview:            shot.preview,
		placing:            shot.placing,
		debug:              shot.debug,
		debugSection:       shot.debugSection,
		selectedBuildingID: selectedBldID,
		revealActive:       shot.revealActive,
		revealElapsed:      shot.revealElapsed,
		showFocusControl:   shot.showFocusControl,
		focusDraftWater:    shot.focusDraftWater,
	}
	hud, ui, err := buildHUD(game, scale)
	if err != nil {
		return err
	}
	game.hud = hud
	game.ui = ui
	game.hud.Refresh(game.world, game.placing, game.debug, game.debugSection, game.preview, false)
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
