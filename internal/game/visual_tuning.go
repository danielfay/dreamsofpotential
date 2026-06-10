package game

const (
	// pulseDuration is how long (seconds) the unaffordable-cost flash lasts.
	pulseDuration    = 0.4
	microPulseTime   = 0.30 // seconds for worker/node/building activity pulse
	pulseMinInterval = 0.50 // activations faster than this become steady-lit

	growthGaugeReleaseTime   = 0.36
	growthGaugeAfterglowTime = 0.72
	growthFieldPulseDelay    = 0.12
	growthFieldPulseTime     = 0.82
	growthNodeCueDelay       = 0.22
	growthNodeCueTime        = 0.48
)
