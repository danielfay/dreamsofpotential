package game

import "image/color"

// Wood resource colours — source of truth is the planet HUD idle swatch.
// Use these everywhere a wood swatch or label appears so all UI elements share
// the same visual identity for the wood resource.
var (
	colWoodResource = color.RGBA{R: 40, G: 160, B: 60, A: 255}  // swatch fill — matches planet HUD wood button
	colWoodLabel    = color.RGBA{R: 140, G: 210, B: 140, A: 230} // text labels for wood amounts and rates

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
	nurtureAttentionPulseDur = 0.35 // duration of each attention flash

	growthGaugeReleaseTime   = 0.36
	growthGaugeAfterglowTime = 0.72
	growthFieldPulseDelay    = 0.12
	growthFieldPulseTime     = 0.82
	growthNodeCueDelay       = 0.22
	growthNodeCueTime        = 0.48

	// reveal animation phases (seconds)
	revealPhaseASecs   = 1.0   // planet pulse + pull phase before system appears
	revealWaveSpeedPxS = 200.0 // pixels/second the reveal wave expands
)
