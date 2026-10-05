# Underwick

A turn-based roguelike in Go with Ebitengine, all in package `main`. `README.md` describes the game and its rules.

## Layout

- `world.go`: rules, turns, combat, monsters; `gen.go`: floor generation; `items.go`, `magic.go` (classes and
  spells), `shop.go`: what the hero can carry, cast and buy.
- `main.go`: everything on screen and all input; `sound.go`: effects and music.
- `save.go`: saves and high scores, kept by `store.go` in files, or by `store_js.go` in the browser's local storage.
- Tests sit beside each file. `sim_test.go` holds the balance bots.

## Assets

- `assets/` holds the Oryx sprites and font. They are licensed, gitignored, and must never be committed or uploaded.
  They are embedded, so nothing builds or tests without them.
- `audio/` is CC0 (Juhani Junkala) and is committed.
- The Oryx pack files its walking frames under the wrong directions; `walkFrames` in `main.go` maps them back.

## Rules that break things silently

- A save is the seed plus the hero's actions, replayed on load. Any change to the rules or to floor generation
  must bump `saveVersion` in `save.go`. Any new kind of player action must be recorded (`w.record`) and replayed
  in `Continue`.
- All randomness comes from the run's seed through `w.rng`. Never let map iteration order affect the game.

## Checking changes

- `go vet ./... && go test ./...`, and `GOOS=js GOARCH=wasm go vet ./...` for the browser build.
- `go run . -shot out.png` saves one rendered frame. To check a panel or a scene, temporarily set up the state in
  `main()`, take the shot, and restore `main.go` afterwards. The shot runs the real game, which keeps its save and
  high scores in the user's config folder: a scene that ends a run records a score and deletes the save, so point
  `dataDir` at a temporary folder first. The README's screenshots in `docs/screenshots/` were made this way, with
  the bots from `sim_test.go` copied into a temporary file to play seeded runs.
- Panels fit about 42 characters a line; check new on-screen text in a screenshot.
- Before and after any balance change, run `UNDERWICK_SIM=1 go test -run Balance -v` (300 seeded runs per bot,
  about a minute) and compare.
- Browser build: `GOOS=js GOARCH=wasm go build -o web/underwick.wasm .`. The output embeds the sprites, so it is
  gitignored. `web/deploy.sh` publishes it to GitHub Pages; it pushes, so run it only when asked.
- macOS app: `mac/app.sh` builds `Underwick.app` (gitignored), with an icon made from the hero sprite.
- The screen grows with the window: `fit` picks the largest whole-number enlargement and widens the floor's view
  and `screenW`/`screenH` to fill the rest. Lay panels out from `screenW`/`screenH`, never fixed widths, and check
  a screenshot at a wide window size too (`ebiten.SetWindowSize` in the temporary `main()` setup).

## Conventions

- `gofmt`, and comments in the existing explanatory style: say why, not what.
- Update `README.md` when keys or rules change.
- Commit only when asked.
