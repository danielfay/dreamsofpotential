package game

import (
	"image/color"
	"math"
)

// Wood resource colours — source of truth is the planet HUD idle swatch.
// Use these everywhere a wood swatch or label appears so all UI elements share
// the same visual identity for the wood resource.
var (
	colWoodResource = color.RGBA{R: 40, G: 160, B: 60, A: 255}   // swatch fill — matches planet HUD wood button
	colWoodLabel    = color.RGBA{R: 140, G: 210, B: 140, A: 230} // text labels for wood amounts and rates

	// Potential token colours. Circles (not squares) are the visual convention for Potential.
	colForestPotential      = color.RGBA{R: 40, G: 160, B: 60, A: 255}   // green circle — same identity as wood
	colForestPotentialLabel = color.RGBA{R: 140, G: 210, B: 140, A: 230} // cost numeric label
	colWaterPotential       = color.RGBA{R: 60, G: 140, B: 220, A: 255}  // blue circle — water resonance
	colWaterPotentialLabel  = color.RGBA{R: 140, G: 180, B: 230, A: 230} // cost numeric label for water

	// Awaken button — enabled state (purple burst).
	colAwakenFill  = color.RGBA{R: 60, G: 40, B: 100, A: 240}
	colAwakenRim   = color.RGBA{R: 160, G: 100, B: 220, A: 200}
	colAwakenGlyph = color.RGBA{R: 200, G: 160, B: 255, A: 220}

	// Awaken button — disabled state (near-black, glyph barely visible).
	colAwakenFillDim  = color.RGBA{R: 15, G: 12, B: 22, A: 160}
	colAwakenRimDim   = color.RGBA{R: 35, G: 28, B: 50, A: 80}
	colAwakenGlyphDim = color.RGBA{R: 50, G: 38, B: 65, A: 90}

	// Enter-planet button (system-view tray, right side).
	colEnterFill  = color.RGBA{R: 30, G: 80, B: 50, A: 240}
	colEnterRim   = color.RGBA{R: 80, G: 200, B: 100, A: 200}
	colEnterGlyph = color.RGBA{R: 60, G: 160, B: 80, A: 220}
	colEnterOrbit = color.RGBA{R: 200, G: 240, B: 200, A: 160}

	// Return-to-system button (planet view, top-right).
	colReturnFill  = color.RGBA{R: 15, G: 30, B: 40, A: 220}
	colReturnRim   = color.RGBA{R: 60, G: 100, B: 140, A: 200}
	colReturnGlyph = color.RGBA{R: 60, G: 140, B: 180, A: 220}
	colReturnOrbit = color.RGBA{R: 140, G: 200, B: 220, A: 150}
	colReturnDot   = color.RGBA{R: 140, G: 200, B: 220, A: 120}

	// System-view tray (bottom panel).
	colSysTrayFill       = color.RGBA{R: 8, G: 8, B: 18, A: 210}
	colSysTrayBorder     = color.RGBA{R: 60, G: 60, B: 90, A: 200}
	colSysUnknownSwatch  = color.RGBA{R: 60, G: 60, B: 80, A: 255}   // planet swatch for unknown kind (dormant)
	colSysFrontierSwatch = color.RGBA{R: 40, G: 100, B: 190, A: 255} // planet swatch for awakened water frontier

	// Wood gauge (thin bar below the resource HUD button in planet view).
	colWoodGaugeFill  = color.RGBA{R: 40, G: 160, B: 60, A: 180}
	colWoodGaugeFrame = color.RGBA{R: 80, G: 80, B: 80, A: 180}

	// Growth gauge overlays (alpha set per-frame from animation progress).
	colGrowthGaugeAfterglow = color.RGBA{R: 70, G: 210, B: 90, A: 255}
	colGrowthGaugeRelease   = color.RGBA{R: 140, G: 255, B: 130, A: 255}

	// Water-influenced growth cue tint — blue-green, event-local only.
	colWaterGrowthTint = color.RGBA{R: 60, G: 200, B: 150, A: 255}

	// Interior water sparkle node colours.
	colSparkle        = color.RGBA{R: 80, G: 190, B: 255, A: 240} // free sparkle
	colSparkleClaimed = color.RGBA{R: 30, G: 80, B: 140, A: 130}  // collected — dimmed until delivered

	// Nurture attention pulse and confirm flash (alpha set per-frame).
	colNurtureAttention = color.RGBA{R: 120, G: 255, B: 150, A: 255}
	colNurtureConfirm   = color.RGBA{R: 200, G: 255, B: 210, A: 255}

	// Inject-circle hover rim (white glow, alpha set per-frame via constant).
	colInjectHover = color.RGBA{R: 255, G: 255, B: 255, A: 180}

	// Unaffordable-cost flash — red stroke around the button (alpha set per-frame).
	colCostPulse = color.RGBA{R: 220, G: 60, B: 60, A: 255}

	// Town-hall growth gauge (oriented rect on the rim showing town growth progress).
	colTownGrowthGaugeFrame = color.RGBA{R: 55, G: 55, B: 65, A: 200}
	colTownGrowthGaugeFill  = color.RGBA{R: 220, G: 200, B: 60, A: 200}

	// Worker colours.
	colWorkerEmpty      = color.RGBA{R: 220, G: 200, B: 150, A: 255} // idle/unladen
	colWorkerReturn     = color.RGBA{R: 125, G: 115, B: 95, A: 255}  // returning empty
	colWorkerLaden      = color.RGBA{R: 255, G: 240, B: 80, A: 255}  // carrying wood (worker yellow)
	colWorkerLadenWater = color.RGBA{R: 60, G: 160, B: 255, A: 255}  // carrying water (blue)

	// Building / UI button colours.
	colTownHall = color.RGBA{R: 215, G: 120, B: 45, A: 255} // town hall — warm terracotta
	colBuilding = color.RGBA{R: 100, G: 62, B: 36, A: 255}  // camp — dark brown
	colDock     = color.RGBA{R: 100, G: 62, B: 36, A: 255}  // dock — camp brown
)

const (
	// HUD scale knobs. Adjust these first when a HUD feels too large/small.
	// The planet top HUD is intentionally smaller than the bottom trays so it
	// does not crowd the planet.
	planetTopHUDScale        = 0.75
	systemTopHUDScale        = 1.00
	systemBottomHUDScale     = 1.00
	selectedBuildingHUDScale = 1.00

	// planetTopHUD*Base values are the current unscaled element sizes for the
	// compact planet top HUD. They are multiplied by planetTopHUDScale.
	planetTopHUDIconBase     = 15
	planetTopHUDActionBase   = 21
	planetTopHUDDigitBase    = 10
	planetTopHUDInnerGapBase = 4
	planetTopHUDGroupGapBase = 9
	planetTopHUDPaddingVBase = 6
	planetTopHUDPaddingHBase = 9
	planetTopHUDHeightBase   = 33
	planetTopHUDFontBase     = 16
	planetTopHUDTextScale    = 0.75
	ebitenUIHUDBaseScale     = 0.75

	// Manual screen-space HUD base sizes. These are multiplied by the relevant
	// HUD scale knob and the current view scale.
	systemTopHUDHeightBase     = 34
	systemTopHUDSquareBase     = 8
	systemTopHUDCircleBase     = 4
	systemTopHUDAllocPipBase   = 4
	systemTopHUDPaddingVBase   = 3
	systemTopHUDRowGapBase     = 3
	systemTopHUDColumnGapBase  = 16
	bottomHUDHeightBase        = 20
	bottomHUDSwatchBase        = 10
	bottomHUDSmallIconBase     = 6
	bottomHUDButtonBase        = 14
	bottomHUDContentGapBase    = 8
	bottomHUDRateGapBase       = 6
	bottomHUDCostGapBase       = 6
	bottomHUDTextGapBase       = 2
	bottomHUDCircleBase        = 3
	selectedHUDActionWidthBase = 52
	selectedHUDActionInsetBase = 2
	selectedHUDContentGapBase  = 10
	selectedHUDIdentityGapBase = 5

	// pulseDuration is how long (seconds) the unaffordable-cost flash lasts.
	pulseDuration    = 0.4
	microPulseTime   = 0.30 // seconds for worker/node/building activity pulse
	pulseMinInterval = 0.50 // activations faster than this become steady-lit

	nurtureConfirmDuration = 0.18 // seconds for the nurture success flash

	holdInitialDelay         = 0.50 // seconds before first repeat fires
	holdRepeatInterval       = 0.15 // starting interval between repeats
	holdMinInterval          = 0.03 // fastest repeat interval (reached after ~4 s of holding)
	holdRampRate             = 0.03 // interval reduction per second of hold duration
	nurtureAttentionInterval = 7.0  // seconds between attention pulse fires
	nurtureAttentionPulseDur = 0.70 // duration of each attention pulse (expanding rect needs more time)

	growthGaugeReleaseTime   = 0.36
	growthGaugeAfterglowTime = 0.72
	growthFieldPulseDelay    = 0.12
	growthFieldPulseTime     = 0.82
	growthNodeCueDelay       = 0.22
	growthNodeCueTime        = 0.48

	// reveal animation phases (seconds)
	revealPhaseASecs   = 1.0   // planet pulse + pull phase before system appears
	revealWaveSpeedPxS = 200.0 // pixels/second the reveal wave expands

	// Completion atmosphere — wide coloured glow drawn behind the planet on completion.
	atmosphereIntroDur   = 3.0  // seconds for the atmosphere to expand into place
	atmosphereBreathFreq = 0.35 // rad/s — one full breath every ~18 s
)

func scaledHUDInt(viewScale int, hudScale float64, base int) int {
	v := int(math.Round(float64(base) * float64(viewScale) * ebitenUIHUDBaseScale * hudScale))
	if v < 1 {
		return 1
	}
	return v
}

func scaledHUDFloat(viewScale, hudScale float64, base float64) float32 {
	v := float32(base * viewScale * hudScale)
	if v < 1 {
		return 1
	}
	return v
}

var (
	// Per-planet atmosphere tint colours. The alpha is set per-layer at draw time.
	colAtmosphereStart = color.RGBA{R: 55, G: 200, B: 80, A: 255}  // starting planet — mid green
	colAtmosphereA     = color.RGBA{R: 40, G: 190, B: 100, A: 255} // echo layout 0 — cool blue-green
	colAtmosphereB     = color.RGBA{R: 110, G: 210, B: 45, A: 255} // echo layout 1 — warm yellow-green
	colAtmosphereWater = color.RGBA{R: 50, G: 160, B: 230, A: 255} // water frontier — azure blue
)

var (
	colWaterGaugeFill  = color.RGBA{R: 60, G: 160, B: 230, A: 180}
	colWaterGaugeFrame = colWoodGaugeFrame
)
