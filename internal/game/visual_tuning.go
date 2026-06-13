package game

import "image/color"

// Wood resource colours — source of truth is the planet HUD idle swatch.
// Use these everywhere a wood swatch or label appears so all UI elements share
// the same visual identity for the wood resource.
var (
	colWoodResource = color.RGBA{R: 40, G: 160, B: 60, A: 255}  // swatch fill — matches planet HUD wood button
	colWoodLabel    = color.RGBA{R: 140, G: 210, B: 140, A: 230} // text labels for wood amounts and rates

	// Potential token colours. Circles (not squares) are the visual convention for Potential.
	colForestPotential      = color.RGBA{R: 40, G: 160, B: 60, A: 255}  // green circle — same identity as wood
	colForestPotentialLabel = color.RGBA{R: 140, G: 210, B: 140, A: 230} // cost numeric label
	colWaterPotential       = color.RGBA{R: 60, G: 140, B: 220, A: 255}  // blue circle — water resonance

	// Lake terrain fill — muted blue, distinct from forest and sky.
	colLake = color.RGBA{R: 28, G: 72, B: 155, A: 130}

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
	colSysTrayFill        = color.RGBA{R: 8, G: 8, B: 18, A: 210}
	colSysTrayBorder      = color.RGBA{R: 60, G: 60, B: 90, A: 200}
	colSysUnknownSwatch   = color.RGBA{R: 60, G: 60, B: 80, A: 255} // planet swatch for unknown kind

	// Wood gauge (thin bar below the resource HUD button in planet view).
	colWoodGaugeFill  = color.RGBA{R: 40, G: 160, B: 60, A: 180}
	colWoodGaugeFrame = color.RGBA{R: 80, G: 80, B: 80, A: 180}

	// Growth gauge overlays (alpha set per-frame from animation progress).
	colGrowthGaugeAfterglow = color.RGBA{R: 70, G: 210, B: 90, A: 255}
	colGrowthGaugeRelease   = color.RGBA{R: 140, G: 255, B: 130, A: 255}

	// Nurture attention pulse and confirm flash (alpha set per-frame).
	colNurtureAttention = color.RGBA{R: 120, G: 255, B: 150, A: 255}
	colNurtureConfirm   = color.RGBA{R: 200, G: 255, B: 210, A: 255}

	// Unaffordable-cost flash — red stroke around the button (alpha set per-frame).
	colCostPulse = color.RGBA{R: 220, G: 60, B: 60, A: 255}

	// Town-hall growth gauge (oriented rect on the rim showing town growth progress).
	colTownGrowthGaugeFrame = color.RGBA{R: 55, G: 55, B: 65, A: 200}
	colTownGrowthGaugeFill  = color.RGBA{R: 220, G: 200, B: 60, A: 200}

	// Worker colours.
	colWorkerEmpty  = color.RGBA{R: 220, G: 200, B: 150, A: 255} // idle/unladen
	colWorkerReturn = color.RGBA{R: 125, G: 115, B: 95, A: 255}  // returning empty
	colWorkerLaden  = color.RGBA{R: 255, G: 240, B: 80, A: 255}  // carrying wood (worker yellow)

	// Building / UI button colours.
	colTownHall     = color.RGBA{R: 215, G: 120, B: 45, A: 255} // town hall — warm terracotta
	colBuilding     = color.RGBA{R: 100, G: 62, B: 36, A: 255}  // camp — dark brown
	colTownCapacity = color.RGBA{R: 205, G: 105, B: 48, A: 255} // town capacity button — lighter orange
)

const (
	// pulseDuration is how long (seconds) the unaffordable-cost flash lasts.
	pulseDuration    = 0.4
	microPulseTime   = 0.30 // seconds for worker/node/building activity pulse
	pulseMinInterval = 0.50 // activations faster than this become steady-lit

	nurtureConfirmDuration   = 0.18 // seconds for the nurture success flash

	holdInitialDelay   = 0.50  // seconds before first repeat fires
	holdRepeatInterval = 0.15  // starting interval between repeats
	holdMinInterval    = 0.03  // fastest repeat interval (reached after ~4 s of holding)
	holdRampRate       = 0.03  // interval reduction per second of hold duration
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
	atmosphereIntroDur  = 3.0  // seconds for the atmosphere to expand into place
	atmosphereBreathFreq = 0.35 // rad/s — one full breath every ~18 s
)

var (
	// Per-planet atmosphere tint colours. The alpha is set per-layer at draw time.
	colAtmosphereStart = color.RGBA{R: 55, G: 200, B: 80, A: 255}  // starting planet — mid green
	colAtmosphereA     = color.RGBA{R: 40, G: 190, B: 100, A: 255} // echo layout 0 — cool blue-green
	colAtmosphereB     = color.RGBA{R: 110, G: 210, B: 45, A: 255} // echo layout 1 — warm yellow-green
)
