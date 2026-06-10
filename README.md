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
make qa PRESET=near-cap-stall
make run
```

Available presets:

- `near-cap-stall` — idle workers, no free nodes, field gauge one Nurture from completing
- `far-cap-stall` — idle workers, no free nodes, field gauge far from completing
- `poor-coverage` — active workers on long routes, free nodes exist, camp or Nurture affordable
- `fresh` — clean reset for natural fresh-play confirmation
