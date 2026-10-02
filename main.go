package main

import (
	"embed"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log"
	"math/rand/v2"
	"os"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	tile         = 12
	viewW, viewH = 20, 15 // tiles of the floor on screen at once
	screenW      = viewW * tile
	screenH      = (viewH+1)*tile + msgH // the view, a HUD row, and the message line
	msgH         = 16                    // the debug font's line height
	scale        = 4
	animTime     = 12 // ticks a walk/attack animation plays after a turn
)

// Copied from the Oryx bundle's Sliced/ folder. Gitignored: the license forbids redistributing them.
//
//go:embed assets
var assets embed.FS

// Tile set per floor, top to bottom: the wall and stairs set, and the floor drawn with it.
var themes = [floors]struct{ wall, floor string }{
	{"grey", "grey"}, {"dirt", "dirt"}, {"cave", "dark"},
	{"hedge", "moss"}, {"stone", "grey"}, {"red", "red"},
	{"frost", "frost"}, {"ice", "cold"}, {"turret", "mud"},
}

var shot = flag.String("shot", "", "save one rendered frame to this PNG and exit, to check rendering without a screen capture")

type Game struct {
	sprites  map[string]*ebiten.Image
	world    *World
	floor    *ebiten.Image // the whole floor, drawn before the camera picks the part on screen
	tick     int
	turnTick int // tick of the last turn, to time its animations
}

// loadSprites keys each sprite by its path under assets/ without extension, e.g. "Character/spark_idle_d_1".
// The folder stays in the key because Bosses/ and Character/ share names (cyclops, demon).
func loadSprites() map[string]*ebiten.Image {
	sprites := map[string]*ebiten.Image{}
	err := fs.WalkDir(assets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".png") {
			return err
		}
		f, err := assets.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		sprites[strings.TrimSuffix(strings.TrimPrefix(path, "assets/"), ".png")] = ebiten.NewImageFromImage(img)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return sprites
}

func justPressed(keys ...ebiten.Key) bool {
	return slices.ContainsFunc(keys, inpututil.IsKeyJustPressed)
}

func (g *Game) Update() error {
	g.tick++
	if g.world.Over {
		if justPressed(ebiten.KeyR) {
			g.world = NewGame(rand.Uint64())
		}
		return nil
	}
	dx, dy, acted := 0, 0, true
	switch {
	case justPressed(ebiten.KeyArrowLeft, ebiten.KeyA):
		dx = -1
	case justPressed(ebiten.KeyArrowRight, ebiten.KeyD):
		dx = 1
	case justPressed(ebiten.KeyArrowUp, ebiten.KeyW):
		dy = -1
	case justPressed(ebiten.KeyArrowDown, ebiten.KeyS):
		dy = 1
	case justPressed(ebiten.KeySpace, ebiten.KeyPeriod):
	default:
		acted = false
	}
	if acted {
		g.world.Step(dx, dy)
		g.turnTick = g.tick
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	w := g.world
	if g.floor == nil {
		g.floor = ebiten.NewImage(mapW*tile, mapH*tile)
	}
	dst := g.floor
	dst.Clear()

	// Tiles never seen stay black; seen ones out of sight are drawn dim, as remembered.
	theme := themes[w.Depth-1]
	for y, row := range w.Level {
		for x, c := range row {
			if !w.Seen[y][x] {
				continue
			}
			name := "World/floor_" + theme.floor
			switch c {
			case '#':
				name = "World/wall_block_" + theme.wall
			case '>':
				name = "World/stair_down_" + theme.wall
			}
			alpha := float32(1)
			if !w.Visible[y][x] {
				alpha = 0.35
			}
			g.draw(dst, name, x, y, alpha)
		}
	}
	// Only monsters in sight are drawn.
	var shown []*Entity
	for _, m := range w.Monsters {
		if w.Visible[m.Y][m.X] {
			shown = append(shown, m)
		}
	}
	for _, m := range shown {
		alpha := float32(1)
		if m.Broken() && g.tick/10%2 == 0 {
			alpha = 0.4 // blink: ready to possess
		}
		g.drawEntity(dst, m, alpha, false)
	}
	// A possessed body gets a spark-blue outline, so it stands out from monsters of the same kind.
	g.drawEntity(dst, w.Player, 1, !w.IsSpark())

	// Over the sprites: HP bars on hurt monsters, and an alert on those that just spotted the player.
	for _, m := range shown {
		x, y, width := float32(m.X*tile+1), float32(m.Y*tile), float32(tile-2)
		if m.Boss { // its sprite reaches half a tile beyond its own
			x, y, width = float32(m.X*tile-tile/2), float32(m.Y*tile-tile/2-2), float32(2*tile)
		}
		if m.HP < m.MaxHP {
			hpBar(dst, x, y, width, 1, m.HP, m.MaxHP)
		}
		if m.Spotted {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x+width/2-4), float64(y-9)) // FX sprites are 8px
			dst.DrawImage(g.sprite(fmt.Sprintf("FX/status_alert_%d", g.tick/10%2+1)), op)
		}
	}

	// Missiles fly from shooter to target while the turn's animation plays.
	if since := g.tick - g.turnTick; since < animTime {
		t := float64(since) / animTime
		for _, s := range w.Shots {
			if !w.Visible[s.FromY][s.FromX] && !w.Visible[s.To.Y][s.To.X] {
				continue
			}
			name := "FX/" + s.Missile
			if s.Missile == "arrow" {
				name = "FX/arrow_x"
				if s.FromX == s.To.X {
					name = "FX/arrow_y"
				}
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate( // FX sprites are 8px, centred in the 12px tile
				float64(s.FromX*tile)+float64((s.To.X-s.FromX)*tile)*t+2,
				float64(s.FromY*tile)+float64((s.To.Y-s.FromY)*tile)*t+2)
			dst.DrawImage(g.sprite(name), op)
		}
	}

	// The camera keeps the player centred, stopping at the floor's edges.
	cx := min(max(w.Player.X-viewW/2, 0), mapW-viewW)
	cy := min(max(w.Player.Y-viewH/2, 0), mapH-viewH)
	screen.DrawImage(dst.SubImage(image.Rect(cx*tile, cy*tile, (cx+viewW)*tile, (cy+viewH)*tile)).(*ebiten.Image), nil)

	// HUD: the body's hearts, or the spark's remaining turns.
	if w.IsSpark() {
		for i := range w.SparkLeft {
			g.draw(screen, "Character/spark_idle_d_1", i, viewH, 1)
		}
	} else {
		rotsNext := w.decay == w.Player.Decay-1 // the last heart goes at the end of this turn
		for i := range w.Player.MaxHP {
			name, alpha := "World/ui_heart", float32(1)
			if i >= w.Player.HP {
				name = "World/ui_heart_empty"
			} else if i == w.Player.HP-1 && rotsNext && g.tick/10%2 == 0 {
				alpha = 0.3
			}
			g.draw(screen, name, i, viewH, alpha)
		}
	}
	for _, m := range w.Monsters {
		if m.Boss && w.Visible[m.Y][m.X] { // health bar between the hearts (up to 10, the orc's) and the depth
			hpBar(screen, float32(10*tile+4), float32(viewH*tile+4), 52, 4, m.HP, m.MaxHP)
		}
	}
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Deep %d/%d", w.Depth, floors), screenW-60, viewH*tile-3)

	// Message line: this step's events, dropping the oldest whole messages when they don't fit.
	log := w.Log
	for len(log) > 1 && len(strings.Join(log, " ")) > screenW/6 { // the debug font is 6px wide
		log = log[1:]
	}
	ebitenutil.DebugPrintAt(screen, strings.Join(log, " "), 2, (viewH+1)*tile)
	switch {
	case w.Won:
		ebitenutil.DebugPrintAt(screen, "You reached the ninth deep. R to play again.", 0, 80)
	case w.Over:
		ebitenutil.DebugPrintAt(screen, "The spark fades. R to retry.", 50, 80)
	}
	if *shot != "" && g.tick > 30 { // let a few idle frames pass first
		saveShot(screen, *shot)
	}
}

func hpBar(dst *ebiten.Image, x, y, width, height float32, hp, maxHP int) {
	vector.FillRect(dst, x, y, width, height, color.RGBA{0x40, 0x10, 0x10, 0xff}, false)
	vector.FillRect(dst, x, y, width*float32(hp)/float32(maxHP), height, color.RGBA{0xe0, 0x30, 0x30, 0xff}, false)
}

func saveShot(screen *ebiten.Image, path string) {
	img := image.NewRGBA(screen.Bounds())
	screen.ReadPixels(img.Pix)
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	f.Close()
	os.Exit(0)
}

func (g *Game) drawEntity(dst *ebiten.Image, e *Entity, alpha float32, outlined bool) {
	since := g.tick - g.turnTick
	anim, frame := e.Anim, since/(animTime/2)%2+1
	if since >= animTime {
		anim, frame = "idle", g.tick/20%2+1
	}
	if e.Boss { // 24px sprite centred on its one tile, spilling over the neighbours
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(e.X*tile-tile/2), float64(e.Y*tile-tile/2))
		dst.DrawImage(g.sprite(fmt.Sprintf("Bosses/%s_%s_%s_%d", e.Name, anim, e.Dir, frame)), op)
		return
	}
	name := fmt.Sprintf("Character/%s_%s_%s_%d", e.Name, anim, e.Dir, frame)
	if outlined {
		// Draw a solid blue silhouette shifted 1px in each direction, then the sprite over it.
		var cm colorm.ColorM
		cm.Scale(0, 0, 0, 1)
		cm.Translate(0.25, 0.6, 1, 0)
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			op := &colorm.DrawImageOptions{}
			op.GeoM.Translate(float64(e.X*tile+d[0]), float64(e.Y*tile+d[1]))
			colorm.DrawImage(dst, g.sprite(name), cm, op)
		}
	}
	g.draw(dst, name, e.X, e.Y, alpha)
}

func (g *Game) sprite(name string) *ebiten.Image {
	img, ok := g.sprites[name]
	if !ok {
		panic("missing sprite " + name)
	}
	return img
}

func (g *Game) draw(dst *ebiten.Image, name string, x, y int, alpha float32) {
	img := g.sprite(name)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x*tile), float64(y*tile))
	op.ColorScale.ScaleAlpha(alpha)
	dst.DrawImage(img, op)
}

func (g *Game) Layout(int, int) (int, int) { return screenW, screenH }

func main() {
	flag.Parse()
	ebiten.SetWindowSize(screenW*scale, screenH*scale)
	ebiten.SetWindowTitle("Ninedeep")
	if err := ebiten.RunGame(&Game{sprites: loadSprites(), world: NewGame(rand.Uint64())}); err != nil {
		log.Fatal(err)
	}
}
