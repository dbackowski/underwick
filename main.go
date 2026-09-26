package main

import (
	"embed"
	"fmt"
	"image"
	_ "image/png"
	"io/fs"
	"log"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	tile    = 12
	screenW = 20 * tile
	screenH = 15 * tile
	scale   = 4
)

// Copied from the Oryx bundle's Sliced/ folder. Gitignored: the license forbids redistributing them.
//
//go:embed assets
var assets embed.FS

var level = []string{
	"####################",
	"#..................#",
	"#..................#",
	"#....####..........#",
	"#....#.............#",
	"#....#......@......#",
	"#..................#",
	"#..........#.......#",
	"#..........#.......#",
	"#..........#####...#",
	"#..................#",
	"#..................#",
	"#..................#",
	"#..................#",
	"####################",
}

type Game struct {
	sprites   map[string]*ebiten.Image
	px, py    int
	dir       string
	tick      int
	walkUntil int
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
	dx, dy := 0, 0
	switch {
	case justPressed(ebiten.KeyArrowLeft, ebiten.KeyA):
		dx, g.dir = -1, "l"
	case justPressed(ebiten.KeyArrowRight, ebiten.KeyD):
		dx, g.dir = 1, "r"
	case justPressed(ebiten.KeyArrowUp, ebiten.KeyW):
		dy, g.dir = -1, "u"
	case justPressed(ebiten.KeyArrowDown, ebiten.KeyS):
		dy, g.dir = 1, "d"
	}
	if (dx != 0 || dy != 0) && level[g.py+dy][g.px+dx] != '#' {
		g.px, g.py = g.px+dx, g.py+dy
		g.walkUntil = g.tick + 12
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	for y, row := range level {
		for x, c := range row {
			name := "World/floor_grey"
			if c == '#' {
				name = "World/wall_block_grey"
			}
			g.draw(screen, name, x, y)
		}
	}
	anim := "idle"
	if g.tick < g.walkUntil {
		anim = "walk"
	}
	frame := g.tick/20%2 + 1
	g.draw(screen, fmt.Sprintf("Character/spark_%s_%s_%d", anim, g.dir, frame), g.px, g.py)
}

func (g *Game) draw(dst *ebiten.Image, name string, x, y int) {
	img, ok := g.sprites[name]
	if !ok {
		panic("missing sprite " + name)
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x*tile), float64(y*tile))
	dst.DrawImage(img, op)
}

func (g *Game) Layout(int, int) (int, int) { return screenW, screenH }

func main() {
	g := &Game{sprites: loadSprites(), dir: "d"}
	for y, row := range level {
		if x := strings.IndexByte(row, '@'); x >= 0 {
			g.px, g.py = x, y
		}
	}
	ebiten.SetWindowSize(screenW*scale, screenH*scale)
	ebiten.SetWindowTitle("Ninedeep")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
