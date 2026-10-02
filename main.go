package main

import (
	"bytes"
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
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	tile         = 12
	viewW, viewH = 20, 15 // tiles of the floor on screen at once
	screenW      = viewW * tile
	screenH      = (viewH+1)*tile + msgH // the view, a HUD row, and two lines of messages
	msgH         = 2*lineH + 2           // room for the font's descenders
	lineH        = 9                     // the Oryx font is drawn at 8px
	scale        = 4
	animTime     = 12 // ticks a walk/attack animation plays after a turn
)

// Copied from the Oryx bundle: its Sliced/ folder and oryx-simplex.ttf. Gitignored: the license forbids
// redistributing them.
//
//go:embed assets
var assets embed.FS

// Tile sets, one per floor and repeating: the wall and stairs set, and the floor drawn with it.
var themes = []struct{ wall, floor string }{
	{"grey", "grey"}, {"dirt", "dirt"}, {"cave", "dark"},
	{"hedge", "moss"}, {"stone", "grey"}, {"red", "red"},
	{"frost", "frost"}, {"ice", "cold"}, {"turret", "mud"},
}

var (
	shot = flag.String("shot", "", "save one rendered frame to this PNG and exit, to check rendering without a screen capture")
	font *text.GoTextFace

	white  = color.RGBA{0xff, 0xff, 0xff, 0xff}
	yellow = color.RGBA{0xff, 0xe0, 0x60, 0xff}
	grey   = color.RGBA{0xa0, 0xa0, 0xa0, 0xff}
)

type Game struct {
	sprites  map[string]*ebiten.Image
	world    *World
	floor    *ebiten.Image // the whole floor, drawn before the camera picks the part on screen
	low      *ebiten.Image // the screen at the art's own size, enlarged by scale with hard pixel edges
	labels   []label       // text for this frame, drawn after the enlargement at full resolution
	mode     string        // "" while playing, or "use" / "drop" while choosing an inventory item
	tick     int
	turnTick int // tick of the last turn, to time its animations
}

// loadSprites keys each sprite by its path under assets/ without extension, e.g. "Character/rat_idle_d_1".
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

func loadFont() *text.GoTextFace {
	data, err := assets.ReadFile("assets/oryx-simplex.ttf")
	if err != nil {
		log.Fatal(err)
	}
	src, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}
	// At full window resolution: drawn small and enlarged, the glyphs' anti-aliased edges blur.
	return &text.GoTextFace{Source: src, Size: 8 * scale}
}

func justPressed(keys ...ebiten.Key) bool {
	return slices.ContainsFunc(keys, inpututil.IsKeyJustPressed)
}

func (g *Game) Update() error {
	g.tick++
	w := g.world
	if w.Over {
		if justPressed(ebiten.KeyR) {
			g.world, g.mode = NewGame(rand.Uint64()), ""
		}
		return nil
	}

	if g.mode != "" { // choosing an item: a letter picks it, Escape gives up
		if justPressed(ebiten.KeyEscape) {
			g.mode = ""
		}
		for _, r := range ebiten.AppendInputChars(nil) {
			if i := int(r - 'a'); i >= 0 && i < len(w.Inventory) {
				if g.mode == "drop" {
					w.Drop(i)
				} else {
					w.Use(i)
				}
				g.mode = ""
				g.turnTick = g.tick
			}
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
	case justPressed(ebiten.KeyG):
		w.PickUp()
		g.turnTick, acted = g.tick, false
	case justPressed(ebiten.KeyI):
		g.mode, acted = "use", false
	case justPressed(ebiten.KeyX):
		g.mode, acted = "drop", false
	default:
		acted = false
	}
	if acted {
		w.Step(dx, dy)
		g.turnTick = g.tick
	}
	return nil
}

func (g *Game) Draw(out *ebiten.Image) {
	w := g.world
	if g.low == nil {
		g.low = ebiten.NewImage(screenW, screenH)
	}
	screen := g.low
	screen.Fill(color.Black)
	g.labels = g.labels[:0]
	if g.floor == nil {
		g.floor = ebiten.NewImage(mapW*tile, mapH*tile)
	}
	dst := g.floor
	dst.Clear()

	// Tiles never seen stay black; seen ones out of sight are drawn dim, as remembered.
	theme := themes[(w.Depth-1)%len(themes)]
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
			g.draw(dst, name, x, y, dim(w, x, y))
		}
	}
	// Items stay where the hero last saw them; nothing else moves them.
	for _, it := range w.Floor {
		if w.Seen[it.Y][it.X] {
			g.draw(dst, "World/"+w.ItemSprite(it), it.X, it.Y, dim(w, it.X, it.Y))
		}
	}
	// Only monsters in sight are drawn. A faint light outline keeps dark ones visible on dark floors.
	var shown []*Entity
	for _, m := range w.Monsters {
		if w.Visible[m.Y][m.X] {
			shown = append(shown, m)
		}
	}
	for _, m := range shown {
		g.drawEntity(dst, m, color.RGBA{0x90, 0x90, 0x90, 0x60})
	}
	// The hero gets a blue outline, so it stands out from monsters that look like heroes.
	g.drawEntity(dst, w.Player, color.RGBA{0x40, 0x99, 0xff, 0xff})

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

	// HUD: the hero's HP and level, a visible boss's HP, gold and the depth.
	hud := float64(viewH*tile + 2)
	hpBar(screen, 2, float32(hud+2), 44, 4, max(w.Player.HP, 0), w.Player.MaxHP)
	g.label(fmt.Sprintf("%d/%d L%d", max(w.Player.HP, 0), w.Player.MaxHP, w.ExpLevel), 50, hud, white)
	for _, m := range w.Monsters {
		if m.Boss && w.Visible[m.Y][m.X] {
			hpBar(screen, 108, float32(hud+2), 44, 4, m.HP, m.MaxHP)
		}
	}
	g.label(fmt.Sprintf("$%d  Deep %d", w.Gold, w.Depth), 160, hud, yellow)

	// Messages: this turn's events over two lines, dropping the oldest whole messages that don't fit.
	lines := wrap(w.Log, screenW-4)
	for i, l := range lines[max(0, len(lines)-2):] {
		g.label(l, 2, float64((viewH+1)*tile+i*lineH), white)
	}

	switch {
	case g.mode != "":
		g.labels = g.labels[:0] // the panel covers the HUD and messages
		g.drawInventory(screen)
	case w.Over:
		panel(screen, 30, 66, screenW-60, 32)
		g.label(fmt.Sprintf("You died on depth %d. Score %d.", w.Depth, w.Score()), 38, 72, white)
		g.label("Press R to play again.", 38, 72+lineH+2, grey)
	}
	// The art, enlarged with hard pixel edges, then the text over it at full resolution.
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	out.DrawImage(screen, op)
	for _, l := range g.labels {
		op := &text.DrawOptions{}
		op.GeoM.Translate(l.x*scale, l.y*scale)
		op.ColorScale.ScaleWithColor(l.c)
		text.Draw(out, l.s, font, op)
	}
	if *shot != "" && g.tick > 30 { // let a few idle frames pass first
		saveShot(out, *shot)
	}
}

// drawInventory lists what the hero carries, lettered, for using or dropping.
func (g *Game) drawInventory(screen *ebiten.Image) {
	w := g.world
	panel(screen, 2, 2, screenW-4, screenH-4)
	title := "Use or wear which? (Esc to close)"
	if g.mode == "drop" {
		title = "Drop which? (Esc to cancel)"
	}
	g.label(title, 6, 4, yellow)
	if len(w.Inventory) == 0 {
		g.label("You carry nothing.", 6, 4+lineH+2, grey)
	}
	for i, it := range w.Inventory {
		y := 4 + float64(i+1)*lineH + 2
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(0.75, 0.75) // the 12px sprite in a 9px line
		op.GeoM.Translate(6, y-1)
		screen.DrawImage(g.sprite("World/"+w.ItemSprite(it)), op)
		label := fmt.Sprintf("%c) %s", 'a'+i, w.ItemName(it))
		if it.Worn {
			label += " (in use)"
		}
		g.label(label, 18, y, white)
	}
}

// wrap packs messages into lines no wider than width, never splitting one message across lines.
func wrap(msgs []string, width int) []string {
	var lines []string
	for _, m := range msgs {
		if n := len(lines); n > 0 && text.Advance(lines[n-1]+" "+m, font)/scale <= float64(width) {
			lines[n-1] += " " + m
		} else {
			lines = append(lines, m)
		}
	}
	return lines
}

type label struct {
	s    string
	x, y float64 // in the art's pixels, like everything else drawn on low
	c    color.Color
}

// label queues text for this frame, positioned in the art's pixels.
func (g *Game) label(s string, x, y float64, c color.Color) {
	g.labels = append(g.labels, label{s, x, y, c})
}

func panel(dst *ebiten.Image, x, y, w, h float32) {
	vector.FillRect(dst, x, y, w, h, color.RGBA{0x10, 0x10, 0x18, 0xf0}, false)
	vector.StrokeRect(dst, x, y, w, h, 1, grey, false)
}

// dim is how bright to draw a seen tile: full in sight, dim when only remembered.
func dim(w *World, x, y int) float32 {
	if w.Visible[y][x] {
		return 1
	}
	return 0.35
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

// drawEntity draws a creature in its current animation frame, over a 1px outline of the given colour.
func (g *Game) drawEntity(dst *ebiten.Image, e *Entity, outline color.RGBA) {
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
	// A silhouette in the outline colour, shifted 1px each way, then the sprite over it.
	var cm colorm.ColorM
	cm.Scale(0, 0, 0, float64(outline.A)/0xff)
	cm.Translate(float64(outline.R)/0xff, float64(outline.G)/0xff, float64(outline.B)/0xff, 0)
	for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		op := &colorm.DrawImageOptions{}
		op.GeoM.Translate(float64(e.X*tile+d[0]), float64(e.Y*tile+d[1]))
		colorm.DrawImage(dst, g.sprite(name), cm, op)
	}
	g.draw(dst, name, e.X, e.Y, 1)
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

func (g *Game) Layout(int, int) (int, int) { return screenW * scale, screenH * scale }

func main() {
	flag.Parse()
	font = loadFont()
	ebiten.SetWindowSize(screenW*scale, screenH*scale)
	ebiten.SetWindowTitle("Underwick")
	if err := ebiten.RunGame(&Game{sprites: loadSprites(), world: NewGame(rand.Uint64())}); err != nil {
		log.Fatal(err)
	}
}
