# Dreams of Potential

A small incremental game built with Go and [Ebiten](https://ebitengine.org).

## Running

```
make run
```

## Common commands

| Command | What it does |
|---|---|
| `make run` | Run the game |
| `make test` | Run all tests |
| `make check` | Run `go vet` |
| `make build` | Compile without running |
| `make screenshots` | Render headless PNGs to `screenshots/` |
| `make qa-list` | List available QA presets |
| `make qa PRESET=<name>` | Write the local save to a QA preset, then `make run` |

## Sim trace (pacing diagnostic)

`TestSimTrace` in `internal/game/simtrace_test.go` runs a headless simulation with a simple player AI and prints a per-minute snapshot table plus event log. Use it to check whether economy constants produce the intended play-session length without needing to run the game manually.

```
go test -v -run TestSimTrace ./internal/game/...
```

Output includes time, worker count, trees, wood, and field EXP at each event (camp placed, worker arrived, town full, planet complete) and at every per-minute checkpoint.

To tune pacing, adjust constants in `internal/game/tuning.go` — especially `townGrowthBaseCap`, `townGrowthCapGrowth`, `townCapacityBaseCost`, and `townCapacityCostGrowth` — then re-run the trace.

### Adding a trace for a new planet

The loop infrastructure lives in `runSimTrace`. Everything planet-specific is isolated in a `SimTraceRunner` implementation. To trace a new planet type:

**1. Implement `SimTraceRunner`** — create a struct in `simtrace_test.go` with these methods:

| Method | Purpose |
|---|---|
| `Setup(w *World)` | Place buildings, set flags, initialise any runner state |
| `ColHeader() string` | Pre-formatted header string for scenario columns (after time and workers) |
| `ColRow(w *World) string` | Pre-formatted values matching ColHeader, called each row |
| `PlayerAI(w *World) []string` | Called before each `Tick` — perform player actions, return log events or nil |
| `Events(w *World) []string` | Called after each `Tick` — detect world changes, return log events or nil |
| `Summary(w *World) string` | Completion message logged on the final row |

`PlayerAI` and `Events` each return a `[]string` of events to log (e.g. `"+camp 2 at 1.05"`, `"+worker → 3"`). The runner struct should track any state it needs to detect changes between ticks (previous worker count, previous resource counts, one-shot flags, etc.).

**2. Add a test function:**

```go
func TestSimTracePlanet2(t *testing.T) {
    runner := &planet2Runner{}
    w := runSimTrace(t, "planet 2 (iron + wood)", 30, runner)
    // log any extra summary stats from w here
}
```

The third argument is the ceiling in minutes — the loop exits early on planet completion (`w.System.Unlocked`).

**3. Run it:**

```
go test -v -run TestSimTracePlanet2 ./internal/game/...
```

## QA presets

Presets stamp the local save to a specific mid-game state so you can test a scenario without playing into it manually.

```
make qa PRESET=town-field-fresh
make run
```

Run `make qa-list` to see available presets.
