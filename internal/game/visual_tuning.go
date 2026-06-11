package game

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
