package main

import (
	"embed"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	tile     = 12
	mapH     = 15
	screenW  = 20 * tile
	screenH  = (mapH + 1) * tile // one extra row for the HUD
	scale    = 4
	animTime = 12 // ticks a walk/attack animation plays after a turn
)

// Copied from the Oryx bundle's Sliced/ folder. Gitignored: the license forbids redistributing them.
//
//go:embed assets
var assets embed.FS

var level = []string{
	"####################",
	"#..................#",
	"#...r..........o...#",
	"#....####..........#",
	"#....#.............#",
	"#....#......@......#",
	"#..................#",
	"#..........#.......#",
	"#..r.......#...a...#",
	"#..........#####...#",
	"#..................#",
	"#......a...........#",
	"#..............r...#",
	"#..................#",
	"####################",
}

var shot = flag.String("shot", "", "save one rendered frame to this PNG and exit, to check rendering without a screen capture")

type Game struct {
	sprites  map[string]*ebiten.Image
	world    *World
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
			g.world = NewWorld(level)
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
	for y, row := range w.Level {
		for x, c := range row {
			name := "World/floor_grey"
			if c == '#' {
				name = "World/wall_block_grey"
			}
			g.draw(screen, name, x, y, 1)
		}
	}
	for _, m := range w.Monsters {
		alpha := float32(1)
		if m.Broken() && g.tick/10%2 == 0 {
			alpha = 0.4 // blink: ready to possess
		}
		g.drawEntity(screen, m, alpha, false)
	}
	// A possessed body gets a spark-blue outline, so it stands out from monsters of the same kind.
	g.drawEntity(screen, w.Player, 1, !w.IsSpark())

	// Arrows fly from shooter to target while the turn's animation plays.
	if since := g.tick - g.turnTick; since < animTime {
		t := float64(since) / animTime
		for _, s := range w.Shots {
			name := "FX/arrow_x"
			if s.FromX == s.To.X {
				name = "FX/arrow_y"
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate( // FX sprites are 8px, centred in the 12px tile
				float64(s.FromX*tile)+float64((s.To.X-s.FromX)*tile)*t+2,
				float64(s.FromY*tile)+float64((s.To.Y-s.FromY)*tile)*t+2)
			screen.DrawImage(g.sprite(name), op)
		}
	}

	// HUD: the body's hearts, or the spark's remaining turns.
	if w.IsSpark() {
		for i := range w.SparkLeft {
			g.draw(screen, "Character/spark_idle_d_1", i, mapH, 1)
		}
	} else {
		for i := range w.Player.MaxHP {
			name := "World/ui_heart"
			if i >= w.Player.HP {
				name = "World/ui_heart_empty"
			}
			g.draw(screen, name, i, mapH, 1)
		}
	}
	if w.Over {
		ebitenutil.DebugPrintAt(screen, "The spark fades. R to retry.", 50, 80)
	}
	if *shot != "" && g.tick > 30 { // let a few idle frames pass first
		saveShot(screen, *shot)
	}
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
	if err := ebiten.RunGame(&Game{sprites: loadSprites(), world: NewWorld(level)}); err != nil {
		log.Fatal(err)
	}
}
