package game

import (
	"image/color"
	"math"

	colorful "github.com/lucasb-eyer/go-colorful"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// system-view palette
var (
	colSysBackground     = color.RGBA{R: 4, G: 4, B: 12, A: 255}
	colSysStar           = color.RGBA{R: 140, G: 140, B: 160, A: 200}
	colSysStarting       = color.RGBA{R: 30, G: 105, B: 50, A: 255}   // deep forest green — awakened planet
	colSysStartRim       = color.RGBA{R: 100, G: 215, B: 115, A: 255} // bright active green rim
	colSysEchoA          = color.RGBA{R: 40, G: 140, B: 60, A: 255}   // dim green — dormant echo A
	colSysEchoB          = color.RGBA{R: 35, G: 120, B: 55, A: 255}   // dim green — dormant echo B
	colSysEchoRimA       = color.RGBA{R: 80, G: 210, B: 145, A: 200}  // muted cool blue-green rim — layout 0
	colSysEchoRimB       = color.RGBA{R: 155, G: 220, B: 70, A: 200}  // muted warm yellow-green rim — layout 1
	colSysEchoActiveRimA = color.RGBA{R: 80, G: 215, B: 145, A: 255}  // bright cool blue-green — awakened/completed layout 0
	colSysEchoActiveRimB = color.RGBA{R: 155, G: 225, B: 70, A: 255}  // bright warm yellow-green — awakened/completed layout 1
	colSysUnknown        = color.RGBA{R: 28, G: 28, B: 38, A: 255}    // dark silhouette
	colSysUnknownRim     = color.RGBA{R: 50, G: 50, B: 70, A: 180}    // faint orbit tint
	colSysFrontierBody   = color.RGBA{R: 20, G: 55, B: 115, A: 255}   // deep blue — awakened water frontier
	colSysFrontierRim    = color.RGBA{R: 60, G: 140, B: 230, A: 255}  // bright blue rim
	colSysOrbit          = color.RGBA{R: 40, G: 40, B: 60, A: 80}     // faint orbit ellipse
	colSysSelect         = color.RGBA{R: 255, G: 240, B: 130, A: 255} // gold selection ring
	colRevealPulse       = color.RGBA{R: 240, G: 210, B: 80, A: 255}  // warm gold reveal pulse
	colRevealEdge        = color.RGBA{R: 120, G: 240, B: 130, A: 200} // green pulse edge
)

// starPositions returns deterministic dim star positions for the system background.
func starPositions() [][2]float32 {
	stars := make([][2]float32, 40)
	// Use golden-ratio distribution so stars look natural without random.Seed side-effects.
	const phi = 2.399
	for i := range stars {
		af := math.Mod(float64(i)*phi, math.Pi*2)
		rf := math.Sqrt(math.Mod(float64(i)*0.617, 1))
		stars[i][0] = float32(160 + 155*rf*math.Cos(af))
		stars[i][1] = float32(120 + 115*rf*math.Sin(af))
	}
	return stars
}

// drawSystemView renders the system-layer view onto scene.
func drawSystemView(scene *ebiten.Image, w *World, debug bool) {
	scene.Fill(colSysBackground)

	// Sparse star background.
	for _, s := range starPositions() {
		vector.FillRect(scene, s[0]-0.5, s[1]-0.5, 1, 1, colSysStar, false)
	}

	// Faint orbit ellipse for the unknown planet.
	if len(w.System.Planets) > 3 {
		unk := w.System.Planets[3]
		cx, cy := float32(unk.Pos.X), float32(unk.Pos.Y)
		drawSystemOrbitRing(scene, cx, cy, float32(unk.Radius)+8, 1.5, colSysOrbit)
	}

	// Draw planets (back to front: unknown, echoes, starting).
	drawOrder := []int{3, 2, 1, 0}
	for _, i := range drawOrder {
		if i >= len(w.System.Planets) {
			continue
		}
		p := w.System.Planets[i]
		drawSystemPlanet(scene, w, p, w.System.Selected == i, w.SimTime, 1.0, debug)
	}
}

// drawSystemPlanet renders one system-view planet disk with its rim ring.
// brightness is [0,1] and is used by the reveal flicker effect.
func drawSystemPlanet(scene *ebiten.Image, w *World, p SystemPlanet, selected bool, simTime float64, brightness float32, debug bool) {
	cx, cy := float32(p.Pos.X), float32(p.Pos.Y)
	r := float32(p.Radius)

	switch p.Kind {
	case PlanetStarting:
		body := scaleColor(colSysStarting, brightness)
		vector.FillCircle(scene, cx, cy, r, body, false)
		rimCol := scaleColor(colSysStartRim, brightness)
		drawSystemOrbitRing(scene, cx, cy, r, 2.5, rimCol)
		// Subtle afterglow on the awakened planet.
		glowAlpha := uint8(float32(18) * brightness)
		vector.FillCircle(scene, cx, cy, r+3, color.RGBA{R: 50, G: 180, B: 70, A: glowAlpha}, false)

	case PlanetEcho:
		col := colSysEchoA
		echoRim := colSysEchoRimA
		echoActiveRim := colSysEchoActiveRimA
		if p.RingColorIdx == 1 {
			col = colSysEchoB
			echoRim = colSysEchoRimB
			echoActiveRim = colSysEchoActiveRimB
		}
		body := scaleColor(col, brightness)
		vector.FillCircle(scene, cx, cy, r, body, false)
		switch {
		case p.Completed:
			glowAlpha := uint8(float32(18) * brightness)
			vector.FillCircle(scene, cx, cy, r+3, color.RGBA{R: echoActiveRim.R, G: echoActiveRim.G, B: echoActiveRim.B, A: glowAlpha}, false)
			rimCol := scaleColor(echoActiveRim, brightness)
			drawSystemOrbitRing(scene, cx, cy, r, 2.5, rimCol)
		case p.Awakened:
			// Active but incomplete: solid bright rim, no twinkle.
			rimCol := scaleColor(echoActiveRim, brightness)
			drawSystemOrbitRing(scene, cx, cy, r, 2.0, rimCol)
		default:
			// Dormant: twinkling rim in the layout's colour family.
			twinkle := float32(0.7 + 0.3*math.Sin(simTime*1.8+float64(p.RingColorIdx)*1.2))
			rimAlpha := uint8(float32(160) * twinkle * brightness)
			drawSystemOrbitRing(scene, cx, cy, r, 1.5, color.RGBA{R: echoRim.R, G: echoRim.G, B: echoRim.B, A: rimAlpha})
		}

	case PlanetUnknown:
		if p.Awakened {
			body := scaleColor(colSysFrontierBody, brightness)
			vector.FillCircle(scene, cx, cy, r, body, false)
			rimCol := scaleColor(colSysFrontierRim, brightness)
			drawSystemOrbitRing(scene, cx, cy, r, 2.0, rimCol)
			glowAlpha := uint8(float32(18) * brightness)
			vector.FillCircle(scene, cx, cy, r+3, color.RGBA{R: colSysFrontierRim.R, G: colSysFrontierRim.G, B: colSysFrontierRim.B, A: glowAlpha}, false)
		} else {
			vector.FillCircle(scene, cx, cy, r, colSysUnknown, false)
			vector.FillCircle(scene, cx, cy, r, colSysUnknownRim, false)
			// Latent water requirement → blue-leaning frontier shimmer.
			if p.AwakenReqWater > 0 {
				shimmer := float32(0.5 + 0.5*math.Sin(simTime*1.5))
				shimAlpha := uint8(float32(28) * shimmer * brightness)
				if shimAlpha > 0 {
					vector.FillCircle(scene, cx, cy, r+1, color.RGBA{R: 120, G: 160, B: 240, A: shimAlpha}, false)
				}
			}
		}
	}

	if selected {
		drawSystemOrbitRing(scene, cx, cy, r+3, 2, colSysSelect)
	}
}

// scaleColor dims a colour toward black by brightness (0=black, 1=unchanged),
// blending in Lab space for perceptually even darkening.
func scaleColor(col color.RGBA, brightness float32) color.RGBA {
	if brightness >= 1 {
		return col
	}
	if brightness <= 0 {
		return color.RGBA{A: col.A}
	}
	c, _ := colorful.MakeColor(color.RGBA{R: col.R, G: col.G, B: col.B, A: 255})
	result := c.BlendLab(colorful.Color{}, float64(1-brightness)).Clamped()
	r8, g8, b8 := result.RGB255()
	return color.RGBA{R: r8, G: g8, B: b8, A: col.A}
}

func drawSystemOrbitRing(scene *ebiten.Image, cx, cy, radius, width float32, col color.RGBA) {
	const steps = 32
	var path vector.Path
	for i := 0; i <= steps; i++ {
		angle := float64(i) * math.Pi * 2 / steps
		x := cx + radius*float32(math.Cos(angle))
		y := cy + radius*float32(math.Sin(angle))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	sop := &vector.StrokeOptions{Width: width}
	drawOp := &vector.DrawPathOptions{}
	drawOp.ColorScale.ScaleWithColor(col)
	vector.StrokePath(scene, &path, sop, drawOp)
}

func (g *Game) drawSystemChannelOverlay(screen *ebiten.Image) {
	for i, ch := range g.world.System.Channels {
		state := channelState(g.world, ch, 1.0)
		if state.fam == nil || state.rate <= 0 {
			continue
		}
		src := g.world.System.Planets[ch.Source]
		tgt := g.world.System.Planets[ch.Target]
		sx, sy := g.worldToScreen(src.Pos)
		tx, ty := g.worldToScreen(tgt.Pos)
		dx := tx - sx
		dy := ty - sy
		dist := float32(math.Hypot(float64(dx), float64(dy)))
		if dist <= 0.001 {
			continue
		}
		ux := dx / dist
		uy := dy / dist
		scale, _, _ := viewGeom(g.screenW, g.screenH)
		startInset := float32(src.Radius * scale)
		endInset := float32(tgt.Radius * scale)
		x0 := sx + ux*startInset
		y0 := sy + uy*startInset
		x1 := tx - ux*endInset
		y1 := ty - uy*endInset
		lx := (x0 + x1) / 2
		ly := (y0 + y1) / 2
		length := float32(math.Hypot(float64(x1-x0), float64(y1-y0)))
		col := state.fam.PotentialColor
		width := float32(channelEmptyLineWidth)
		col.A = channelEmptyLineAlpha
		if state.stocked {
			width = float32(channelStockedLineWidth)
			col.A = channelStockedLineAlpha
		}
		if !tgt.Awakened {
			col = blendColor(col, color.RGBA{R: 255, G: 255, B: 255, A: col.A}, channelDormantBlend)
		}
		drawOrientedRect(screen, lx, ly, ux, uy, -uy, ux, length/2, width/2, col)

		if state.valid {
			dotCount := 1
			if state.stocked {
				dotCount = 2
			}
			for j := 0; j < dotCount; j++ {
				t := math.Mod(g.world.SimTime*channelFlowSpeed+float64(i)*0.19+float64(j)*0.47, 1)
				px := x0 + (x1-x0)*float32(t)
				py := y0 + (y1-y0)*float32(t)
				dotCol := brighten(state.fam.PotentialColor, 80)
				if state.stocked {
					dotCol.A = channelFlowDotStockedAlpha
				} else {
					dotCol.A = channelFlowDotEmptyAlpha
				}
				vector.FillCircle(screen, px, py, channelFlowDotRadius, dotCol, false)
			}
		}
	}

	if !g.pendingChannelActive {
		return
	}
	source := g.world.System.Selected
	if source < 0 || source >= len(g.world.System.Planets) {
		return
	}
	fam := familyForResource(g.pendingChannelResource)
	if fam == nil {
		return
	}
	pulse := float32(0.5 + 0.5*math.Sin(g.world.SimTime*validTargetPulseHz*math.Pi*2))
	current := findChannel(g.world, source, fam.Resource)
	for i, p := range g.world.System.Planets {
		if !canAssignChannel(g.world, source, fam.Resource, i) {
			continue
		}
		cx, cy := g.worldToScreen(p.Pos)
		scale, _, _ := viewGeom(g.screenW, g.screenH)
		radius := float32(p.Radius*scale) + validTargetRingInset + pulse*validTargetRingPulse
		col := fam.PotentialColor
		col.A = uint8(float32(validTargetRingAlpha) * (0.6 + 0.4*pulse))
		width := float32(validTargetRingWidth)
		if current != nil && current.Target == i {
			col = colSysSelect
			col.A = 255
			width = float32(validTargetRingWidth + 0.75)
		}
		drawSystemOrbitRing(screen, cx, cy, radius, width, col)
	}
}

// ── Reveal animation ──────────────────────────────────────────────────────────

// drawReveal renders the one-time unlock reveal animation onto scene.
// Phase A (0..revealPhaseASecs): planet dims with a completion pulse.
// Phase B (revealPhaseASecs..revealDuration): system view appears with wave.
func drawReveal(scene *ebiten.Image, w *World, elapsed float64) {
	if elapsed < revealPhaseASecs {
		drawRevealPhaseA(scene, w, elapsed)
	} else {
		drawRevealPhaseB(scene, w, elapsed-revealPhaseASecs)
	}
}

func drawRevealPhaseA(scene *ebiten.Image, w *World, elapsed float64) {
	DrawWorld(scene, w, nil, false)
	t := float32(elapsed / revealPhaseASecs) // 0 → 1
	t = clamp32(t, 0, 1)

	// Progressive dim overlay.
	alpha := uint8(float32(160) * smoothStep(t))
	if alpha > 0 {
		scene.Fill(color.RGBA{R: 0, G: 0, B: 0, A: alpha})
	}

	// Completion pulse — warm gold expanding ring from planet center.
	cx, cy := float32(w.Planet.Center.X), float32(w.Planet.Center.Y)
	pRadius := float32(w.Planet.Radius) * (0.3 + 1.4*smoothStep(t))
	pulseAlpha := uint8(float32(200) * (1 - t))
	if pulseAlpha > 0 {
		drawSystemOrbitRing(scene, cx, cy, pRadius, 3, color.RGBA{R: colRevealPulse.R, G: colRevealPulse.G, B: colRevealPulse.B, A: pulseAlpha})
		edgeAlpha := uint8(float32(120) * (1 - t))
		drawSystemOrbitRing(scene, cx, cy, pRadius+4, 1.5, color.RGBA{R: colRevealEdge.R, G: colRevealEdge.G, B: colRevealEdge.B, A: edgeAlpha})
	}
}

func drawRevealPhaseB(scene *ebiten.Image, w *World, phaseElapsed float64) {
	phaseDur := revealDuration - revealPhaseASecs
	t := clamp32(float32(phaseElapsed/phaseDur), 0, 1) // 0 → 1 over Phase B

	// Compute wave radius.
	waveRadius := float32(phaseElapsed * revealWaveSpeedPxS)

	// Per-echo flicker state: brightness based on whether wave has passed.
	echoGlow := [2]float32{0, 0}
	if len(w.System.Planets) >= 3 {
		for ei := 0; ei < 2; ei++ {
			ep := w.System.Planets[1+ei]
			dist := float32(w.System.Planets[0].Pos.Dist(ep.Pos))
			if waveRadius > dist {
				// Flicker: on/off/on/off then steady. Use square-wave on (wave - dist) time.
				flickerTime := float32(phaseElapsed) - dist/float32(revealWaveSpeedPxS)
				flickerPhase := flickerTime * 8.0 // ~8 flickers per second
				flicker := float32(1)
				iPhase := int(flickerPhase)
				if iPhase < 4 && iPhase%2 == 1 {
					flicker = 0.2 // dim flicker
				}
				// Settle after 4 flickers.
				if flickerTime > 0.5 {
					flicker = 1.0
				}
				echoGlow[ei] = clamp32(flicker, 0, 1)
			}
		}
	}

	// Unknown shimmer state.
	unknownShimmer := float32(0)
	if len(w.System.Planets) >= 4 {
		unk := w.System.Planets[3]
		dist := float32(w.System.Planets[0].Pos.Dist(unk.Pos))
		if waveRadius > dist {
			shimmerTime := float32(phaseElapsed) - dist/float32(revealWaveSpeedPxS)
			// Single faint shimmer that fades out quickly.
			unknownShimmer = clamp32(1-shimmerTime*3, 0, 0.4)
		}
	}

	// Draw system view base.
	scene.Fill(colSysBackground)
	for _, s := range starPositions() {
		vector.FillRect(scene, s[0]-0.5, s[1]-0.5, 1, 1, colSysStar, false)
	}

	// Unknown with shimmer.
	if len(w.System.Planets) >= 4 {
		unk := w.System.Planets[3]
		cx, cy := float32(unk.Pos.X), float32(unk.Pos.Y)
		drawSystemOrbitRing(scene, cx, cy, float32(unk.Radius)+8, 1.5, colSysOrbit)
		vector.FillCircle(scene, cx, cy, float32(unk.Radius), colSysUnknown, false)
		if unknownShimmer > 0 {
			shimAlpha := uint8(float32(80) * unknownShimmer)
			vector.FillCircle(scene, cx, cy, float32(unk.Radius)+2, color.RGBA{R: 200, G: 200, B: 220, A: shimAlpha}, false)
		}
	}

	// Echoes with flicker.
	for ei := 0; ei < 2; ei++ {
		if 1+ei >= len(w.System.Planets) {
			break
		}
		drawSystemPlanet(scene, w, w.System.Planets[1+ei], false, w.SimTime, echoGlow[ei], false)
	}

	// Starting planet: bright, selected.
	if len(w.System.Planets) > 0 {
		drawSystemPlanet(scene, w, w.System.Planets[0], true, w.SimTime, 1.0, false)
	}

	// Expanding wave ring.
	if waveRadius > 0 && len(w.System.Planets) > 0 {
		sp := w.System.Planets[0]
		waveAlpha := uint8(float32(160) * clamp32(1-t*1.2, 0, 1))
		if waveAlpha > 0 {
			drawSystemOrbitRing(scene,
				float32(sp.Pos.X), float32(sp.Pos.Y),
				waveRadius, 2,
				color.RGBA{R: colRevealPulse.R, G: colRevealPulse.G, B: colRevealPulse.B, A: waveAlpha})
		}
	}

	// Fade in system UI overlay signal at end of Phase B.
	_ = t // available for future HUD fade-in if needed
}
