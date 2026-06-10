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

## QA presets

Presets stamp the local save to a specific mid-game state so you can test a scenario without playing into it manually.

```
make qa PRESET=town-field-fresh
make run
```

Available presets:

- `fresh` — clean reset for natural fresh-play confirmation
- `poor-coverage` — active workers on long routes, free nodes exist, camp or Nurture affordable
- `near-cap-boosted` — idle workers, no free nodes, field gauge one Nurture from completing
- `far-cap-boosted` — idle workers, no free nodes, field gauge far from completing
- `active-charges` — Nurture charges active
- `town-growth-arrival` — Town Growth one delivery from spawning a new worker
- `town-growth-cramped` — 10 workers, first planet feeling close to needing a new objective
- `town-growth-capacity-blocked` — Town Growth full, all dwelling slots occupied, no spawn
- `town-field-fresh` — Town Hall just placed, town field visible with one built slot and potential slots
- `town-field-growing` — several dwelling slots built, unused capacity, Town Growth partial
- `town-field-full` — all dwelling slots built, capacity action disabled despite affordable wood
