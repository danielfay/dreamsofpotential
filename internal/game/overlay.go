package game

import (
	"image/color"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// drawOverlay draws HUD affordances on top of EbitenUI in native screen space.
// Widget Rects are valid here because ui.Draw already laid them out.
func (g *Game) drawOverlay(screen *ebiten.Image) {
	if g.showMenu || g.debug {
		return
	}

	if g.revealActive {
		return
	}

	if g.world.System.View == ViewSystem {
		g.drawSystemOverlay(screen)
		return
	}

	if g.world.System.Unlocked {
		g.drawReturnToSystemButton(screen)
	}

	g.drawAffordabilityProgress(screen)

	if g.world.ResourceDiscovered && len(g.world.Planet.Fields) > 0 {
		f := g.world.Planet.Fields[0]
		frac := float32(0)
		if fp := g.world.Planet.FieldProgress[f.Kind]; fp != nil && fp.Cap > 0 {
			frac = float32(fp.EXP / fp.Cap)
			if frac > 1 {
				frac = 1
			}
		}
		r := g.hud.resourceHUD.GetWidget().Rect
		x := float32(r.Min.X)
		y := float32(r.Max.Y) + 2
		w := float32(r.Max.X - r.Min.X)
		const h = float32(3)
		vector.StrokeRect(screen, x, y, w, h, 1, colWoodGaugeFrame, false)
		if frac > 0 {
			vector.FillRect(screen, x, y, w*frac, h, colWoodGaugeFill, false)
		}
		if g.world.growthCue.Kind == KindWood && g.world.growthCue.GaugeAfterglow > 0 {
			t := g.world.growthCue.GaugeAfterglow / growthGaugeAfterglowTime
			col, _ := growthGaugeFlashColors(KindWood)
			col.A = uint8(120 * t)
			vector.FillRect(screen, x, y, w, h, col, false)
		}
		if g.world.growthCue.Kind == KindWood && g.world.growthCue.GaugeRelease > 0 {
			t := g.world.growthCue.GaugeRelease / growthGaugeReleaseTime
			_, col := growthGaugeFlashColors(KindWood)
			col.A = uint8(210 * t)
			vector.StrokeRect(screen, x-1, y-1, w+2, h+2, 1, col, false)
		}

		if g.world.Economy.WaterDiscovered {
			var waterFrac float32
			if fp := g.world.Planet.FieldProgress[KindWater]; fp != nil && fp.Cap > 0 {
				waterFrac = float32(fp.EXP / fp.Cap)
				if waterFrac > 1 {
					waterFrac = 1
				}
			}
			wr := g.hud.waterHUD.GetWidget().Rect
			wx := float32(wr.Min.X)
			wy := float32(wr.Max.Y) + 2
			ww := float32(wr.Max.X - wr.Min.X)
			vector.StrokeRect(screen, wx, wy, ww, h, 1, colWaterGaugeFrame, false)
			if waterFrac > 0 {
				vector.FillRect(screen, wx, wy, ww*waterFrac, h, colWaterGaugeFill, false)
			}
			if g.world.growthCue.Kind == KindWater && g.world.growthCue.GaugeAfterglow > 0 {
				t := g.world.growthCue.GaugeAfterglow / growthGaugeAfterglowTime
				col, _ := growthGaugeFlashColors(KindWater)
				col.A = uint8(120 * t)
				vector.FillRect(screen, wx, wy, ww, h, col, false)
			}
			if g.world.growthCue.Kind == KindWater && g.world.growthCue.GaugeRelease > 0 {
				t := g.world.growthCue.GaugeRelease / growthGaugeReleaseTime
				_, col := growthGaugeFlashColors(KindWater)
				col.A = uint8(210 * t)
				vector.StrokeRect(screen, wx-1, wy-1, ww+2, h+2, 1, col, false)
			}
		}

		if activeIncomingChannelForResource(g.world, KindWood) {
			glow := brighten(colWoodResource, 72)
			glow.A = 48
			vector.FillRect(screen, x, float32(r.Min.Y), w, float32(r.Dy()), glow, false)
		}
		if g.world.Economy.WaterDiscovered && activeIncomingChannelForResource(g.world, KindWater) {
			wr := g.hud.waterHUD.GetWidget().Rect
			wx := float32(wr.Min.X)
			ww := float32(wr.Dx())
			glow := brighten(colSparkle, 72)
			glow.A = 48
			vector.FillRect(screen, wx, float32(wr.Min.Y), ww, float32(wr.Dy()), glow, false)
		}

		if len(g.world.Workers) > 0 && g.world.Economy.TownGrowthCap > 0 && g.hud.workerHUD != nil {
			popFrac := float32(g.world.Economy.TownGrowth / g.world.Economy.TownGrowthCap)
			if popFrac > 1 {
				popFrac = 1
			}
			pr := g.hud.workerHUD.GetWidget().Rect
			px := float32(pr.Min.X)
			py := float32(pr.Max.Y) + 2
			pw := float32(pr.Max.X - pr.Min.X)
			vector.StrokeRect(screen, px, py, pw, h, 1, colTownGrowthGaugeFrame, false)
			if popFrac > 0 {
				vector.FillRect(screen, px, py, pw*popFrac, h, colTownGrowthGaugeFill, false)
			}
		}

		sr := g.hud.resourceSquare.GetWidget().Rect
		srx := float32(sr.Min.X)
		sry := float32(sr.Min.Y)
		srw := float32(sr.Max.X - sr.Min.X)
		srh := float32(sr.Max.Y - sr.Min.Y)
		if g.nurtureAttentionPulseLeft > 0 {
			drawAttentionRipple(screen, srx+srw/2, sry+srh/2, srw, srh,
				g.nurtureAttentionPulseLeft, nurtureAttentionPulseDur, colNurtureAttention, 0)
		}
		if g.nurtureToggleActive {
			col := colNurtureConfirm
			col.A = 40
			vector.FillRect(screen, srx, sry, srw, srh, col, false)
		}
		if g.nurtureConfirmLeft > 0 {
			t := float32(g.nurtureConfirmLeft / nurtureConfirmDuration)
			col := colNurtureConfirm
			col.A = uint8(210 * t)
			vector.FillRect(screen, srx, sry, srw, srh, col, false)
		}

		if g.workerRatioAttentionLeft > 0 && g.hud.workerSquare != nil {
			wr := g.hud.workerSquare.GetWidget().Rect
			wrx := float32(wr.Min.X)
			wry := float32(wr.Min.Y)
			wrw := float32(wr.Max.X - wr.Min.X)
			wrh := float32(wr.Max.Y - wr.Min.Y)
			drawAttentionRipple(screen, wrx+wrw/2, wry+wrh/2, wrw, wrh,
				g.workerRatioAttentionLeft, nurtureAttentionPulseDur, colNurtureAttention, 0)
		}
	}

	g.drawDockTray(screen)
	g.drawWorkerHUDOverlay(screen)
	g.drawFocusControl(screen)

	if g.pulseTime > 0 {
		colPulse := colCostPulse
		colPulse.A = uint8(96 * g.pulseTime / pulseDuration)
		if g.pulseTarget&costPulseWood != 0 && g.hud.resourceHUD != nil {
			pr := g.hud.resourceHUD.GetWidget().Rect
			vector.FillRect(screen,
				float32(pr.Min.X), float32(pr.Min.Y),
				float32(pr.Max.X-pr.Min.X), float32(pr.Max.Y-pr.Min.Y),
				colPulse, false)
		}
		if g.pulseTarget&costPulseWater != 0 && g.hud.waterHUD != nil {
			pr := g.hud.waterHUD.GetWidget().Rect
			vector.FillRect(screen,
				float32(pr.Min.X), float32(pr.Min.Y),
				float32(pr.Max.X-pr.Min.X), float32(pr.Max.Y-pr.Min.Y),
				colPulse, false)
		}
		if g.pulseTarget&costPulseNurture != 0 {
			pr := g.hud.resourceSquare.GetWidget().Rect
			vector.FillRect(screen,
				float32(pr.Min.X), float32(pr.Min.Y),
				float32(pr.Max.X-pr.Min.X), float32(pr.Max.Y-pr.Min.Y),
				colPulse, false)
		}
	}
}

// drawAffordabilityProgress was used to fill disabled HUD buttons from bottom to top.
// Camp and capacity buttons have moved to the world (auto-placement) and TH tray respectively,
// so this is now a no-op retained for potential future use.
func (g *Game) drawAffordabilityProgress(_ *ebiten.Image) {}

func affordabilityFrac(wood, cost float64) float32 {
	if cost <= 0 || wood >= cost {
		return 1
	}
	if wood <= 0 {
		return 0
	}
	return float32(wood / cost)
}

func (g *Game) worldToScreen(v Vec) (float32, float32) {
	scale, offX, offY := viewGeom(g.screenW, g.screenH)
	return float32(offX + v.X*scale), float32(offY + v.Y*scale)
}

func drawAttentionRipple(screen *ebiten.Image, cx, cy, baseW, baseH float32, timeLeft, duration float64, baseColor color.RGBA, startScale float32) {
	if timeLeft <= 0 || duration <= 0 {
		return
	}
	t := float32(timeLeft / duration)
	expand := 1 - t
	halfW := baseW / 2 * (startScale + expand*1.35)
	halfH := baseH / 2 * (startScale + expand*1.35)
	if halfW <= 0.5 || halfH <= 0.5 {
		return
	}
	col := baseColor
	col.A = uint8(220 * t)
	vector.StrokeRect(screen, cx-halfW, cy-halfH, halfW*2, halfH*2, 1.5, col, false)
}

func drawButtonProgress(screen *ebiten.Image, btn *widget.Button, frac float32, col color.RGBA) {
	if frac <= 0 || frac >= 1 || !btn.GetWidget().Disabled {
		return
	}
	r := btn.GetWidget().Rect
	w := float32(r.Max.X - r.Min.X)
	h := float32(r.Max.Y-r.Min.Y) * frac
	if w <= 0 || h <= 0 {
		return
	}
	x := float32(r.Min.X)
	y := float32(r.Max.Y) - h
	vector.FillRect(screen, x, y, w, h, col, false)
}

func activeIncomingChannelForResource(w *World, resource ResourceKind) bool {
	for _, ch := range w.System.Channels {
		if ch.Target != w.Active || ch.Resource != resource {
			continue
		}
		state := channelState(w, ch, 1.0)
		if state.valid {
			return true
		}
	}
	return false
}

func growthGaugeFlashColors(kind ResourceKind) (afterglow, release color.RGBA) {
	base := colWoodResource
	if kind == KindWater {
		base = colSparkle
	}
	afterglow = brighten(base, 54)
	release = brighten(base, 108)
	return afterglow, release
}
